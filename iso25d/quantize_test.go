package iso25d

import (
	"math"
	"testing"
)

// TestProjectQuantized guards the cross-toolchain determinism fix: projected
// coordinates must land exactly on the 1e-6 quantum grid, so a sub-ULP
// difference in how a Go release compiles a*b+c (FMA contraction) can never
// flip a downstream threshold decision and drift the goldens. A perturbation
// of 1 ULP on the input must produce the identical quantized output.
func TestProjectQuantized(t *testing.T) {
	pts := [][3]float64{
		{0, 0, 0}, {140, 140, 80}, {66.6, 60.1, 18}, {1234.5, 987.6, 44.4},
	}
	for _, p := range pts {
		x, y := project(p[0], p[1], p[2])
		for _, v := range []float64{x, y} {
			q := math.Round(v*projQuantum) / projQuantum
			if v != q {
				t.Errorf("project(%v) = %v not on the 1e-6 quantum grid", p, v)
			}
		}
		// Simulated toolchain noise: nudge inputs by 1 ULP; quantized output
		// must be bit-identical.
		nx, ny := project(math.Nextafter(p[0], math.Inf(1)), p[1], p[2])
		if nx != x || ny != y {
			t.Errorf("project not stable under 1-ULP input perturbation at %v: (%v,%v) vs (%v,%v)", p, x, y, nx, ny)
		}
	}
}
