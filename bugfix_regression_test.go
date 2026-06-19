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

// TestSetField_IDRenameCascades: renaming a part's id must rewrite every
// reference (connector from/to incl .anchor, place, annotation anchor), not
// strand them.
func TestSetField_IDRenameCascades(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: web, shape: rectangle, geom: {w:80,d:60,h:30} }\n" +
		"      - { id: db, shape: rectangle, geom: {w:80,d:60,h:30}, offset:{wx:160,wy:0}, place: {rightOf: web} }\n" +
		"    connectors:\n      - { from: web.right, to: db }\n"
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "set-field", Target: "node", ID: "web", Fields: map[string]string{"id": "webNEW"}})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "from: web.") || strings.Contains(s, "rightOf: web\n") || strings.Contains(s, "rightOf: web ") {
		t.Fatalf("references not cascaded:\n%s", s)
	}
	if !strings.Contains(s, "from: webNEW.right") || !strings.Contains(s, "rightOf: webNEW") {
		t.Fatalf("references not rewritten to new id:\n%s", s)
	}
	if _, issues, _ := RenderSource("yaml", out); hasErr(issues) {
		t.Fatalf("renamed doc has dangling refs:\n%s", errMsgs(issues))
	}
}

// TestSetField_ContainerToLeafRefused: demoting a populated container to a
// non-container shape must error, not silently orphan its children.
func TestSetField_ContainerToLeafRefused(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - id: zone\n        shape: group\n        geom: {w:300,d:200,h:6}\n        parts:\n" +
		"          - { id: cache, shape: rectangle, geom: {w:80,d:60,h:30}, offset:{wx:30,wy:30} }\n"
	if _, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "set-field", Target: "node", ID: "zone", Fields: map[string]string{"shape": "rectangle"}}); err == nil {
		t.Fatal("demoting a populated group to a leaf shape should error")
	}
}

// TestSetField_ReservedWordValueQuoted: a string value that looks like a YAML
// reserved word (null/true) must round-trip as that literal text; numeric
// fields must stay bare numbers.
func TestSetField_ReservedWordValueQuoted(t *testing.T) {
	base := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:60,h:30}, label: X }\n"
	for _, v := range []string{"null", "true"} {
		out, _ := ApplyOpText("yaml", []byte(base), EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"label": v}})
		doc, err := LoadInput(context.Background(), "yaml", out, LayoutDagre)
		if err != nil {
			t.Fatalf("reparse: %v", err)
		}
		got := ""
		for _, p := range doc.Scene().Parts {
			if p != nil && p.ID == "a" {
				got = p.Label
			}
		}
		if got != v {
			t.Fatalf("label %q round-tripped to %q (reserved word not quoted)", v, got)
		}
	}
	// numeric field stays bare + renders
	wout, _, issues, _ := ApplyOp("yaml", []byte(base), EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"geom.w": "120"}})
	if hasErr(issues) {
		t.Fatalf("numeric geom.w write broke render:\n%s\n%s", errMsgs(issues), wout)
	}
}

// TestSetField_NoSilentCorruption: an edit that would make the document
// unparseable (e.g. a scalar into the list-typed content.rows) must error, not
// return corrupt text.
func TestSetField_NoSilentCorruption(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:60,h:30} }\n"
	if _, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"content.rows": "hello"}}); err == nil {
		t.Fatal("writing a scalar into the list-typed content.rows should error, not corrupt")
	}
}

// TestSetField_EscapedQuoteCommaSurvives: a value containing an escaped quote
// and a comma must not corrupt a SUBSEQUENT inline-map edit (splitTopCommas
// must respect backslash escapes).
func TestSetField_EscapedQuoteCommaSurvives(t *testing.T) {
	base := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:60,h:30} }\n"
	s1, err := ApplyOpText("yaml", []byte(base), EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"content.header": `x"y,z`}})
	if err != nil {
		t.Fatalf("first edit: %v", err)
	}
	s2, err := ApplyOpText("yaml", s1, EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"content.header": "safe"}})
	if err != nil {
		t.Fatalf("second edit corrupted by the escaped-quote value: %v", err)
	}
	if _, issues, _ := RenderSource("yaml", s2); hasErr(issues) {
		t.Fatalf("doc unrenderable after edits:\n%s\n%s", errMsgs(issues), s2)
	}
}

