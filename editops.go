package isotopo

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/MarkovWangRR/iso-topology/yamledit"
)

// This file lifts Studio's "direct-manipulation → DSL rewrite" loop out of the
// HTTP handlers and the (formerly internal) yamledit package into a public,
// STATELESS contract any embedder — a cloud service, a WASM module — can call.
//
// The whole loop was always stateless: the editor POSTs the full DSL text and
// the server transforms that text (the single source of truth), never holding
// state. ApplyOpText is that transform; ApplyOp bundles a re-render.

// EditOp describes one direct-manipulation edit. It mirrors the operations the
// Studio canvas produces; Kind selects which fields are read.
//
//	move       — Target node|edge + DWX/DWY world drag delta (+ Snap grid step,
//	             or Waypoints for a drawio-style per-segment edge edit)
//	set-field  — Target node|edge|canvas + Fields: dotted YAML path → value
//	add        — append a default rectangle part to the scene
//	delete     — Target node|edge (a node also drops its connectors)
//	duplicate  — Target node: clone it with a fresh id, offset down-right
type EditOp struct {
	Kind      string            `json:"kind"`
	Target    string            `json:"target,omitempty"` // node | edge | canvas
	ID        string            `json:"id,omitempty"`     // part id
	CI        int               `json:"ci,omitempty"`     // connector index
	Fields    map[string]string `json:"fields,omitempty"` // set-field: path → value
	DWX       float64           `json:"dwx,omitempty"`    // move: world drag delta
	DWY       float64           `json:"dwy,omitempty"`
	Snap      float64           `json:"snap,omitempty"`      // move: grid step (0 = none)
	Waypoints [][2]float64      `json:"waypoints,omitempty"` // edge move: interior corners
}

// RenderSource validates and renders a DSL document to one topology SVG. Parse
// and validation errors are returned as Issues (with an error-severity entry),
// not as err — err is reserved for future I/O paths and is currently always nil.
// The layout engine is the default (dagre); d2 auto-layout honours it, YAML/JSON
// have no layout step.
func RenderSource(format string, src []byte) (svg string, issues []Issue, err error) {
	doc, derr := LoadInput(context.Background(), format, src, LayoutDagre)
	if derr != nil {
		return "", []Issue{{Severity: SeverityError, Path: "$", Message: derr.Error()}}, nil
	}
	issues = Validate(doc)
	if format != "d2" {
		issues = append(issues, UnknownKeyIssues(src)...)
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			return "", issues, nil
		}
	}
	if scene := doc.Scene(); scene != nil {
		svg = RenderWithCanvas(scene, doc.Theme, doc.Canvas, doc.Annotations)
	}
	if svg == "" {
		issues = append(issues, Issue{
			Severity: SeverityError, Path: "$",
			Message: "document renders no scene — it has no nodes (or the scene resolves empty)",
		})
	}
	return svg, issues, nil
}

// ApplyOp applies a direct-manipulation edit to DSL text and re-renders it:
// newSrc is the rewritten document (comments and formatting preserved), svg and
// issues are RenderSource of that result. The operation is stateless — newSrc
// is a pure function of (format, src, op). err is non-nil only when the op
// itself can't be applied (e.g. the target id isn't in the source).
func ApplyOp(format string, src []byte, op EditOp) (newSrc []byte, svg string, issues []Issue, err error) {
	newSrc, err = ApplyOpText(format, src, op)
	if err != nil {
		return src, "", nil, err
	}
	svg, issues, _ = RenderSource(format, newSrc)
	return newSrc, svg, issues, nil
}

// ApplyOpText applies an edit and returns ONLY the rewritten DSL text — the
// comment-preserving transform, without rendering. Embedders that render
// separately (or with a non-default layout engine) use this.
func ApplyOpText(format string, src []byte, op EditOp) ([]byte, error) {
	switch op.Kind {
	case "move":
		return applyMove(format, src, op)
	case "set-field":
		return applySetField(src, op)
	case "add":
		out, ok := yamledit.AddPart(string(src))
		if !ok {
			return src, fmt.Errorf("add: scene parts block not found")
		}
		return []byte(out), nil
	case "delete":
		return applyDelete(src, op)
	case "duplicate":
		return applyDuplicate(format, src, op)
	default:
		return src, fmt.Errorf("unknown op kind %q", op.Kind)
	}
}

