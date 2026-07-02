package isotopo

import "fmt"

// roundedSideBandMin is the cornerRadius at/above which a part's side renders as
// a single continuous "band" (one rounded silhouette) rather than discrete
// faces. Below it the rounding is a thin bevel where the dropped right face is
// visually negligible — so the warning below only fires once the band is real.
const roundedSideBandMin = 6.0

// roundedSideHueGap is the RGB distance above which a dropped right face is a
// real loss rather than normal iso shading. Bundled samples' left↔right shading
// tops out near 135 (same hue, different lightness); a clearly different right
// (e.g. green vs blue ≈ 361) is the footgun. 200 sits comfortably between.
const roundedSideHueGap = 200.0

// RoundedSideIgnoredIssues warns when an explicit solid palette.right is set on a
// part that renders ROUNDED (cornerRadius >= the band threshold, including the
// default-rounded group/boundary slab). The rounded renderer (iso25d/rounded.go)
// collapses the two side walls into one band whose colour comes from top + left
// (left = the band's bottom stop); it never reads the solid right face — so a
// right colour the author set is silently dropped. This is the negative feedback
// for that footgun: set cornerRadius:0 for distinct left/right faces, or fold the
// colour into palette.left / a side gradient.
func RoundedSideIgnoredIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue
	for nodeID, n := range doc.Nodes {
		if n == nil || n.Shape != "composite" {
			continue
		}
		walkRoundedSide(doc.Theme, n.Parts, fmt.Sprintf("nodes.%s", nodeID), &issues)
	}
	return issues
}

func walkRoundedSide(theme *Theme, parts []*CompositePart, path string, issues *[]Issue) {
	for i, p := range parts {
		if p == nil {
			continue
		}
		ppath := fmt.Sprintf("%s.parts[%d]", path, i)
		if p.ID != "" {
			ppath = fmt.Sprintf("%s.parts[%s]", path, p.ID)
		}
		merged := ResolveStyleWithRole(theme, p.Shape, p.Role, p.Preset, p.Style)
		if merged != nil && merged.Palette != nil {
			pal := merged.Palette
			// The band's bottom colour comes from left (or top if left is unset).
			// Only flag when the dropped right is FAR from that — normal iso
			// shading (right a shade off left) renders fine as the band.
			ref := pal.Left
			if ref == "" {
				ref = pal.Top
			}
			if pal.Right != "" && colorDistFar(pal.Right, ref) &&
				effectiveCornerRadius(p, merged) >= roundedSideBandMin {
				*issues = append(*issues, Issue{
					Severity: SeverityWarning,
					Path:     ppath + ".style.palette.right",
					Message: fmt.Sprintf("palette.right %q is ignored on a rounded part (cornerRadius >= %.0f) — the rounded side renders as one band driven by top + left; the right face is dropped. Set effects.cornerRadius: 0 for distinct left/right faces, or move the colour into palette.left / a side gradient.",
						pal.Right, roundedSideBandMin),
					Suggest: "set effects.cornerRadius: 0 for distinct left/right faces, or fold the colour into palette.left",
				})
			}
		}
		if len(p.Parts) > 0 {
			walkRoundedSide(theme, p.Parts, ppath, issues)
		}
	}
}

// effectiveCornerRadius is the radius the part will actually render at: the
// resolved style value if set, else the default-rounded slab for a preset-less
// group/boundary, else 0 (sharp).
func effectiveCornerRadius(p *CompositePart, merged *Style) float64 {
	if merged != nil && merged.Effects != nil && merged.Effects.CornerRadius != nil {
		return *merged.Effects.CornerRadius
	}
	if isContainerShape(p.Shape) && p.Preset == "" {
		return defaultGroupCornerRadius
	}
	return 0
}

// colorDistFar reports whether two colours are perceptually far apart (Euclidean
// RGB distance > roundedSideHueGap). Non-#RRGGBB values (named colours, alpha,
// gradients) are unjudgeable → false, so the check stays quiet rather than
// guessing.
func colorDistFar(a, b string) bool {
	ar, ag, ab, oka := parseHex6(a)
	br, bg, bb, okb := parseHex6(b)
	if !oka || !okb {
		return false
	}
	dr, dg, db := float64(ar-br), float64(ag-bg), float64(ab-bb)
	return dr*dr+dg*dg+db*db > roundedSideHueGap*roundedSideHueGap
}
