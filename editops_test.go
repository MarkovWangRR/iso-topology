package isotopo

import (
	"context"
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

func TestFields_NodeSchemaValuesAndHints(t *testing.T) {
	fs, err := Fields("yaml", []byte(editFixture), "node", "a", 0)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	get := func(path string) (Field, bool) {
		for _, f := range fs {
			if f.Path == path {
				return f, true
			}
		}
		return Field{}, false
	}
	if f, ok := get("label"); !ok || f.Value != "A" {
		t.Fatalf("label field: got %+v ok=%v (want Value A)", f, ok)
	}
	// node "a" inherits palette.top from preset "tile" (#FFFFFF) — surfaced as Eff.
	if f, ok := get("style.palette.top"); !ok || f.Value != "" || f.Eff == "" {
		t.Fatalf("palette.top: want empty Value + non-empty Eff (inherited), got %+v ok=%v", f, ok)
	}
	if _, ok := get("@iconColor"); !ok {
		t.Fatal("node schema must include the synthetic @iconColor field")
	}
	// genuinely-unset positional field gets an Empty placeholder, not a blank.
	if f, ok := get("offset.wx"); !ok || f.Empty == "" {
		t.Fatalf("offset.wx should carry an Empty placeholder, got %+v ok=%v", f, ok)
	}
}

func TestFields_Errors(t *testing.T) {
	if _, err := Fields("yaml", []byte(editFixture), "node", "nope", 0); err == nil {
		t.Fatal("expected error for a missing node id")
	}
	if _, err := Fields("yaml", []byte(editFixture), "bogus", "", 0); err == nil {
		t.Fatal("expected error for an unknown kind")
	}
}

const iconFixture = `# icon fixture
nodes:
  scene:
    shape: composite
    parts:
      - {id: cpu, shape: rectangle, geom: {w: 100, d: 100, h: 20}, icon: "iso://glyph/cpu", label: CPU}
`

const containerFixture = `# a lane (group) wrapping two leaf parts
nodes:
  scene:
    shape: composite
    parts:
      - id: lane
        shape: group
        label: "Lane"
        parts:
          - { id: x, shape: rectangle, geom: { w: 100, d: 100, h: 20 }, label: X }
          - { id: y, shape: rectangle, geom: { w: 100, d: 100, h: 20 }, label: Y }
`

func TestApplyOp_DuplicateContainerBlocked(t *testing.T) {
	// Duplicating a container used to clone its children with colliding ids;
	// the guard must refuse it. A leaf child still duplicates fine.
	if _, err := ApplyOpText("yaml", []byte(containerFixture),
		EditOp{Kind: "duplicate", Target: "node", ID: "lane"}); err == nil {
		t.Fatal("expected duplicate of a container to be blocked")
	}
	out, err := ApplyOpText("yaml", []byte(containerFixture),
		EditOp{Kind: "duplicate", Target: "node", ID: "x"})
	if err != nil {
		t.Fatalf("duplicate of a leaf child should succeed: %v", err)
	}
	if !strings.Contains(string(out), "x_copy") {
		t.Fatalf("leaf duplicate produced no clone:\n%s", out)
	}
}

func TestApplyOp_DeleteContainerBlocked(t *testing.T) {
	// Deleting a container would wipe a whole lane in one click — blocked.
	// Its leaf children delete normally.
	if _, err := ApplyOpText("yaml", []byte(containerFixture),
		EditOp{Kind: "delete", Target: "node", ID: "lane"}); err == nil {
		t.Fatal("expected delete of a container to be blocked")
	}
	out, err := ApplyOpText("yaml", []byte(containerFixture),
		EditOp{Kind: "delete", Target: "node", ID: "x"})
	if err != nil {
		t.Fatalf("delete of a leaf child should succeed: %v", err)
	}
	if strings.Contains(string(out), "id: x,") || strings.Contains(string(out), "label: X") {
		t.Fatalf("leaf child x not removed:\n%s", out)
	}
}

func TestValidate_DuplicatePartID(t *testing.T) {
	dup := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 10, d: 10, h: 10 } }
      - { id: a, shape: rectangle, geom: { w: 10, d: 10, h: 10 } }
`
	doc, err := LoadInput(context.Background(), "yaml", []byte(dup), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var found bool
	for _, i := range Validate(doc) {
		if i.Severity == SeverityError && strings.Contains(i.Message, "duplicate part id") {
			found = true
		}
	}
	if !found {
		t.Fatal("validator did not flag the duplicate part id")
	}
}

const rerouteFixture = `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 70, d: 70, h: 30 }, offset: { wx: 0,   wy: 0 } }
      - { id: c, shape: rectangle, geom: { w: 70, d: 70, h: 30 }, offset: { wx: 200, wy: 0 } }
      - { id: b, shape: rectangle, geom: { w: 70, d: 70, h: 30 }, offset: { wx: 400, wy: 0 } }
    connectors:
      - { from: a, to: b }
`

func TestApplyOp_MoveRoutesEdgeAroundObstacle(t *testing.T) {
	// a→b runs straight through c; moving a must reroute the edge around it
	// (orthogonal + waypoints), the Studio drag → auto-route behaviour.
	out, err := ApplyOpText("yaml", []byte(rerouteFixture),
		EditOp{Kind: "move", Target: "node", ID: "a", DWX: 0, DWY: 0})
	if err != nil {
		t.Fatalf("ApplyOpText move: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "routing: orthogonal") || !strings.Contains(s, "waypoints:") {
		t.Fatalf("expected the a→b edge to be rerouted around c:\n%s", s)
	}
}

func TestApplyOp_MoveLeavesClearEdgeStraight(t *testing.T) {
	// With nothing in the way, a move must NOT rewrite the edge's routing.
	clear := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 70, d: 70, h: 30 }, offset: { wx: 0, wy: 0 } }
      - { id: b, shape: rectangle, geom: { w: 70, d: 70, h: 30 }, offset: { wx: 300, wy: 0 } }
    connectors:
      - { from: a, to: b }
`
	out, err := ApplyOpText("yaml", []byte(clear),
		EditOp{Kind: "move", Target: "node", ID: "a", DWX: 10, DWY: 0})
	if err != nil {
		t.Fatalf("ApplyOpText move: %v", err)
	}
	if strings.Contains(string(out), "routing: orthogonal") || strings.Contains(string(out), "waypoints:") {
		t.Fatalf("a clear edge must stay straight after a move:\n%s", out)
	}
}

func TestApplyOp_IconColorTint(t *testing.T) {
	// set-field with the synthetic @iconColor must splice the tint into the icon
	// ref suffix, headless — and preserve comments.
	outB, err := ApplyOpText("yaml", []byte(iconFixture), EditOp{Kind: "set-field", Target: "node", ID: "cpu",
		Fields: map[string]string{"@iconColor": "#33C1FF"}})
	if err != nil {
		t.Fatalf("ApplyOpText: %v", err)
	}
	out := string(outB)
	if !strings.Contains(out, "iso://glyph/cpu/33C1FF") {
		t.Fatalf("@iconColor not spliced into the icon ref:\n%s", out)
	}
	if !strings.Contains(out, "# icon fixture") {
		t.Fatalf("comment not preserved:\n%s", out)
	}
}