// TestReparent_IntoFlowStyleGroup: reparenting into a single-line flow group
// (`- { id: g, shape: group, … }`, as produced by authoring or a shape morph)
// must expand it to block form and nest the child — not slam an unparseable
// block `parts:` under a closed flow scalar.
func TestReparent_IntoFlowStyleGroup(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - { id: leaf, shape: rectangle, geom: {w:80,d:60,h:30} }\n" +
		"      - { id: grp, shape: group, geom: {w:300,d:200,h:6}, label: G }\n"
	out, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "reparent", ID: "leaf", Target: "grp"})
	if err != nil {
		t.Fatalf("reparent into flow group: %v", err)
	}
	if _, issues, _ := RenderSource("yaml", out); hasErr(issues) {
		t.Fatalf("flow-group reparent produced a broken doc:\n%s\n%s", errMsgs(issues), out)
	}
	if !strings.Contains(string(out), "id: grp") || !strings.Contains(string(out), "parts:") {
		t.Fatalf("group not expanded / child not nested:\n%s", out)
	}
}

// TestApplyOp_UnparseableGuard: ANY edit op that would make the document
// unparseable must return an error on the original source, never corrupt text.
func TestApplyOp_UnparseableGuard(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:60,h:30} }\n"
	// content.rows (a list field) set to a scalar would break the YAML.
	if _, err := ApplyOpText("yaml", []byte(src), EditOp{Kind: "set-field", Target: "node", ID: "a", Fields: map[string]string{"content.rows": "boom"}}); err == nil {
		t.Fatal("an unparseable-making edit should error, not corrupt")
	}
}

// TestValidate_OversizeSidesAndPadding: a huge geom.sides (render DoS) and a
// non-finite/huge canvas.padding (viewBox overflow) must be flagged.
func TestValidate_OversizeSidesAndPadding(t *testing.T) {
	sides := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: prism, geom: {w:80,d:60,h:30, sides: 1000000000} }\n"
	if _, issues, _ := RenderSource("yaml", []byte(sides)); !hasErr(issues) {
		t.Error("oversize geom.sides should be flagged")
	}
	pad := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:80,d:60,h:30} }\ncanvas: { padding: 1e20 }\n"
	if _, issues, _ := RenderSource("yaml", []byte(pad)); !hasErr(issues) {
		t.Error("oversize canvas.padding should be flagged")
	}
}

// TestMove_OffsetlessIdempotentInBoundary: a zero-delta move of an offset-less
// node inside a non-layout container must be a true no-op (the resolver-derived
// base must match the rendered position, not a divergent solved baseline).
func TestMove_OffsetlessIdempotentInBoundary(t *testing.T) {
	src := "nodes:\n  scene:\n    shape: composite\n    parts:\n" +
		"      - id: b\n        shape: boundary\n        geom: {w:300,d:200,h:2}\n        parts:\n" +
		"          - { id: db, shape: rectangle, geom: {w:80,d:60,h:30} }\n" +
		"          - { id: x, shape: rectangle, geom: {w:80,d:60,h:30} }\n"
	before, _, _ := RenderSource("yaml", []byte(src))
	out, _ := ApplyOpText("yaml", []byte(src), EditOp{Kind: "move", Target: "node", ID: "db", DWX: 0, DWY: 0})
	after, _, _ := RenderSource("yaml", out)
	if before != after {
		t.Errorf("zero-delta move of an offset-less node in a boundary changed the render")
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
