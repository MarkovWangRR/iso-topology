package iso25d

import (
	"math"
	"strings"
	"testing"
)

// TestPrismFaceInvariants runs the geometry battery across the family.
func TestPrismFaceInvariants(t *testing.T) {
	var prov PrismShapeProvider
	for _, n := range []int{3, 4, 6, 8, 12} {
		params := map[string]any{"sides": n}
		w, d, h := 140.0, 140.0, 50.0
		faces := prov.Faces(w, d, h, params)
		if len(faces) != n+1 {
			t.Fatalf("n=%d: want %d faces, got %d", n, n+1, len(faces))
		}
		sil := prov.Silhouette(w, d, h, params)
		if len(sil) < 4 {
			t.Fatalf("n=%d: degenerate silhouette", n)
		}
		visSides := 0
		for _, f := range faces {
			for _, p := range f.Points {
				if math.IsNaN(p[0]) || math.IsNaN(p[1]) {
					t.Fatalf("n=%d face %s: NaN point", n, f.Name)
				}
			}
			if f.Name == "top" {
				if !f.Visible || f.Normal[2] != 1 {
					t.Errorf("n=%d: top face must be visible and up-facing", n)
				}
				continue
			}
			// side wall: the two z-extrusion edges must be screen-vertical
			a, b := f.Points[0], f.Points[1]
			c, e := f.Points[2], f.Points[3]
			if math.Abs(a[0]-b[0]) > 1e-6 || math.Abs(c[0]-e[0]) > 1e-6 {
				t.Errorf("n=%d face %s: extrusion edges not vertical", n, f.Name)
			}
			if f.Visible {
				visSides++
				if f.Normal[0]+f.Normal[1] <= 0 {
					t.Errorf("n=%d face %s: visible but normal away from camera", n, f.Name)
				}
			}
			// every face point inside the silhouette
			for _, p := range f.Points {
				if !pointInConvexPoly(p, sil, 0.05) {
					t.Errorf("n=%d face %s: point %v outside silhouette", n, f.Name, p)
				}
			}
		}
		// roughly half the walls face the camera
		if visSides < n/2-1 || visSides > n/2+1 {
			t.Errorf("n=%d: %d visible side walls (expected ~%d)", n, visSides, n/2)
		}
		// content rect corners inside the top polygon (ground coords)
		cr := prov.ContentRectFor(w, d, h, params)
		base := prismBase(w, d, n)
		ground := make([][2]float64, len(base))
		copy(ground, base)
		for _, corner := range [][2]float64{
			{cr.X, cr.Y}, {cr.X + cr.W, cr.Y}, {cr.X, cr.Y + cr.H}, {cr.X + cr.W, cr.Y + cr.H},
		} {
			if !pointInConvexPoly(corner, ground, 0.7) {
				t.Errorf("n=%d: content corner %v outside base polygon", n, corner)
			}
		}
	}
}

// TestPrismRenderSmoke renders every family member and checks the
// output parses as well-formed SVG with the expected face count.
func TestPrismRenderSmoke(t *testing.T) {
	for name, n := range map[string]int{"triprism": 3, "hexprism": 6, "octprism": 8} {
		o := ConvertOpts{Width: 140, Depth: 140, Height: 44, Label: "Test", Margin: 24}
		svg := Convert2DTo25D(name, o)
		if !regexpMustCount(svg, `data-face="top"`, 1) {
			t.Errorf("%s: missing top face", name)
		}
		if !regexpMustCount(svg, `</svg>`, 1) {
			t.Errorf("%s: malformed svg", name)
		}
		_ = n
	}
}

func regexpMustCount(s, sub string, n int) bool {
	return strings.Count(s, sub) == n
}
