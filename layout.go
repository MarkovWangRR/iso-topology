// Layout solving — turn declarative `layout` (container arrangement)
// and `place` (sibling relations) into concrete part Offsets, so
// authors and agents never hand-compute world coordinates.
//
// The pass runs BEFORE lowering (lower.go) and writes its results into
// the same Offset field the lowering pass already consumes, so the
// rest of the pipeline is untouched. It is deterministic (topological
// solve, no iteration, no randomness) and idempotent: solved parts get
// their Place/Layout cleared, so a second application is a no-op.
//
// All lengths in layout/place are in CELLS. One cell = node.gridStep,
// else canvas.gridStep, else 40 world units — the same lattice as the
// canvas grid texture and the orthogonal connector channels, so solved
// arrangements stay in register with both.
package isotopo

import (
	"fmt"
	"math"
)

const defaultCellSize = 40.0

// Default footprints, mirroring render.go's fallback geometry.
const (
	defaultPartW  = 140.0
	defaultPartD  = 140.0
	defaultPartH  = 80.0
	defaultGroupW = 360.0
	defaultGroupD = 240.0
)

func layoutCell(n *Node, canvas *Canvas) float64 {
	if n != nil && n.GridStep > 0 {
		return n.GridStep
	}
	if canvas != nil && canvas.GridStep > 0 {
		return canvas.GridStep
	}
	return defaultCellSize
}

// applyLayout solves every layout/place declaration in a composite
// node, mutating part Offsets in place. Returned issues are advisory
// for render callers (Validate surfaces the same issues to agents).
func applyLayout(n *Node, canvas *Canvas) []Issue {
	if n == nil || n.Shape != "composite" {
		return nil
	}
	cell := layoutCell(n, canvas)
	var issues []Issue
	solveContainer(n.Parts, n.Layout, nil, cell, "nodes.scene", &issues)
	n.Layout = nil
	return issues
}

// solveContainer resolves one sibling set. Child groups are solved
// first (depth-first) so their footprints are final before this level
// arranges or places them. owner is the group that holds these parts
// (nil at the composite root) — it is the auto-size target.
func solveContainer(parts []*CompositePart, lay *Layout, owner *CompositePart, cell float64, path string, issues *[]Issue) {
	for i, p := range parts {
		if p != nil && p.Shape == "group" && len(p.Parts) > 0 {
			solveContainer(p.Parts, p.Layout, p, cell, fmt.Sprintf("%s.parts[%d]", path, i), issues)
			p.Layout = nil
		}
	}
	if lay != nil {
		arrangeContainer(parts, lay, owner, cell, path, issues)
	} else {
		placed := placeSiblings(parts, cell, path, issues)
		if owner != nil && placed {
			autosizeGroup(owner, parts, cell)
		}
	}
	checkSiblingOverlaps(parts, path, issues)
}

func partFootprint(p *CompositePart) (w, d float64) {
	w, d = defaultPartW, defaultPartD
	if p.Shape == "group" {
		w, d = defaultGroupW, defaultGroupD
	}
	if p.Geom != nil {
		if p.Geom.W > 0 {
			w = p.Geom.W
		}
		if p.Geom.D > 0 {
			d = p.Geom.D
		}
	}
	return w, d
}

func partHeight(p *CompositePart) float64 {
	if p.Geom != nil && p.Geom.H > 0 {
		return p.Geom.H
	}
	if p.Shape == "group" {
		return 8
	}
	return defaultPartH
}

// ── container arrangement (layout: row / column / grid) ─────────────

