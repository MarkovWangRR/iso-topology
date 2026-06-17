package isotopo

import (
	"context"
	"strings"
	"testing"
)

// ── BuildInteractionModel ────────────────────────────────────────────────────

func TestBuildInteractionModel_NilDoc(t *testing.T) {
	if m := BuildInteractionModel(nil); m != nil {
		t.Errorf("nil doc: expected nil model, got %v", m)
	}
}

func TestBuildInteractionModel_NoScene(t *testing.T) {
	doc := &Document{}
	if m := BuildInteractionModel(doc); m != nil {
		t.Errorf("doc with no scene: expected nil model, got %v", m)
	}
}

func TestBuildInteractionModel_Parts(t *testing.T) {
	doc, err := LoadInput(context.Background(), "yaml", []byte(subgraphFixture), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	model := BuildInteractionModel(doc)
	if len(model) == 0 {
		t.Fatal("expected non-empty model")
	}

	byID := map[string]PartModel{}
	for _, pm := range model {
		byID[pm.ID] = pm
	}

	// Every part declared in the fixture must appear in the model.
	for _, id := range []string{"agent_a", "agent_b", "runner", "sandbox", "reliab", "server"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("part %q missing from interaction model", id)
		}
	}

	// runner is a container (boundary shape with nested parts).
	if r, ok := byID["runner"]; !ok || !r.Container {
		t.Errorf("runner should be marked as container, got %+v", byID["runner"])
	}
	// agent_a is a leaf node.
	if a, ok := byID["agent_a"]; !ok || a.Container {
		t.Errorf("agent_a should NOT be a container, got %+v", byID["agent_a"])
	}
}

