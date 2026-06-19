package isotopo

import (
	"context"
	"strings"
	"testing"
)

// hasErr reports whether any issue is error-severity.
func hasErr(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

func errMsgs(issues []Issue) string {
	var b strings.Builder
	for _, i := range issues {
		b.WriteString(string(i.Severity) + " " + i.Path + ": " + i.Message + "\n")
	}
	return b.String()
}

// TestNilPointers_NoPanic locks the parse-time nil normalization: empty values
// the decoders leave as nil pointers must not crash Validate or RenderSource.
func TestNilPointers_NoPanic(t *testing.T) {
	cases := []string{
		"nodes:\n  scene:\n", // nil node
		"nodes:\n  scene:\n    shape: composite\n    parts:\n      -\n",                     // nil part
		"nodes:\n  scene:\n    shape: composite\n    parts: []\n    connectors:\n      -\n", // nil connector
		"nodes:\n  scene:\n    shape: composite\n    parts: []\nannotations:\n  -\n",        // nil annotation
	}
	for _, src := range cases {
		doc, err := Parse([]byte(src))
		if err != nil {
			continue // a clean parse error is an acceptable signal; the point is no panic
		}
		_ = Validate(doc)                           // must not panic
		_, _, _ = RenderSource("yaml", []byte(src)) // must not panic
	}
}

// TestValidate_NonFiniteGeom: NaN/Inf dimensions are errors, not corrupt SVG.
func TestValidate_NonFiniteGeom(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: { w: .inf, d: 50, h: .nan } }\n"
	_, issues, _ := RenderSource("yaml", []byte(src))
	if !hasErr(issues) {
		t.Fatalf("non-finite geom should be an error; got:\n%s", errMsgs(issues))
	}
}

// TestConnectorLabel_XMLEscaped: XML-unsafe label chars must be escaped so the
// SVG stays well-formed (previously a silent corruption — err=nil, broken SVG).
func TestConnectorLabel_XMLEscaped(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30} }\n" +
		"      - { id: b, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:160,wy:0} }\n" +
		"    connectors:\n      - { from: a, to: b, label: \"R&D <x>\" }\n"
	svg, issues, _ := RenderSource("yaml", []byte(src))
	if hasErr(issues) {
		t.Fatalf("unexpected errors:\n%s", errMsgs(issues))
	}
	if strings.Contains(svg, "R&D") || strings.Contains(svg, "<x>") {
		t.Fatalf("label not escaped — raw R&D / <x> present in SVG")
	}
	if !strings.Contains(svg, "R&amp;D") || !strings.Contains(svg, "&lt;x&gt;") {
		t.Fatalf("expected escaped &amp;/&lt;&gt; in SVG")
	}
}

// TestDuplicate_IDInLabelNotCorrupted: the rename must touch only the part's own
// id key, not a matching substring inside a quoted label value.
func TestDuplicate_IDInLabelNotCorrupted(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, label: \"node id: a here\" }\n"
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "duplicate", Target: "node", ID: "a"})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if _, issues, _ := RenderSource("yaml", out); hasErr(issues) {
		t.Fatalf("duplicate corrupted the YAML:\n%s\n%s", errMsgs(issues), out)
	}
	if !strings.Contains(string(out), `label: "node id: a here"`) {
		t.Fatalf("label value was mangled by the id rename:\n%s", out)
	}
}

// TestSetField_IDPlusFields applies an id rename together with another field
// without dropping either (and deterministically, regardless of map order).
func TestSetField_IDPlusFields(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, label: X }\n"
	op := EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"id": "renamed", "label": "X2"}}
	var first string
	for i := 0; i < 5; i++ { // determinism: same result every run
		out, err := ApplyOpText("yaml", []byte(src), op)
		if err != nil {
			t.Fatalf("set-field: %v", err)
		}
		s := string(out)
		if !strings.Contains(s, "id: renamed") || !strings.Contains(s, "label: X2") {
			t.Fatalf("id rename + field edit was dropped:\n%s", s)
		}
		if i == 0 {
			first = s
		} else if s != first {
			t.Fatalf("set-field result is non-deterministic across runs")
		}
	}
}

