package isotopo

import "math"

// occRect is a screen-space (or world top-down) axis-aligned box.
type occRect struct{ x0, y0, x1, y1 float64 }

func occOverlap(a, b occRect) bool {
	return a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1
}

func occDims(p *CompositePart) (w, d, h float64) {
	w, d, h = 140, 140, 80
	if p.Geom != nil {
		if p.Geom.W > 0 {
			w = p.Geom.W
		}
		if p.Geom.D > 0 {
			d = p.Geom.D
		}
		if p.Geom.H > 0 {
			h = p.Geom.H
		}
	}
	return
}

// occScreenBox projects a part's 8 world corners to its screen bounding box.
func occScreenBox(p *CompositePart) (occRect, bool) {
	if p.Offset == nil {
		return occRect{}, false
	}
	w, d, h := occDims(p)
	ox, oy, oz := p.Offset.WX, p.Offset.WY, p.Offset.WZ
	r := occRect{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, c := range [8][3]float64{
		{ox, oy, oz}, {ox + w, oy, oz}, {ox + w, oy + d, oz}, {ox, oy + d, oz},
		{ox, oy, oz + h}, {ox + w, oy, oz + h}, {ox + w, oy + d, oz + h}, {ox, oy + d, oz + h},
	} {
		sx, sy := projectIso(c[0], c[1], c[2])
		r.x0, r.x1 = math.Min(r.x0, sx), math.Max(r.x1, sx)
		r.y0, r.y1 = math.Min(r.y0, sy), math.Max(r.y1, sy)
	}
	return r, true
}

// occTopBox is the part's top-down (x,y) footprint.
func occTopBox(p *CompositePart) occRect {
	w, d, _ := occDims(p)
	var ox, oy float64
	if p.Offset != nil {
		ox, oy = p.Offset.WX, p.Offset.WY
	}
	return occRect{ox, oy, ox + w, oy + d}
}

// occLabelBoxes returns a group label's screen box and top-down footprint. The
// label is iso-text tilted along world +x from its anchor, so the box spans
// roughly its text width in world x and its font size in z.
func occLabelBoxes(p *CompositePart) (occRect, occRect) {
	size := 11.0
	if p.Style != nil && p.Style.Text != nil && p.Style.Text.Size != nil && *p.Style.Text.Size > 0 {
		size = *p.Style.Text.Size
	}
	tw := float64(len([]rune(p.Label))) * size * 0.62
	var ox, oy, oz float64
	if p.Offset != nil {
		ox, oy, oz = p.Offset.WX, p.Offset.WY, p.Offset.WZ
	}
	scr := occRect{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, c := range [4][3]float64{
		{ox, oy, oz}, {ox + tw, oy, oz}, {ox, oy, oz + size}, {ox + tw, oy, oz + size},
	} {
		sx, sy := projectIso(c[0], c[1], c[2])
		scr.x0, scr.x1 = math.Min(scr.x0, sx), math.Max(scr.x1, sx)
		scr.y0, scr.y1 = math.Min(scr.y0, sy), math.Max(scr.y1, sy)
	}
	return scr, occRect{ox, oy, ox + tw, oy + size}
}

// resolveGroupLabelOcclusion is the missing auto-layout constraint: a group's
// label must not be covered by an upper-layer node. It judges occlusion in BOTH
// the 2.5D screen projection (a taller node BEHIND the label in the top-down
// plan can still cover it once projected) and the top-down footprint, then
// repaints any occluded label LAST so it sits on top. Labels that are already
// clear keep their position, so the common case is unchanged.
func resolveGroupLabelOcclusion(flat []*CompositePart) []*CompositePart {
	hasLabel := false
	for _, p := range flat {
		if p != nil && p.groupLabel {
			hasLabel = true
			break
		}
	}
	if !hasLabel {
		return flat
	}

	occluded := make([]bool, len(flat))
	for i, lb := range flat {
		if lb == nil || !lb.groupLabel {
			continue
		}
		lbScr, lbTop := occLabelBoxes(lb)
		// Only LATER-painted parts can cover the label; substrates/slabs sit
		// below it and other labels never occlude.
		for j := i + 1; j < len(flat); j++ {
			q := flat[j]
			if q == nil || q.groupLabel || q.isSubstrate || q.Shape == "iso_text" || isContainerShape(q.Shape) {
				continue
			}
			qScr, ok := occScreenBox(q)
			if !ok {
				continue
			}
			// 2.5D cover OR top-down cover (the user asked for both views).
			if occOverlap(lbScr, qScr) || occOverlap(lbTop, occTopBox(q)) {
				occluded[i] = true
				break
			}
		}
	}

	out := make([]*CompositePart, 0, len(flat))
	var lifted []*CompositePart
	for i, p := range flat {
		if occluded[i] {
			lifted = append(lifted, p)
		} else {
			out = append(out, p)
		}
	}
	return append(out, lifted...)
}
