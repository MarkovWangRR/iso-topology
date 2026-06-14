package main

import (
	"strings"
	"testing"

	isotopo "github.com/MarkovWangRR/iso-topology"
	"github.com/MarkovWangRR/iso-topology/internal/yamledit"
)

// TestDeleteContainerCleansRefs locks the E2E-found bugs: deleting a container
// node must also drop everything that referenced its (now-gone) nested parts —
// connectors AND annotations — or the scene fails validation with a dangling
// reference.
func TestDeleteContainerCleansRefs(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    parts:
      - id: grp
        shape: group
        parts:
          - { id: a, shape: rectangle, label: "A" }
          - { id: b, shape: cylinder, label: "B" }
      - { id: c, shape: rectangle, label: "C" }
    connectors:
      - { from: c, to: a }
      - { from: a, to: b }
annotations:
  - { anchor: b, text: "note" }
`
	out, ok := yamledit.DeletePart(src, "grp")
	if !ok {
		t.Fatal("DeletePart(grp) returned not-ok")
	}
	for _, bad := range []string{"id: a", "id: b", "to: a", "from: a", "anchor: b"} {
		if strings.Contains(out, bad) {
			t.Errorf("after deleting container grp, leftover %q (dangling ref):\n%s", bad, out)
		}
	}
	if !strings.Contains(out, "id: c") {
		t.Error("sibling c should survive deleting grp")
	}
}

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

// TestShapeOptionsAreReal pins the invariant the box/sphere/polygon bug
// violated: every option the Studio shape picker offers must be a token the
// renderer accepts. shapeOptions() is now DERIVED from the capability report
// so this holds by construction — the test guards against anyone re-hardcoding
// the list or the alias mapping (iso_text→text) breaking.
func TestShapeOptionsAreReal(t *testing.T) {
	ok := acceptedShapeTokens()
	for _, s := range shapeOptions() {
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
	for _, s := range shapeOptions() {
		if !want[shapeClass(s)] {
			t.Errorf("shapeClass(%q) = %q, not a known class", s, shapeClass(s))
		}
	}
}