// arrangeContainer lays kids out on row-major tracks. Row and column
// are degenerate grids (cols = n, cols = 1), which makes track packing
// exact for them too: one child per column track means each column is
// exactly that child's width.
func arrangeContainer(parts []*CompositePart, lay *Layout, owner *CompositePart, cell float64, path string, issues *[]Issue) {
	kids := make([]*CompositePart, 0, len(parts))
	for i, p := range parts {
		if p == nil {
			continue
		}
		if p.Place != nil {
			*issues = append(*issues, Issue{
				Severity: SeverityWarning,
				Path:     fmt.Sprintf("%s.parts[%d].place", path, i),
				Message:  "place is ignored inside a layout container; use offset for fine-tuning",
			})
			p.Place = nil
		}
		kids = append(kids, p)
	}
	if len(kids) == 0 {
		return
	}

	gap := 1.0
	if lay.Gap != nil && *lay.Gap >= 0 {
		gap = *lay.Gap
	}
	pad := gap
	if lay.Padding != nil && *lay.Padding >= 0 {
		pad = *lay.Padding
	}
	gapW, padW := gap*cell, pad*cell
	align := lay.Align
	if align == "" {
		align = "center"
	}

	cols := 0
	switch lay.Mode {
	case "column":
		cols = 1
	case "grid":
		cols = lay.Cols
		if cols <= 0 {
			cols = int(math.Ceil(math.Sqrt(float64(len(kids)))))
		}
	case "ring":
		// v2.4 — hub-and-spoke: kids[0] is the hub at the centre,
		// kids[1..] sit on a world-space circle around it, starting at
		// the back (-y) and proceeding clockwise. The radius keeps one
		// `gap` between the hub's footprint and each satellite's.
		arrangeRing(kids, gapW, padW, owner, path, issues)
		return
	case "row", "":
		cols = len(kids)
	default:
		*issues = append(*issues, Issue{
			Severity: SeverityError,
			Path:     path + ".layout.mode",
			Message:  fmt.Sprintf("unknown layout mode %q", lay.Mode),
			Suggest:  nearest(lay.Mode, []string{"row", "column", "grid", "ring"}),
		})
		cols = len(kids)
	}
	rows := (len(kids) + cols - 1) / cols

	colW := make([]float64, cols)
	rowD := make([]float64, rows)
	for idx, k := range kids {
		c, r := idx%cols, idx/cols
		w, d := partFootprint(k)
		if w > colW[c] {
			colW[c] = w
		}
		if d > rowD[r] {
			rowD[r] = d
		}
	}
	colX := make([]float64, cols)
	x := padW
	for c := range colW {
		colX[c] = x
		x += colW[c] + gapW
	}
	rowY := make([]float64, rows)
	y := padW
	for r := range rowD {
		rowY[r] = y
		y += rowD[r] + gapW
	}

	for idx, k := range kids {
		c, r := idx%cols, idx/cols
		w, d := partFootprint(k)
		kx := colX[c] + alignSlack(align, colW[c]-w)
		ky := rowY[r] + alignSlack(align, rowD[r]-d)
		bakeOffset(k, kx, ky)
	}

	if owner != nil {
		contentW := padW*2 - gapW + sumPlusGaps(colW, gapW)
		contentD := padW*2 - gapW + sumPlusGaps(rowD, gapW)
		ensureFootprint(owner, contentW, contentD)
	}
}

// arrangeRing places kids[0] at the centre and kids[1..] on a circle
// of radius (hubSpan + satSpan)/2 + gap around it, equally spaced,
// first satellite at the back (-y). Positions are normalised so the
// content bbox starts at padding, then the owner auto-sizes.
func arrangeRing(kids []*CompositePart, gapW, padW float64, owner *CompositePart, path string, issues *[]Issue) {
	if len(kids) < 2 {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Path:     path + ".layout",
			Message:  "ring layout needs a hub plus at least one satellite; falling back to centre placement",
		})
		if len(kids) == 1 {
			bakeOffset(kids[0], padW, padW)
		}
		return
	}
	hw, hd := partFootprint(kids[0])
	maxSat := 0.0
	for _, k := range kids[1:] {
		w, d := partFootprint(k)
		maxSat = math.Max(maxSat, math.Max(w, d))
	}
	radius := (math.Max(hw, hd)+maxSat)/2 + gapW
	n := len(kids) - 1

	// Top-left positions relative to the hub centre at (0, 0).
	xs := make([]float64, len(kids))
	ys := make([]float64, len(kids))
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, k := range kids {
		w, d := partFootprint(k)
		cx, cy := 0.0, 0.0
		if i > 0 {
			theta := -math.Pi/2 + 2*math.Pi*float64(i-1)/float64(n)
			cx, cy = radius*math.Cos(theta), radius*math.Sin(theta)
		}
		xs[i], ys[i] = cx-w/2, cy-d/2
		minX, minY = math.Min(minX, xs[i]), math.Min(minY, ys[i])
		maxX, maxY = math.Max(maxX, xs[i]+w), math.Max(maxY, ys[i]+d)
	}
	for i, k := range kids {
		bakeOffset(k, xs[i]-minX+padW, ys[i]-minY+padW)
	}
	if owner != nil {
		ensureFootprint(owner, (maxX-minX)+2*padW, (maxY-minY)+2*padW)
	}
}

