package isotopo

import (
	"strings"
	"testing"
)

func lintSrc(t *testing.T, src string) []Issue {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return VisualLint(doc)
}

func hasCollision(issues []Issue, idA, idB string) bool {
	for _, i := range issues {
		if strings.Contains(i.Message, "overlapping space") &&
			strings.Contains(i.Message, `"`+idA+`"`) && strings.Contains(i.Message, `"`+idB+`"`) {
			return true
		}
	}
	return false
}

// Two groups pushed onto the same spot: their inner atomic parts collide across
// containers — invisible to the same-container sibling check, caught here.
func TestVisualLintCrossContainerCollision(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    parts:
      - id: g1
        shape: group
        offset: { wx: 0, wy: 0 }
        parts:
          - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }
      - id: g2
        shape: group
        offset: { wx: 10, wy: 10 }
        parts:
          - { id: b, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }
`
	if !hasCollision(lintSrc(t, src), "a", "b") {
		t.Errorf("expected a/b cross-container collision warning, got %+v", lintSrc(t, src))
	}
}

// A well-separated two-group scene must stay silent (no crying wolf).
func TestVisualLintCleanSceneSilent(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    layout: { mode: row, gap: 2 }
    parts:
      - id: g1
        shape: group
        layout: { mode: column }
        parts:
          - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }
      - id: g2
        shape: group
        layout: { mode: column }
        parts:
          - { id: b, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }
`
	if issues := lintSrc(t, src); len(issues) != 0 {
		t.Errorf("clean scene should produce no lint, got %+v", issues)
	}
}

// Stack replicas share z by design and must never be flagged.
func TestVisualLintIgnoresStackReplicas(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: pods, shape: rectangle, geom: { w: 80, d: 80, h: 20 }, stack: { count: 3, gap: 6 } }
`
	for _, i := range lintSrc(t, src) {
		if strings.Contains(i.Message, "overlapping space") {
			t.Errorf("stack replicas wrongly flagged: %s", i.Message)
		}
	}
}
