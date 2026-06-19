package isotopo

import (
	"context"
	"regexp"
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

func TestApplyOp_MoveNestedNodeInLayoutGroup(t *testing.T) {
	// Dragging a node nested in a row-layout group must actually move it: the
	// group's layout is frozen and the child gets an explicit, nudged offset.
	// (Regression: the old root-only freeze left nested nodes stuck in place.)
	src := `nodes:
  scene:
    shape: composite
    parts:
      - id: lane
        shape: group
        label: Lane
        layout: { mode: row, gap: 1 }
        parts:
          - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 20 }, label: A }
          - { id: b, shape: rectangle, geom: { w: 80, d: 80, h: 20 }, label: B }
`
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "b", DWX: -40, DWY: -70})
	if err != nil {
		t.Fatalf("move nested: %v", err)
	}
	s := string(out)
	// lane's row layout is frozen, and b now carries its own offset.
	laneBlock := s[strings.Index(s, "id: lane"):]
	if strings.Contains(laneBlock[:strings.Index(laneBlock, "id: a")], "layout:") {
		t.Fatalf("lane layout should be frozen after a child drag:\n%s", s)
	}
	bLine := ""
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, "id: b") {
			bLine = l
		}
	}
	if !strings.Contains(bLine, "offset") {
		t.Fatalf("nested node b should have gained an offset:\n%s", s)
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

const nestFixture = `nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        label: G
        parts:
          - { id: x, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, offset: { wx: 0, wy: 0 }, label: X }
          - { id: y, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, label: Y }
`

func TestApplyOp_ReparentOutAndBack(t *testing.T) {
	// x out of g → scene root (sibling of g), then back into g.
	toRoot, err := ApplyOpText("yaml", []byte(nestFixture), EditOp{Kind: "reparent", ID: "x", Target: ""})
	if err != nil {
		t.Fatalf("reparent to root: %v", err)
	}
	root := string(toRoot)
	// x must now sit at the scene-parts indent (6 spaces), not g's child indent.
	// A position-preserving reparent prepends a re-homed `offset:` ahead of the
	// id, so match the item by indent + id rather than a fixed key order.
	if indentOfItemWithID(root, "x") != 6 {
		t.Fatalf("x should be a scene-root part (6-space indent) after reparent:\n%s", root)
	}
	back, err := ApplyOpText("yaml", toRoot, EditOp{Kind: "reparent", ID: "x", Target: "g"})
	if err != nil {
		t.Fatalf("reparent back into g: %v", err)
	}
	if indentOfItemWithID(string(back), "x") != 10 {
		t.Fatalf("x should be g's child again (10-space indent):\n%s", string(back))
	}
}

// indentOfItemWithID returns the leading-space count of the `- { ... id: <id> ... }`
// flow-list item carrying the given id, or -1 if not found. Key-order independent.
func indentOfItemWithID(src, id string) int {
	for _, ln := range strings.Split(src, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "- {") && strings.Contains(t, "id: "+id+",") {
			return len(ln) - len(strings.TrimLeft(ln, " "))
		}
	}
	return -1
}

