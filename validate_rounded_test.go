package isotopo

import (
	"strings"
	"testing"
)

func roundedMsgs(t *testing.T, src string) []string {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []string
	for _, i := range RoundedSideIgnoredIssues(doc) {
		out = append(out, i.Message)
	}
	return out
}

// A default-rounded group with a wildly different solid right is the footgun.
func TestRoundedSide_DistinctRightWarns(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        style: { palette: { top: "#E4761B", left: "#00FF00", right: "#0000FF" } }
        parts:
          - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 20 } }
`
	msgs := roundedMsgs(t, src)
	if len(msgs) == 0 || !strings.Contains(msgs[0], "#0000FF") || !strings.Contains(msgs[0], "ignored on a rounded") {
		t.Fatalf("a distinct solid right on a rounded group must warn, got %v", msgs)
	}
}

// Normal iso shading (right a shade off left) renders fine as the band — silent.
func TestRoundedSide_ShadingIsSilent(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        style: { palette: { top: "#8C7DF2", left: "#332A93", right: "#5747CE" } }
        parts:
          - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 20 } }
`
	if msgs := roundedMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("normal same-hue shading must stay silent, got %v", msgs)
	}
}

// A SHARP node (cornerRadius 0) renders both faces — distinct right is fine.
func TestRoundedSide_SharpNodeIsSilent(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, style: { palette: { top: "#E4761B", left: "#00FF00", right: "#0000FF" }, effects: { cornerRadius: 0 } } }
`
	if msgs := roundedMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("a sharp node renders both side faces; must stay silent, got %v", msgs)
	}
}

// No right set → nothing dropped → silent.
func TestRoundedSide_NoRightIsSilent(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        style: { palette: { top: "#E4761B" } }
        parts:
          - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 20 } }
`
	if msgs := roundedMsgs(t, src); len(msgs) != 0 {
		t.Fatalf("no explicit right means nothing is dropped; must stay silent, got %v", msgs)
	}
}