// TestSetField_NewlineValuePreserved: a value with a newline must round-trip
// through the quoted scalar instead of being folded back to a space.
func TestSetField_NewlineValuePreserved(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, label: X }\n"
	op := EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"label": "hello\nworld"}}
	out, err := ApplyOpText("yaml", []byte(src), op)
	if err != nil {
		t.Fatalf("set-field: %v", err)
	}
	doc, err := LoadInput(context.Background(), "yaml", out, LayoutDagre)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	var label string
	for _, p := range doc.Scene().Parts {
		if p != nil && p.ID == "a" {
			label = p.Label
		}
	}
	if label != "hello\nworld" {
		t.Fatalf("newline value corrupted on round-trip: got %q", label)
	}
}

// TestValidate_DefaultFillNoFalseContrast: an unstyled part must not trip the
// contrast lint, which had hard-coded the wrong default top fill.
func TestValidate_DefaultFillNoFalseContrast(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:100,d:100,h:40} }\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, i := range VisualContrastIssues(doc) {
		if strings.Contains(i.Message, "indistinguishable") || strings.Contains(i.Message, "low contrast") {
			t.Fatalf("false contrast warning on default styling: %s", i.Message)
		}
	}
}

// TestValidate_StackDuplicateID: a stack replica id colliding with an explicit
// part of the same id is a duplicate.
func TestValidate_StackDuplicateID(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, stack: {count: 2} }\n" +
		"      - { id: \"a~1\", shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:200,wy:0} }\n"
	doc, _ := Parse([]byte(src))
	found := false
	for _, i := range Validate(doc) {
		if strings.Contains(i.Message, "duplicate part id") && strings.Contains(i.Message, "a~1") {
			found = true
		}
	}
	if !found {
		t.Fatal("stack replica colliding with explicit id not flagged as duplicate")
	}
}

// TestValidate_SelfLoopWarns: from==to is a degenerate edge worth warning about.
func TestValidate_SelfLoopWarns(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30} }\n    connectors:\n      - { from: a, to: a }\n"
	doc, _ := Parse([]byte(src))
	found := false
	for _, i := range Validate(doc) {
		if strings.Contains(i.Message, "self-loop") {
			found = true
		}
	}
	if !found {
		t.Fatal("self-loop connector not warned")
	}
}

// TestAddEdge_SameIndentSequence: add-edge into a connectors: block whose items
// sit at the SAME column as the key (valid YAML) must not produce mixed-indent,
// unparseable output.
func TestAddEdge_SameIndentSequence(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: b, shape: rectangle, geom: {w:80,d:80,h:30} }\n" +
		"      - { id: c, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:160,wy:0} }\n" +
		"    connectors:\n    - { from: b, to: c }\n" // items at connectors: column
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "add-edge", Fields: map[string]string{"from": "b", "to": "c"}})
	if err != nil {
		t.Fatalf("add-edge: %v", err)
	}
	if _, issues, _ := RenderSource("yaml", out); hasErr(issues) {
		t.Fatalf("add-edge corrupted same-column connectors block:\n%s\n%s", errMsgs(issues), out)
	}
}

// TestSetField_RenameToExistingIDRejected: renaming a part onto an id another
// part already uses must be refused, not silently create a duplicate.
func TestSetField_RenameToExistingIDRejected(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30} }\n" +
		"      - { id: b, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:160,wy:0} }\n"
	if _, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "set-field", Target: "node", ID: "b", Fields: map[string]string{"id": "a"}}); err == nil {
		t.Fatal("renaming b onto existing id a should error")
	}
	// renaming to a fresh id still works
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "set-field", Target: "node", ID: "b", Fields: map[string]string{"id": "bee"}})
	if err != nil || !strings.Contains(string(out), "id: bee") {
		t.Fatalf("rename to fresh id should work: err=%v\n%s", err, out)
	}
}

