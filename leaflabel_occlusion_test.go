package isotopo

import (
	"strings"
	"testing"
)

// leafNode builds an ordinary labelled node at a world offset.
func leafNode(id, label string, wx, wy, wz, w, d, h float64) *CompositePart {
	return &CompositePart{
		ID: id, Shape: "rectangle", Label: label,
		Geom:   &Geom{W: w, D: d, H: h},
		Offset: &WorldPoint{WX: wx, WY: wy, WZ: wz},
		Style:  &Style{Palette: &Palette{Top: "#3A6EA5", Left: "#28507A", Right: "#1F3F61"}},
	}
}

// A later-painted opaque node whose silhouette covers a node's top-face centre
// (here: same footprint, taller) hides the face label and must warn, naming
// both the label and the occluder.
func TestLeafLabelOcclusion_Warns(t *testing.T) {
	back := leafNode("back", "BACK", 0, 0, 0, 120, 80, 26)
	front := leafNode("front", "FRONT", 0, 0, 0, 120, 80, 200) // same footprint, taller, painted later
	issues := labelOcclusionInFlat([]*CompositePart{back, front}, nil, "scene")
	if len(issues) == 0 {
		t.Fatal("a node whose face label centre is covered must be flagged")
	}
	m := issues[0].Message
	if !strings.Contains(m, `"back"`) || !strings.Contains(m, `"front"`) || !strings.Contains(m, "hidden") {
		t.Fatalf("warning must name the label node and the occluder, got %q", m)
	}
}

// When the occluder is nowhere near the label centre, stay silent — the
// silhouette test must not over-report (the bounding-box version did).
func TestLeafLabelOcclusion_ClearIsSilent(t *testing.T) {
	back := leafNode("back", "BACK", 0, 0, 0, 120, 80, 26)
	far := leafNode("front", "FRONT", 900, 900, 0, 120, 80, 200)
	if issues := labelOcclusionInFlat([]*CompositePart{back, far}, nil, "scene"); len(issues) != 0 {
		t.Fatalf("a node clear of the label must not warn, got %v", issues)
	}
}

// A see-through "ghost" volume (no top fill) painted over the label hides
// nothing and must not warn.
func TestLeafLabelOcclusion_GhostIgnored(t *testing.T) {
	back := leafNode("back", "BACK", 0, 0, 0, 120, 80, 26)
	ghost := leafNode("ghost", "", 0, 0, 0, 120, 80, 200)
	ghost.Style = &Style{Palette: &Palette{Top: "none", Left: "none", Right: "none"}}
	if issues := labelOcclusionInFlat([]*CompositePart{back, ghost}, nil, "scene"); len(issues) != 0 {
		t.Fatalf("a transparent ghost volume must not be flagged as occluding, got %v", issues)
	}
}

// A screen-oriented label is painted on the top screen layer and can never be
// occluded by a node body, so it must not warn even when a node covers its tile.
func TestLeafLabelOcclusion_ScreenOrientSkipped(t *testing.T) {
	back := leafNode("back", "BACK", 0, 0, 0, 120, 80, 26)
	back.Style.Text = &Text{Orient: "screen"}
	front := leafNode("front", "FRONT", 0, 0, 0, 120, 80, 200)
	if issues := labelOcclusionInFlat([]*CompositePart{back, front}, nil, "scene"); len(issues) != 0 {
		t.Fatalf("a screen-oriented label must not be flagged, got %v", issues)
	}
}

// A node's own stack replica overlapping it is by-design, not an occlusion.
func TestLeafLabelOcclusion_StackSiblingSkipped(t *testing.T) {
	base := leafNode("pods", "Pods", 0, 0, 0, 120, 80, 26)
	clone := leafNode("pods~1", "Pods", 0, 0, 0, 120, 80, 200)
	if issues := labelOcclusionInFlat([]*CompositePart{base, clone}, nil, "scene"); len(issues) != 0 {
		t.Fatalf("a stack replica must not be flagged as occluding its base, got %v", issues)
	}
}
