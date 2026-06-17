package isotopo

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/MarkovWangRR/iso-topology/yamledit"
	"gopkg.in/yaml.v3"
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
	case "add-edge":
		from, to := op.Fields["from"], op.Fields["to"]
		if from == "" || to == "" {
			return src, fmt.Errorf("add-edge: from and to are required")
		}
		out, ok := yamledit.AddConnector(string(src), from, to)
		if !ok {
			return src, fmt.Errorf("add-edge: connectors block not found and no parts block to anchor to")
		}
		return []byte(out), nil
	case "delete":
		return applyDelete(format, src, op)
	case "duplicate":
		return applyDuplicate(format, src, op)
	case "reparent":
		return applyReparent(format, src, op)
	default:
		return src, fmt.Errorf("unknown op kind %q", op.Kind)
	}
}

// rerouteMovedConnectors gives obstacle avoidance to the edges touching a
// just-moved node (the Studio drag → auto-route behaviour). For each connector
// on movedID whose straight line now tunnels another node, it computes an
// avoiding orthogonal route with the scorecard kernel and writes
// `routing: orthogonal` + interior `waypoints` to that connector. Edges that
// are already clear — or that can't be cleared — are left untouched, so a
// default straight edge only becomes routed when a move actually creates a
// collision. Non-yaml formats are returned unchanged.
func rerouteMovedConnectors(format string, src []byte, movedID string) []byte {
	if format == "d2" {
		return src
	}
	doc, err := LoadInput(context.Background(), format, src, LayoutDagre)
	if err != nil {
		return src
	}
	scene := doc.Scene()
	if scene == nil {
		return src
	}
	rects, byID, _ := buildPlanModelOpt(scene, doc.Theme, doc.Canvas, false)
	var leaves []planRect
	for _, r := range rects {
		if !r.container {
			leaves = append(leaves, r)
		}
	}
	out := string(src)
	for ci, c := range scene.Connectors {
		if c == nil {
			continue
		}
		fromID, toID := connectorTarget(c.From), connectorTarget(c.To)
		if fromID != movedID && toID != movedID {
			continue
		}
		fr, okF := byID[fromID]
		to, okT := byID[toID]
		if !okF || !okT {
			continue
		}
		ez := edgeZLevel(fr, to)
		// Inflate obstacles by a clearance margin so routes are pushed to keep
		// a visible gap (a route that merely grazes a face is not "avoiding").
		obstacles := inflateRects(leaves, 12)
		straight := [][2]float64{{fr.x + fr.w/2, fr.y + fr.d/2}, {to.x + to.w/2, to.y + to.d/2}}
		if routeTunnels(straight, fromID, toID, ez, obstacles) == 0 {
			continue // already clear — leave the edge as the author drew it
		}
		route := avoidingRoute(fr, to, obstacles)
		if routeTunnels(route, fromID, toID, ez, obstacles) > 0 {
			continue // no candidate clears it — don't write a still-broken detour
		}
		var wps [][2]float64
		for _, p := range route[1 : len(route)-1] {
			if len(wps) > 0 && math.Abs(wps[len(wps)-1][0]-p[0]) < 0.5 && math.Abs(wps[len(wps)-1][1]-p[1]) < 0.5 {
				continue // drop coincident corners
			}
			wps = append(wps, p)
		}
		if len(wps) == 0 {
			continue
		}
		line := yamledit.FindConnectorLine(out, ci)
		if line < 0 {
			continue
		}
		if o2, ok := yamledit.SetField(out, line, []string{"routing"}, "orthogonal"); ok {
			out = o2
		}
		if line = yamledit.FindConnectorLine(out, ci); line < 0 {
			continue
		}
		if o2, ok := yamledit.UpsertInlineList(out, line, "waypoints", wps); ok {
			out = o2
		}
	}
	return []byte(out)
}

// inflateRects grows each footprint by m on every side (x/y only — z-floor
// stays) so routing keeps a visible clearance instead of grazing a face.
func inflateRects(rs []planRect, m float64) []planRect {
	out := make([]planRect, len(rs))
	for i, r := range rs {
		out[i] = r
		out[i].x, out[i].y = r.x-m, r.y-m
		out[i].w, out[i].d = r.w+2*m, r.d+2*m
	}
	return out
}

