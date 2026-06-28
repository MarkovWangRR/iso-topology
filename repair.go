package isotopo

import (
	"math"
	"regexp"
)

var captionOcclRe = regexp.MustCompile(`group label "([^"]+)" is covered`)

// RepairScene is the Phase-1 projection-repair loop (docs/design/layout-engine-
// master-plan.md): it measures the rendered scene and locally repairs the
// defects that only exist in the projected output, re-checking each round until
// the scene is clean or a budget is hit — "optimize in the space you render".
// v1 repairs two defect classes:
//   - caption clearance: a group label ridden by its front-most child → widen
//     the group's front padding;
//   - node overlap: two top-level footprints collide → push them apart.
//
// It is a strict no-op on an already-clean scene (0 iterations, byte-identical
// output), so it is safe to run unconditionally. Returns the doc and iteration
// count.
func RepairScene(doc *Document) (*Document, int) {
	if doc == nil {
		return doc, 0
	}
	const maxIters = 16
	iters := 0
	for ; iters < maxIters; iters++ {
		changed := repairCaptions(doc)
		if scene := doc.Scene(); scene != nil && repairOverlaps(doc, scene) {
			changed = true
		}
		if !changed {
			break // converged (or nothing left we can fix)
		}
	}
	return doc, iters
}

// repairCaptions widens the front padding of every group whose caption is ridden
// by its own child. One pass; returns whether anything was bumped.
func repairCaptions(doc *Document) bool {
	const (
		step   = 0.5
		capPad = 8.0
	)
	occluded := map[string]bool{}
	for _, is := range LabelOcclusionIssues(doc) {
		if m := captionOcclRe.FindStringSubmatch(is.Message); m != nil {
			occluded[m[1]] = true
		}
	}
	if len(occluded) == 0 {
		return false
	}
	bumped := false
	walkParts(doc, func(p *CompositePart) {
		if p.Layout == nil || p.Label == "" || len(p.Parts) == 0 || !occluded[p.Label] {
			return
		}
		if cur := groupPadding(p); cur < capPad {
			np := math.Min(cur+step, capPad)
			p.Layout.Padding = &np
			bumped = true
		}
	})
	return bumped
}

// repairOverlaps pushes overlapping TOP-LEVEL parts apart along their axis of
// least penetration by nudging each side's offset. One pass; returns whether
// anything moved. Positions are read from a solved clone so the source
// declarations are untouched; the push is applied as an offset delta on the real
// parts (offset is a fine-tune on top of place, so the solver honours it).
func repairOverlaps(doc *Document, scene *Node) bool {
	clone := cloneSceneForEval(scene)
	applyLayout(clone, doc.Canvas)
	type rect struct {
		idx        int
		x, y, w, d float64
	}
	var rs []rect
	for i, p := range clone.Parts {
		if p == nil {
			continue
		}
		// A nil offset means the part sits at the world origin (the unplaced
		// anchor), NOT "skip me" — else an overlap with the anchor is missed.
		var x, y float64
		if p.Offset != nil {
			x, y = p.Offset.WX, p.Offset.WY
		}
		w, d := partFootprint(p)
		rs = append(rs, rect{i, x, y, w, d})
	}
	const margin = 10.0
	moved := false
	for a := 0; a < len(rs); a++ {
		for b := a + 1; b < len(rs); b++ {
			ra, rb := rs[a], rs[b]
			ox := math.Min(ra.x+ra.w, rb.x+rb.w) - math.Max(ra.x, rb.x)
			oy := math.Min(ra.y+ra.d, rb.y+rb.d) - math.Max(ra.y, rb.y)
			if ox <= 0 || oy <= 0 {
				continue // no footprint overlap
			}
			// push half the penetration (+margin) each, along the smaller axis.
			if ox <= oy {
				push := (ox + margin) / 2
				if ra.x <= rb.x {
					nudge(scene.Parts[ra.idx], -push, 0)
					nudge(scene.Parts[rb.idx], +push, 0)
				} else {
					nudge(scene.Parts[ra.idx], +push, 0)
					nudge(scene.Parts[rb.idx], -push, 0)
				}
			} else {
				push := (oy + margin) / 2
				if ra.y <= rb.y {
					nudge(scene.Parts[ra.idx], 0, -push)
					nudge(scene.Parts[rb.idx], 0, +push)
				} else {
					nudge(scene.Parts[ra.idx], 0, +push)
					nudge(scene.Parts[rb.idx], 0, -push)
				}
			}
			moved = true
		}
	}
	return moved
}

// nudge adds a world (dx,dy) delta to a part's offset, creating it if absent.
func nudge(p *CompositePart, dx, dy float64) {
	if p == nil {
		return
	}
	if p.Offset == nil {
		p.Offset = &WorldPoint{}
	}
	p.Offset.WX += dx
	p.Offset.WY += dy
}

// groupPadding returns a group's effective front padding in cells (Padding,
// else Gap, else the layout default of 1).
func groupPadding(p *CompositePart) float64 {
	if p.Layout != nil {
		if p.Layout.Padding != nil {
			return *p.Layout.Padding
		}
		if p.Layout.Gap != nil {
			return *p.Layout.Gap
		}
	}
	return 1.0
}

// walkParts visits every CompositePart in the document, depth-first.
func walkParts(doc *Document, fn func(*CompositePart)) {
	var rec func(parts []*CompositePart)
	rec = func(parts []*CompositePart) {
		for _, p := range parts {
			if p == nil {
				continue
			}
			fn(p)
			rec(p.Parts)
		}
	}
	for _, n := range doc.Nodes {
		if n != nil {
			rec(n.Parts)
		}
	}
}
