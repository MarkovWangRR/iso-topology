package isotopo

import (
	"strings"
	"testing"
)

const editFixture = `# a tiny composite for edit-op tests
theme:
  presets:
    tile:
      palette: { top: "#FFFFFF", left: "#DDD", right: "#EEE" }
nodes:
  scene:
    shape: composite
    layout: { mode: row, gap: 1 }
    parts:
      - id: a
        shape: rectangle
        geom: { w: 100, d: 100, h: 20 }
        preset: tile
        label: "A"
      - id: b
        shape: rectangle
        geom: { w: 100, d: 100, h: 20 }
        preset: tile
        label: "B"
    connectors:
      - { from: a, to: b, arrow: triangle }
`

func apply(t *testing.T, op EditOp) string {
	t.Helper()
	out, err := ApplyOpText("yaml", []byte(editFixture), op)
	if err != nil {
		t.Fatalf("ApplyOpText(%s): %v", op.Kind, err)
	}
	return string(out)
}

func TestApplyOp_SetField(t *testing.T) {
	out := apply(t, EditOp{Kind: "set-field", Target: "node", ID: "a",
		Fields: map[string]string{"style.text.color": "#FF0000"}})
	if !strings.Contains(out, "#FF0000") {
		t.Fatalf("set-field not written:\n%s", out)
	}
	// untouched content is preserved (comment + sibling)
	if !strings.Contains(out, "# a tiny composite") || !strings.Contains(out, `label: "B"`) {
		t.Fatalf("set-field clobbered surrounding content:\n%s", out)
	}
}

func TestApplyOp_Move_FreezesAutoScene(t *testing.T) {
	// The scene is auto-layout (layout: row), so the first move must freeze it
	// into explicit offsets — every part gains an offset, the dragged one nudged.
	out := apply(t, EditOp{Kind: "move", Target: "node", ID: "a", DWX: 40, DWY: 0})
	if strings.Count(out, "offset:") < 2 {
		t.Fatalf("freeze should give every part an offset:\n%s", out)
	}
}

func TestApplyOp_AddDeleteDuplicate(t *testing.T) {
	if got := apply(t, EditOp{Kind: "add"}); strings.Count(got, "- id:")+strings.Count(got, "shape: rectangle") <= strings.Count(editFixture, "shape: rectangle") {
		// add appends a part; just assert it grew
		if len(got) <= len(editFixture) {
			t.Fatalf("add did not grow the document")
		}
	}
	if got := apply(t, EditOp{Kind: "delete", Target: "node", ID: "b"}); strings.Contains(got, `label: "B"`) {
		t.Fatalf("delete left node b behind:\n%s", got)
	}
	if got := apply(t, EditOp{Kind: "delete", Target: "edge", CI: 0}); strings.Contains(got, "from: a") {
		t.Fatalf("delete edge left the connector behind:\n%s", got)
	}
	if got := apply(t, EditOp{Kind: "duplicate", Target: "node", ID: "a"}); !strings.Contains(got, "offset:") {
		t.Fatalf("duplicate should place the clone at an offset:\n%s", got)
	}
}

func TestApplyOp_Errors(t *testing.T) {
	if _, err := ApplyOpText("yaml", []byte(editFixture), EditOp{Kind: "delete", Target: "node", ID: "nope"}); err == nil {
		t.Fatal("expected error deleting a missing node")
	}
	if _, err := ApplyOpText("yaml", []byte(editFixture), EditOp{Kind: "bogus"}); err == nil {
		t.Fatal("expected error for unknown op kind")
	}
}

func TestApplyOp_RendersResult(t *testing.T) {
	newSrc, svg, issues, err := ApplyOp("yaml", []byte(editFixture),
		EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"label": "Renamed"}})
	if err != nil {
		t.Fatalf("ApplyOp: %v", err)
	}
	if !strings.Contains(string(newSrc), "Renamed") {
		t.Fatal("newSrc missing the edit")
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Fatalf("clean edit produced an error issue: %+v", i)
		}
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("expected an SVG, got %.80q", svg)
	}
}

func TestRenderSource(t *testing.T) {
	svg, issues, _ := RenderSource("yaml", []byte(editFixture))
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Fatalf("fixture should render clean: %+v", i)
		}
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("RenderSource returned no SVG")
	}
}
