package isotopo

import (
	"strings"
	"testing"
)

func fitMsgs(t *testing.T, src string) []string {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []string
	for _, i := range ContainerFitIssues(doc) {
		out = append(out, i.Message)
	}
	return out
}

// A cloud authored small (124×86) renders at its 200×140 floor; inside a fixed
// 180-wide group that bursts the slab and must warn using the EFFECTIVE width.
func TestContainerFit_CloudBurstsFixedGroup(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        geom: { w: 180, d: 160, h: 8 }
        parts:
          - { id: saas, shape: cloud, geom: { w: 124, d: 86, h: 46 } }
`
	msgs := fitMsgs(t, src)
	hit := false
	for _, m := range msgs {
		if strings.Contains(m, `"saas"`) && strings.Contains(m, `"g"`) && strings.Contains(m, "bursts the slab") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("a cloud's effective 200 width must burst a fixed 180 group, got %v", msgs)
	}
}

// A child comfortably inside a fixed group stays silent.
func TestContainerFit_RoomyIsSilent(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        geom: { w: 400, d: 300, h: 8 }
        parts:
          - { id: a, shape: rectangle, geom: { w: 120, d: 80, h: 24 } }
`
	if msgs := fitMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("a roomy fixed group must stay silent, got %v", msgs)
	}
}

// An AUTO-sized group (no geom.w/d) grows to fit, so it is never "burst".
func TestContainerFit_AutoGroupNeverBursts(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        layout: { mode: row, gap: 0.8 }
        parts:
          - { id: saas, shape: cloud, geom: { w: 124, d: 86, h: 46 } }
`
	if msgs := fitMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("an auto-sized group must never report a burst, got %v", msgs)
	}
}
