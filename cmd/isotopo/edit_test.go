package main

import (
	"strings"
	"testing"
)

// The Studio write-back is deliberate line/regex surgery (NOT a yaml.Node
// round-trip, which would reflow the whole document — violating the "never
// reflow the user's YAML" principle). These tests lock that surgery's two
// invariants so the ~dozen helpers can't silently regress:
//   1. an edit touches ONLY the lines it must — every other line, and every
//      comment, is preserved byte-for-byte.
//   2. the edit produces the intended value in both flow and block forms.

const editDoc = `canvas: { background: "#FFFFFF", grid: iso }   # canvas comment
nodes:
  scene:
    shape: composite
    layout: { mode: auto }   # keep me
    parts:
      - { id: a, shape: rectangle, geom: { w: 90, d: 90, h: 30 }, label: "A" }   # flow part
      - id: b
        shape: cylinder        # block part
        geom: { w: 84, d: 84, h: 50 }
        label: "B"
    connectors:
      - { from: a, to: b, arrow: triangle }
`

// onlyChangedLines returns the line numbers whose content differs.
func onlyChangedLines(a, b string) []int {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	var diff []int
	for i := 0; i < len(la) || i < len(lb); i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			diff = append(diff, i)
		}
	}
	return diff
}

func lineContaining(src, needle string) int {
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, needle) {
			return i
		}
	}
	return -1
}

func TestEditScalarTouchesOneLine(t *testing.T) {
	// change block part b's label — only b's label line may change.
	out, ok := setField(editDoc, findPartIDLine(editDoc, "b"), []string{"label"}, "Bee")
	if !ok {
		t.Fatal("setField not ok")
	}
	// setField writes the scalar bare (no quotes) — locking the real contract.
	if !strings.Contains(out, "label: Bee") {
		t.Fatalf("label not written:\n%s", out)
	}
	changed := onlyChangedLines(editDoc, out)
	want := lineContaining(editDoc, `label: "B"`)
	if len(changed) != 1 || changed[0] != want {
		t.Errorf("expected only line %d to change, got %v", want, changed)
	}
	// every comment survived
	for _, c := range []string{"# canvas comment", "# keep me", "# flow part", "# block part"} {
		if !strings.Contains(out, c) {
			t.Errorf("comment %q lost", c)
		}
	}
}

func TestEditNestedFlowAndBlock(t *testing.T) {
	// flow form: set a deep nested value on part a inside its braces.
	out, _ := setField(editDoc, findPartIDLine(editDoc, "a"), []string{"style", "palette", "top"}, "#101010")
	if !strings.Contains(out, `style: { palette: { top: "#101010" } }`) {
		t.Errorf("flow nested create wrong:\n%s", firstPartLine(out, "a"))
	}
	if d := onlyChangedLines(editDoc, out); len(d) != 1 {
		t.Errorf("flow nested edit should touch one line, touched %v", d)
	}
	// block form: set geom.w on part b's inline geom child line.
	out2, _ := setField(editDoc, findPartIDLine(editDoc, "b"), []string{"geom", "w"}, "120")
	if !strings.Contains(out2, "geom: { w: 120, d: 84, h: 50 }") {
		t.Errorf("block nested edit wrong:\n%s", out2)
	}
}

func TestDeletePreservesNeighbours(t *testing.T) {
	out, ok := deletePart(editDoc, "a")
	if !ok {
		t.Fatal("deletePart not ok")
	}
	// a and the a→b connector are gone; b and all comments remain.
	if strings.Contains(out, "id: a") {
		t.Error("part a not removed")
	}
	if strings.Contains(out, "from: a") {
		t.Error("connector referencing a not removed")
	}
	for _, keep := range []string{"id: b", `label: "B"`, "# canvas comment", "# keep me", "# block part"} {
		if !strings.Contains(out, keep) {
			t.Errorf("delete clobbered %q:\n%s", keep, out)
		}
	}
}

func firstPartLine(src, id string) string {
	i := findPartIDLine(src, id)
	if i < 0 {
		return ""
	}
	return strings.Split(src, "\n")[i]
}
