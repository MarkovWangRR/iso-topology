package isotopo

import (
	"fmt"
	"math"
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// Render returns the SVG string for a single Node with an optional Theme
// applied as the inherited default Style. The returned SVG carries no
// baked-in background — composition / page bg is the topology's concern.
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

// partInfo is the post-flatten dimensional summary used to project
// connector anchors back to screen coords. Top-level so the injection
// helper can take it as a typed slice.
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
// and screen-space annotation callouts on top.
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

// lowerCompositeParts pre-processes the author-facing parts list into
// a flat slice the existing renderer understands. Two transforms here:
//
//  1. `stack: {count: N, gap: g}` → emit N copies at increasing wz, with
//     suffixed ids so connectors can still target the top of the stack.
//  2. `shape: group` with nested `parts` → emit one low-extrusion
//     translucent substrate rectangle FIRST (painter-ordered behind its
//     children) and then recursively lower the children with the
//     group's offset added on.
//
// The (offX, offY, offZ) accumulator is how nested groups inherit
// their parent's world position.
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
// connectors → screen labels → annotations.
func injectCanvasBackground(svg string, c *Canvas) string {
	if c == nil {
		return svg
	}
	bg := strings.TrimSpace(c.Background)
	grid := strings.ToLower(strings.TrimSpace(c.Grid))
	if bg == "" && grid == "" {
		return svg
	}
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	tagEnd := strings.Index(svg[start:], ">")
	if tagEnd < 0 {
		return svg
	}
	openTag := svg[start : start+tagEnd+1]
	vb := extractAttr(openTag, "viewBox")
	if vb == "" {
		return svg
	}
	var x, y, w, h float64
	if _, err := fmt.Sscanf(vb, "%f %f %f %f", &x, &y, &w, &h); err != nil {
		return svg
	}

	var defs strings.Builder
	fillRef := bg
	if grid != "" && grid != "none" {
		gridColor := c.GridColor
		if gridColor == "" {
			gridColor = "#E2E6EE"
		}
		opts := &RenderOpts{
			BgColor:     bg,
			BgGridColor: gridColor,
			BgGridStep:  c.GridStep,
		}
		switch grid {
		case "iso", "grid":
			opts.Background = BgGrid
		case "dots":
			opts.Background = BgDots
		case "hatch":
			opts.Background = BgHatch
		case "solid":
			opts.Background = BgSolid
		default:
			opts.Background = BgGrid
		}
		fillRef = emitBackgroundDefs(&defs, opts)
	}
	if fillRef == "" {
		return svg
	}

	var sb strings.Builder
	if defs.Len() > 0 {
		fmt.Fprintf(&sb, `<defs>%s</defs>`, defs.String())
	}
	fmt.Fprintf(&sb,
		`<rect data-layer="canvas-bg" x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"/>`,
		x, y, w, h, escAttr(fillRef),
	)
	insertAt := start + tagEnd + 1
	return svg[:insertAt] + sb.String() + svg[insertAt:]
}

// injectScreenLabels appends screen-space horizontal label boxes for any
// part flagged with `style.text.orient: screen`. The label sits centred
// horizontally on the part's projected (top-face centre) and slightly
// below the iso shape's projected bounding box.
func injectScreenLabels(svg string, infos []partInfo) string {
	const (
		cos30 = 0.8660254037844387
		sin30 = 0.5
	)
	project := func(wx, wy, wz float64) (float64, float64) {
		return wx*cos30 - wy*cos30, wx*sin30 + wy*sin30 - wz
	}
	// Same bbox math as injectCompositeConnectors so the (tx, ty) shift
	// matches what iso25d.RenderComposite applied internally.
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	any := false
	for _, p := range infos {
		if p.screenLabel != "" {
			any = true
		}
		corners := [8][3]float64{
			{p.offWX, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ + p.h},
			{p.offWX, p.offWY + p.d, p.offWZ + p.h},
		}
		for _, c := range corners {
			sx, sy := project(c[0], c[1], c[2])
			if sx < minX {
				minX = sx
			}
			if sx > maxX {
				maxX = sx
			}
			if sy < minY {
				minY = sy
			}
			if sy > maxY {
				maxY = sy
			}
		}
	}
	if !any {
		return svg
	}
	pad := 12.0
	tx, ty := -minX+pad, -minY+pad

	var sb strings.Builder
	sb.WriteString(`<g data-layer="screen-labels">`)
	maxLabelY := 0.0
	maxLabelX := 0.0
	for _, p := range infos {
		if p.screenLabel == "" {
			continue
		}
		// Find the projected y of the part's BOTTOM-front corner — that's
		// the lowest screen point of the shape's footprint.
		bottomFrontX, bottomFrontY := project(p.offWX+p.w/2, p.offWY+p.d, p.offWZ)
		cx := bottomFrontX + tx
		baseY := bottomFrontY + ty + 14 // 14px gap under the part

		text := p.screenLabel
		family := "Inter, sans-serif"
		fontSize := p.labelFontSize
		boxW := float64(len(text))*fontSize*0.58 + 16
		boxH := fontSize + 10
		bg := p.labelBg
		if bg == "" {
			bg = "transparent"
		}
		border := p.labelBorder
		if border == "" {
			border = "none"
		}
		color := p.labelColor
		if color == "" {
			color = "#FFFFFF"
		}
		if bg != "transparent" || border != "none" {
			strokeAttr := ""
			if border != "none" {
				strokeAttr = fmt.Sprintf(` stroke="%s" stroke-width="1"`, border)
			}
			fmt.Fprintf(&sb,
				`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="2" ry="2" fill="%s"%s/>`,
				cx-boxW/2, baseY, boxW, boxH, bg, strokeAttr,
			)
		}
		fmt.Fprintf(&sb,
			`<text x="%.2f" y="%.2f" dy=".35em" font-family="%s" font-size="%.1f" font-weight="600" fill="%s" text-anchor="middle">%s</text>`,
			cx, baseY+boxH/2, family, fontSize, color, escapeXML(text),
		)
		if labelBottom := baseY + boxH; labelBottom > maxLabelY {
			maxLabelY = labelBottom
		}
		if labelRight := cx + boxW/2; labelRight > maxLabelX {
			maxLabelX = labelRight
		}
	}
	sb.WriteString(`</g>`)
	idx := strings.LastIndex(svg, "</svg>")
	if idx < 0 {
		return svg
	}
	return growViewBox(svg[:idx]+sb.String()+svg[idx:], maxLabelX+pad, maxLabelY+pad)
}

// growViewBox parses the leading <svg ...> tag and, if needed, expands
// viewBox/width/height so the post-hoc-inserted screen labels are not
// clipped. Idempotent — shrinks to a no-op when the current viewBox is
// already large enough.
func growViewBox(svg string, needW, needH float64) string {
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	end := strings.Index(svg[start:], ">")
	if end < 0 {
		return svg
	}
	end += start
	header := svg[start : end+1]
	rest := svg[end+1:]

	out := header
	// Update viewBox if present and too small.
	if vb := extractAttr(out, "viewBox"); vb != "" {
		var x, y, w, h float64
		if _, err := fmt.Sscanf(vb, "%f %f %f %f", &x, &y, &w, &h); err == nil {
			grew := false
			if needW > x+w {
				w = needW - x
				grew = true
			}
			if needH > y+h {
				h = needH - y
				grew = true
			}
			if grew {
				newVB := fmt.Sprintf("%.2f %.2f %.2f %.2f", x, y, w, h)
				out = replaceAttr(out, "viewBox", newVB)
				out = replaceAttr(out, "width", fmt.Sprintf("%.2f", w))
				out = replaceAttr(out, "height", fmt.Sprintf("%.2f", h))
			}
		}
	}
	return svg[:start] + out + rest
}

func extractAttr(tag, name string) string {
	key := name + `="`
	i := strings.Index(tag, key)
	if i < 0 {
		return ""
	}
	i += len(key)
	j := strings.Index(tag[i:], `"`)
	if j < 0 {
		return ""
	}
	return tag[i : i+j]
}

func replaceAttr(tag, name, val string) string {
	key := name + `="`
	i := strings.Index(tag, key)
	if i < 0 {
		return tag
	}
	i += len(key)
	j := strings.Index(tag[i:], `"`)
	if j < 0 {
		return tag
	}
	return tag[:i] + val + tag[i+j:]
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// injectCompositeConnectors computes the iso-projected screen coords of
// each part's named anchor, then splices <line> elements for each
// connector before the closing </svg>. Mirrors the (tx, ty) shift that
// iso25d.RenderComposite applies internally.
func injectCompositeConnectors(svg string, conns []*Connector, infos []partInfo) string {
	const (
		cos30 = 0.8660254037844387
		sin30 = 0.5
	)
	project := func(wx, wy, wz float64) (float64, float64) {
		return wx*cos30 - wy*cos30, wx*sin30 + wy*sin30 - wz
	}

	// Build the same bbox iso25d.RenderComposite uses for its (tx, ty).
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range infos {
		corners := [8][3]float64{
			{p.offWX, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ + p.h},
			{p.offWX, p.offWY + p.d, p.offWZ + p.h},
		}
		for _, c := range corners {
			sx, sy := project(c[0], c[1], c[2])
			if sx < minX {
				minX = sx
			}
			if sx > maxX {
				maxX = sx
			}
			if sy < minY {
				minY = sy
			}
			if sy > maxY {
				maxY = sy
			}
		}
	}
	pad := 12.0
	tx, ty := -minX+pad, -minY+pad

	byID := map[string]partInfo{}
	for _, p := range infos {
		if p.id != "" {
			byID[p.id] = p
		}
	}

	// parseAnchor splits "partID.anchor" into (id, anchor). Bare "partID"
	// defaults to "top-mid".
	parseAnchor := func(ref string) (id, anchor string) {
		dot := strings.Index(ref, ".")
		if dot < 0 {
			return ref, "top-mid"
		}
		return ref[:dot], ref[dot+1:]
	}

	// anchorWorld returns the iso-world anchor coords for ref = "partID" or
	// "partID.anchor". Anchors default to the top-face centre.
	anchorWorld := func(ref string) (wx, wy, wz float64, ok bool) {
		id, anchor := parseAnchor(ref)
		p, found := byID[id]
		if !found {
			return 0, 0, 0, false
		}
		wx, wy, wz = p.offWX+p.w/2, p.offWY+p.d/2, p.offWZ+p.h
		switch anchor {
		case "left-mid", "left":
			wx, wy = p.offWX, p.offWY+p.d/2
		case "right-mid", "right":
			wx, wy = p.offWX+p.w, p.offWY+p.d/2
		case "back-mid", "back":
			wx, wy = p.offWX+p.w/2, p.offWY
		case "front-mid", "front":
			wx, wy = p.offWX+p.w/2, p.offWY+p.d
		case "top-mid", "top", "center":
			// keep defaults
		case "bottom-mid", "bottom":
			wz = p.offWZ
		}
		return wx, wy, wz, true
	}

	// anchorExit returns the unit outward-normal of an anchor in the iso
	// world's (x, y) ground plane. top/bottom/center have no horizontal
	// normal — caller falls back to the x-axis.
	anchorExit := func(ref string) (dx, dy float64) {
		_, anchor := parseAnchor(ref)
		switch anchor {
		case "left-mid", "left":
			return -1, 0
		case "right-mid", "right":
			return 1, 0
		case "back-mid", "back":
			return 0, -1
		case "front-mid", "front":
			return 0, 1
		}
		return 1, 0
	}

	// anchorFaceMidZ returns the vertical middle (in world z) of the
	// referenced part's side face. Used by the orthogonal router to pick
	// a routing height that lies inside BOTH endpoints' side faces, so
	// every segment of the path lies on a single horizontal world plane
	// and projects to pure ±tan30° iso-axis slopes — i.e. it aligns with
	// the TopoDSL grid lattice with zero off-axis tilt.
	anchorFaceMidZ := func(ref string) float64 {
		id, _ := parseAnchor(ref)
		p, found := byID[id]
		if !found {
			return 0
		}
		return p.offWZ + p.h/2
	}

	// anchorRefineSilhouette adjusts a bbox-based side anchor (wx, wy)
	// onto the actual visible silhouette of non-prismatic shapes:
	//
	//   circle / sphere: the silhouette at a given z is a disc of radius
	//       sqrt(r² − (z − cz)²) centred at the sphere centroid. Anchors
	//       slide inward when z is off the equator.
	//   cloud:           the rendered outline insets from the bbox by
	//       leftX=0.04·w / rightX=0.96·w (matches sampleCloudOutline);
	//       back/front sit on the trunk's top/bottom edges.
	//
	// Other shapes are pass-through (bbox already matches silhouette).
	anchorRefineSilhouette := func(ref string, wx, wy, z float64) (float64, float64) {
		id, anchor := parseAnchor(ref)
		p, found := byID[id]
		if !found {
			return wx, wy
		}
		switch p.shape {
		case "circle":
			cx := p.offWX + p.w/2
			cy := p.offWY + p.d/2
			cz := p.offWZ + p.h/2
			r := math.Min(math.Min(p.w, p.d), p.h) / 2
			dz := z - cz
			if math.Abs(dz) >= r {
				return wx, wy
			}
			rXY := math.Sqrt(r*r - dz*dz)
			switch anchor {
			case "left", "left-mid":
				return cx - rXY, cy
			case "right", "right-mid":
				return cx + rXY, cy
			case "back", "back-mid":
				return cx, cy - rXY
			case "front", "front-mid":
				return cx, cy + rXY
			}
		case "cloud":
			leftX := 0.04 * p.w
			rightX := 0.96 * p.w
			horizonY := 0.10 * p.d // top of bumps row
			bottomY := 0.85 * p.d  // bottom of trunk
			switch anchor {
			case "left", "left-mid":
				return p.offWX + leftX, p.offWY + p.d/2
			case "right", "right-mid":
				return p.offWX + rightX, p.offWY + p.d/2
			case "back", "back-mid":
				return p.offWX + p.w/2, p.offWY + horizonY
			case "front", "front-mid":
				return p.offWX + p.w/2, p.offWY + bottomY
			}
		}
		return wx, wy
	}
	anchorScreen := func(ref string) (float64, float64, bool) {
		wx, wy, wz, ok := anchorWorld(ref)
		if !ok {
			return 0, 0, false
		}
		x, y := project(wx, wy, wz)
		return x + tx, y + ty, true
	}

	// anchorSideKey normalises "id.left-mid" and "id.left" to one key so
	// multiple connectors touching the same side cluster together in the
	// fan-out accounting below.
	anchorSideKey := func(ref string) string {
		id, anchor := parseAnchor(ref)
		switch anchor {
		case "left", "left-mid":
			anchor = "left"
		case "right", "right-mid":
			anchor = "right"
		case "back", "back-mid":
			anchor = "back"
		case "front", "front-mid":
			anchor = "front"
		case "top", "top-mid", "center":
			anchor = "top"
		case "bottom", "bottom-mid":
			anchor = "bottom"
		}
		return id + "/" + anchor
	}

	// v1.6.2 fan-out: count how many orthogonal connectors share each
	// (partID, side) so the per-connector pass can stagger their stubs
	// along the face's tangent and stop them overlapping on the first
	// 24-unit lead-out. Source-side and target-side are bucketed
	// separately — a single anchor may legitimately host both incoming
	// and outgoing channels.
	//
	// v1.6.7 collinear immunity: a connector whose source and target
	// already lie on a single iso axis (so the L collapses to a single
	// segment) does NOT participate in fan-out — shifting it along the
	// tangent would break the collinearity and force a useless kink.
	// Only the genuinely-overlapping subset of edges per anchor needs
	// to be staggered.
	isCollinearConn := make([]bool, len(conns))
	srcSideCount := map[string]int{}
	tgtSideCount := map[string]int{}
	for i, c := range conns {
		if c.Routing != "orthogonal" {
			continue
		}
		sWX, sWY, _, ok1 := anchorWorld(c.From)
		tWX, tWY, _, ok2 := anchorWorld(c.To)
		if !ok1 || !ok2 {
			continue
		}
		sdx, sdy := anchorExit(c.From)
		tdx, tdy := anchorExit(c.To)
		routeZ := math.Min(anchorFaceMidZ(c.From), anchorFaceMidZ(c.To))
		sWX, sWY = anchorRefineSilhouette(c.From, sWX, sWY, routeZ)
		tWX, tWY = anchorRefineSilhouette(c.To, tWX, tWY, routeZ)

		// Collinear iff face normals oppose AND perpendicular distance
		// is zero (within a small tolerance — refinement and fan-out
		// could otherwise nudge things by a fraction).
		const tol = 0.5
		collinear := false
		if sdx == -tdx && sdy == -tdy {
			if math.Abs(sdx) > math.Abs(sdy) {
				if math.Abs(sWY-tWY) < tol {
					collinear = true
				}
			} else if math.Abs(sdy) > math.Abs(sdx) {
				if math.Abs(sWX-tWX) < tol {
					collinear = true
				}
			}
		}
		isCollinearConn[i] = collinear
		if !collinear {
			srcSideCount[anchorSideKey(c.From)]++
			tgtSideCount[anchorSideKey(c.To)]++
		}
	}
	srcSideIdx := map[string]int{}
	tgtSideIdx := map[string]int{}

	var sb strings.Builder
	sb.WriteString(`<g data-layer="connectors">`)
	for ci, c := range conns {
		stroke, width, dash := "#7A8390", 1.4, ""
		if c.Stroke != nil {
			if c.Stroke.Color != "" {
				stroke = c.Stroke.Color
			}
			if c.Stroke.Width != nil && *c.Stroke.Width > 0 {
				width = *c.Stroke.Width
			}
			dash = c.Stroke.Dash
		}
		dashAttr := ""
		if dash != "" {
			dashAttr = fmt.Sprintf(` stroke-dasharray="%s"`, dash)
		}

		// Build the polyline waypoints in screen coords.
		var pts [][2]float64
		switch c.Routing {
		case "orthogonal":
			// Anchor-aware L/Z in the iso world ground plane.
			//
			// Each endpoint first walks `stub` along its face normal (so the
			// line cleanly exits the part's side), then bends along the two
			// world axes to meet the other endpoint's stub. The intermediate
			// axis order is chosen by which axis the source exits along, so
			// the very first segment never crosses the source's footprint.
			//
			// Iso-axis alignment invariant (v1.6.1):
			// In 2.5D iso projection the only directions that align with
			// the TopoDSL diamond grid are world-axis +x and +y; world-axis
			// +z projects to screen vertical which does NOT match the grid.
			// To make every segment strictly ±tan30° in screen, the entire
			// path is routed on a single horizontal world plane at z =
			// min(srcFaceMidZ, tgtFaceMidZ). That height is guaranteed to
			// lie inside both endpoints' side faces (any face-mid z ≤ h),
			// so the endpoints attach inside the visible silhouette and
			// no vertical "drop" segment is ever needed.
			sWX, sWY, _, ok1 := anchorWorld(c.From)
			tWX, tWY, _, ok2 := anchorWorld(c.To)
			if !ok1 || !ok2 {
				continue
			}
			sdx, sdy := anchorExit(c.From)
			tdx, tdy := anchorExit(c.To)
			routeZ := math.Min(anchorFaceMidZ(c.From), anchorFaceMidZ(c.To))

			// v1.6.3 shape-aware anchor refinement: sphere/cloud
			// silhouettes don't reach their bbox edges, so the bbox
			// anchor would render the line ending in empty space. Slide
			// the (wx, wy) along the face normal onto the real silhouette
			// at the chosen routing z.
			sWX, sWY = anchorRefineSilhouette(c.From, sWX, sWY, routeZ)
			tWX, tWY = anchorRefineSilhouette(c.To, tWX, tWY, routeZ)

			// v1.6.2 fan-out: when N>1 connectors share a side, slide
			// each endpoint along the face's tangent (perpendicular to
			// its outward normal, in the world xy plane) so they exit
			// from distinct points. The tangent is a 90° rotation of
			// the normal: (-sdy, sdx). Channel width is fixed in world
			// units; stays well inside the face span for the default
			// 70..200-wide parts used in the demos.
			const channelW = 14.0
			var sStagger, tStagger float64
			// Collinear edges keep stagger 0 — shifting them would
			// break the single-segment route the user laid out for.
			if !isCollinearConn[ci] {
				srcKey := anchorSideKey(c.From)
				tgtKey := anchorSideKey(c.To)
				sN, sIdx := srcSideCount[srcKey], srcSideIdx[srcKey]
				tN, tIdx := tgtSideCount[tgtKey], tgtSideIdx[tgtKey]
				srcSideIdx[srcKey]++
				tgtSideIdx[tgtKey]++
				sStagger = (float64(sIdx) - float64(sN-1)/2) * channelW
				tStagger = (float64(tIdx) - float64(tN-1)/2) * channelW
			}
			sTanX, sTanY := -sdy, sdx
			tTanX, tTanY := -tdy, tdx
			sWX += sTanX * sStagger
			sWY += sTanY * sStagger
			tWX += tTanX * tStagger
			tWY += tTanY * tStagger

			// v1.6.6 arrow-gap pullback: with connectors now beneath the
			// part z-layer, an arrow tip sitting AT the silhouette gets
			// half-occluded by the part. Retract the target endpoint
			// along its outward normal by `arrowGap` world units so the
			// triangle lands fully OUTSIDE the silhouette. Source side
			// stays flush — the line should still appear to emerge from
			// the source face's edge. cos30² + sin30² = 1 so the world
			// magnitude equals the screen magnitude (no axis-dependent
			// scaling needed).
			if c.Arrow == "triangle" {
				const arrowGap = 8.0
				tWX += tdx * arrowGap
				tWY += tdy * arrowGap
			}

			const stub = 24.0
			sStubX, sStubY := sWX+sdx*stub, sWY+sdy*stub
			tStubX, tStubY := tWX+tdx*stub, tWY+tdy*stub

			var worldPts [][3]float64
			if math.Abs(sdx) > math.Abs(sdy) {
				// Source exits along world x → walk x then y.
				worldPts = [][3]float64{
					{sWX, sWY, routeZ},
					{sStubX, sStubY, routeZ},
					{tStubX, sStubY, routeZ},
					{tStubX, tStubY, routeZ},
					{tWX, tWY, routeZ},
				}
			} else {
				// Source exits along world y → walk y then x.
				worldPts = [][3]float64{
					{sWX, sWY, routeZ},
					{sStubX, sStubY, routeZ},
					{sStubX, tStubY, routeZ},
					{tStubX, tStubY, routeZ},
					{tWX, tWY, routeZ},
				}
			}
			// v1.6 — if every waypoint shares the same world x OR the same
			// world y, the L-shape has degenerated to a single iso-axis line.
			// Emit just (source, target) so the path doesn't render multiple
			// collinear bends (which look like a thicker line at line joints).
			const eps = 0.01
			allSameX, allSameY := true, true
			for _, p := range worldPts[1:] {
				if math.Abs(p[0]-worldPts[0][0]) > eps {
					allSameX = false
				}
				if math.Abs(p[1]-worldPts[0][1]) > eps {
					allSameY = false
				}
			}
			if allSameX || allSameY {
				x1, y1 := project(worldPts[0][0], worldPts[0][1], worldPts[0][2])
				last := worldPts[len(worldPts)-1]
				x2, y2 := project(last[0], last[1], last[2])
				pts = append(pts, [2]float64{x1 + tx, y1 + ty})
				pts = append(pts, [2]float64{x2 + tx, y2 + ty})
				break
			}
			// Drop coincident consecutive waypoints so straight-shot
			// connectors don't emit zero-length segments.
			for _, p := range worldPts {
				x, y := project(p[0], p[1], p[2])
				sx, sy := x+tx, y+ty
				if n := len(pts); n > 0 &&
					math.Abs(pts[n-1][0]-sx) < eps && math.Abs(pts[n-1][1]-sy) < eps {
					continue
				}
				pts = append(pts, [2]float64{sx, sy})
			}
		default: // "straight" or empty
			x1, y1, ok1 := anchorScreen(c.From)
			x2, y2, ok2 := anchorScreen(c.To)
			if !ok1 || !ok2 {
				continue
			}
			pts = [][2]float64{{x1, y1}, {x2, y2}}
		}

		// Emit polyline (M ... L ...).
		var d strings.Builder
		for i, p := range pts {
			if i == 0 {
				fmt.Fprintf(&d, "M %.2f,%.2f", p[0], p[1])
			} else {
				fmt.Fprintf(&d, " L %.2f,%.2f", p[0], p[1])
			}
		}
		fmt.Fprintf(&sb,
			`<path data-connector="" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="round"%s/>`,
			d.String(), stroke, width, dashAttr,
		)

		// Arrow on last segment.
		if c.Arrow == "triangle" && len(pts) >= 2 {
			end := pts[len(pts)-1]
			prev := pts[len(pts)-2]
			theta := math.Atan2(end[1]-prev[1], end[0]-prev[0])
			size := 6.0 + width
			tipX, tipY := end[0], end[1]
			b1x := tipX - size*math.Cos(theta) - size*0.5*math.Sin(theta)
			b1y := tipY - size*math.Sin(theta) + size*0.5*math.Cos(theta)
			b2x := tipX - size*math.Cos(theta) + size*0.5*math.Sin(theta)
			b2y := tipY - size*math.Sin(theta) - size*0.5*math.Cos(theta)
			fmt.Fprintf(&sb,
				`<polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s"/>`,
				tipX, tipY, b1x, b1y, b2x, b2y, stroke,
			)
		}

		// Take midpoint for label position from the polyline midpoint.
		x1, y1 := pts[0][0], pts[0][1]
		x2, y2 := pts[len(pts)-1][0], pts[len(pts)-1][1]
		if strings.TrimSpace(c.Label) != "" {
			mx, my := (x1+x2)/2, (y1+y2)/2
			bg := c.LabelBg
			if bg == "" {
				bg = "#FFFFFFEE"
			}
			textW := float64(len(c.Label))*7 + 12
			fmt.Fprintf(&sb,
				`<rect x="%.2f" y="%.2f" width="%.2f" height="20" rx="4" ry="4" fill="%s"/>`,
				mx-textW/2, my-10, textW, bg,
			)
			fmt.Fprintf(&sb,
				`<text x="%.2f" y="%.2f" dy=".35em" font-family="Inter, sans-serif" font-size="11" font-weight="600" fill="#1F2433" text-anchor="middle">%s</text>`,
				mx, my, c.Label,
			)
		}
	}
	sb.WriteString(`</g>`)

	// v1.6.5 — splice the connector layer IMMEDIATELY AFTER the opening
	// <svg ...> tag, BEFORE the <g data-part="..."> blocks. SVG paint
	// order = document order, so this puts every connector underneath
	// every part, letting iso silhouettes occlude lines that cross them
	// (the natural 3D z-order: a node is a body, lines run behind it).
	// Screen labels stay at the document end so they always paint on top.
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	tagEnd := strings.Index(svg[start:], ">")
	if tagEnd < 0 {
		return svg
	}
	insertAt := start + tagEnd + 1
	return svg[:insertAt] + sb.String() + svg[insertAt:]
}

// injectAnnotations paints each callout as a rounded text box plus a
// thin leader line back to its anchor's projected silhouette. Same
// projection math as injectScreenLabels — reused inline so we don't
// have to extract a shared helper across two tight callers.
func injectAnnotations(svg string, anns []*Annotation, infos []partInfo) string {
	if len(anns) == 0 || len(infos) == 0 {
		return svg
	}
	project := func(wx, wy, wz float64) (float64, float64) {
		return wx*cos30 - wy*cos30, wx*sin30 + wy*sin30 - wz
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range infos {
		corners := [8][3]float64{
			{p.offWX, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ + p.h},
			{p.offWX, p.offWY + p.d, p.offWZ + p.h},
		}
		for _, c := range corners {
			sx, sy := project(c[0], c[1], c[2])
			if sx < minX {
				minX = sx
			}
			if sx > maxX {
				maxX = sx
			}
			if sy < minY {
				minY = sy
			}
			if sy > maxY {
				maxY = sy
			}
		}
	}
	pad := 12.0
	tx, ty := -minX+pad, -minY+pad

	byID := make(map[string]partInfo, len(infos))
	for _, p := range infos {
		if p.id != "" {
			byID[p.id] = p
		}
	}

	var sb strings.Builder
	sb.WriteString(`<g data-layer="annotations" font-family="Inter, sans-serif">`)
	maxRightX, maxBottomY := 0.0, 0.0
	minAnnX, minAnnY := math.Inf(1), math.Inf(1)
	for _, a := range anns {
		p, ok := byID[a.Anchor]
		if !ok {
			continue
		}
		// Anchor on the top-face centre — visually the part's "head".
		ax, ay := project(p.offWX+p.w/2, p.offWY+p.d/2, p.offWZ+p.h)
		ax += tx
		ay += ty

		side := a.Side
		if side == "" {
			side = "right"
		}
		dist := a.Distance
		if dist <= 0 {
			dist = 60
		}
		fontSize := a.FontSize
		if fontSize <= 0 {
			fontSize = 12
		}
		bg := a.Bg
		if bg == "" {
			bg = "#FFFFFF"
		}
		border := a.Border
		if border == "" {
			border = "#1F2937"
		}
		color := a.Color
		if color == "" {
			color = "#1F2937"
		}

		lines := strings.Split(a.Text, "\n")
		longest := 0
		for _, l := range lines {
			if len(l) > longest {
				longest = len(l)
			}
		}
		boxW := float64(longest)*fontSize*0.58 + 20
		boxH := float64(len(lines))*(fontSize+4) + 14

		var bx, by float64
		switch side {
		case "top":
			bx = ax - boxW/2
			by = ay - dist - boxH
		case "bottom":
			bx = ax - boxW/2
			by = ay + dist
		case "left":
			bx = ax - dist - boxW
			by = ay - boxH/2
		default: // right
			bx = ax + dist
			by = ay - boxH/2
		}

		// Leader line from anchor to nearest edge of the box.
		var lx2, ly2 float64
		switch side {
		case "top":
			lx2, ly2 = bx+boxW/2, by+boxH
		case "bottom":
			lx2, ly2 = bx+boxW/2, by
		case "left":
			lx2, ly2 = bx+boxW, by+boxH/2
		default:
			lx2, ly2 = bx, by+boxH/2
		}
		fmt.Fprintf(&sb,
			`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1" stroke-dasharray="3 3"/>`,
			ax, ay, lx2, ly2, escAttr(border),
		)
		fmt.Fprintf(&sb,
			`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="6" ry="6" fill="%s" stroke="%s" stroke-width="1"/>`,
			bx, by, boxW, boxH, escAttr(bg), escAttr(border),
		)
		for i, line := range lines {
			ty := by + 12 + float64(i+1)*(fontSize+1) - 4
			fmt.Fprintf(&sb,
				`<text x="%.2f" y="%.2f" font-size="%.2f" fill="%s" text-anchor="middle">%s</text>`,
				bx+boxW/2, ty, fontSize, escAttr(color), escapeXML(line),
			)
		}

		if r := bx + boxW; r > maxRightX {
			maxRightX = r
		}
		if b := by + boxH; b > maxBottomY {
			maxBottomY = b
		}
		if bx < minAnnX {
			minAnnX = bx
		}
		if by < minAnnY {
			minAnnY = by
		}
	}
	sb.WriteString(`</g>`)
	if math.IsInf(minAnnX, 1) {
		minAnnX = 0
	}
	if math.IsInf(minAnnY, 1) {
		minAnnY = 0
	}

	// Annotations on top/left side spill PAST the viewBox origin, so
	// growing only width/height won't help. Re-expand the viewBox in all
	// four directions (and shift the canvas-bg rect to match) before
	// splicing the layer in just before </svg>.
	svg = growViewBoxAround(svg, minSvgRect{
		minX: minAnnX - 8, minY: minAnnY - 8,
		maxX: maxRightX + 8, maxY: maxBottomY + 8,
	})
	closeIdx := strings.LastIndex(svg, "</svg>")
	if closeIdx < 0 {
		return svg
	}
	return svg[:closeIdx] + sb.String() + svg[closeIdx:]
}

type minSvgRect struct {
	minX, minY, maxX, maxY float64
}

// growViewBoxAround expands the SVG's viewBox in all four directions to
// contain the requested rect (in SVG user-space coords). It also
// updates any data-layer="canvas-bg" rect so the backdrop continues to
// fill the full viewBox after expansion. Idempotent — no-op if the
// existing viewBox already contains the rect.
func growViewBoxAround(svg string, need minSvgRect) string {
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	tagEnd := strings.Index(svg[start:], ">")
	if tagEnd < 0 {
		return svg
	}
	tagEnd += start
	header := svg[start : tagEnd+1]
	vb := extractAttr(header, "viewBox")
	if vb == "" {
		return svg
	}
	var x, y, w, h float64
	if _, err := fmt.Sscanf(vb, "%f %f %f %f", &x, &y, &w, &h); err != nil {
		return svg
	}
	x2, y2 := x+w, y+h
	if need.minX < x {
		x = need.minX
	}
	if need.minY < y {
		y = need.minY
	}
	if need.maxX > x2 {
		x2 = need.maxX
	}
	if need.maxY > y2 {
		y2 = need.maxY
	}
	w, h = x2-x, y2-y
	newVB := fmt.Sprintf("%.2f %.2f %.2f %.2f", x, y, w, h)
	header = replaceAttr(header, "viewBox", newVB)
	header = replaceAttr(header, "width", fmt.Sprintf("%.2f", w))
	header = replaceAttr(header, "height", fmt.Sprintf("%.2f", h))
	body := svg[tagEnd+1:]
	// Resize the canvas-bg rect to track the new viewBox.
	body = resizeCanvasBg(body, x, y, w, h)
	return svg[:start] + header + body
}

// resizeCanvasBg finds the data-layer="canvas-bg" rect and rewrites its
// x/y/width/height so it always covers the full viewBox after growth.
func resizeCanvasBg(body string, x, y, w, h float64) string {
	marker := `<rect data-layer="canvas-bg"`
	i := strings.Index(body, marker)
	if i < 0 {
		return body
	}
	end := strings.Index(body[i:], "/>")
	if end < 0 {
		return body
	}
	end += i
	rewritten := fmt.Sprintf(
		`<rect data-layer="canvas-bg" x="%.2f" y="%.2f" width="%.2f" height="%.2f"`,
		x, y, w, h,
	)
	// Preserve the original fill attr by appending whatever was after
	// width=... up to "/>". Simplest robust path: extract fill and append.
	if fill := extractAttr(body[i:end+2], "fill"); fill != "" {
		rewritten += fmt.Sprintf(` fill="%s"`, fill)
	}
	rewritten += `/>`
	return body[:i] + rewritten + body[end+2:]
}

// RenderDocument renders every node in a Document and returns a map of
// node-id → SVG string. The document's Theme is applied to every node.
// v2 — the document's top-level "scene" composite picks up Canvas
// (iso-grid backdrop) and Annotations (screen-space callouts).
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

// walkAtomicParts invokes fn on every NON-group CompositePart in the
// tree (skipping the group containers themselves but recursing into
// their nested parts). This is the canonical "all individually
// embeddable nodes" view of the document.
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
