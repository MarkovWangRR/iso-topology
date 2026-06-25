package isotopo

import (
	"strings"
	"testing"
)

func contrastMsgs(t *testing.T, src string) []string {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []string
	for _, i := range VisualContrastIssues(doc) {
		if strings.Contains(i.Message, "low contrast against group fill") {
			out = append(out, i.Message)
		}
	}
	return out
}

// A near-white child on a white tray that separates via a drop shadow reads
// fine — the contrast lint must NOT flag it (the premium white-card case).
func TestContrast_ShadowSeparatedChildSilent(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        style: { palette: { top: "#FFFFFF" } }
        parts:
          - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 20 },
              style: { palette: { top: "#FCFCFC" }, effects: { dropShadow: { dx: 0, dy: 8, blur: 12, color: "#00000022" } } } }
`
	if msgs := contrastMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("a shadow-separated child must not be flagged, got %v", msgs)
	}
}

// A border (stroke) is also a separation channel — silent.
func TestContrast_BorderSeparatedChildSilent(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        style: { palette: { top: "#FFFFFF" } }
        parts:
          - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 20 },
              style: { palette: { top: "#FCFCFC" }, stroke: { color: "#C9CDD3", width: 1 } } }
`
	if msgs := contrastMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("a border-separated child must not be flagged, got %v", msgs)
	}
}

// No separation channel at all (no border, no shadow) → it genuinely melts → warn.
func TestContrast_BareLowContrastChildWarns(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        style: { palette: { top: "#FFFFFF" } }
        parts:
          - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 20 }, style: { palette: { top: "#FCFCFC" } } }
`
	msgs := contrastMsgs(t, src)
	if len(msgs) == 0 || !strings.Contains(msgs[0], "no border/shadow") {
		t.Fatalf("a borderless low-contrast child must warn, got %v", msgs)
	}
}
