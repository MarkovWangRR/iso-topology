package isotopo

import "testing"

// These guard a contrast-checker bug: VisualContrastIssues read colours straight
// off node.Style, ignoring theme.presets and per-face gradients (and the
// renderer's actual ResolveStyle merge). Any preset-/gradient-styled part was
// therefore measured against the DEFAULT box fill, producing false low-contrast
// and "indistinguishable from background" warnings on heroes, glows, and the
// white-card-on-light-grey design language. The fix resolves the effective
// style first, skips the background check for shadow/outline/stroke-separated
// parts, and skips the text check for label-less parts.

// A preset that supplies a dark top + a part with light text must read as HIGH
// contrast. Before the fix the preset fill was invisible here → default mid-blue
// fill → false low-contrast warning.
func TestVisualContrast_PresetFillResolved(t *testing.T) {
	doc := &Document{
		Theme: &Theme{Presets: map[string]*Style{
			"dark": {Palette: &Palette{Top: "#1A1A1A"}},
		}},
		Nodes: map[string]*Node{
			"scene": {Shape: "composite", Parts: []*CompositePart{
				{ID: "box", Shape: "rectangle", Preset: "dark", Label: "Hi",
					Style: &Style{Text: &Text{Color: "#FFFFFF"}}},
			}},
		},
	}
	if hasWarning(VisualContrastIssues(doc), "low contrast between top fill") {
		t.Errorf("preset dark top vs white text is high-contrast; should not warn")
	}
}

// A gradient top has no solid `top`; the checker must read the gradient's
// representative stop, not fall through to the default fill.
func TestVisualContrast_GradientFillResolved(t *testing.T) {
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {Shape: "composite", Parts: []*CompositePart{
				{ID: "hero", Shape: "rectangle", Label: "Core",
					Style: &Style{
						Palette: &Palette{TopGradient: &FaceGradient{From: "#101010", To: "#000000"}},
						Text:    &Text{Color: "#FFFFFF"},
					}},
			}},
		},
	}
	if hasWarning(VisualContrastIssues(doc), "low contrast between top fill") {
		t.Errorf("gradient dark top vs white text is high-contrast; should not warn")
	}
}

// A white card on a light-grey canvas is separated by its drop shadow, not by
// top-fill contrast — the background check must not fire. The same card WITHOUT
// any separator must still warn (the check still catches genuine vanishing).
func TestVisualContrast_ShadowSeparatedNotFlagged(t *testing.T) {
	mk := func(withShadow bool) *Document {
		st := &Style{Palette: &Palette{Top: "#FFFFFF"}}
		if withShadow {
			st.Effects = &Effects{DropShadow: &DropShadow{Dy: 16, Blur: 22, Color: "#00000033"}}
		}
		return &Document{
			Canvas: &Canvas{Background: "#F2F3F5"},
			Nodes: map[string]*Node{
				"scene": {Shape: "composite", Parts: []*CompositePart{
					{ID: "card", Shape: "rectangle", Style: st},
				}},
			},
		}
	}
	if hasWarning(VisualContrastIssues(mk(true)), "indistinguishable from canvas") {
		t.Errorf("shadowed card is separated from the canvas; should not warn")
	}
	if !hasWarning(VisualContrastIssues(mk(false)), "indistinguishable from canvas") {
		t.Errorf("unshadowed near-bg card should still warn (genuine catch)")
	}
}

// A part with no label must not be checked for top-fill-vs-text contrast.
func TestVisualContrast_NoLabelNoTextCheck(t *testing.T) {
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {Shape: "composite", Parts: []*CompositePart{
				// near-identical fill/text, but no label → no text check
				{ID: "chip", Shape: "rectangle",
					Style: &Style{Palette: &Palette{Top: "#E0E0E0"}, Text: &Text{Color: "#D8D8D8"}}},
			}},
		},
	}
	if hasWarning(VisualContrastIssues(doc), "low contrast between top fill") {
		t.Errorf("label-less part should not trigger fill-vs-text contrast warning")
	}
}
