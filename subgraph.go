// Subgraph preview: render a SUBSET of a composite scene as a self-contained
// SVG. Agents (and Studio) often want to inspect just one node, one container
// group, or one edge with its two endpoints — without re-reading the whole
// diagram. RenderSubgraph crops the scene to a selection, re-runs layout on
// just that selection so the preview is clean and tightly framed, and reuses
// the same canvas/theme as the full render.
package isotopo

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RenderSubgraph renders a preview of a SUBSET of the document's composite
// scene. ids selects what to show; each entry is one of:
//
//	"<part-id>"   — a part anywhere in the scene tree. A container part
//	                (boundary / group) brings its whole subtree along.
//	"edge:<N>"    — connector N (0-based, matching evaluate's connector
//	                index). Pulls in the connector AND both of its endpoint
//	                parts so the edge always has something to dock to.
//
// A connector is included automatically whenever BOTH its endpoints land
// inside the selection, so previewing two connected nodes shows the wire
// between them. The selection is re-laid-out on its own (auto when any
// connector is present, else a simple row) and cropped to its own bounds,
// so the result is a standalone SVG independent of the rest of the scene.
//
// err is returned for an empty selection, an unknown id, an out-of-range
// edge index, or a document with no composite scene.
func RenderSubgraph(doc *Document, ids []string) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("subgraph: nil document")
	}
	scene := doc.Scene()
	if scene == nil {
		return "", fmt.Errorf("subgraph: document has no composite scene to subset")
	}

	// Split ids into part references and edge references; edge refs also
	// contribute their two endpoint parts to the wanted set.
	wanted := map[string]bool{}
	var edgeIdx []int
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(id, "edge:"); ok {
			n, err := strconv.Atoi(rest)
			if err != nil || n < 0 || n >= len(scene.Connectors) {
				return "", fmt.Errorf("subgraph: %q is not a valid connector index (scene has %d connector(s))", id, len(scene.Connectors))
			}
			edgeIdx = append(edgeIdx, n)
			c := scene.Connectors[n]
			wanted[connectorEndpoint(c.From)] = true
			wanted[connectorEndpoint(c.To)] = true
			continue
		}
		wanted[id] = true
	}
	if len(wanted) == 0 {
		return "", fmt.Errorf("subgraph: no parts selected")
	}

	// Resolve each wanted id to a part somewhere in the tree.
	for id := range wanted {
		if findPartByID(scene.Parts, id) == nil {
			return "", fmt.Errorf("subgraph: part %q not found in scene", id)
		}
	}

	// A container subsumes any of its descendants that were also named —
	// rendering the container already shows them. Pre-order index drives a
	// deterministic top-level order (declared order in the scene).
	parentOf := map[string]string{}
	buildParentMap(scene.Parts, "", parentOf)
	orderIdx := map[string]int{}
	{
		i := 0
		var walk func(ps []*CompositePart)
		walk = func(ps []*CompositePart) {
			for _, p := range ps {
				if p == nil || p.ID == "" {
					continue
				}
				orderIdx[p.ID] = i
				i++
				walk(p.Parts)
			}
		}
		walk(scene.Parts)
	}

	roots := make([]string, 0, len(wanted))
	for id := range wanted {
		if !subsumedBySelection(id, parentOf, wanted) {
			roots = append(roots, id)
		}
	}
	sort.Slice(roots, func(a, b int) bool { return orderIdx[roots[a]] < orderIdx[roots[b]] })

	// Clone each root part (and its subtree) and strip its position hints so
	// the subset's own layout pass places it cleanly. The subtree keeps its
	// internal offsets/layout, so containers preview intact.
	parts := make([]*CompositePart, 0, len(roots))
	inside := map[string]bool{}
	for _, id := range roots {
		cp := clonePartTree(findPartByID(scene.Parts, id))
		cp.Place = nil
		cp.Offset = nil
		cp.Position = nil
		parts = append(parts, cp)
		collectSubtreeIDs(cp, inside)
	}

	// Connectors: the explicitly-named edges first (dedup), then every other
	// connector whose both endpoints fall inside the selection.
	var conns []*Connector
	seen := map[int]bool{}
	for _, n := range edgeIdx {
		conns = append(conns, scene.Connectors[n])
		seen[n] = true
	}
	for n, c := range scene.Connectors {
		if seen[n] {
			continue
		}
		if inside[connectorEndpoint(c.From)] && inside[connectorEndpoint(c.To)] {
			conns = append(conns, c)
		}
	}

	// Re-layout the subset on its own: auto when wired (connector-driven
	// ranks), else a simple row so disconnected picks don't pile up.
	var lay *Layout
	if len(conns) > 0 {
		lay = &Layout{Mode: "auto"}
	} else if len(parts) > 1 {
		g := 1.0
		lay = &Layout{Mode: "row", Gap: &g}
	}

	sub := &Node{
		Shape:      "composite",
		Parts:      parts,
		Connectors: conns,
		Layout:     lay,
		GridStep:   scene.GridStep,
		Anchors:    scene.Anchors,
	}
	svg := RenderWithCanvas(sub, doc.Theme, doc.Canvas, nil)
	if svg == "" {
		return "", fmt.Errorf("subgraph: selection rendered no scene")
	}
	return svg, nil
}

// connectorEndpoint strips a "partID.anchor" connector reference down to the
// bare partID (mirrors the target() helper in arrangeAuto).
func connectorEndpoint(ref string) string {
	if d := strings.IndexByte(ref, '.'); d >= 0 {
		return ref[:d]
	}
	return ref
}

// findPartByID returns the first part with the given id anywhere in the
// parts forest (depth-first), or nil.
func findPartByID(parts []*CompositePart, id string) *CompositePart {
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.ID == id {
			return p
		}
		if hit := findPartByID(p.Parts, id); hit != nil {
			return hit
		}
	}
	return nil
}

// buildParentMap records childID → parentID for every nested part (top-level
// parts have no entry).
func buildParentMap(parts []*CompositePart, parent string, out map[string]string) {
	for _, p := range parts {
		if p == nil || p.ID == "" {
			continue
		}
		if parent != "" {
			out[p.ID] = parent
		}
		buildParentMap(p.Parts, p.ID, out)
	}
}

// subsumedBySelection reports whether any ancestor of id is itself selected —
// in which case id is already shown by that ancestor's subtree.
func subsumedBySelection(id string, parentOf map[string]string, wanted map[string]bool) bool {
	for a := parentOf[id]; a != ""; a = parentOf[a] {
		if wanted[a] {
			return true
		}
	}
	return false
}

// collectSubtreeIDs adds p.ID and every descendant id to set.
func collectSubtreeIDs(p *CompositePart, set map[string]bool) {
	if p == nil {
		return
	}
	if p.ID != "" {
		set[p.ID] = true
	}
	for _, c := range p.Parts {
		collectSubtreeIDs(c, set)
	}
}

// clonePartTree deep-copies a part and its subtree. Pointer fields that the
// render/layout pipeline never mutates (Geom, Style, Icon, …) are shared; the
// fields layout DOES write (Offset, Layout, Place, Position) get fresh struct
// copies via the per-part shallow copy, and the Parts slice is rebuilt so the
// clone's tree is independent of the source document.
func clonePartTree(p *CompositePart) *CompositePart {
	if p == nil {
		return nil
	}
	cp := *p
	if len(p.Parts) > 0 {
		cp.Parts = make([]*CompositePart, len(p.Parts))
		for i, c := range p.Parts {
			cp.Parts[i] = clonePartTree(c)
		}
	}
	return &cp
}
