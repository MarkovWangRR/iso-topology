package iso25d

import (
	"math"
	"strings"
	"testing"
)

func TestWedgeFaceInvariants(t *testing.T) {
	var prov WedgeShapeProvider
	w, d, h := 160.0, 140.0, 80.0
	faces := prov.Faces(w, d, h, nil)

	visibleCount := 0
	for _, f := range faces {
		// No NaN in any point.
		for _, p := range f.Points {
			if math.IsNaN(p[0]) || math.IsNaN(p[1]) {
				t.Errorf("face %q has NaN point %v", f.Name, p)
			}
		}
		if f.Visible {
			visibleCount++
			// Visible faces must have outward normals.
			switch f.Name {
			case "slope":
				if f.Normal[1] <= 0 || f.Normal[2] <= 0 {
					t.Errorf("slope face normal should have ny>0 and nz>0, got %v", f.Normal)
				}
			case "right":
				if math.Abs(f.Normal[0]-1) > 1e-6 {
					t.Errorf("right face normal should be (1,0,0), got %v", f.Normal)
				}
			default:
				t.Errorf("unexpected visible face: %q", f.Name)
			}
		}
	}
	if visibleCount != 2 {
		t.Errorf("expected 2 visible faces (slope + right), got %d", visibleCount)
	}
}

func TestWedgeRenderSmoke(t *testing.T) {
	o := DefaultIsoBox()
	o.Width = 160
	o.Depth = 140
	o.Height = 80
	o.Label = "Ramp"
	svg := RenderIsoWedge(o)
	if !strings.HasPrefix(svg, "<svg") {
		t.Fatalf("expected SVG output, got: %s", svg[:min(80, len(svg))])
	}
	if !strings.Contains(svg, "<polygon") {
		t.Error("expected at least one <polygon> in wedge SVG")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
