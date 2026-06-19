package isotopo

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// ACCEPTANCE SUITE for the position-resolution refactor.
//
// These tests are BEHAVIOURAL: they assert only what is observable in the
// rendered SVG (node screen positions + edge routes), so they are independent of
// how positions are computed internally and survive the refactor as its contract.
//
// Locked decisions (from the owner):
//   - Reparent into ANY container — including auto-arrange (autosize / layout)
//     groups — must PRESERVE the node's on-screen position (most predictable).
//   - The refactor may NOT change rendered output: goldens stay byte-identical
//     (enforced by the existing Golden tests, not here).
//
// The refactor is SUCCESSFUL iff every test here is green AND the full existing
// suite (incl. Golden) stays green. Run on the pre-refactor baseline this file
// is partially RED — that red set is the precise refactor target.
// ─────────────────────────────────────────────────────────────────────────────

const posEps = 0.5 // sub-pixel screen tolerance

var rePartPos = regexp.MustCompile(`data-part-id="([^"]+)"[^>]*transform="translate\(([-0-9.]+) ([-0-9.]+)\)`)

// nodePositions returns id → (x,y) screen position for every rendered part.
func nodePositions(t *testing.T, src string) map[string][2]float64 {
	t.Helper()
	svg, issues, _ := RenderSource("yaml", []byte(src))
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Fatalf("render error: %s\n%s", i.Message, src)
		}
	}
	out := map[string][2]float64{}
	for _, m := range rePartPos.FindAllStringSubmatch(svg, -1) {
		out[m[1]] = [2]float64{atofAcc(m[2]), atofAcc(m[3])}
	}
	return out
}

