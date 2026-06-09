// Composite lowering — turn the author-facing parts tree into a flat
// slice the iso25d renderer understands. Handles the v2 primitives
// `group` (nested parts) and `stack` (vertical replication).
package isotopo

import "fmt"

type partInfo struct {
	id      string
	shape   string // v1.6.3 — needed by shape-aware anchor refinement
	w, d, h float64
	offWX, offWY, offWZ float64
	// v1.6 screen-label intent: when set, renderComposite suppresses the
	// part's iso-tilted top-face label and instead splices a horizontal
	// label box under the part's projected bounding box.
	screenLabel    string
	labelBg        string
	labelBorder    string
	labelColor     string
	labelFontSize  float64
}

// renderComposite walks parts and delegates to iso25d.RenderComposite,
// then post-processes the resulting SVG to inject any DSL-declared
// connectors between parts. v1.4 — connectors are now first-class.
// v2 — also lowers `group` shapes (translucent substrate + nested parts)
// and `stack` (vertical replication), then paints the canvas backdrop

func lowerCompositeParts(in []*CompositePart, offX, offY, offZ float64) []*CompositePart {
	out := make([]*CompositePart, 0, len(in))
	for _, p := range in {
		if p == nil {
			continue
		}
		base := translatePart(p, offX, offY, offZ)
		if p.Shape == "group" {
			// Substrate first (z-ordered behind children).
			out = append(out, groupSubstrate(base))
			// base.Offset is the group's ABSOLUTE iso position (it already
			// includes offX/Y/Z). Nested children expect their parent's
			// absolute pos as the new origin, so seed the recursion with
			// base.Offset directly — not offX + base.Offset (that would
			// double-count when we recurse beyond one level of nesting).
			childOffX, childOffY, childOffZ := 0.0, 0.0, offZ+groupSubstrateHeight(base)
			if base.Offset != nil {
				childOffX = base.Offset.WX
				childOffY = base.Offset.WY
			}
			out = append(out, lowerCompositeParts(p.Parts, childOffX, childOffY, childOffZ)...)
			continue
		}
		if p.Stack != nil && p.Stack.Count > 1 {
			out = append(out, expandStack(base, p.Stack)...)
			continue
		}
		out = append(out, base)
	}
	return out
}

// translatePart returns a shallow clone of p with offX/offY/offZ added
// to its Offset. Used by lowerCompositeParts to propagate parent group
// offsets without mutating the author's input tree.
func translatePart(p *CompositePart, offX, offY, offZ float64) *CompositePart {
	if offX == 0 && offY == 0 && offZ == 0 {
		return p
	}
	cp := *p
	if cp.Offset == nil {
		cp.Offset = &WorldPoint{WX: offX, WY: offY, WZ: offZ}
	} else {
		w := *cp.Offset
		w.WX += offX
		w.WY += offY
		w.WZ += offZ
		cp.Offset = &w
	}
	return &cp
}

// expandStack returns Count copies of p with z-stepped offsets. The
// bottom copy keeps the original id; copies above are suffixed "~k" so
// connectors can address a specific layer ("workers~2.right-mid").
func expandStack(p *CompositePart, s *Stack) []*CompositePart {
	gap := s.Gap
	if gap <= 0 {
		gap = 6
		if p.Geom != nil && p.Geom.H > 0 {
			gap = p.Geom.H + 4
		}
	}
	out := make([]*CompositePart, 0, s.Count)
	for k := 0; k < s.Count; k++ {
		cp := *p
		cp.Stack = nil
		if k > 0 {
			cp.ID = fmt.Sprintf("%s~%d", p.ID, k)
		}
		base := WorldPoint{}
		if cp.Offset != nil {
			base = *cp.Offset
		}
		base.WZ += float64(k) * gap
		cp.Offset = &base
		out = append(out, &cp)
	}
	return out
}

// groupSubstrate emits the translucent rounded panel that visually
// represents the group container. Its size is the group's own Geom (so
// the author or agent controls dimensions explicitly — much simpler
// for agent generation than auto-bbox math) and its style defaults to
// a low-opacity tinted panel that reads as "inside this region".
func groupSubstrate(p *CompositePart) *CompositePart {
	cp := *p
	cp.Shape = "rectangle"
	cp.Parts = nil
	if cp.Geom == nil {
		cp.Geom = &Geom{W: 360, D: 240, H: groupSubstrateHeight(p)}
	} else if cp.Geom.H == 0 {
		cp.Geom.H = groupSubstrateHeight(p)
	}
	if cp.Style == nil {
		cp.Style = &Style{}
	}
	if cp.Style.Palette == nil {
		cp.Style.Palette = &Palette{Top: "#EEF1F8", Left: "#C9D1E5", Right: "#D9DFF0"}
	}
	if cp.Style.Stroke == nil {
		w := 1.0
		cp.Style.Stroke = &Stroke{Color: "#7C8DB5", Width: &w}
	}
	if cp.Style.Effects == nil {
		r := 14.0
		cp.Style.Effects = &Effects{CornerRadius: &r}
	} else if cp.Style.Effects.CornerRadius == nil {
		r := 14.0
		cp.Style.Effects.CornerRadius = &r
	}
	return &cp
}

func groupSubstrateHeight(p *CompositePart) float64 {
	if p != nil && p.Geom != nil && p.Geom.H > 0 {
		return p.Geom.H
	}
	return 8
}

// injectCanvasBackground splices a viewBox-sized backdrop rect under
// every visible iso element. Solid fill is one path; iso grid / dots /
// hatch reuse the existing emitBackgroundDefs pattern emitter. Inserted
// as the FIRST child of the <svg> so paint order is bg → parts →

func walkAtomicParts(parts []*CompositePart, fn func(*CompositePart)) {
	for _, p := range parts {
		if p == nil || p.ID == "" {
			if p != nil && p.Shape == "group" {
				walkAtomicParts(p.Parts, fn)
			}
			continue
		}
		if p.Shape == "group" {
			walkAtomicParts(p.Parts, fn)
			continue
		}
		fn(p)
	}
}
