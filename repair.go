package isotopo

import (
	"math"
	"regexp"
)

var captionOcclRe = regexp.MustCompile(`group label "([^"]+)" is covered`)

// RepairScene is the Phase-1 projection-repair loop (docs/design/layout-engine-
// master-plan.md), v1 scoped to caption clearance: it detects iso-screen caption
// occlusions (a group's label ridden by its front-most child) and locally widens
// each offending group's front padding, re-checking each round, until the
// captions clear or a budget is hit — "optimize in the space you render".
//
// It is a strict no-op on a scene with no caption occlusion (already-clean
// diagrams stay byte-identical), so it is safe to run unconditionally. Returns
// the (mutated) doc and the iteration count.
func RepairScene(doc *Document) (*Document, int) {
	if doc == nil {
		return doc, 0
	}
	const (
		maxIters = 10
		step     = 0.5 // cells added to a group's padding each round
		capPad   = 8.0 // never grow a group's padding past this (avoid runaway)
	)
	iters := 0
	for ; iters < maxIters; iters++ {
		occluded := map[string]bool{}
		for _, is := range LabelOcclusionIssues(doc) {
			if m := captionOcclRe.FindStringSubmatch(is.Message); m != nil {
				occluded[m[1]] = true
			}
		}
		if len(occluded) == 0 {
			break // converged
		}
		bumped := false
		walkParts(doc, func(p *CompositePart) {
			if p.Layout == nil || p.Label == "" || len(p.Parts) == 0 || !occluded[p.Label] {
				return
			}
			cur := groupPadding(p)
			if cur < capPad {
				np := math.Min(cur+step, capPad)
				p.Layout.Padding = &np
				bumped = true
			}
		})
		if !bumped {
			break // every offending group is already at the cap — can't improve
		}
	}
	return doc, iters
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
