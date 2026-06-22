package isotopo

import (
	"fmt"
	"math"
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// ── colour helpers ────────────────────────────────────────────────────────────

// parseHex parses #RRGGBB or #RGB; returns (r,g,b,true) or (0,0,0,false).
func parseHex(s string) (r, g, b uint8, ok bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '#' {
		return
	}
	s = s[1:]
	switch len(s) {
	case 6:
		var v uint32
		for _, c := range s {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= uint32(c - '0')
			case c >= 'a' && c <= 'f':
				v |= uint32(c-'a') + 10
			case c >= 'A' && c <= 'F':
				v |= uint32(c-'A') + 10
			default:
				return
			}
		}
		r, g, b = uint8(v>>16), uint8(v>>8), uint8(v)
		ok = true
	case 3:
		var v uint32
		for _, c := range s {
			v <<= 4
			var nib uint32
			switch {
			case c >= '0' && c <= '9':
				nib = uint32(c - '0')
			case c >= 'a' && c <= 'f':
				nib = uint32(c-'a') + 10
			case c >= 'A' && c <= 'F':
				nib = uint32(c-'A') + 10
			default:
				return
			}
			v |= nib
		}
		r = uint8((v>>8)&0xf*17)
		g = uint8((v>>4)&0xf*17)
		b = uint8(v&0xf*17)
		ok = true
	}
	return
}

func sRGBLinear(c uint8) float64 {
	f := float64(c) / 255
	if f <= 0.04045 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

func relativeLuminance(r, g, b uint8) float64 {
	return 0.2126*sRGBLinear(r) + 0.7152*sRGBLinear(g) + 0.0722*sRGBLinear(b)
}

func contrastRatio(l1, l2 float64) float64 {
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// lumOf parses a hex colour and returns (luminance, true) or (0, false).
func lumOf(hex string) (float64, bool) {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return 0, false
	}
	return relativeLuminance(r, g, b), true
}

// topFillColor returns the effective top-face fill colour for a part or its
// defaults.
func topFillColor(st *Style) string {
	if st != nil && st.Palette != nil {
		if st.Palette.Top != "" {
			return st.Palette.Top
		}
		// A gradient top has no single colour; the "from" stop is the
		// representative the eye reads at the lit corner. Without this the
		// check fell through to the default box fill and false-flagged every
		// gradient-topped part (heroes, glows) as low-contrast.
		if st.Palette.TopGradient != nil && st.Palette.TopGradient.From != "" {
			return st.Palette.TopGradient.From
		}
	}
	// Single source of truth: the same default the renderer paints. Hard-coding a
	// different value here made every unstyled part false-fail the contrast check.
	return iso25d.DefaultIsoBox().TopFill
}

// visuallySeparated reports whether a part carries its own separation from the
// canvas — a drop shadow, a silhouette outline, or a stroke. Such a part never
// "vanishes into the background" even when its top fill nearly matches the
// canvas (the intentional white-card-on-light-grey design language), so the
// background-contrast heuristic must not fire on it.
func visuallySeparated(st *Style) bool {
	if st == nil {
		return false
	}
	if st.Stroke != nil && st.Stroke.Color != "" {
		return true
	}
	if st.Effects != nil && (st.Effects.DropShadow != nil || st.Effects.Outline != nil) {
		return true
	}
	return false
}

func textColor(st *Style) string {
	if st != nil && st.Text != nil && st.Text.Color != "" {
		return st.Text.Color
	}
	return "#1E293B"
}

// faceSplitIssues warns when effects.faceSplit is enabled but cannot take
// effect, so the flag is never a silent no-op. It applies only to the rounded
// box family (needs cornerRadius>0): on a sharp box the faces already shade
// independently, and a non-box shape ignores it entirely.
func faceSplitIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue
	for nodeID, n := range doc.Nodes {
		prefix := fmt.Sprintf("nodes.%s", nodeID)
		var walk func(parts []*CompositePart, path string)
		walk = func(parts []*CompositePart, path string) {
			for _, p := range parts {
				if p == nil {
					continue
				}
				pPath := fmt.Sprintf("%s.parts[%s]", path, p.ID)
				eff := ResolveStyle(doc.Theme, p.Shape, p.Preset, p.Style)
				if eff != nil && eff.Effects != nil && eff.Effects.FaceSplit != nil && *eff.Effects.FaceSplit {
					cr := 0.0
					if eff.Effects.CornerRadius != nil {
						cr = *eff.Effects.CornerRadius
					}
					switch {
					case !boxFamilyShape(p.Shape):
						issues = append(issues, Issue{
							Severity: SeverityWarning,
							Path:     pPath + ".style.effects.faceSplit",
							Message:  fmt.Sprintf("faceSplit applies only to the rounded box family; shape %q ignores it", p.Shape),
						})
					case cr <= 0:
						issues = append(issues, Issue{
							Severity: SeverityWarning,
							Path:     pPath + ".style.effects.faceSplit",
							Message:  "faceSplit has no effect without cornerRadius>0 — a sharp box already shades its left/right faces independently",
						})
					}
				}
				walk(p.Parts, pPath)
			}
		}
		walk(n.Parts, prefix)
	}
	return issues
}

