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
	count := strings.Count(svg, "<polygon")
	if count < 18 {
		t.Errorf("rack 5 slots: expected >=18 polygons, got %d", count)
	}
}
