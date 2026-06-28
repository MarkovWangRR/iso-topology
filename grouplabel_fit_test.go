package isotopo

import "testing"

// An auto-sized group must grow to fit its own caption: a long label on a
// narrow content box widens the slab so the caption never overruns it.
func TestLabelReservedFootprint_GrowsForLongCaption(t *testing.T) {
	g := &CompositePart{
		ID: "obs", Shape: "group",
		Label: "LangSmith · Observability & Tracing Suite",
		Parts: []*CompositePart{{ID: "a", Shape: "rectangle"}},
	}
	contentW := 168.0 // two small chips in a row — far narrower than the caption
	w, _ := labelReservedFootprint(g, contentW, 200, 28)
	if w <= contentW {
		t.Fatalf("slab width must grow to fit a long caption: got %.0f, content was %.0f", w, contentW)
	}
}

// A short caption that already fits must not change the footprint — existing
// diagrams stay byte-identical (the golden suite depends on this).
func TestLabelReservedFootprint_NoOpWhenItFits(t *testing.T) {
	g := &CompositePart{
		ID: "svc", Shape: "group", Label: "API",
		Parts: []*CompositePart{{ID: "a", Shape: "rectangle"}},
	}
	w, d := labelReservedFootprint(g, 400, 300, 80)
	if w != 400 || d != 300 {
		t.Fatalf("a caption that already fits must not resize the slab: got (%.0f, %.0f)", w, d)
	}
}

// A group with no caption (or no children) reserves nothing.
func TestLabelReservedFootprint_NoCaptionNoChange(t *testing.T) {
	cases := []*CompositePart{
		{ID: "g", Shape: "group", Parts: []*CompositePart{{ID: "a"}}},                                          // no label
		{ID: "g", Shape: "group", Label: "A very long caption that would otherwise force a wide slab indeed"}, // no children
	}
	for i, g := range cases {
		w, d := labelReservedFootprint(g, 120, 120, 40)
		if w != 120 || d != 120 {
			t.Fatalf("case %d: must not reserve without both a caption and children: got (%.0f, %.0f)", i, w, d)
		}
	}
}

// Explicit author dimensions still win end-to-end: a group sized by the author
// keeps that size even when the caption would want more (the reservation only
// enlarges an auto-derived footprint, via ensureFootprint).
func TestGroupCaption_ExplicitDimsRespected(t *testing.T) {
	doc := &Document{Nodes: map[string]*Node{
		"scene": {Shape: "composite", Parts: []*CompositePart{
			{
				ID: "obs", Shape: "group",
				Label: "LangSmith · Observability & Tracing Suite",
				Geom:  &Geom{W: 150, D: 120}, // author-fixed, deliberately narrow
				Layout: &Layout{Mode: "row"},
				Parts: []*CompositePart{{ID: "a", Shape: "rectangle", Geom: &Geom{W: 60, D: 50, H: 16}}},
			},
		}},
	}}
	applyLayout(doc.Nodes["scene"], nil)
	g := doc.Nodes["scene"].Parts[0]
	if g.Geom.W != 150 || g.Geom.D != 120 {
		t.Fatalf("explicit group dims must be respected, got W=%.0f D=%.0f", g.Geom.W, g.Geom.D)
	}
}