func sumPlusGaps(tracks []float64, gapW float64) float64 {
	total := 0.0
	for _, t := range tracks {
		total += t + gapW
	}
	return total
}

func alignSlack(align string, slack float64) float64 {
	if slack <= 0 {
		return 0
	}
	switch align {
	case "start":
		return 0
	case "end":
		return slack
	default: // center
		return slack / 2
	}
}

// bakeOffset writes a solved position into the part. The author's own
// Offset (if any) survives as a fine-tune delta on top.
func bakeOffset(p *CompositePart, x, y float64) {
	if p.Offset == nil {
		p.Offset = &WorldPoint{}
	}
	p.Offset.WX += x
	p.Offset.WY += y
}

// ensureFootprint grows/sets the owner group's Geom so the substrate
// wraps the arranged content. Explicit author dimensions win.
func ensureFootprint(owner *CompositePart, w, d float64) {
	if owner.Geom == nil {
		owner.Geom = &Geom{}
	}
	if owner.Geom.W <= 0 {
		owner.Geom.W = w
	}
	if owner.Geom.D <= 0 {
		owner.Geom.D = d
	}
}

// ── sibling relations (place: rightOf / leftOf / inFrontOf / behind) ─

// placeSiblings solves the place graph for one sibling set. Returns
// whether any part used place (callers auto-size groups only then, so
// pure-offset legacy documents render byte-identically).
func placeSiblings(parts []*CompositePart, cell float64, path string, issues *[]Issue) bool {
	any := false
	for _, p := range parts {
		if p != nil && p.Place != nil {
			any = true
		}
	}
	if !any {
		return false
	}

	byID := map[string]*CompositePart{}
	idxOf := map[*CompositePart]int{}
	for i, p := range parts {
		if p == nil {
			continue
		}
		idxOf[p] = i
		if p.ID != "" {
			byID[p.ID] = p
		}
	}

	// DFS solve with memo + on-stack cycle detection. pos holds each
	// part's FINAL position (solved constraint + author delta), so
	// dependents follow when the author fine-tunes a reference part.
	pos := map[*CompositePart][3]float64{}
	const (
		stUnseen = 0
		stOnPath = 1
		stDone   = 2
	)
	state := map[*CompositePart]int{}

	var solve func(p *CompositePart) [3]float64
	solve = func(p *CompositePart) [3]float64 {
		if state[p] == stDone {
			return pos[p]
		}
		ppath := fmt.Sprintf("%s.parts[%d].place", path, idxOf[p])
		dx, dy, dz := 0.0, 0.0, 0.0
		if p.Offset != nil {
			dx, dy, dz = p.Offset.WX, p.Offset.WY, p.Offset.WZ
		}
		if p.Place == nil {
			pos[p] = [3]float64{dx, dy, dz}
			state[p] = stDone
			return pos[p]
		}
		if state[p] == stOnPath {
			*issues = append(*issues, Issue{
				Severity: SeverityError,
				Path:     ppath,
				Message:  "place relations form a cycle; falling back to offset for this part",
			})
			pos[p] = [3]float64{dx, dy, dz}
			state[p] = stDone
			return pos[p]
		}
		state[p] = stOnPath

		pl := p.Place
		gap := 1.0
		if pl.Gap != nil && *pl.Gap >= 0 {
			gap = *pl.Gap
		}
		gapX, gapY := gap, gap
		if pl.GapX != nil && *pl.GapX >= 0 {
			gapX = *pl.GapX
		}
		if pl.GapY != nil && *pl.GapY >= 0 {
			gapY = *pl.GapY
		}
		gapXW, gapYW := gapX*cell, gapY*cell
		align := pl.Align
		if align == "" {
			align = "center"
		}

		if pl.RightOf != "" && pl.LeftOf != "" {
			*issues = append(*issues, Issue{
				Severity: SeverityError,
				Path:     ppath,
				Message:  "rightOf and leftOf are mutually exclusive; using rightOf",
			})
			pl.LeftOf = ""
		}
		if pl.InFrontOf != "" && pl.Behind != "" {
			*issues = append(*issues, Issue{
				Severity: SeverityError,
				Path:     ppath,
				Message:  "inFrontOf and behind are mutually exclusive; using inFrontOf",
			})
			pl.Behind = ""
		}

		resolveRef := func(field, ref string) *CompositePart {
			r, ok := byID[ref]
			if !ok {
				*issues = append(*issues, Issue{
					Severity: SeverityError,
					Path:     ppath + "." + field,
					Message:  fmt.Sprintf("place.%s references %q which is not a sibling part id", field, ref),
					Suggest:  nearestSibling(ref, byID),
				})
				return nil
			}
			if r == p {
				*issues = append(*issues, Issue{
					Severity: SeverityError,
					Path:     ppath + "." + field,
					Message:  fmt.Sprintf("place.%s references the part itself", field),
				})
				return nil
			}
			return r
		}

		w, d := partFootprint(p)
		x, y, z := 0.0, 0.0, 0.0
		xConstrained, yConstrained, zConstrained := false, false, false
		var xRef, yRef, zRef *CompositePart

		if pl.RightOf != "" {
			if r := resolveRef("rightOf", pl.RightOf); r != nil {
				rp := solve(r)
				rw, _ := partFootprint(r)
				x = rp[0] + rw + gapXW
				xConstrained, xRef = true, r
			}
		} else if pl.LeftOf != "" {
			if r := resolveRef("leftOf", pl.LeftOf); r != nil {
				rp := solve(r)
				x = rp[0] - gapXW - w
				xConstrained, xRef = true, r
			}
		}
		if pl.InFrontOf != "" {
			if r := resolveRef("inFrontOf", pl.InFrontOf); r != nil {
				rp := solve(r)
				_, rd := partFootprint(r)
				y = rp[1] + rd + gapYW
				yConstrained, yRef = true, r
			}
		} else if pl.Behind != "" {
			if r := resolveRef("behind", pl.Behind); r != nil {
				rp := solve(r)
				y = rp[1] - gapYW - d
				yConstrained, yRef = true, r
			}
		}
		// v2.4 — above: sit flush ON TOP of the sibling (z = its top).
		if pl.Above != "" {
			if r := resolveRef("above", pl.Above); r != nil {
				rp := solve(r)
				z = rp[2] + partHeight(r)
				zConstrained, zRef = true, r
			}
		}

		// Unpinned ground axes align to a reference: prefer the
		// same-plane constraint's ref, else the above-ref (so a bare
		// `above:` centres the part on its base).
		if !xConstrained {
			if ref := firstRef(yRef, zRef); ref != nil {
				rp := solve(ref)
				rw, _ := partFootprint(ref)
				x = rp[0] + alignTrack(align, rw, w)
			}
		}
		if !yConstrained {
			if ref := firstRef(xRef, zRef); ref != nil {
				rp := solve(ref)
				_, rd := partFootprint(ref)
				y = rp[1] + alignTrack(align, rd, d)
			}
		}

		// Author offset degrades to a fine-tune delta on top of the
		// solved (or aligned) position. An axis with no constraint and
		// no alignment falls back to the delta alone, matching the
		// pre-place behavior.
		x += dx
		y += dy
		z += dz
		_ = zConstrained

		pos[p] = [3]float64{x, y, z}
		state[p] = stDone
		p.Offset = &WorldPoint{WX: x, WY: y, WZ: z}
		p.Place = nil
		return pos[p]
	}

	for _, p := range parts {
		if p != nil {
			solve(p)
		}
	}
	return true
}

