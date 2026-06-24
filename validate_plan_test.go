package isotopo

import (
	"strings"
	"testing"
)

// planFootprintMsgs returns the plan-collision messages for a scene source.
func planFootprintMsgs(t *testing.T, src string) []string {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []string
	for _, i := range PlanFootprintIssues(doc) {
		out = append(out, i.Message)
	}
	return out
}

func hasMsg(msgs []string, subs ...string) bool {
	for _, m := range msgs {
		all := true
		for _, s := range subs {
			if !strings.Contains(m, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// Two sibling leaves overlapping in plan (one nudged onto the other) must warn.
func TestPlanFootprint_SiblingOverlap(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 100, d: 100, h: 30 } }
      - { id: b, shape: rectangle, geom: { w: 100, d: 100, h: 30 }, place: { rightOf: a, gap: 2 }, offset: { wx: -140 } }
`
	msgs := planFootprintMsgs(t, src)
	if !hasMsg(msgs, "overlap", `"a"`, `"b"`, "plan") {
		t.Fatalf("expected a plan overlap warning for a/b, got %v", msgs)
	}
}

// Footprints sharing exactly an edge (zero gap) must warn as a touch.
func TestPlanFootprint_Touch(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 100, d: 100, h: 30 } }
      - { id: b, shape: rectangle, geom: { w: 100, d: 100, h: 30 }, offset: { wx: 100, wy: 0 } }
`
	msgs := planFootprintMsgs(t, src)
	if !hasMsg(msgs, "touch", `"a"`, `"b"`) {
		t.Fatalf("expected a plan touch warning for a/b, got %v", msgs)
	}
}

// A child inside its own group is legitimate containment — never flagged.
func TestPlanFootprint_ContainmentAllowed(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        layout: { mode: column, gap: 0.8 }
        parts:
          - { id: c1, shape: rectangle, geom: { w: 100, d: 60, h: 20 } }
          - { id: c2, shape: rectangle, geom: { w: 100, d: 60, h: 20 } }
`
	msgs := planFootprintMsgs(t, src)
	if len(msgs) != 0 {
		t.Fatalf("a well-formed group must not flag containment, got %v", msgs)
	}
}

// Cleanly gapped siblings stay silent.
func TestPlanFootprint_CleanSiblings(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 100, d: 100, h: 30 } }
      - { id: b, shape: rectangle, geom: { w: 100, d: 100, h: 30 }, place: { rightOf: a, gap: 2 } }
`
	msgs := planFootprintMsgs(t, src)
	if len(msgs) != 0 {
		t.Fatalf("cleanly separated siblings must stay silent, got %v", msgs)
	}
}

// Two sibling GROUP slabs that overlap must warn, naming it a group footprint.
func TestPlanFootprint_GroupSlabsOverlap(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g1
        shape: group
        layout: { mode: column, gap: 0.6 }
        parts:
          - { id: x1, shape: rectangle, geom: { w: 120, d: 60, h: 20 } }
          - { id: x2, shape: rectangle, geom: { w: 120, d: 60, h: 20 } }
      - id: g2
        shape: group
        place: { rightOf: g1, gap: 2 }
        offset: { wx: -260 }
        layout: { mode: column, gap: 0.6 }
        parts:
          - { id: y1, shape: rectangle, geom: { w: 120, d: 60, h: 20 } }
          - { id: y2, shape: rectangle, geom: { w: 120, d: 60, h: 20 } }
`
	msgs := planFootprintMsgs(t, src)
	if !hasMsg(msgs, "footprints (group)", `"g1"`, `"g2"`) {
		t.Fatalf("expected a group-slab overlap warning for g1/g2, got %v", msgs)
	}
}
