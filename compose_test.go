package isotopo

import (
	"fmt"
	"strings"
	"testing"
)

// sloppyVariant builds a freehand-scattered scene: n parts at pseudo-random
// (deterministic) off-grid offsets, no shared tracks.
func sloppyVariant(seed int) []byte {
	coords := [][2]int{}
	x, y := 13+seed*7, 27+seed*11
	for i := 0; i < 6; i++ {
		coords = append(coords, [2]int{x, y})
		x = (x*73 + 131 + seed*17) % 700
		y = (y*57 + 213 + seed*29) % 800
	}
	var b strings.Builder
	b.WriteString("nodes:\n  scene:\n    shape: composite\n    parts:\n")
	for i, c := range coords {
		fmt.Fprintf(&b, "      - { id: p%d, shape: rectangle, offset: { wx: %d, wy: %d }, geom: { w: 120, d: 90, h: 30 }, label: \"P%d\" }\n", i, c[0], c[1], i)
	}
	return []byte(b.String())
}

// TestComposeLiftsSloppyScenes is the M3 benchmark gate: on freehand-scattered
// scenes the compose pass must lift the composition score by the pre-registered
// margin (+0.15) with ZERO defect regressions, and be idempotent.
func TestComposeLiftsSloppyScenes(t *testing.T) {
	for seed := 0; seed < 3; seed++ {
		src := sloppyVariant(seed)
		before, err := Parse(src)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		b := EvaluateComposition(before)
		rb := Readability(before)

		out, fixes, err := RepairSourceWithOptions("yaml", src, RepairOptions{Compose: true})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if len(fixes) == 0 {
			t.Fatalf("seed %d: compose found nothing on a scattered scene (before align=%.2f)", seed, b.Alignment)
		}
		after, err := Parse(out)
		if err != nil {
			t.Fatalf("seed %d: persisted source does not parse: %v", seed, err)
		}
		a := EvaluateComposition(after)
		ra := Readability(after)

		if a.Score < b.Score+0.15 {
			t.Errorf("seed %d: lift too small: %.2f -> %.2f (pre-registered margin +0.15)", seed, b.Score, a.Score)
		}
		if ra.Overlaps > rb.Overlaps || ra.Tunnels > rb.Tunnels || ra.Occlusions > rb.Occlusions {
			t.Errorf("seed %d: defect regression: before {ov:%d tu:%d oc:%d} after {ov:%d tu:%d oc:%d}",
				seed, rb.Overlaps, rb.Tunnels, rb.Occlusions, ra.Overlaps, ra.Tunnels, ra.Occlusions)
		}

		// Idempotence on the persisted result.
		out2, fixes2, err := RepairSourceWithOptions("yaml", out, RepairOptions{Compose: true})
		if err != nil {
			t.Fatalf("seed %d: second pass: %v", seed, err)
		}
		if len(fixes2) != 0 || string(out2) != string(out) {
			t.Errorf("seed %d: compose not idempotent (second pass: %d fixes)", seed, len(fixes2))
		}
	}
}

// TestComposePreservesComments: the persistence path is the comment-preserving
// edit-op machinery — compose must not be the pass that breaks that contract.
func TestComposePreservesComments(t *testing.T) {
	src := []byte(`# top comment
nodes:
  scene:
    shape: composite
    parts:
      # the api box
      - { id: a, shape: rectangle, offset: { wx: 13, wy: 27 },  geom: { w: 120, d: 90, h: 30 }, label: "A" }
      - { id: b, shape: rectangle, offset: { wx: 431, wy: 88 }, geom: { w: 120, d: 90, h: 30 }, label: "B" }
      - { id: c, shape: rectangle, offset: { wx: 197, wy: 411 },geom: { w: 120, d: 90, h: 30 }, label: "C" }
`)
	out, fixes, err := RepairSourceWithOptions("yaml", src, RepairOptions{Compose: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected compose fixes")
	}
	if !strings.Contains(string(out), "# top comment") || !strings.Contains(string(out), "# the api box") {
		t.Error("comments lost during compose persistence")
	}
}

// TestComposeNeverCreatesOverlap: a snap whose target track would collide with
// another footprint must be skipped, not applied.
func TestComposeNeverCreatesOverlap(t *testing.T) {
	// b is 20u off a's y-track, but a third part occupies the landing zone:
	// snapping b fully onto the track would overlap it.
	src := []byte(`
nodes:
  scene:
    shape: composite
    parts:
      - { id: a,  shape: rectangle, offset: { wx: 0,   wy: 0 },  geom: { w: 120, d: 90, h: 30 }, label: "A" }
      - { id: mid,shape: rectangle, offset: { wx: 140, wy: 0 },  geom: { w: 120, d: 90, h: 30 }, label: "M" }
      - { id: b,  shape: rectangle, offset: { wx: 150, wy: 110 },geom: { w: 120, d: 90, h: 30 }, label: "B" }
`)
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	before := Readability(doc)
	doc2, _ := Parse(src)
	ComposeScene(doc2)
	after := Readability(doc2)
	if after.Overlaps > before.Overlaps {
		t.Errorf("compose created an overlap: %d -> %d", before.Overlaps, after.Overlaps)
	}
}
