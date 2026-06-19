package isotopo

import (
	"fmt"
	"strings"
	"testing"
)

// TestEditMatrix_OpByContainer makes the operation × container-kind cross-
// product EXPLICIT: for every container flavour, place a known child and run
// each child-targeting op on it, asserting the floor invariants. Most bugs lived
// in an (op, container) cell nobody had enumerated; this fails the day a new
// container kind or op forgets to handle a cell.
func TestEditMatrix_OpByContainer(t *testing.T) {
	// minimal scene with one container `g` (of the given kind) holding child `c`,
	// a sibling `s` inside it, plus a root node `r` and an edge c→r.
	build := func(kind string) string {
		var g strings.Builder
		g.WriteString("nodes:\n  scene:\n    shape: composite\n    parts:\n")
		g.WriteString("      - id: g\n")
		if kind == "boundary" {
			g.WriteString("        shape: boundary\n")
		} else {
			g.WriteString("        shape: group\n")
		}
		if kind == "autosize" {
			g.WriteString("        geom: { h: 6 }\n")
		} else {
			g.WriteString("        geom: { w: 320, d: 220, h: 6 }\n")
		}
		g.WriteString("        offset: { wx: 120, wy: 80 }\n")
		switch kind {
		case "layout-row":
			g.WriteString("        layout: { mode: row, gap: 24 }\n")
		case "layout-col":
			g.WriteString("        layout: { mode: column, gap: 24 }\n")
		case "layout-grid":
			g.WriteString("        layout: { mode: grid, cols: 2, gap: 24 }\n")
		}
		g.WriteString("        parts:\n")
		// child c in flow form, sibling s in block form — exercise both
		g.WriteString("          - { id: c, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:20,wy:20} }\n")
		g.WriteString("          - id: s\n            shape: cylinder\n            geom: { w: 80, d: 60, h: 30 }\n            offset: { wx: 140, wy: 20 }\n")
		g.WriteString("      - { id: r, shape: rectangle, geom: {w:80,d:60,h:30}, offset: {wx:60,wy:380} }\n")
		g.WriteString("    connectors:\n      - { from: c, to: r }\n")
		return g.String()
	}

	ops := []struct {
		name string
		op   EditOp
	}{
		{"move-child", EditOp{Kind: "move", Target: "node", ID: "c", DWX: 30, DWY: -20}},
		{"move-child-zero", EditOp{Kind: "move", Target: "node", ID: "c", DWX: 0, DWY: 0}},
		{"reparent-out", EditOp{Kind: "reparent", ID: "c", Target: ""}},
		{"reparent-cross", EditOp{Kind: "reparent", ID: "r", Target: "g"}},
		{"set-field-label", EditOp{Kind: "set-field", Target: "node", ID: "c", Fields: map[string]string{"label": "X"}}},
		{"set-field-id", EditOp{Kind: "set-field", Target: "node", ID: "c", Fields: map[string]string{"id": "c2"}}},
		{"delete-child", EditOp{Kind: "delete", Target: "node", ID: "c"}},
		{"duplicate-child", EditOp{Kind: "duplicate", Target: "node", ID: "c"}},
		{"add-edge", EditOp{Kind: "add-edge", Fields: map[string]string{"from": "s", "to": "r"}}},
	}

	for _, kind := range containerKinds {
		if kind == "plain" || kind == "offset" || kind == "nested" {
			kind = "group" // these collapse to a plain group base for this matrix
		}
		base := build(kind)
		if _, iss, _ := RenderSource("yaml", []byte(base)); hasErr(iss) {
			t.Fatalf("matrix base for %q is not clean:\n%s", kind, errMsgs(iss))
		}
		for _, o := range ops {
			t.Run(fmt.Sprintf("%s/%s", kind, o.name), func(t *testing.T) {
				out, err := ApplyOpText("yaml", []byte(base), o.op)
				if v := checkFloorInvariants(base, out, err); v != "" {
					t.Fatalf("%s\nbase=\n%s\nout=\n%s", v, base, out)
				}
				// For a successful non-delete op the operated node must still
				// exist and the doc must render.
				if err == nil && !strings.HasPrefix(o.name, "delete") {
					if _, iss, _ := RenderSource("yaml", out); hasErr(iss) {
						t.Fatalf("op %q on %q left an unrenderable doc:\n%s\n%s", o.name, kind, errMsgs(iss), out)
					}
				}
			})
		}
	}
}