// avoidingRoute returns the orthogonal route between two footprints that least
// tunnels the obstacles: the normal staircases PLUS detour candidates whose
// cross-leg runs in a clear lane just beyond the obstacle cluster (so the route
// goes AROUND it, not through it). Cost ranks by tunnelling, then bends/length.
func avoidingRoute(fr, to planRect, obstacles []planRect) [][2]float64 {
	ez := edgeZLevel(fr, to)
	cands := planRouteCandidates(fr, to)

	var obs []planRect
	for _, r := range obstacles {
		if r.id == fr.id || r.id == to.id || r.h <= planThinH || !sameFloor(ez, r) || enclosesBoth(r, fr, to) {
			continue
		}
		obs = append(obs, r)
	}
	if len(obs) > 0 {
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for _, r := range obs {
			minX, minY = math.Min(minX, r.x), math.Min(minY, r.y)
			maxX, maxY = math.Max(maxX, r.x+r.w), math.Max(maxY, r.y+r.d)
		}
		const mgn = 16.0
		fcx, fcy := fr.x+fr.w/2, fr.y+fr.d/2
		tcx, tcy := to.x+to.w/2, to.y+to.d/2
		// vertical detours: exit along y to a clear lane above/below, cross, return
		for _, laneY := range []float64{minY - mgn, maxY + mgn} {
			sy, ey := fr.y, to.y
			if laneY > fcy {
				sy = fr.y + fr.d
			}
			if laneY > tcy {
				ey = to.y + to.d
			}
			cands = append(cands, [][2]float64{{fcx, sy}, {fcx, laneY}, {tcx, laneY}, {tcx, ey}})
		}
		// horizontal detours: exit along x to a clear lane left/right, cross, return
		for _, laneX := range []float64{minX - mgn, maxX + mgn} {
			sx, ex := fr.x, to.x
			if laneX > fcx {
				sx = fr.x + fr.w
			}
			if laneX > tcx {
				ex = to.x + to.w
			}
			cands = append(cands, [][2]float64{{sx, fcy}, {laneX, fcy}, {laneX, tcy}, {ex, tcy}})
		}
	}

	best := cands[0]
	bestCost := routeCost(best, fr, to, obstacles, nil)
	for _, c := range cands[1:] {
		if cost := routeCost(c, fr, to, obstacles, nil); cost < bestCost {
			best, bestCost = c, cost
		}
	}
	return best
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
			return rerouteMovedConnectors(format, []byte(out), op.ID), nil
		}
		cx, cy, cz, found := ResolvePartOffset(doc, op.ID)
		if !found {
			return src, fmt.Errorf("move: part %q not found", op.ID)
		}
		out, ok := yamledit.UpsertInlineKey(s, yamledit.FindPartIDLine(s, op.ID), "offset", snap(cx+op.DWX), snap(cy+op.DWY), cz)
		if !ok {
			return src, fmt.Errorf("move: part %q line not found", op.ID)
		}
		return rerouteMovedConnectors(format, []byte(out), op.ID), nil
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

// foldIconColor returns op.Fields with the synthetic "@iconColor" key resolved
// into a concrete `icon` write: the tint lives in the icon-ref suffix
// (iso://glyph|si/<name>/RRGGBB), not a YAML key. The base ref is the icon set
// in this same op if present, else the node's current icon. Non-node targets and
// non-glyph icons can't be tinted → the key is just dropped. The caller's map is
// never mutated.
func foldIconColor(src []byte, op EditOp) map[string]string {
	hex, ok := op.Fields["@iconColor"]
	if !ok {
		return op.Fields
	}
	out := make(map[string]string, len(op.Fields))
	for k, v := range op.Fields {
		if k != "@iconColor" {
			out[k] = v
		}
	}
	if op.Target == "node" {
		base := op.Fields["icon"]
		if base == "" {
			var root map[string]interface{}
			if yaml.Unmarshal(src, &root) == nil {
				if nm := yamledit.FindNodeMap(root, op.ID); nm != nil {
					base, _ = nm["icon"].(string)
				}
			}
		}
		if spliced := spliceIconColor(base, hex); spliced != "" {
			out["icon"] = spliced
		}
	}
	return out
}

