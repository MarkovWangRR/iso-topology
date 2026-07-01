package isotopo

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestDefaultLabelSizesLegible guards the default text sizes for node and edge
// labels. The historical defaults (node 16, edge/screen 11) read too small at
// 1:1; they were raised to node 18, edge 13, screen 13. Node labels still
// auto-shrink to fit a small face, so this asserts the *default* (unconstrained)
// sizes via a scene with roomy nodes.
func TestDefaultLabelSizesLegible(t *testing.T) {
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{ID: "a", Shape: "rectangle", Label: "Service A",
						Geom: &Geom{W: 140, D: 140, H: 60}},
					{ID: "b", Shape: "rectangle", Label: "Service B",
						Geom: &Geom{W: 140, D: 140, H: 60}, Offset: &WorldPoint{WX: 260}},
				},
				Connectors: []*Connector{{From: "a", To: "b", Label: "HTTPS"}},
			},
		},
	}
	svg := RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)

	// Node label: default must be >= 18 (unconstrained roomy face).
	if !strings.Contains(svg, `>Service A</text>`) {
		t.Fatal("node label not rendered")
	}
	nodeSize := fontSizeOfLabel(t, svg, "Service A")
	if nodeSize < 18 {
		t.Errorf("default node label size = %.1f; want >= 18", nodeSize)
	}
	// Edge label: default must be >= 13.
	edgeSize := fontSizeOfLabel(t, svg, "HTTPS")
	if edgeSize < 13 {
		t.Errorf("default edge label size = %.1f; want >= 13", edgeSize)
	}
}

// TestGroupCaptionSizeLegible guards the group-caption default (raised 11 -> 13
// to match the edge-label tier), so category headers read at 1:1.
func TestGroupCaptionSizeLegible(t *testing.T) {
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{ID: "grp", Shape: "group", Label: "Databases",
						Parts: []*CompositePart{
							{ID: "a", Shape: "rectangle", Label: "A", Geom: &Geom{W: 120, D: 100, H: 30}},
						}},
				},
			},
		},
	}
	svg := RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
	if sz := fontSizeOfLabel(t, svg, "Databases"); sz < 13 {
		t.Errorf("group caption size = %.1f; want >= 13", sz)
	}
}

// fontSizeOfLabel extracts the font-size of the <text> whose content is `label`.
func fontSizeOfLabel(t *testing.T, svg, label string) float64 {
	t.Helper()
	re := regexp.MustCompile(`font-size="([0-9.]+)"[^>]*>` + regexp.QuoteMeta(label) + `</text>`)
	m := re.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no <text> found for label %q", label)
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}
