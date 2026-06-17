package iso25d

import (
	"strings"
	"testing"
)

func TestRackRenderSmoke(t *testing.T) {
	svg := Convert2DTo25D("rack", ConvertOpts{
		Width: 80, Depth: 60, Height: 220,
		TopFill: "#334155", LeftFill: "#0F172A", RightFill: "#1E293B",
		Stroke: "#64748B", StrokeWidth: 1.2,
		Label: "Server Rack",
		Params: map[string]any{"slots": 5},
	})
	if !strings.HasPrefix(svg, "<svg") {
		t.Error("rack: expected SVG output")
	}
	// Closed shell = two camera-facing walls + top; each slot adds a recessed
	// band on both walls (5 slots → 3 + 2*5 = 13 faces). data-face="right" is
	// the front-right wall — the closure fix — so assert it's present.
	if !strings.Contains(svg, `data-face="right"`) || !strings.Contains(svg, `data-face="left"`) {
		t.Error("rack: expected a closed shell with both visible walls")
	}
	count := strings.Count(svg, "<polygon")
	if count < 13 {
		t.Errorf("rack 5 slots: expected >=13 polygons, got %d", count)
	}
}