func applyMove(format string, src []byte, op EditOp) ([]byte, error) {
	doc, derr := LoadInput(context.Background(), format, src, LayoutDagre)
	if derr != nil {
		return src, derr
	}
	snap := func(v float64) float64 {
		if op.Snap > 0 {
			return math.Round(v/op.Snap) * op.Snap
		}
		return v
	}
	s := string(src)
	switch op.Target {
	case "node":
		// drawio model: the FIRST manual move freezes the whole scene into
		// explicit coordinates and drops auto-layout, so the engine never
		// re-decides positions; later moves just nudge one node.
		if SceneNeedsFreeze(doc) {
			offs := ResolveAllOffsets(doc)
			out := yamledit.FreezeLayoutText(s)
			ids := make([]string, 0, len(offs))
			for id := range offs {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				o := offs[id]
				wx, wy, wz := o[0], o[1], o[2]
				if id == op.ID {
					wx, wy = snap(wx+op.DWX), snap(wy+op.DWY)
				}
				var ok bool
				out, ok = yamledit.UpsertInlineKey(out, yamledit.FindPartIDLine(out, id), "offset", wx, wy, wz)
				if !ok {
					return src, fmt.Errorf("move: part %q not found after freeze", id)
				}
			}
			return []byte(out), nil
		}
		cx, cy, cz, found := ResolvePartOffset(doc, op.ID)
		if !found {
			return src, fmt.Errorf("move: part %q not found", op.ID)
		}
		out, ok := yamledit.UpsertInlineKey(s, yamledit.FindPartIDLine(s, op.ID), "offset", snap(cx+op.DWX), snap(cy+op.DWY), cz)
		if !ok {
			return src, fmt.Errorf("move: part %q line not found", op.ID)
		}
		return []byte(out), nil
	case "edge":
		// Per-segment waypoint edit (drawio-style) when supplied; else the
		// legacy single-corner bend accumulated from the current value.
		if len(op.Waypoints) > 0 {
			out, ok := yamledit.UpsertInlineList(s, yamledit.FindConnectorLine(s, op.CI), "waypoints", op.Waypoints)
			if !ok {
				return src, fmt.Errorf("move: connector %d not found", op.CI)
			}
			return []byte(out), nil
		}
		bx, by := ConnectorBend(doc, op.CI)
		out, ok := yamledit.UpsertInlineKey(s, yamledit.FindConnectorLine(s, op.CI), "bend", bx+op.DWX, by+op.DWY, 0)
		if !ok {
			return src, fmt.Errorf("move: connector %d not found", op.CI)
		}
		return []byte(out), nil
	}
	return src, fmt.Errorf("move: target must be node|edge")
}

func applySetField(src []byte, op EditOp) ([]byte, error) {
	out := string(src)
	// Editing the canvas when no `canvas:` block exists yet: create an empty
	// one at the top so the writes below have somewhere to land.
	if op.Target == "canvas" && yamledit.FindCanvasLine(out) < 0 {
		out = "canvas: {}\n" + out
	}
	for key, val := range op.Fields {
		line, ok := targetLine(out, op)
		if !ok {
			return src, fmt.Errorf("set-field: target must be node|edge|canvas")
		}
		if line < 0 {
			return src, fmt.Errorf("set-field: target %q not found in source", op.ID)
		}
		// Re-find the target each write: a write can shift line numbers.
		out, _ = yamledit.SetField(out, line, strings.Split(key, "."), val)
	}
	return []byte(out), nil
}

func applyDelete(src []byte, op EditOp) ([]byte, error) {
	s := string(src)
	switch op.Target {
	case "node":
		out, ok := yamledit.DeletePart(s, op.ID)
		if !ok {
			return src, fmt.Errorf("delete: node %q not found", op.ID)
		}
		return []byte(out), nil
	case "edge":
		if start, end, found := yamledit.ConnectorItemRange(s, op.CI); found {
			return []byte(yamledit.DeleteLineRange(s, start, end)), nil
		}
		return src, fmt.Errorf("delete: connector %d not found", op.CI)
	}
	return src, fmt.Errorf("delete: target must be node|edge")
}

func applyDuplicate(format string, src []byte, op EditOp) ([]byte, error) {
	ox, oy := 40.0, 40.0
	if doc, derr := LoadInput(context.Background(), format, src, LayoutDagre); derr == nil {
		if cx, cy, _, found := ResolvePartOffset(doc, op.ID); found {
			ox, oy = cx+40, cy+40
		}
	}
	out, ok := yamledit.DuplicatePart(string(src), op.ID, ox, oy)
	if !ok {
		return src, fmt.Errorf("duplicate: node %q not found", op.ID)
	}
	return []byte(out), nil
}

// targetLine resolves an op's edit target to the source line its block starts
// on (node id / connector index / canvas block).
func targetLine(src string, op EditOp) (int, bool) {
	switch op.Target {
	case "node":
		return yamledit.FindPartIDLine(src, op.ID), true
	case "edge":
		return yamledit.FindConnectorLine(src, op.CI), true
	case "canvas":
		return yamledit.FindCanvasLine(src), true
	}
	return -1, false
}
