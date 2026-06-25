package isotopo

import "testing"

// rectsOverlap must be DEPTH-AWARE: two parts that share an x,y footprint but sit
// on different floors (one placed `above` the other — a topper, a chip resting on
// a plate) are STACKED, not colliding. Before the z-band check they registered as
// phantom node-overlaps the layout could never resolve by nudging gaps — the #1
// source of the diagram agent's measure_layout thrash / timeouts.
func TestRectsOverlap_DepthAware(t *testing.T) {
	body := planRect{x: 0, y: 0, w: 100, d: 100, z: 0, h: 40}
	cases := []struct {
		name string
		b    planRect
		want bool
	}{
		{"same floor, overlapping footprint", planRect{x: 50, y: 50, w: 100, d: 100, z: 0, h: 40}, true},
		{"stacked on top (above): same footprint, higher floor", planRect{x: 0, y: 0, w: 100, d: 100, z: 40, h: 20}, false},
		{"flat decoration on top (h≈0)", planRect{x: 0, y: 0, w: 100, d: 100, z: 40, h: 0}, false},
		{"disjoint footprint", planRect{x: 200, y: 0, w: 100, d: 100, z: 0, h: 40}, false},
		{"genuinely interpenetrating (same floor + footprint)", planRect{x: 10, y: 10, w: 100, d: 100, z: 10, h: 40}, true},
	}
	for _, c := range cases {
		if got := rectsOverlap(body, c.b); got != c.want {
			t.Errorf("%s: rectsOverlap = %v, want %v", c.name, got, c.want)
		}
		// symmetry
		if got := rectsOverlap(c.b, body); got != c.want {
			t.Errorf("%s (swapped): rectsOverlap = %v, want %v", c.name, got, c.want)
		}
	}
}
