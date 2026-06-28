package isotopo

import (
	"strings"
	"testing"
)

func fptr(v float64) *float64 { return &v }

// captionRideIssue returns the first caption-ride warning whose message mentions
// the given caption, or "" if none.
func captionRideMsg(issues []Issue, caption string) string {
	for _, is := range issues {
		if strings.Contains(is.Message, caption) && strings.Contains(is.Message, "group label") {
			return is.Message
		}
	}
	return ""
}

// A column group packs children to the front edge, where the caption is
// anchored — so the front-most child rides the caption. validate must flag it,
// name THAT child (not a neighbour), and classify it as the group's own child.
func TestCaptionRide_NamesFrontChild(t *testing.T) {
	doc := &Document{Nodes: map[string]*Node{
		"scene": {Shape: "composite", Parts: []*CompositePart{{
			ID: "lane", Shape: "group", Label: "Ingestion Lane",
			Layout: &Layout{Mode: "column", Gap: fptr(0.7)},
			Parts: []*CompositePart{
				{ID: "back", Shape: "rectangle", Geom: &Geom{W: 130, D: 96, H: 34}},
				{ID: "front", Shape: "rectangle", Geom: &Geom{W: 130, D: 96, H: 34}},
			},
		}}},
	}}
	msg := captionRideMsg(LabelOcclusionIssues(doc), "Ingestion Lane")
	if msg == "" {
		t.Fatal("expected a caption-ride warning for the front-most child")
	}
	if !strings.Contains(msg, `"front"`) || !strings.Contains(msg, "own child") {
		t.Fatalf("warning must name the front child as the group's own child, got %q", msg)
	}
	if strings.Contains(msg, `"back"`) {
		t.Fatalf("the back child does not ride the caption and must not be named, got %q", msg)
	}
}

// A see-through (no top fill) front child paints nothing over the caption, so it
// must not be flagged as riding it.
func TestCaptionRide_GhostFrontChildIgnored(t *testing.T) {
	doc := &Document{Nodes: map[string]*Node{
		"scene": {Shape: "composite", Parts: []*CompositePart{{
			ID: "lane", Shape: "group", Label: "Ingestion Lane",
			Layout: &Layout{Mode: "column", Gap: fptr(0.7)},
			Parts: []*CompositePart{
				{ID: "back", Shape: "rectangle", Geom: &Geom{W: 130, D: 96, H: 34}},
				{ID: "front", Shape: "rectangle", Geom: &Geom{W: 130, D: 96, H: 34},
					Style: &Style{Palette: &Palette{Top: "none", Left: "none", Right: "none"}}},
			},
		}}},
	}}
	if msg := captionRideMsg(LabelOcclusionIssues(doc), "Ingestion Lane"); msg != "" {
		t.Fatalf("a see-through ghost front child must not be flagged, got %q", msg)
	}
}

// A roomy group whose children clear the caption stays silent (threshold guards
// against flagging mere grazes).
func TestCaptionRide_RoomyGroupSilent(t *testing.T) {
	doc := &Document{Nodes: map[string]*Node{
		"scene": {Shape: "composite", Parts: []*CompositePart{{
			ID: "lane", Shape: "group", Label: "Lane",
			Layout: &Layout{Mode: "column", Gap: fptr(4.0)}, // large front padding
			Parts: []*CompositePart{
				{ID: "only", Shape: "rectangle", Geom: &Geom{W: 90, D: 60, H: 18}},
			},
		}}},
	}}
	if msg := captionRideMsg(LabelOcclusionIssues(doc), "Lane"); msg != "" {
		t.Fatalf("a roomy group must not flag a caption ride, got %q", msg)
	}
}