func atofAcc(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func applyT(t *testing.T, src string, op EditOp) string {
	t.Helper()
	out, err := ApplyOpText("yaml", []byte(src), op)
	if err != nil {
		t.Fatalf("op %+v: %v", op, err)
	}
	return string(out)
}

// assertPreserved checks that `target` stayed within posEps and that NO OTHER
// node moved (collateral drift). before/after are position maps.
func assertPreserved(t *testing.T, target string, before, after map[string][2]float64) {
	t.Helper()
	for id, b := range before {
		a, ok := after[id]
		if !ok {
			continue // node may legitimately disappear only on delete; not here
		}
		dx, dy := math.Abs(a[0]-b[0]), math.Abs(a[1]-b[1])
		if dx > posEps || dy > posEps {
			role := "COLLATERAL (should not move)"
			if id == target {
				role = "TARGET (should preserve)"
			}
			t.Errorf("%s node %q moved (%.2f,%.2f)→(%.2f,%.2f) Δ(%.2f,%.2f)",
				role, id, b[0], b[1], a[0], a[1], dx, dy)
		}
	}
}

// assertRelative checks that the listed leaf nodes preserve their RELATIVE
// positions: every one moves by the SAME delta. A uniform delta is an allowed
// canvas reframe (e.g. an autosize slab growing to wrap the reparented node);
// only a node moving DIFFERENTLY from the others is a real drift. Container
// slabs are intentionally not listed — they may resize/reposition.
func assertRelative(t *testing.T, leaves []string, before, after map[string][2]float64) {
	t.Helper()
	var rdx, rdy float64
	have := false
	for _, id := range leaves {
		b, okb := before[id]
		a, oka := after[id]
		if !okb || !oka {
			t.Fatalf("leaf %q missing from render before/after", id)
		}
		dx, dy := a[0]-b[0], a[1]-b[1]
		if !have {
			rdx, rdy, have = dx, dy, true
			continue
		}
		if math.Abs(dx-rdx) > posEps || math.Abs(dy-rdy) > posEps {
			t.Errorf("leaf %q moved Δ(%.2f,%.2f) but the others moved Δ(%.2f,%.2f) — relative position changed",
				id, dx, dy, rdx, rdy)
		}
	}
}

// ── G2: reparent preserves the node's screen position, no collateral ──────────

func TestAccept_ReparentPreserves(t *testing.T) {
	cases := []struct {
		name, src, id, target string
		leaves                []string
	}{
		{"2a_out_of_offset0_group",
			scene(`- id: g
        shape: group
        geom: { w: 360, d: 220, h: 6 }
        parts:
          - { id: n, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:30,wy:30} }
      - { id: keep, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:60,wy:340} }`),
			"n", "", []string{"n", "keep"}},
		{"2b_out_of_offset_group",
			scene(`- id: g
        shape: group
        geom: { w: 360, d: 220, h: 6 }
        offset: { wx: 120, wy: 80 }
        parts:
          - { id: n, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:30,wy:30} }
      - { id: keep, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:60,wy:340} }`),
			"n", "", []string{"n", "keep"}},
		{"2c_out_of_nested_group",
			scene(`- id: g0
        shape: group
        geom: { w: 460, d: 320, h: 6 }
        offset: { wx: 50, wy: 40 }
        parts:
          - id: g1
            shape: group
            geom: { w: 300, d: 200, h: 6 }
            offset: { wx: 40, wy: 40 }
            parts:
              - { id: n, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }
      - { id: keep, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:60,wy:420} }`),
			"n", "", []string{"n", "keep"}},
		{"2d_cross_group",
			scene(`- id: ga
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 30, wy: 30 }
        parts:
          - { id: n, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }
      - id: gb
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 400, wy: 60 }
        parts:
          - { id: other, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }`),
			"n", "gb", []string{"n", "other"}},
		{"2e_into_autosize_group",
			scene(`- id: ga
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 30, wy: 30 }
        parts:
          - { id: n, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }
      - id: gb
        shape: group
        geom: { h: 6 }
        offset: { wx: 400, wy: 60 }
        parts:
          - { id: other, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }`),
			"n", "gb", []string{"n", "other"}},
		{"2f_into_authored_geom_group",
			scene(`- id: ga
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 30, wy: 30 }
        parts:
          - { id: n, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }
      - id: gb
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 400, wy: 60 }
        parts:
          - { id: other, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }`),
			"n", "gb", []string{"n", "other"}},
		{"2g_into_layout_row_group",
			scene(`- id: ga
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 30, wy: 30 }
        parts:
          - { id: n, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:30,wy:30} }
      - id: gb
        shape: group
        geom: { w: 320, d: 160, h: 6 }
        offset: { wx: 400, wy: 60 }
        layout: { mode: row, gap: 24 }
        parts:
          - { id: o1, shape: rectangle, geom: {w:80,d:60,h:30} }
          - { id: o2, shape: rectangle, geom: {w:80,d:60,h:30} }`),
			"n", "gb", []string{"n", "o1", "o2"}},
		{"2h_out_of_layout_row_group",
			scene(`- id: ga
        shape: group
        geom: { w: 320, d: 160, h: 6 }
        offset: { wx: 40, wy: 40 }
        layout: { mode: row, gap: 24 }
        parts:
          - { id: n, shape: rectangle, geom: {w:80,d:60,h:30} }
          - { id: sib, shape: rectangle, geom: {w:80,d:60,h:30} }
      - { id: keep, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:60,wy:400} }`),
			"n", "", []string{"n", "sib", "keep"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := nodePositions(t, c.src)
			after := nodePositions(t, applyT(t, c.src, EditOp{Kind: "reparent", ID: c.id, Target: c.target}))
			assertRelative(t, c.leaves, before, after)
		})
	}
}

// 2g: edges stay docked to their endpoints after a reparent (route ends at the
// part). We approximate "docked" by: the moved node still has its edge and the
// render carries a data-connector for it (no detachment / drop).
func TestAccept_ReparentEdgesStayConnected(t *testing.T) {
	src := scene(`- id: g
        shape: group
        geom: { w: 360, d: 220, h: 6 }
        offset: { wx: 100, wy: 60 }
        parts:
          - { id: a, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:30,wy:30} }
      - { id: b, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:60,wy:340} }`) +
		"\n    connectors:\n      - { from: a, to: b }\n"
	out := applyT(t, src, EditOp{Kind: "reparent", ID: "a", Target: ""})
	svg, _, _ := RenderSource("yaml", []byte(out))
	if !strings.Contains(svg, `data-connector="0"`) {
		t.Fatalf("edge a→b lost / not rendered after reparent:\n%s", out)
	}
}

// ── G3: move is exact + local; zero-delta move is a true no-op ────────────────

func TestAccept_MoveLocalAndExact(t *testing.T) {
	src := scene(`- id: g
        shape: group
        geom: { w: 360, d: 220, h: 6 }
        offset: { wx: 40, wy: 40 }
        parts:
          - { id: a, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:30,wy:30} }
          - { id: b, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:210,wy:30} }
      - { id: c, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:60,wy:320} }`)

	t.Run("zero_delta_is_noop", func(t *testing.T) {
		b, _, _ := RenderSource("yaml", []byte(src))
		a, _, _ := RenderSource("yaml", []byte(applyT(t, src, EditOp{Kind: "move", Target: "node", ID: "a", DWX: 0, DWY: 0})))
		if a != b {
			t.Errorf("zero-delta move changed the render (should be a no-op)")
		}
	})

	t.Run("only_target_moves", func(t *testing.T) {
		before := nodePositions(t, src)
		after := nodePositions(t, applyT(t, src, EditOp{Kind: "move", Target: "node", ID: "a", DWX: 40, DWY: 0}))
		for id, bpos := range before {
			apos := after[id]
			moved := math.Abs(apos[0]-bpos[0]) > posEps || math.Abs(apos[1]-bpos[1]) > posEps
			if id == "a" && !moved {
				t.Errorf("target a did not move")
			}
			if id != "a" && moved {
				t.Errorf("collateral: %q moved on a move of a", id)
			}
		}
	})
}

// ── G4: inverse identity / no drift ───────────────────────────────────────────

func TestAccept_InverseIdentity(t *testing.T) {
	src := scene(`- id: g1
        shape: group
        geom: { w: 360, d: 220, h: 6 }
        offset: { wx: 80, wy: 60 }
        parts:
          - { id: n, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:30,wy:30} }
          - { id: sib, shape: rectangle, geom: {w:90,d:70,h:30}, offset: {wx:210,wy:30} }`)

	t.Run("move_then_back", func(t *testing.T) {
		before := nodePositions(t, src)
		s1 := applyT(t, src, EditOp{Kind: "move", Target: "node", ID: "n", DWX: 70, DWY: -40})
		s2 := applyT(t, s1, EditOp{Kind: "move", Target: "node", ID: "n", DWX: -70, DWY: 40})
		assertPreserved(t, "n", before, nodePositions(t, s2))
	})

	t.Run("reparent_out_then_in", func(t *testing.T) {
		before := nodePositions(t, src)
		s1 := applyT(t, src, EditOp{Kind: "reparent", ID: "n", Target: ""})
		s2 := applyT(t, s1, EditOp{Kind: "reparent", ID: "n", Target: "g1"})
		assertPreserved(t, "n", before, nodePositions(t, s2))
	})

	t.Run("ten_cycles_no_drift", func(t *testing.T) {
		before := nodePositions(t, src)
		s := src
		for i := 0; i < 10; i++ {
			s = applyT(t, s, EditOp{Kind: "reparent", ID: "n", Target: ""})
			s = applyT(t, s, EditOp{Kind: "reparent", ID: "n", Target: "g1"})
		}
		assertPreserved(t, "n", before, nodePositions(t, s))
	})
}

// ── G5: the two text readers agree (tight vs spaced flow map) ─────────────────

func TestAccept_ParserConsistency(t *testing.T) {
	// A second fixed node anchors the framing so the offset under test is
	// actually observable (a lone node's offset is normalised away by centring).
	ref := `- { id: ref, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:0,wy:0} }`
	tight := scene(ref + "\n      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:120,wy:90} }")
	spaced := scene(ref + "\n      - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, offset: { wx: 120, wy: 90 } }")
	pt := nodePositions(t, tight)["a"]
	ps := nodePositions(t, spaced)["a"]
	if math.Abs(pt[0]-ps[0]) > posEps || math.Abs(pt[1]-ps[1]) > posEps {
		t.Errorf("tight %v vs spaced %v offset render differently (two readers disagree)", pt, ps)
	}
}

// scene wraps a parts: body into a full composite document.
func scene(partsBody string) string {
	return "nodes:\n  scene:\n    shape: composite\n    parts:\n      " +
		strings.ReplaceAll(partsBody, "\n", "\n") + "\n"
}
