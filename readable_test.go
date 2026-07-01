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
