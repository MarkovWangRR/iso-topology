package isotopo

import "testing"

// The screen-space text contract: never cross or touch a part's
// projection (clearance enforced), prefer the legacy spot when it is
// clean, and fall outward — toward the picture's periphery — when it
// is not.

func TestPlaceTextKeepsCleanSpot(t *testing.T) {
	anchor := screenRect{100, 100, 200, 180}
	x, y := placeTextBox(60, 20, anchor, 120, 190, 150, 140, nil)
	if x != 120 || y != 190 {
		t.Fatalf("clean preferred spot must be kept, got (%v, %v)", x, y)
	}
}

func TestPlaceTextAvoidsCrossingAndTouching(t *testing.T) {
	anchor := screenRect{100, 100, 200, 180}
	// An obstacle sits exactly where the preferred spot is.
	blocker := screenRect{100, 185, 220, 230}
	x, y := placeTextBox(60, 20, anchor, 120, 190, 150, 140, []screenRect{blocker})
	box := screenRect{x, y, x + 60, y + 20}
	if box.intersects(blocker.inflate(textClearance)) {
		t.Fatalf("placed box still touches the obstacle: %+v", box)
	}
	if box.intersects(anchor.inflate(textClearance)) {
		t.Fatalf("placed box touches its own anchor: %+v", box)
	}
}

func TestPlaceTextStaysInPreferredDirectionFirst(t *testing.T) {
	// Below is blocked NEAR the anchor only → consistency wins: the
	// label escalates distance WITHIN "below" rather than switching
	// sides.
	anchor := screenRect{0, 100, 100, 180}
	blockNear := screenRect{-50, 182, 150, 230}
	x, y := placeTextBox(60, 20, anchor, 20, 190, 400, 140, []screenRect{blockNear})
	if y <= anchor.y1 {
		t.Fatalf("label should remain below, got y=%v", y)
	}
	if x != anchor.cx()-30 {
		t.Fatalf("below placement should stay centred, got x=%v", x)
	}
}

func TestPlaceTextFallsOutward(t *testing.T) {
	// Anchor on the far LEFT of the scene; below is blocked at EVERY
	// distance → the next candidate must be the leftward (peripheral)
	// side, not rightward into the scene.
	anchor := screenRect{0, 100, 100, 180}
	blockBelow := screenRect{-200, 182, 300, 600}
	x, _ := placeTextBox(60, 20, anchor, 20, 190, 400, 140, []screenRect{blockBelow})
	if x >= anchor.x0 {
		t.Fatalf("text should fall outward (left), got x=%v", x)
	}
}

func TestLeaderStartsOnSilhouetteEdge(t *testing.T) {
	anchor := screenRect{100, 100, 200, 200}
	// Box below the node: leader must start on the BOTTOM edge, never
	// inside the body.
	lx, ly := leaderStart(anchor, 150, 300)
	if ly != anchor.y1 || lx != 150 {
		t.Fatalf("leader should start at bottom edge centre, got (%v, %v)", lx, ly)
	}
	// Box to the right: starts on the right edge.
	lx, ly = leaderStart(anchor, 350, 150)
	if lx != anchor.x1 || ly != 150 {
		t.Fatalf("leader should start at right edge, got (%v, %v)", lx, ly)
	}
}

func TestPlaceTextStackedLabelsDontCollide(t *testing.T) {
	anchor := screenRect{100, 100, 200, 180}
	first := screenRect{120, 190, 180, 210}
	x, y := placeTextBox(60, 20, anchor, 120, 190, 150, 300, []screenRect{first})
	box := screenRect{x, y, x + 60, y + 20}
	if box.intersects(first.inflate(textClearance)) {
		t.Fatalf("second label overlaps the first: %+v", box)
	}
}