func applySetField(src []byte, op EditOp) ([]byte, error) {
	out := string(src)
	fields := foldIconColor(src, op)
	// Editing the canvas when no `canvas:` block exists yet: create an empty
	// one at the top so the writes below have somewhere to land.
	if op.Target == "canvas" && yamledit.FindCanvasLine(out) < 0 {
		out = "canvas: {}\n" + out
	}
	for key, val := range fields {
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

// findPart locates a composite part by id anywhere in the document's node
// trees, or nil if no part carries that id (a bare top-level node is not a
// part). Mirrors the walk in ResolvePartStyle.
func findPart(doc *Document, id string) *CompositePart {
	if doc == nil {
		return nil
	}
	var found *CompositePart
	var walk func(ps []*CompositePart)
	walk = func(ps []*CompositePart) {
		for _, p := range ps {
			if p == nil || found != nil {
				continue
			}
			if p.ID == id {
				found = p
				return
			}
			walk(p.Parts)
		}
	}
	for _, n := range doc.Nodes {
		if n == nil {
			continue
		}
		walk(n.Parts)
		if found != nil {
			break
		}
	}
	return found
}

// partIsContainer reports whether a part is a container — a group/boundary
// substrate or any part that actually wraps nested parts. Structural edits are
// blocked on these: duplicating one clones its children with COLLIDING ids
// (the rename touches only the container's own id), and deleting one removes a
// whole lane and its contents in a single click. Edit the YAML directly for
// those.
func partIsContainer(p *CompositePart) bool {
	return p != nil && (isContainerShape(p.Shape) || len(p.Parts) > 0)
}

func applyDelete(format string, src []byte, op EditOp) ([]byte, error) {
	s := string(src)
	switch op.Target {
	case "node":
		if doc, derr := LoadInput(context.Background(), format, src, LayoutDagre); derr == nil {
			if partIsContainer(findPart(doc, op.ID)) {
				return src, fmt.Errorf("delete: %q is a container — remove its nested parts first (deleting a whole lane from the canvas is disabled)", op.ID)
			}
		}
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
		if partIsContainer(findPart(doc, op.ID)) {
			return src, fmt.Errorf("duplicate: %q is a container — duplicating it would clone its nested parts with colliding ids; duplicate the children individually", op.ID)
		}
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

// applyReparent moves a node into another group (op.Target) or to the scene
// root (op.Target == ""). It's the engine half of Studio's drag-into / drag-out
// of a group: the part's stale offset is dropped so the new parent lays it out.
func applyReparent(format string, src []byte, op EditOp) ([]byte, error) {
	if doc, derr := LoadInput(context.Background(), format, src, LayoutDagre); derr == nil {
		// No-op when the parent doesn't actually change, so an in-group drag
		// (which also fires a reparent to the same group) keeps its offset.
		if parentOf(doc, op.ID) == op.Target {
			return src, nil
		}
		if op.Target != "" {
			if !partIsContainer(findPart(doc, op.Target)) {
				return src, fmt.Errorf("reparent: target %q is not a container", op.Target)
			}
			// A container can't be moved into its own subtree (it would orphan
			// itself); refuse op.Target that lives inside op.ID.
			if moving := findPart(doc, op.ID); partContains(moving, op.Target) {
				return src, fmt.Errorf("reparent: cannot move %q into its own descendant %q", op.ID, op.Target)
			}
		}
	}
	out, ok := yamledit.MovePart(string(src), op.ID, op.Target)
	if !ok {
		return src, fmt.Errorf("reparent: could not move %q into %q", op.ID, op.Target)
	}
	return []byte(out), nil
}

// parentOf returns the id of the container holding the given part, or "" if it
// sits at the scene root (or isn't found).
func parentOf(doc *Document, id string) string {
	if doc == nil {
		return ""
	}
	var found string
	var walk func(parentID string, ps []*CompositePart)
	walk = func(parentID string, ps []*CompositePart) {
		for _, p := range ps {
			if p == nil || found != "" {
				continue
			}
			if p.ID == id {
				found = parentID
				return
			}
			walk(p.ID, p.Parts)
		}
	}
	for _, n := range doc.Nodes {
		if n != nil {
			walk("", n.Parts)
		}
	}
	return found
}

// partContains reports whether want is p or anywhere in p's subtree.
func partContains(p *CompositePart, want string) bool {
	if p == nil {
		return false
	}
	if p.ID == want {
		return true
	}
	for _, c := range p.Parts {
		if partContains(c, want) {
			return true
		}
	}
	return false
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
