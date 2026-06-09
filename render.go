// Public rendering API: Document/Node → SVG string.
//
// This file is intentionally small. It holds only the four exported
// entry points and the renderComposite orchestrator that calls the
// lowering pass (lower.go) and the SVG layer injectors (inject.go).
//
// Anything below the API in the stack lives in dedicated files:
//
//	lower.go      composite/group/stack expansion
//	inject.go     canvas-bg / screen-label / connector / annotation injectors
//	svgutil.go    SVG-string parse/edit helpers used by injectors
package isotopo

import (
	"github.com/MarkovWangRR/iso-topology/iso25d"
)

func Render(n *Node, theme *Theme) string {
	return RenderWithCanvas(n, theme, nil, nil)
}

// RenderWithCanvas is Render plus document-level canvas + annotations:
// the canvas controls the iso-aware background (ground grid / dots /
// hatch); the annotations are screen-space callouts pinned to specific
// composite parts via leader lines. Both layers are optional — passing
// nil produces exactly what Render does.
func RenderWithCanvas(n *Node, theme *Theme, canvas *Canvas, anns []*Annotation) string {
	if n == nil {
		return ""
	}
	if n.Shape == "composite" {
		return renderComposite(n, theme, canvas, anns)
	}
	shape, opts := Flatten(n, theme)
	return iso25d.Convert2DTo25D(shape, opts)
}

// renderComposite walks a composite node's parts, calls the lowering
// pass to expand groups and stacks, delegates the iso geometry to
// iso25d.RenderComposite, then layers in canvas / connectors / screen
// labels / annotations through the injectors in inject.go.
func renderComposite(n *Node, theme *Theme, canvas *Canvas, anns []*Annotation) string {
	flat := lowerCompositeParts(n.Parts, 0, 0, 0)
	if len(flat) == 0 {
		return ""
	}

	infos := make([]partInfo, len(flat))

	parts := make([]iso25d.CompositePart, 0, len(flat))
	for i, p := range flat {
		// v1.6 — screen-orient labels are rendered separately below as
		// SVG-screen-space boxes. Strip Label from the iso flatten so the
		// shape doesn't double-print it on the top face. We must look at
		// the *merged* style (theme → per-shape → part) so theme-level
		// `text.orient: screen` propagates to every part.
		mergedForLabel := ResolveStyle(theme, p.Shape, p.Style)
		isoLabel := p.Label
		var screenLabel, labelBg, labelBorder, labelColor string
		var labelSize float64 = 11
		if mergedForLabel != nil && mergedForLabel.Text != nil && mergedForLabel.Text.Orient == "screen" {
			screenLabel = p.Label
			isoLabel = ""
			labelBg = mergedForLabel.Text.BoxBg
			labelBorder = mergedForLabel.Text.BoxBorder
			labelColor = mergedForLabel.Text.Color
			if mergedForLabel.Text.Size != nil && *mergedForLabel.Text.Size > 0 {
				labelSize = *mergedForLabel.Text.Size
			}
		}
		sub := &Node{
			Shape:   p.Shape,
			Geom:    p.Geom,
			Style:   p.Style,
			Label:   isoLabel,
			Icon:    p.Icon,
			Content: p.Content,
		}
		shape, opts := Flatten(sub, theme)

		ox, oy, oz := 0.0, 0.0, 0.0
		if p.Position != nil && n.GridStep > 0 {
			ox = float64(p.Position.I) * n.GridStep
			oy = float64(p.Position.J) * n.GridStep
		}
		if p.Offset != nil {
			ox += p.Offset.WX
			oy += p.Offset.WY
			oz += p.Offset.WZ
		}

		cp := iso25d.CompositePart{Shape: shape, Opts: opts, OffWX: ox, OffWY: oy, OffWZ: oz}
		parts = append(parts, cp)

		w, d, h := 140.0, 140.0, 80.0
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
		infos[i] = partInfo{
			id: p.ID, shape: p.Shape,
			w: w, d: d, h: h, offWX: ox, offWY: oy, offWZ: oz,
			screenLabel: screenLabel, labelBg: labelBg, labelBorder: labelBorder,
			labelColor: labelColor, labelFontSize: labelSize,
		}
	}

	svg := iso25d.RenderComposite(parts)
	// Paint order matters: each "insert after <svg>" call pushes the
	// previous insertion deeper into the body, so the LAST call ends up
	// first (= behind everything). Connectors must paint UNDER parts but
	// OVER the canvas backdrop, so the order here is:
	//
	//   1. connectors first (will end up just below parts)
	//   2. canvas-bg last  (will end up at the very top of doc order
	//                       = painted first = furthest back)
	if len(n.Connectors) > 0 {
		svg = injectCompositeConnectors(svg, n.Connectors, infos)
	}
	if canvas != nil {
		svg = injectCanvasBackground(svg, canvas)
	}
	svg = injectScreenLabels(svg, infos)
	if len(anns) > 0 {
		svg = injectAnnotations(svg, anns, infos)
	}
	return svg
}

// RenderDocument renders every node in a Document and returns a map of
// node-id → SVG string. The "scene" node (canonical convention for the
// document-level composite) also picks up the document's Canvas and
// Annotations layers; all other nodes are rendered without them.
func RenderDocument(doc *Document) map[string]string {
	out := make(map[string]string, len(doc.Nodes))
	for id, n := range doc.Nodes {
		if id == "scene" {
			out[id] = RenderWithCanvas(n, doc.Theme, doc.Canvas, doc.Annotations)
			continue
		}
		out[id] = Render(n, doc.Theme)
	}
	return out
}

// RenderParts produces one standalone SVG per "atomic" element in the
// document — the second tier of the dsl2topo output.
//
// For composite scenes (the canonical case — both .d2 auto-layout and
// most YAML examples produce a single "scene" composite), each
// CompositePart is lifted into its own Node and rendered in isolation
// so callers can embed an individual icon independently. v2 — recurses
// into `group` parts so nested elements get their own per-part output
// too (otherwise grouping would hide individual nodes from the gallery).
//
// For non-composite documents, the top-level Nodes ARE the atomic
// elements and this delegates to RenderDocument.
func RenderParts(doc *Document) map[string]string {
	out := make(map[string]string)
	for id, n := range doc.Nodes {
		if n.Shape != "composite" {
			out[id] = Render(n, doc.Theme)
			continue
		}
		walkAtomicParts(n.Parts, func(p *CompositePart) {
			sub := &Node{
				Shape:   p.Shape,
				Geom:    p.Geom,
				Style:   p.Style,
				Label:   p.Label,
				Icon:    p.Icon,
				Content: p.Content,
			}
			out[p.ID] = Render(sub, doc.Theme)
		})
		_ = id
	}
	return out
}

