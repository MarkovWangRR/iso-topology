package iso25d

import (
	"math"
	"strings"
	"testing"
)

// TestWedgeFaceInvariants checks geometry correctness for WedgeShapeProvider.
func TestWedgeFaceInvariants(t *testing.T) {
	var prov WedgeShapeProvider
	w, d, h := 160.0, 120.0, 50.0
	faces := prov.Faces(w, d, h, nil)

	// Must have at least 2 visible faces (slope + right triangle)
	visCount := 0
	for _, f := range faces {
		for _, p := range f.Points {
			if math.IsNaN(p[0]) || math.IsNaN(p[1]) {
				t.Fatalf("face %s: NaN point", f.Name)
			}
		}
		if f.Visible {
			visCount++
			// Visible face normals should face toward the camera (1,1,1)
			dot := f.Normal[0] + f.Normal[1] + f.Normal[2]
			if dot <= -1e-6 {
				t.Errorf("face %s visible but normal away from camera: normal=%v dot=%f", f.Name, f.Normal, dot)
			}
		}
	}
	if visCount < 2 {
		t.Errorf("expected at least 2 visible faces, got %d", visCount)
	}

	// Silhouette must be non-degenerate
	sil := prov.Silhouette(w, d, h, nil)
	if len(sil) < 4 {
		t.Errorf("degenerate silhouette: %d points", len(sil))
	}
}

// TestWedgeRenderSmoke renders a wedge and checks SVG validity.
func TestWedgeRenderSmoke(t *testing.T) {
	o := DefaultIsoBox()
	o.Width = 160
	o.Depth = 120
	o.Height = 50
	o.TopFill = "#FEF3C7"
	o.LeftFill = "#D97706"
	o.RightFill = "#F59E0B"
	o.Label = "Wedge"

	svg := RenderIsoWedge(o)
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("output is not SVG: %q", svg[:min(50, len(svg))])
	}
	if !strings.Contains(svg, "</svg>") {
		t.Error("SVG not closed")
	}
	if strings.Contains(svg, "NaN") {
		t.Error("SVG contains NaN values")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