func firstRef(refs ...*CompositePart) *CompositePart {
	for _, r := range refs {
		if r != nil {
			return r
		}
	}
	return nil
}

func alignTrack(align string, refSpan, span float64) float64 {
	switch align {
	case "start":
		return 0
	case "end":
		return refSpan - span
	default: // center
		return (refSpan - span) / 2
	}
}

func nearestSibling(bad string, byID map[string]*CompositePart) string {
	cand := make([]string, 0, len(byID))
	for id := range byID {
		cand = append(cand, id)
	}
	return nearest(bad, cand)
}

// autosizeGroup wraps a place-arranged group around its children: the
// content bbox is normalised to start at one padding cell from the
// substrate edge, and missing Geom dimensions are derived. Explicit
// author dimensions are respected (no shrink, no shift skip — children
// still normalise so place chains that went negative stay on-board).
func autosizeGroup(owner *CompositePart, parts []*CompositePart, cell float64) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range parts {
		if p == nil {
			continue
		}
		x, y := 0.0, 0.0
		if p.Offset != nil {
			x, y = p.Offset.WX, p.Offset.WY
		}
		w, d := partFootprint(p)
		minX, minY = math.Min(minX, x), math.Min(minY, y)
		maxX, maxY = math.Max(maxX, x+w), math.Max(maxY, y+d)
	}
	if math.IsInf(minX, 1) {
		return
	}
	padW := cell
	for _, p := range parts {
		if p == nil {
			continue
		}
		bakeOffset(p, padW-minX, padW-minY)
	}
	ensureFootprint(owner, (maxX-minX)+2*padW, (maxY-minY)+2*padW)
}