func TestBuildInteractionModel_Dimensions(t *testing.T) {
	doc, err := LoadInput(context.Background(), "yaml", []byte(subgraphFixture), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	model := BuildInteractionModel(doc)
	for _, pm := range model {
		if pm.W <= 0 || pm.D <= 0 {
			t.Errorf("part %q has non-positive dimensions w=%g d=%g", pm.ID, pm.W, pm.D)
		}
	}
}

func TestBuildInteractionModel_Anchors(t *testing.T) {
	doc, err := LoadInput(context.Background(), "yaml", []byte(subgraphFixture), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	model := BuildInteractionModel(doc)
	for _, pm := range model {
		if len(pm.Anchors) != 4 {
			t.Errorf("part %q: expected 4 anchors, got %d", pm.ID, len(pm.Anchors))
			continue
		}
		names := map[string]bool{}
		for _, a := range pm.Anchors {
			names[a.Name] = true
		}
		for _, want := range []string{"top", "right", "bottom", "left"} {
			if !names[want] {
				t.Errorf("part %q missing anchor %q", pm.ID, want)
			}
		}
		// Each anchor must lie within or on the part's bounding box (with a
		// small tolerance for floating-point edge-snapping).
		for _, a := range pm.Anchors {
			const tol = 0.01
			if a.WX < pm.X-tol || a.WX > pm.X+pm.W+tol ||
				a.WY < pm.Y-tol || a.WY > pm.Y+pm.D+tol {
				t.Errorf("part %q anchor %q (%g,%g) outside AABB (%g,%g)+(%g,%g)",
					pm.ID, a.Name, a.WX, a.WY, pm.X, pm.Y, pm.W, pm.D)
			}
		}
	}
}

// ── overlapArea ──────────────────────────────────────────────────────────────

func TestOverlapArea(t *testing.T) {
	cases := []struct {
		name         string
		ax, ay, aw, ad float64
		bx, by, bw, bd float64
		want           float64
	}{
		{"no overlap right",   0, 0, 10, 10, 15, 0, 10, 10, 0},
		{"no overlap above",   0, 0, 10, 10, 0, 15, 10, 10, 0},
		{"full contain",       0, 0, 10, 10, 2, 2, 6, 6, 36},
		{"partial overlap",    0, 0, 10, 10, 5, 5, 10, 10, 25},
		{"touching edge",      0, 0, 10, 10, 10, 0, 10, 10, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := overlapArea(c.ax, c.ay, c.aw, c.ad, c.bx, c.by, c.bw, c.bd)
			if got != c.want {
				t.Errorf("overlapArea = %g, want %g", got, c.want)
			}
		})
	}
}

// ── WorldDropTarget ──────────────────────────────────────────────────────────

// helper: build a flat PartModel slice from (id, container, x, y, w, d) tuples.
func makeModel(parts ...[6]float64) []PartModel {
	// parts: [0]=id_index(unused here), use string names via separate helper
	panic("use makeModelNamed")
}

type pmSpec struct {
	id        string
	container bool
	x, y, w, d float64
}

func buildModel(specs []pmSpec) []PartModel {
	out := make([]PartModel, len(specs))
	for i, s := range specs {
		out[i] = PartModel{ID: s.id, Container: s.container, X: s.x, Y: s.y, W: s.w, D: s.d}
	}
	return out
}

func TestWorldDropTarget_NoContainers(t *testing.T) {
	model := buildModel([]pmSpec{
		{"node_a", false, 0, 0, 100, 100},
		{"node_b", false, 200, 0, 100, 100},
	})
	if got := WorldDropTarget(model, "node_a", 0.5); got != "" {
		t.Errorf("no containers: expected empty target, got %q", got)
	}
}

func TestWorldDropTarget_DraggedNotInModel(t *testing.T) {
	model := buildModel([]pmSpec{
		{"grp", true, 0, 0, 200, 200},
	})
	if got := WorldDropTarget(model, "ghost", 0.5); got != "" {
		t.Errorf("unknown drag id: expected empty target, got %q", got)
	}
}

func TestWorldDropTarget_FullContain(t *testing.T) {
	// node fully inside group
	model := buildModel([]pmSpec{
		{"grp",  true,  0,  0, 200, 200},
		{"node", false, 50, 50, 80,  80},
	})
	if got := WorldDropTarget(model, "node", 0.5); got != "grp" {
		t.Errorf("full contain: expected grp, got %q", got)
	}
}

func TestWorldDropTarget_SmallGroupLargeNode(t *testing.T) {
	// group is smaller than the node — classic failure case for pixel hit-test.
	// The node's centre overlaps the group, so >50% of the group's area is
	// covered (but we test that the NODE's overlap fraction qualifies).
	// Node: 0..100 x 0..100; Group: 40..60 x 40..60 (entirely inside node).
	// Overlap = 20*20 = 400; node area = 10000; frac = 0.04 < 0.5 → no target
	// at 50% threshold. Use a lower threshold (0.01) to confirm it's found.
	model := buildModel([]pmSpec{
		{"grp",  true,  40, 40, 20,  20},
		{"node", false,  0,  0, 100, 100},
	})
	// At minFrac=0.01: overlap=400, nodeArea=10000 → frac=0.04 ≥ 0.01 → grp wins.
	if got := WorldDropTarget(model, "node", 0.01); got != "grp" {
		t.Errorf("small group, low threshold: expected grp, got %q", got)
	}
	// At minFrac=0.5: frac=0.04 < 0.5 → root.
	if got := WorldDropTarget(model, "node", 0.5); got != "" {
		t.Errorf("small group, high threshold: expected root, got %q", got)
	}
}

func TestWorldDropTarget_TwoGroupsPickLarger(t *testing.T) {
	// node overlaps two groups — the one with more overlap area wins.
	// node: 0..100 x 0..100
	// grp_left: -50..60 x 0..100 → overlap with node = 60*100 = 6000
	// grp_right: 50..200 x 0..100 → overlap with node = 50*100 = 5000
	model := buildModel([]pmSpec{
		{"grp_left",  true, -50, 0, 110, 100},
		{"grp_right", true,  50, 0, 150, 100},
		{"node",      false,  0, 0, 100, 100},
	})
	if got := WorldDropTarget(model, "node", 0.1); got != "grp_left" {
		t.Errorf("two groups: expected grp_left (more overlap), got %q", got)
	}
}

func TestWorldDropTarget_DropToRoot(t *testing.T) {
	// node doesn't overlap any container at all
	model := buildModel([]pmSpec{
		{"grp",  true,  200, 200, 100, 100},
		{"node", false,   0,   0, 100, 100},
	})
	if got := WorldDropTarget(model, "node", 0.5); got != "" {
		t.Errorf("no overlap: expected root (%q), got %q", "", got)
	}
}

func TestWorldDropTarget_ContainerExcludesSelf(t *testing.T) {
	// dragID is itself a container — should not match itself.
	model := buildModel([]pmSpec{
		{"grp", true, 0, 0, 100, 100},
	})
	if got := WorldDropTarget(model, "grp", 0.0); got != "" {
		t.Errorf("container dragging itself: expected root, got %q", got)
	}
}

// ── faceAnchors / anchor geometry ───────────────────────────────────────────

func TestFaceAnchors_Names(t *testing.T) {
	r := planRect{id: "x", x: 10, y: 20, w: 100, d: 80}
	anchors := faceAnchors(r)
	if len(anchors) != 4 {
		t.Fatalf("expected 4 anchors, got %d", len(anchors))
	}
	names := map[string]bool{}
	for _, a := range anchors {
		names[a.Name] = true
	}
	for _, want := range []string{"top", "right", "bottom", "left"} {
		if !names[want] {
			t.Errorf("missing anchor %q", want)
		}
	}
}

func TestFaceAnchors_Positions(t *testing.T) {
	r := planRect{id: "x", x: 0, y: 0, w: 100, d: 60}
	byName := map[string]AnchorPoint{}
	for _, a := range faceAnchors(r) {
		byName[a.Name] = a
	}

	// top: centre x, min y
	if byName["top"].WX != 50 || byName["top"].WY != 0 {
		t.Errorf("top anchor = (%g,%g), want (50,0)", byName["top"].WX, byName["top"].WY)
	}
	// bottom: centre x, max y
	if byName["bottom"].WX != 50 || byName["bottom"].WY != 60 {
		t.Errorf("bottom anchor = (%g,%g), want (50,60)", byName["bottom"].WX, byName["bottom"].WY)
	}
	// left: min x, centre y
	if byName["left"].WX != 0 || byName["left"].WY != 30 {
		t.Errorf("left anchor = (%g,%g), want (0,30)", byName["left"].WX, byName["left"].WY)
	}
	// right: max x, centre y
	if byName["right"].WX != 100 || byName["right"].WY != 30 {
		t.Errorf("right anchor = (%g,%g), want (100,30)", byName["right"].WX, byName["right"].WY)
	}
}

func TestFaceAnchors_OnBoundary(t *testing.T) {
	// Every anchor must lie on the perimeter of the AABB.
	r := planRect{id: "y", x: 5, y: 10, w: 80, d: 40}
	for _, a := range faceAnchors(r) {
		onEdge := a.WX == r.x || a.WX == r.x+r.w || a.WY == r.y || a.WY == r.y+r.d
		if !onEdge {
			t.Errorf("anchor %q (%g,%g) not on AABB boundary", a.Name, a.WX, a.WY)
		}
	}
}

// ── AddConnector with anchors (integration via ApplyOpText) ─────────────────

func TestApplyOpText_AddEdgeWithAnchors(t *testing.T) {
	src := []byte(subgraphFixture)
	op := EditOp{
		Kind:   "add-edge",
		Fields: map[string]string{"from": "agent_a", "to": "server", "fromAnchor": "right", "toAnchor": "left"},
	}
	out, err := ApplyOpText("yaml", src, op)
	if err != nil {
		t.Fatalf("ApplyOpText add-edge with anchors: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "agent_a.right") {
		t.Errorf("fromAnchor not written: %s", outStr)
	}
	if !strings.Contains(outStr, "server.left") {
		t.Errorf("toAnchor not written: %s", outStr)
	}
}

func TestApplyOpText_AddEdgeNoAnchors(t *testing.T) {
	src := []byte(subgraphFixture)
	op := EditOp{
		Kind:   "add-edge",
		Fields: map[string]string{"from": "agent_a", "to": "server"},
	}
	out, err := ApplyOpText("yaml", src, op)
	if err != nil {
		t.Fatalf("ApplyOpText add-edge: %v", err)
	}
	outStr := string(out)
	// No anchor dot should appear in the new connector line.
	// Check that the newly added connector line has no anchor dot suffix.
	for _, l := range splitLines(outStr) {
		if strings.Contains(l, "from: agent_a") && strings.Contains(l, "to: server") {
			if strings.Contains(l, "agent_a.") || strings.Contains(l, "server.") {
				t.Errorf("anchor dot in no-anchor add-edge: %s", l)
			}
			break
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
