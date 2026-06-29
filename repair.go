package isotopo

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// RepairAndReport runs the projection-repair loop and returns the iteration
// count plus a human/agent-readable list of what it cleared (occlusions that
// vanished, overlap pairs that were separated). Side-effect-free measurement:
// the defect snapshots are taken via the clone-based detectors. This is the L1
// "what I fixed" report of the agent-loop harness plan.
func RepairAndReport(doc *Document) (iters int, fixed []string) {
	if doc == nil {
		return 0, nil
	}
	beforeOcc := occlusionMessages(doc)
	beforeOver := Readability(doc).Overlaps
	_, iters = RepairScene(doc)
	afterOcc := occlusionMessages(doc)
	afterOver := Readability(doc).Overlaps
	for msg := range beforeOcc {
		if !afterOcc[msg] {
			fixed = append(fixed, "cleared occlusion: "+msg)
		}
	}
	sort.Strings(fixed)
	if d := beforeOver - afterOver; d > 0 {
		fixed = append(fixed, fmt.Sprintf("separated %d overlapping node pair(s)", d))
	}
	return iters, fixed
}

// occlusionMessages snapshots the current label/caption occlusions as a set,
// trimmed to the defect phrase (before the em-dash rationale) so before/after
// comparison is stable.
func occlusionMessages(doc *Document) map[string]bool {
	out := map[string]bool{}
	for _, is := range LabelOcclusionIssues(doc) {
		msg := is.Message
		if i := strings.Index(msg, " — "); i >= 0 {
			msg = msg[:i]
		}
		out[msg] = true
	}
	return out
}

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
	// Headroom: separating one genuine overlap can cascade into neighbours, so a
	// dense scene may need several passes to reach the no-change fixpoint. The
	// loop breaks early the instant nothing changes, so this only costs
	// iterations on scenes that genuinely need them.
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

// repairOverlaps pushes genuinely-colliding TOP-LEVEL parts apart along their
// axis of least penetration by nudging each side's offset. One pass; returns
// whether anything moved.
//
// It uses the evaluator's OWN overlap test (rectsOverlap on the plan model) —
// an (x,y) collision that ALSO shares a vertical band — instead of a naive
// footprint test. Parts stacked on different floors (place: above — a chip on a
// plate, a stacked board) deliberately share a footprint and must NOT be pushed
// apart; a naive test shreds such a clean Z-stack and never converges. Positions
// come from a clone so the source declarations are untouched; the push lands as
// an offset delta on the real parts (offset is a fine-tune on top of place).
func repairOverlaps(doc *Document, scene *Node) bool {
	rects, _, _ := buildPlanModel(cloneSceneForEval(scene), doc.Theme, doc.Canvas)
	// Map every part id (at any depth) to its TOP-LEVEL ancestor — the part we
	// can actually relocate by nudging its offset. A collision between a loose
	// node and another group's *child* is resolved by pushing the two top-level
	// owners apart.
	descTop := map[string]*CompositePart{}
	var mark func(top, p *CompositePart)
	mark = func(top, p *CompositePart) {
		if p.ID != "" {
			descTop[p.ID] = top
		}
		for _, c := range p.Parts {
			if c != nil {
				mark(top, c)
			}
		}
	}
	for _, p := range scene.Parts {
		if p != nil {
			mark(p, p)
		}
	}
	var leaves []planRect
	for _, r := range rects {
		if !r.container {
			leaves = append(leaves, r)
		}
	}
	const margin = 10.0
	moved := false
	for a := 0; a < len(leaves); a++ {
		for b := a + 1; b < len(leaves); b++ {
			ra, rb := leaves[a], leaves[b]
			pa, pb := descTop[ra.id], descTop[rb.id]
			if pa == nil || pb == nil || pa == pb {
				continue // same top-level owner (intra-group) or unknown — skip
			}
			if !rectsOverlap(ra, rb) {
				continue // no real collision (e.g. Z-stacked) — leave it alone
			}
			ox := math.Min(ra.x+ra.w, rb.x+rb.w) - math.Max(ra.x, rb.x)
			oy := math.Min(ra.y+ra.d, rb.y+rb.d) - math.Max(ra.y, rb.y)
			// push half the penetration (+margin) each, along the smaller axis.
			if ox <= oy {
				push := (ox + margin) / 2
				if ra.x <= rb.x {
					nudge(pa, -push, 0)
					nudge(pb, +push, 0)
				} else {
					nudge(pa, +push, 0)
					nudge(pb, -push, 0)
				}
			} else {
				push := (oy + margin) / 2
				if ra.y <= rb.y {
					nudge(pa, 0, -push)
					nudge(pb, 0, +push)
				} else {
					nudge(pa, 0, +push)
					nudge(pb, 0, -push)
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