// ── post-solve diagnostics ───────────────────────────────────────────

// checkSiblingOverlaps warns when two siblings' boxes intersect in
// BOTH the ground plane and the z extent (ground overlap alone is fine
// — stacking a part on top of another is a legitimate composition).
func checkSiblingOverlaps(parts []*CompositePart, path string, issues *[]Issue) {
	type box struct {
		idx            int
		id             string
		x0, y0, x1, y1 float64
		z0, z1         float64
	}
	boxes := make([]box, 0, len(parts))
	for i, p := range parts {
		if p == nil || isGhostPart(p) {
			continue
		}
		x, y, z := 0.0, 0.0, 0.0
		if p.Offset != nil {
			x, y, z = p.Offset.WX, p.Offset.WY, p.Offset.WZ
		}
		w, d := partFootprint(p)
		name := p.ID
		if name == "" {
			name = fmt.Sprintf("parts[%d]", i)
		}
		boxes = append(boxes, box{
			idx: i, id: name,
			x0: x, y0: y, x1: x + w, y1: y + d,
			z0: z, z1: z + partHeight(p),
		})
	}
	const eps = 0.01
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			ox := math.Min(a.x1, b.x1) - math.Max(a.x0, b.x0)
			oy := math.Min(a.y1, b.y1) - math.Max(a.y0, b.y0)
			oz := math.Min(a.z1, b.z1) - math.Max(a.z0, b.z0)
			if ox > eps && oy > eps && oz > eps {
				*issues = append(*issues, Issue{
					Severity: SeverityWarning,
					Path:     fmt.Sprintf("%s.parts[%d]", path, b.idx),
					Message: fmt.Sprintf("%q overlaps sibling %q by %.0f×%.0f world units; increase gap or move it",
						b.id, a.id, ox, oy),
				})
			}
		}
	}
}

// isGhostPart reports whether a part is pure decoration that other
// parts may legitimately overlap: every face fill is "none" (dashed
// inset frames, ghost volumes) or it renders as a wireframe.
func isGhostPart(p *CompositePart) bool {
	if p.Style == nil {
		return false
	}
	if e := p.Style.Effects; e != nil && e.Wireframe != nil && *e.Wireframe {
		return true
	}
	if pal := p.Style.Palette; pal != nil &&
		pal.Top == "none" && pal.Left == "none" && pal.Right == "none" {
		return true
	}
	return false
}

// ── validate-time dry run ────────────────────────────────────────────

// layoutIssues runs the solver against a throwaway clone of the node's
// parts tree so Validate can report layout errors/warnings without
// mutating the author's document.
func layoutIssues(n *Node, canvas *Canvas) []Issue {
	if n == nil || n.Shape != "composite" {
		return nil
	}
	clone := &Node{Shape: n.Shape, GridStep: n.GridStep, Parts: cloneParts(n.Parts)}
	if n.Layout != nil {
		l := *n.Layout
		clone.Layout = &l
	}
	return applyLayout(clone, canvas)
}

func cloneParts(parts []*CompositePart) []*CompositePart {
	if parts == nil {
		return nil
	}
	out := make([]*CompositePart, len(parts))
	for i, p := range parts {
		if p == nil {
			continue
		}
		cp := *p
		if p.Offset != nil {
			o := *p.Offset
			cp.Offset = &o
		}
		if p.Geom != nil {
			g := *p.Geom
			cp.Geom = &g
		}
		if p.Place != nil {
			pl := *p.Place
			cp.Place = &pl
		}
		if p.Layout != nil {
			l := *p.Layout
			cp.Layout = &l
		}
		cp.Parts = cloneParts(p.Parts)
		out[i] = &cp
	}
	return out
}
