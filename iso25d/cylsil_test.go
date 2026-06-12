package iso25d

import (
	"math"
	"testing"
)

func TestCylinderSilhouette(t *testing.T) {
	var prov CylinderShapeProvider
	w, d, h := 120.0, 120.0, 70.0
	sil := prov.Silhouette(w, d, h, nil)
	if len(sil) < 10 {
		t.Fatalf("degenerate silhouette: %d pts", len(sil))
	}
	rx := w / 2
	ry := rx * sin30 / cos30
	cx := (w-d)/2*cos30 + d*cos30
	cyTop := (w + d) / 2 * sin30
	// the capsule must contain both ellipse centres and respect width
	for _, p := range [][2]float64{{cx, cyTop}, {cx, cyTop + h}} {
		if !pointInConvexPoly(p, sil, 0.1) {
			t.Errorf("centre %v outside silhouette", p)
		}
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range sil {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	if math.Abs((maxX-minX)-w) > 0.5 {
		t.Errorf("capsule width %.1f, want %.1f", maxX-minX, w)
	}
	if math.Abs((maxY-minY)-(h+2*ry)) > 0.5 {
		t.Errorf("capsule height %.1f, want %.1f", maxY-minY, h+2*ry)
	}
}