// TestApplyOp_ReparentPreservesPosition guards the "node flies away" fix: a
// reparent out of a manual group (and back) must NOT snap the node to the
// connectivity-driven auto-layout spot. It re-homes the node's exact world
// position as an explicit offset, so its rendered screen coordinates are
// unchanged across the round-trip.
func TestApplyOp_ReparentPreservesPosition(t *testing.T) {
	const src = `nodes:
  scene:
    shape: composite
    parts:
      - id: vpc
        shape: group
        geom: { w: 460, d: 240, h: 6 }
        parts:
          - { id: db, shape: cylinder, geom: { w: 100, d: 100, h: 40 }, offset: { wx: 300, wy: 30 }, label: DB }
    connectors:
      - { from: db, to: db }
`
	posOf := func(yamlSrc, id string) (string, bool) {
		svg, issues, _ := RenderSource("yaml", []byte(yamlSrc))
		for _, i := range issues {
			if i.Severity == SeverityError {
				t.Fatalf("render error: %s", i.Message)
			}
		}
		re := regexp.MustCompile(`data-part-id="` + id + `"[^>]*transform="translate\(([^)]*)\)`)
		m := re.FindStringSubmatch(svg)
		if len(m) < 2 {
			return "", false
		}
		return m[1], true
	}

	want, ok := posOf(src, "db")
	if !ok {
		t.Fatal("baseline: db not rendered")
	}

	toRoot, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "reparent", ID: "db", Target: ""})
	if err != nil {
		t.Fatalf("reparent to root: %v", err)
	}
	if got, _ := posOf(string(toRoot), "db"); got != want {
		t.Fatalf("reparent->root moved db: want %q got %q (the 'fly' bug)", want, got)
	}

	back, err := ApplyOpText("yaml", toRoot, EditOp{Kind: "reparent", ID: "db", Target: "vpc"})
	if err != nil {
		t.Fatalf("reparent back into vpc: %v", err)
	}
	if got, _ := posOf(string(back), "db"); got != want {
		t.Fatalf("round-trip moved db: want %q got %q", want, got)
	}
}

// TestApplyMove_NestedNodeUnderSceneFreeze guards a node nested in a non-layout
// group from being swallowed by the root-only freeze path: a top-level place:
// makes SceneNeedsFreeze true, but moving the nested node must still nudge it.
func TestApplyMove_NestedNodeUnderSceneFreeze(t *testing.T) {
	const src = `nodes:
  scene:
    shape: composite
    parts:
      - { id: anchor, shape: rectangle, geom: { w: 60, d: 60, h: 30 }, place: { gravity: NW }, label: A }
      - id: g
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        offset: { wx: 120, wy: 0 }
        parts:
          - { id: inner, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, offset: { wx: 20, wy: 20 }, label: I }
`
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "inner", DWX: 40, DWY: 0})
	if err != nil {
		t.Fatalf("move nested: %v", err)
	}
	if !strings.Contains(string(out), "offset: { wx: 60, wy: 20 }") {
		t.Fatalf("nested move swallowed — inner.offset should be wx:60:\n%s", out)
	}
}

// TestApplyMove_NestedBlockOffsetNotCorrupted guards the UpsertInlineKey indent
// match: moving a parent that lacks its own offset must add it at the parent's
// indent, not hijack a nested child's block-form `offset:` line (which produced
// invalid YAML).
func TestApplyMove_NestedBlockOffsetNotCorrupted(t *testing.T) {
	const src = `nodes:
  scene:
    shape: composite
    parts:
      - id: outer
        shape: group
        geom: { w: 300, d: 200, h: 6 }
        parts:
          - id: inner
            shape: rectangle
            geom: { w: 80, d: 80, h: 30 }
            offset: { wx: 20, wy: 20 }
            label: I
`
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "outer", DWX: 50, DWY: 0})
	if err != nil {
		t.Fatalf("move outer: %v", err)
	}
	if _, issues, _ := RenderSource("yaml", out); true {
		for _, i := range issues {
			if i.Severity == SeverityError {
				t.Fatalf("nested block offset corrupted: %s\n%s", i.Message, out)
			}
		}
	}
	if !strings.Contains(string(out), "offset: { wx: 20, wy: 20 }") {
		t.Fatalf("inner.offset must survive the parent move:\n%s", out)
	}
}

func TestApplyOp_ReparentSameParentNoop(t *testing.T) {
	out, err := ApplyOpText("yaml", []byte(nestFixture), EditOp{Kind: "reparent", ID: "x", Target: "g"})
	if err != nil {
		t.Fatalf("reparent: %v", err)
	}
	if string(out) != nestFixture {
		t.Fatalf("reparent to the SAME parent must be a no-op; got:\n%s", out)
	}
}

func TestApplyOp_ReparentRejectsIntoOwnChild(t *testing.T) {
	// can't move g into its own child x.
	if _, err := ApplyOpText("yaml", []byte(nestFixture), EditOp{Kind: "reparent", ID: "g", Target: "x"}); err == nil {
		t.Fatal("expected reparent of a container into its own descendant to be refused")
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
