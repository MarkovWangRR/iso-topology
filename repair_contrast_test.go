package isotopo

import "testing"

// TestRepairContrastLiftsLabel verifies the issue #8 auto-repair: a labelled
// part whose text/fill contrast is below 3.0 gets its label retinted until it
// passes, and a scene that already passes is left byte-identical (no-op).
func TestRepairContrastLiftsLabel(t *testing.T) {
	// A part with a mid-grey top fill and near-matching grey text — ~2:1, below
	// the 3.0 threshold. Repair must lift it to >= 3.0.
	low := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "n",
						Shape: "rectangle",
						Label: "Service",
						Style: &Style{
							Palette: &Palette{Top: "#8A8A8A"},
							Text:    &Text{Color: "#B0B0B0"},
						},
					},
				},
			},
		},
	}
	if got := contrastIssueCount(low); got == 0 {
		t.Fatal("test setup: expected a sub-threshold label before repair")
	}
	RepairScene(low)
	if got := contrastIssueCount(low); got != 0 {
		t.Errorf("after repair still %d low-contrast label(s); want 0", got)
	}

	// No-op guarantee: a scene that already passes must be untouched.
	clean := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{ID: "n", Shape: "rectangle", Label: "OK",
						Style: &Style{Palette: &Palette{Top: "#101418"}, Text: &Text{Color: "#FFFFFF"}}},
				},
			},
		},
	}
	before := clean.Nodes["scene"].Parts[0].Style.Text.Color
	RepairScene(clean)
	if after := clean.Nodes["scene"].Parts[0].Style.Text.Color; after != before {
		t.Errorf("repair mutated an already-legible label: %q -> %q", before, after)
	}
}