// ── 1. VisualContrastIssues ───────────────────────────────────────────────────

// VisualContrastIssues checks colour contrast ratios:
//
//   - top face fill vs label text colour (< 3.0 → warning)
//   - top face fill vs canvas background (< 1.5 → warning)
//   - group/boundary top fill vs direct child top fill (< 1.3 → warning)
func VisualContrastIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue

	// Canvas background luminance
	canvasBg := "#F8FAFC"
	if doc.Canvas != nil && doc.Canvas.Background != "" {
		canvasBg = doc.Canvas.Background
	}
	bgLum, bgOK := lumOf(canvasBg)

	for nodeID, n := range doc.Nodes {
		prefix := fmt.Sprintf("nodes.%s", nodeID)
		var walkContrast func(parts []*CompositePart, pathPrefix string)
		walkContrast = func(parts []*CompositePart, pathPrefix string) {
			for _, p := range parts {
				if p == nil {
					continue
				}
				pPath := fmt.Sprintf("%s.parts[%s]", pathPrefix, p.ID)
				if p.ID == "" {
					pPath = pathPrefix + ".parts[?]"
				}

				// Resolve the EFFECTIVE style the renderer actually paints
				// (theme → per-shape → preset → node overrides) before reading
				// colours — otherwise preset-/gradient-sourced fills are invisible
				// here and the check false-flags them against the default box fill.
				eff := ResolveStyle(doc.Theme, p.Shape, p.Preset, p.Style)
				fill := topFillColor(eff)
				fillLum, fillOK := lumOf(fill)

				if fillOK {
					// a) top fill vs label text — only meaningful when the part
					// actually carries a label on its face.
					txt := textColor(eff)
					if txtLum, ok := lumOf(txt); ok && p.Label != "" {
						cr := contrastRatio(fillLum, txtLum)
						if cr < 3.0 {
							issues = append(issues, Issue{
								Severity: SeverityWarning,
								Path:     pPath + ".style.fill",
								Message:  fmt.Sprintf("low contrast between top fill %s and text %s (ratio %.2f < 3.0)", fill, txt, cr),
							})
						}
					}

					// b) top fill vs canvas background — skipped when the part
					// separates itself from the canvas via shadow/outline/stroke.
					if bgOK && !visuallySeparated(eff) {
						cr := contrastRatio(fillLum, bgLum)
						if cr < 1.5 {
							issues = append(issues, Issue{
								Severity: SeverityWarning,
								Path:     pPath + ".style.fill",
								Message:  fmt.Sprintf("top fill %s is nearly indistinguishable from canvas background %s (ratio %.2f < 1.5)", fill, canvasBg, cr),
							})
						}
					}
				}

				// c) group/boundary vs direct children
				if isContainerShape(p.Shape) && fillOK {
					for _, child := range p.Parts {
						if child == nil {
							continue
						}
						childFill := topFillColor(ResolveStyle(doc.Theme, child.Shape, child.Preset, child.Style))
						if childLum, ok := lumOf(childFill); ok {
							cr := contrastRatio(fillLum, childLum)
							if cr < 1.3 {
								childPath := fmt.Sprintf("%s.parts[%s]", pPath, child.ID)
								issues = append(issues, Issue{
									Severity: SeverityWarning,
									Path:     childPath + ".style.fill",
									Message:  fmt.Sprintf("child top fill %s has low contrast against group fill %s (ratio %.2f < 1.3)", childFill, fill, cr),
								})
							}
						}
					}
					walkContrast(p.Parts, pPath)
				} else {
					walkContrast(p.Parts, pPath)
				}
			}
		}
		walkContrast(n.Parts, prefix)
	}
	return issues
}

// ── 2 + 3. labelIssues ───────────────────────────────────────────────────────

// labelIssues checks label truncation risk (item 2) and label length (item 3).
func labelIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue

	for nodeID, n := range doc.Nodes {
		prefix := fmt.Sprintf("nodes.%s", nodeID)
		var walkLabel func(parts []*CompositePart, pathPrefix string)
		walkLabel = func(parts []*CompositePart, pathPrefix string) {
			for _, p := range parts {
				if p == nil {
					continue
				}
				pPath := fmt.Sprintf("%s.parts[%s]", pathPrefix, p.ID)
				if p.ID == "" {
					pPath = pathPrefix + ".parts[?]"
				}

				// Only leaf parts (non-container, or container but we still check label)
				if p.Label != "" {
					// Item 3: label length
					runes := []rune(p.Label)
					if len(runes) > 40 {
						issues = append(issues, Issue{
							Severity: SeverityWarning,
							Path:     pPath + ".label",
							Message:  fmt.Sprintf("label is long (%d chars); consider using a shorter label or splitting into label+sublabel", len(runes)),
						})
					}

					// Item 2: truncation risk
					if p.Geom != nil && p.Geom.W > 0 {
						estPx := len(p.Label) * 7
						availPx := int(p.Geom.W * 40 * 0.9 * 0.95)
						if estPx > availPx {
							issues = append(issues, Issue{
								Severity: SeverityWarning,
								Path:     pPath + ".label",
								Message:  fmt.Sprintf("label may be truncated: estimated width %dpx exceeds node width %dupx", estPx, int(p.Geom.W*40)),
							})
						}
					}
				}

				walkLabel(p.Parts, pPath)
			}
		}
		walkLabel(n.Parts, prefix)
	}
	return issues
}

