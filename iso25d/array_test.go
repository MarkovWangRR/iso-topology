package iso25d

import (
	"strings"
	"testing"
)

func TestArrayRenderSmoke(t *testing.T) {
	for _, name := range []string{"array1d", "array2d", "array3d"} {
		svg := Convert2DTo25D(name, ConvertOpts{
			Width: 200, Depth: 200, Height: 100,
			TopFill: "#DBEAFE", LeftFill: "#1E40AF", RightFill: "#2563EB",
			Stroke: "#1E3A8A", StrokeWidth: 0.8,
		})
		if !strings.HasPrefix(svg, "<svg") {
			t.Errorf("%s: expected SVG output", name)
		}
		if !strings.Contains(svg, "polygon") {
			t.Errorf("%s: expected polygons in SVG", name)
		}
	}
}

func TestArrayCellCount(t *testing.T) {
	svg := Convert2DTo25D("array2d", ConvertOpts{
		Width: 200, Depth: 200, Height: 60,
		TopFill: "#DBEAFE", LeftFill: "#1E40AF", RightFill: "#2563EB",
		Params: map[string]any{"countX": 3, "countY": 3},
	})
	count := strings.Count(svg, "<polygon")
	if count < 27 {
		t.Errorf("array2d 3x3: expected >=27 polygons, got %d", count)
	}
}
