package main

import (
	"testing"

	isotopo "github.com/MarkovWangRR/iso-topology"
)

// acceptedShapeTokens is the union of every iso shape name and accepted alias
// the renderer understands — the single source of truth in capabilities.
func acceptedShapeTokens() map[string]bool {
	set := map[string]bool{}
	for _, s := range isotopo.CapabilityReport().Shapes {
		set[s.IsoName] = true
		for _, a := range s.AcceptedAs {
			set[a] = true
		}
	}
	return set
}

// TestShapeOptionsAreReal guards the Studio shape picker against the
// box/sphere/polygon desync class: every option the picker offers must be a
// token the renderer actually accepts, or it would silently fall back to
// rectangle. This couples the hand-maintained shapeOptions list to the
// capability report so a bad token fails CI instead of shipping.
func TestShapeOptionsAreReal(t *testing.T) {
	ok := acceptedShapeTokens()
	for _, s := range shapeOptions {
		if !ok[s] {
			t.Errorf("shapeOptions has %q which is not an accepted shape token "+
				"(see `isotopo capabilities` → shapes); it would silently render as rectangle", s)
		}
	}
}

// TestShapeClassesKnown ensures every offered shape resolves to a real colour
// class (not the catch-all), so the detail editor offers the right controls.
func TestShapeClassesKnown(t *testing.T) {
	want := map[string]bool{"faces": true, "outline": true, "text": true, "fill": true}
	for _, s := range shapeOptions {
		if !want[shapeClass(s)] {
			t.Errorf("shapeClass(%q) = %q, not a known class", s, shapeClass(s))
		}
	}
}
