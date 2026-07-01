package isotopo

import "testing"

func readableDoc(bg string) *Document {
	return &Document{
		Canvas: &Canvas{Background: bg},
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{ID: "a", Shape: "rectangle", Label: "Auto"},
					{ID: "b", Shape: "rectangle", Label: "Explicit",
						Style: &Style{Text: &Text{Orient: "iso"}}}, // author opted in to on-face
					{ID: "g", Shape: "group", Label: "Group", Parts: []*CompositePart{
						{ID: "child", Shape: "rectangle", Label: "Child"},
					}},
				},
			},
		},
	}
}

// TestApplyReadableProfile covers issue #11: the profile flips gap-filled labels
// to legible screen chips (canvas-aware), recurses into groups, respects an
// author's explicit orientation, and enforces a padding floor.
func TestApplyReadableProfile(t *testing.T) {
	// Light canvas → dark text chip.
	light := readableDoc("#F6F7FB")
	ApplyReadableProfile(light)
	a := light.Nodes["scene"].Parts[0].Style.Text
	if a.Orient != "screen" {
		t.Errorf("labeled part: orient = %q; want screen", a.Orient)
	}
	if a.Color != "#14181F" {
		t.Errorf("light canvas: label color = %q; want dark #14181F", a.Color)
	}
	if a.BoxBg == "" {
		t.Error("light canvas: expected a contrast chip background")
	}
	// Explicit orientation is respected, not overridden.
	if b := light.Nodes["scene"].Parts[1].Style.Text; b.Orient != "iso" {
		t.Errorf("explicit orient overridden: got %q; want iso", b.Orient)
	}
	// Recurses into group children.
	if c := light.Nodes["scene"].Parts[2].Parts[0].Style.Text; c == nil || c.Orient != "screen" {
		t.Error("group child label not made readable")
	}
	// Padding floor.
	if light.Canvas.Padding < 48 {
		t.Errorf("padding floor not applied: %v", light.Canvas.Padding)
	}

	// Dark canvas → light text chip.
	dark := readableDoc("#0E1116")
	ApplyReadableProfile(dark)
	if col := dark.Nodes["scene"].Parts[0].Style.Text.Color; col != "#F5F7FA" {
		t.Errorf("dark canvas: label color = %q; want light #F5F7FA", col)
	}
}

// TestReadableMinFootprint covers Option A: the readable profile enlarges small
// leaf nodes to a minimum footprint (so tiny logo/label tiles grow), while
// leaving big nodes, containers, and geomless nodes alone.
func TestReadableMinFootprint(t *testing.T) {
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{ID: "chip", Shape: "rectangle", Icon: "iso://si/github",
						Geom: &Geom{W: 66, D: 60, H: 18}}, // small leaf → bumped
					{ID: "big", Shape: "rectangle", Label: "Big",
						Geom: &Geom{W: 200, D: 160, H: 40}}, // already large → untouched
					{ID: "grp", Shape: "group", Geom: &Geom{W: 66, D: 60, H: 8},
						Parts: []*CompositePart{{ID: "inner", Shape: "rectangle", Icon: "iso://si/go",
							Geom: &Geom{W: 66, D: 60, H: 18}}}}, // container untouched; child bumped
				},
			},
		},
	}
	ApplyReadableProfile(doc)
	p := doc.Nodes["scene"].Parts
	if p[0].Geom.W != readableMinNodeW || p[0].Geom.D != readableMinNodeD {
		t.Errorf("small leaf not bumped: got %gx%g; want %dx%d", p[0].Geom.W, p[0].Geom.D, readableMinNodeW, readableMinNodeD)
	}
	if p[1].Geom.W != 200 || p[1].Geom.D != 160 {
		t.Errorf("large node was modified: %gx%g", p[1].Geom.W, p[1].Geom.D)
	}
	if p[2].Geom.W != 66 { // container itself left alone
		t.Errorf("container footprint changed: %g", p[2].Geom.W)
	}
	if c := p[2].Parts[0].Geom; c.W != readableMinNodeW || c.D != readableMinNodeD {
		t.Errorf("nested leaf not bumped: %gx%g", c.W, c.D)
	}
}
