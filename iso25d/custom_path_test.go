package iso25d

import (
	"strings"
	"testing"
)

func TestCustomPathParseTriangle(t *testing.T) {
	pts := parseMLZPath("M 50,0 L 100,100 L 0,100 Z")
	if len(pts) != 3 {
		t.Fatalf("expected 3 points, got %d: %v", len(pts), pts)
	}
}

func TestCustomPathParseRectangle(t *testing.T) {
	pts := parseMLZPath("M 0,0 L 100,0 L 100,100 L 0,100 Z")
	if len(pts) != 4 {
		t.Fatalf("expected 4 points, got %d: %v", len(pts), pts)
	}
}

func TestCustomPathEmptyFallback(t *testing.T) {
	// Empty path should produce the fallback rectangle (4 points).
	base := customPathBase(100, 80, nil)
	if len(base) != 4 {
		t.Fatalf("empty path fallback should be a 4-point rectangle, got %d", len(base))
	}
}

func TestCustomPathRenderTriangle(t *testing.T) {
	o := DefaultIsoBox()
	o.Width = 140
	o.Depth = 140
	o.Height = 50
	svg := RenderIsoCustomPath(o, "M 70,0 L 140,140 L 0,140 Z")
	if !strings.HasPrefix(svg, "<svg") {
		t.Fatalf("expected SVG, got: %s", svg[:minCP(80, len(svg))])
	}
	if !strings.Contains(svg, "<polygon") {
		t.Error("expected polygons in custom_path SVG")
	}
}

func TestCustomPathRenderEmpty(t *testing.T) {
	o := DefaultIsoBox()
	o.Width = 100
	o.Depth = 100
	o.Height = 60
	svg := RenderIsoCustomPath(o, "")
	if !strings.HasPrefix(svg, "<svg") {
		t.Fatalf("expected SVG even with empty path, got: %s", svg[:minCP(80, len(svg))])
	}
}

func minCP(a, b int) int {
	if a < b {
		return a
	}
	return b
}
