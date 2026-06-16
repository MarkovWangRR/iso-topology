package iso25d

import (
	"strings"
	"testing"
)

// TestCustomPathTriangle tests with a simple triangle path.
func TestCustomPathTriangle(t *testing.T) {
	var prov CustomPathShapeProvider
	params := map[string]any{"path": "M 50,0 L 100,100 L 0,100 Z"}
	w, d, h := 160.0, 120.0, 40.0
	faces := prov.Faces(w, d, h, params)
	if len(faces) < 2 {
		t.Errorf("triangle: expected at least 2 faces, got %d", len(faces))
	}
	// Top face must be present
	hasTop := false
	for _, f := range faces {
		if f.Name == "top" {
			hasTop = true
		}
	}
	if !hasTop {
		t.Error("triangle: missing top face")
	}
}

// TestCustomPathRectangle tests with a rectangle path (should behave like box).
func TestCustomPathRectangle(t *testing.T) {
	var prov CustomPathShapeProvider
	params := map[string]any{"path": "M 0,0 L 100,0 L 100,100 L 0,100 Z"}
	w, d, h := 160.0, 120.0, 40.0
	faces := prov.Faces(w, d, h, params)
	if len(faces) < 2 {
		t.Errorf("rectangle: expected at least 2 faces, got %d", len(faces))
	}
}

// TestCustomPathFallback tests fallback on empty path.
func TestCustomPathFallback(t *testing.T) {
	var prov CustomPathShapeProvider
	params := map[string]any{"path": ""}
	w, d, h := 160.0, 120.0, 40.0
	faces := prov.Faces(w, d, h, params)
	// Fallback to box = 3 faces
	if len(faces) == 0 {
		t.Error("empty path: expected fallback faces, got 0")
	}
}

// TestCustomPathRenderSmoke tests that rendering produces valid SVG.
func TestCustomPathRenderSmoke(t *testing.T) {
	o := DefaultIsoBox()
	o.Width = 160
	o.Depth = 120
	o.Height = 40
	o.TopFill = "#DBEAFE"
	o.LeftFill = "#1D4ED8"
	o.RightFill = "#3B82F6"
	o.Label = "Custom"

	params := map[string]any{"path": "M 50,0 L 100,100 L 0,100 Z"}
	svg := RenderIsoCustomPath(o, params)
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("output is not SVG: %q", svg[:minInt(50, len(svg))])
	}
	if !strings.Contains(svg, "</svg>") {
		t.Error("SVG not closed")
	}
	if strings.Contains(svg, "NaN") {
		t.Error("SVG contains NaN values")
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