// ── 4. outDegreeIssues ───────────────────────────────────────────────────────

// outDegreeIssues warns when a single part has 5 or more outgoing connectors.
func outDegreeIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue

	scene := doc.Scene()
	if scene == nil {
		return nil
	}

	// Count out-degree per part id (strip anchor suffix)
	outCount := map[string]int{}
	firstIdx := map[string]int{}
	for i, c := range scene.Connectors {
		if c.From == "" {
			continue
		}
		id := connectorTarget(c.From)
		outCount[id]++
		if _, seen := firstIdx[id]; !seen {
			firstIdx[id] = i
		}
	}

	// Emit at most one issue per part (the first connector that pushed it to ≥5)
	type hit struct {
		id    string
		count int
		idx   int
	}
	var hits []hit
	for id, cnt := range outCount {
		if cnt >= 5 {
			hits = append(hits, hit{id, cnt, firstIdx[id]})
		}
	}
	// Stable order
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[i].idx > hits[j].idx {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	for _, h := range hits {
		issues = append(issues, Issue{
			Severity: SeverityWarning,
			Path:     fmt.Sprintf("nodes.scene.connectors[%d]", h.idx),
			Message:  fmt.Sprintf("part %q has %d outgoing connectors; consider grouping into a sub-diagram", h.id, h.count),
		})
	}
	return issues
}

// ── 5. nestingIssues ─────────────────────────────────────────────────────────

// nestingIssues warns when group nesting exceeds 3 levels deep.
func nestingIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue

	for nodeID, n := range doc.Nodes {
		prefix := fmt.Sprintf("nodes.%s", nodeID)
		var walkNesting func(parts []*CompositePart, pathPrefix string, depth int)
		walkNesting = func(parts []*CompositePart, pathPrefix string, depth int) {
			for _, p := range parts {
				if p == nil {
					continue
				}
				pPath := fmt.Sprintf("%s.parts[%s]", pathPrefix, p.ID)
				if p.ID == "" {
					pPath = pathPrefix + ".parts[?]"
				}
				if depth > 3 {
					issues = append(issues, Issue{
						Severity: SeverityWarning,
						Path:     pPath,
						Message:  fmt.Sprintf("nesting depth %d exceeds recommended maximum of 3", depth),
					})
				}
				if isContainerShape(p.Shape) {
					walkNesting(p.Parts, pPath, depth+1)
				}
			}
		}
		walkNesting(n.Parts, prefix, 1)
	}
	return issues
}

// ── connectorTierIssues ────────────────────────────────────────────────────────

// connectorTierIssues warns when a connector's two endpoints sit at
// different base heights (wz / tier). Orthogonal connectors hug the
// ground plane, so a height mismatch forces a vertical drop segment
// (riser): fine when it expresses a deliberate cross-tier call, but
// usually it's an accident — peer nodes were given different wz, or one
// endpoint is inside a group whose substrate height lifted it.
func connectorTierIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	scene := doc.Scene()
	if scene == nil || len(scene.Connectors) == 0 {
		return nil
	}

	flat := lowerCompositeParts(scene.Parts, 0, 0, 0)
	wzByID := make(map[string]float64, len(flat))
	for _, p := range flat {
		if p == nil || p.ID == "" {
			continue
		}
		if p.Offset != nil {
			wzByID[p.ID] = p.Offset.WZ
		} else {
			wzByID[p.ID] = 0
		}
	}

	const eps = 0.5
	var issues []Issue
	for i, c := range scene.Connectors {
		if c.From == "" || c.To == "" {
			continue
		}
		fromID := connectorTarget(c.From)
		toID := connectorTarget(c.To)
		fz, ok1 := wzByID[fromID]
		tz, ok2 := wzByID[toID]
		if !ok1 || !ok2 {
			continue
		}
		if math.Abs(fz-tz) > eps {
			issues = append(issues, Issue{
				Severity: SeverityWarning,
				Path:     fmt.Sprintf("nodes.scene.connectors[%d]", i),
				Message: fmt.Sprintf(
					"connector endpoints %q (wz %.0f) and %q (wz %.0f) sit on different tiers; the orthogonal route will sprout a vertical drop segment. Put both on the same wz unless this is a deliberate cross-tier call.",
					fromID, fz, toID, tz),
			})
		}
	}
	return issues
}