// TestPaletteColor_XMLEscaped: a palette colour must be escaped into the fill
// attribute, not injected raw (same guarantee as connector labels / strokes).
func TestPaletteColor_XMLEscaped(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, style: { palette: { top: \"\\\"><inject\" } } }\n"
	svg, issues, _ := RenderSource("yaml", []byte(src))
	if hasErr(issues) {
		t.Fatalf("unexpected errors:\n%s", errMsgs(issues))
	}
	if strings.Contains(svg, `"><inject`) {
		t.Fatalf("palette colour injected raw into SVG")
	}
}

// TestMove_FractionalPrecisionPreserved: offsets must keep sub-integer precision
// (freezing a layout-solved scene used to round to the nearest pixel, drifting
// every node). Integers still print without a decimal point.
func TestMove_FractionalPrecisionPreserved(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:10,wy:10} }\n"
	frac, _ := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "a", DWX: 0.5, DWY: 0.25})
	if !strings.Contains(string(frac), "wx: 10.5") || !strings.Contains(string(frac), "wy: 10.25") {
		t.Fatalf("fractional offset was rounded away:\n%s", frac)
	}
	whole, _ := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "a", DWX: 30, DWY: 0})
	if !strings.Contains(string(whole), "wx: 40,") || strings.Contains(string(whole), "wx: 40.0") {
		t.Fatalf("integer offset should print without a decimal:\n%s", whole)
	}
}

// TestAddPart_SameIndentSequence: the `add` op into a parts: block whose items
// share the key's column must not produce mixed-indent, unparseable YAML
// (whole-class fix shared with add-edge via seqBlockEnd).
func TestAddPart_SameIndentSequence(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n    - { id: a, shape: rectangle, geom: {w:40,d:40,h:20} }\n"
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "add"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, issues, _ := RenderSource("yaml", out); hasErr(issues) {
		t.Fatalf("add corrupted same-column parts block:\n%s\n%s", errMsgs(issues), out)
	}
}

// TestValidate_NonFiniteOffset: NaN/Inf offsets are errors, not corrupt SVG.
func TestValidate_NonFiniteOffset(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:80,h:40}, offset: { wx: .nan, wy: 0 } }\n"
	if _, issues, _ := RenderSource("yaml", []byte(src)); !hasErr(issues) {
		t.Fatalf("non-finite offset should be an error")
	}
}

// TestDelete_AnchorDefiningNodeRefused: deleting a node whose block defines a
// YAML anchor still aliased elsewhere must error, not silently corrupt.
func TestDelete_AnchorDefiningNodeRefused(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: docs, shape: rectangle, geom: {w:40,d:40,h:20}, style: &tile {palette: {top: \"#abc\"}} }\n" +
		"      - { id: etl, shape: rectangle, geom: {w:40,d:40,h:20}, offset: {wx:120,wy:0}, style: *tile }\n"
	if _, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "delete", Target: "node", ID: "docs"}); err == nil {
		t.Fatal("deleting an anchor-defining node still aliased elsewhere should error")
	}
	// deleting the alias-using sibling is fine
	if _, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "delete", Target: "node", ID: "etl"}); err != nil {
		t.Fatalf("deleting a non-anchor node should work: %v", err)
	}
}

// TestApplyMove_IdempotentInAutosizeGroup: a zero-delta move must not shift the
// node, even inside an autosize group (geom.h only) where the resolved position
// folds in slab padding. Reads the authored offset instead of the resolved one.
func TestApplyMove_IdempotentInAutosizeGroup(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - id: g\n        shape: group\n        geom: { h: 6 }\n        parts:\n" +
		"          - { id: a, shape: rectangle, geom: {w:80,d:80,h:30}, offset: {wx:20,wy:20} }\n"
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "a", DWX: 0, DWY: 0})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if !strings.Contains(string(out), "wx: 20") || !strings.Contains(string(out), "wy: 20") {
		t.Fatalf("zero-delta move shifted the node (non-idempotent):\n%s", out)
	}
	// a real move applies on top of the authored base
	out2, _ := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "a", DWX: 30, DWY: 10})
	if !strings.Contains(string(out2), "wx: 50") || !strings.Contains(string(out2), "wy: 30") {
		t.Fatalf("real move incorrect (want wx:50,wy:30):\n%s", out2)
	}
}
