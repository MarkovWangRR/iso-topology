package iso25d

import (
	"fmt"
	"math"
	"strings"
)

// RenderIsoBoxRounded draws a true iso-extruded rounded rectangle prism:
//   - the TOP FACE is a complete rounded rectangle (sampled outline)
//     projected to the iso top plane via the project() function — visually a
//     rounded rhombus
//   - the SIDE WALL is a thin band between the top face's *front-visible*
//     outline portion and the same portion shifted down by Height
//   - one continuous silhouette stroke traces the visible outer boundary
//
// This matches the Vite/JS cushion-badge baseline: distinct top + side, sharp
// outline, single colour on each face. CornerRadius is in local (W, D) units.
func RenderIsoBoxRounded(o IsoBoxOpts) string {
	w, d, h, m := o.Width, o.Depth, o.Height, o.Margin
	if m <= 0 {
		m = 24
	}
	r := o.CornerRadius
	if r > w/2 {
		r = w / 2
	}
	if r > d/2 {
		r = d / 2
	}
	if r <= 0 {
		r = math.Min(w, d) * 0.18
	}

	outline := sampleRoundedRectCW(w, d, r)
	n := len(outline)

	p := make([]roundedPr, n)
	minX := math.Inf(1)
	maxX := math.Inf(-1)
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for i, q := range outline {
		sx, sy := project(q[0], q[1], 0)
		p[i] = roundedPr{topX: sx, topY: sy - h, botX: sx, botY: sy}
		if sx < minX {
			minX = sx
		}
		if sx > maxX {
			maxX = sx
		}
		if sy-h < minY {
			minY = sy - h
		}
		if sy > maxY {
			maxY = sy
		}
	}
	tx, ty := -minX+m, -minY+m
	viewW := maxX - minX + 2*m
	viewH := maxY - minY + 2*m
	for i := range p {
		p[i].topX += tx
		p[i].topY += ty
		p[i].botX += tx
		p[i].botY += ty
	}

	// Front-facing test in local frame (screen-CW outline → outward normal
	// = rotate tangent 90° CW; visible against iso (1,1,0) camera when
	// dy > dx where (dx, dy) is segment tangent in local coords).
	visible := make([]bool, n)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		dx := outline[j][0] - outline[i][0]
		dy := outline[j][1] - outline[i][1]
		visible[i] = dy > dx
	}

	// Find the single visible run [transStart .. transEnd] inclusive.
	transStart, transEnd := -1, -1
	for i := 0; i < n; i++ {
		prev := (i - 1 + n) % n
		if visible[i] && !visible[prev] {
			transStart = i
		}
		if !visible[i] && visible[prev] {
			transEnd = prev
		}
	}

	var sb strings.Builder
	sb.WriteString(svgHeader(viewW, viewH))

	// Drop shadow filter def.
	shadowID := ""
	hasShadow := strings.TrimSpace(o.ShadowColor) != "" &&
		(o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0)
	if hasShadow {
		shadowID = "rounded-shadow"
	}

	// Side gradient def — vertical, RightFill (lighter) at top → LeftFill
	// at bottom. v2.2 — author Left/RightGradient win over solid fills:
	// the rounded silhouette has a single continuous side band, so the
	// two face gradients collapse onto it as (right.from → left.to),
	// mirroring the solid right-top/left-bottom convention.
	topStop := o.RightFill
	botStop := o.LeftFill
	if o.RightGradient != nil && strings.TrimSpace(o.RightGradient.From) != "" {
		topStop = o.RightGradient.From
	}
	if o.LeftGradient != nil && strings.TrimSpace(o.LeftGradient.To) != "" {
		botStop = o.LeftGradient.To
	}
	if topStop == "" {
		topStop = botStop
	}
	if botStop == "" {
		botStop = topStop
	}
	wantGradient := !o.Wireframe && botStop != ""
	var defs strings.Builder
	if hasShadow {
		emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
	}
	gradID := ""
	if wantGradient {
		gradID = "rounded-side"
		fmt.Fprintf(&defs,
			`<linearGradient id="%s" x1="50%%" y1="0%%" x2="50%%" y2="100%%"><stop offset="0%%" stop-color="%s"/><stop offset="100%%" stop-color="%s"/></linearGradient>`,
			gradID, escapeAttr(topStop), escapeAttr(botStop),
		)
	}
	// v2.1 — author-specified top-face linear gradient. When set, it
	// overrides the solid TopFill and is referenced via url(#grad-top).
	topGradID := ""
	if o.TopGradient != nil && strings.TrimSpace(o.TopGradient.From) != "" {
		topGradID = "rounded-top"
		emitLinearGradient(&defs, topGradID, o.TopGradient)
	}
	// v2.1 — backglow filter: an oversized blur that gets painted as a
	// halo behind the silhouette. We use feGaussianBlur on a flood so the
	// halo respects the part's footprint without requiring an extra path.
	backglowID := ""
	if strings.TrimSpace(o.BackglowColor) != "" && o.BackglowRadius > 0 {
		backglowID = "rounded-backglow"
		op := o.BackglowOpacity
		if op <= 0 {
			op = 0.6
		}
		fmt.Fprintf(&defs,
			`<filter id="%s" x="-50%%" y="-50%%" width="200%%" height="200%%">`+
				`<feGaussianBlur in="SourceGraphic" stdDeviation="%.2f"/>`+
				`<feComponentTransfer><feFuncA type="linear" slope="%.2f"/></feComponentTransfer>`+
				`</filter>`,
			backglowID, o.BackglowRadius, op,
		)
	}
	// v2.3 — hatch/dots texture overlaid on the top face.
	patID := emitPatternDef(&defs, "rounded-pattern", o.PatternKind, o.PatternColor, o.PatternSpacing, o.PatternAngle)
	// v2.6 — film-grain noise over the faces.
	grainID := emitGrainFilter(&defs, "rounded-grain", o.GrainIntensity, o.GrainScale)
	if defs.Len() > 0 {
		sb.WriteString(`<defs>`)
		sb.WriteString(defs.String())
		sb.WriteString(`</defs>`)
	}

	// Wrapper.
	wrapAttrs := ""
	if o.Opacity > 0 && o.Opacity < 1 {
		wrapAttrs += fmt.Sprintf(` opacity="%.3f"`, o.Opacity)
	}
	// v3.0 — stroke.dash was honoured by every box renderer except this
	// one, so any rounded part (incl. every group substrate) silently
	// rendered solid outlines.
	if strings.TrimSpace(o.StrokeDasharray) != "" {
		wrapAttrs += fmt.Sprintf(` stroke-dasharray="%s"`, escapeAttr(o.StrokeDasharray))
	}
	if shadowID != "" {
		wrapAttrs += fmt.Sprintf(` filter="url(#%s)"`, shadowID)
	}
	// v2.1 — paint the backglow halo first, so it sits behind every
	// visible face. We re-use the top-face perimeter as the halo shape
	// and rely on the SVG filter chain to blur + fade it.
	if backglowID != "" {
		fmt.Fprintf(&sb,
			`<path data-face="backglow" d="%s" fill="%s" stroke="none" filter="url(#%s)"/>`,
			perimeterPath(p, false), escapeAttr(o.BackglowColor), backglowID,
		)
	}
	fmt.Fprintf(&sb, `<g data-face="wrap"%s>`, wrapAttrs)
	if grainID != "" {
		fmt.Fprintf(&sb, `<g filter="url(#%s)">`, grainID)
	}

	stroke := o.Stroke
	if stroke == "" {
		stroke = "#1F2433"
	}
	sw := o.StrokeWidth
	if sw <= 0 {
		sw = 1.4
	}
	topFill := o.TopFill
	if topFill == "" {
		topFill = "#FFFFFF"
	}
	if topGradID != "" {
		topFill = "url(#" + topGradID + ")"
	}
	sideFill := "none"
	if wantGradient && gradID != "" {
		sideFill = "url(#" + gradID + ")"
	} else if o.LeftFill != "" {
		sideFill = o.LeftFill
	}

	// Build paths.
	frontIdx := []int{}
	if transStart >= 0 && transEnd >= 0 {
		i := transStart
		for {
			frontIdx = append(frontIdx, i)
			if i == transEnd {
				break
			}
			i = (i + 1) % n
			if len(frontIdx) > n {
				break
			}
		}
	}

	if o.Wireframe {
		// Top face stroked outline.
		fmt.Fprintf(&sb, `<path data-face="top-wire" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"/>`,
			perimeterPath(p, false), escapeAttr(stroke), sw)
		if len(frontIdx) >= 2 {
			// Vertical edges at the two transitions.
			ts, te := frontIdx[0], frontIdx[len(frontIdx)-1]
			fmt.Fprintf(&sb, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.2f" stroke-linecap="round"/>`,
				p[ts].topX, p[ts].topY, p[ts].botX, p[ts].botY, escapeAttr(stroke), sw)
			fmt.Fprintf(&sb, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.2f" stroke-linecap="round"/>`,
				p[te].topX, p[te].topY, p[te].botX, p[te].botY, escapeAttr(stroke), sw)
			// Bottom front-arc.
			fmt.Fprintf(&sb, `<path data-face="bot-front-wire" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"/>`,
				botFrontPath(p, frontIdx), escapeAttr(stroke), sw)
		}
	} else {
		// 1. Side wall fill (top-front-arc + right vertical + reverse bot-front-arc + left vertical).
		if len(frontIdx) >= 2 {
			fmt.Fprintf(&sb, `<path data-face="side" d="%s" fill="%s" stroke="none"/>`,
				sideWallPath(p, frontIdx), sideFill,
			)
		}
		// 2. Top face filled + stroked (full rounded perimeter at z=h).
		fmt.Fprintf(&sb, `<path data-face="top" d="%s" fill="%s" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"/>`,
			perimeterPath(p, false), topFill, escapeAttr(stroke), sw)
		if patID != "" {
			fmt.Fprintf(&sb, `<path data-face="top-pattern" d="%s" fill="url(#%s)" stroke="none"/>`,
				perimeterPath(p, false), patID)
		}
		// 3. Silhouette stroke for the parts NOT on the top-face path: two
		//    vertical edges + the bot-front-arc.
		if len(frontIdx) >= 2 {
			fmt.Fprintf(&sb, `<path data-face="silhouette" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round" stroke-linecap="round"/>`,
				silhouetteSkirtPath(p, frontIdx), escapeAttr(stroke), sw,
			)
		}
	}

	// Top-face content: 2D cell grid OR label/icon (cells take precedence,
	// same rule as the regular box renderer — v1.3 patch that closes the
	// cornerRadius/cells mutual-exclusion gap surfaced by the Vite repro).
	topOriginX, topOriginY := project(0, 0, h)
	topOriginX += tx
	topOriginY += ty
	if grainID != "" {
		sb.WriteString(`</g>`)
	}
	writeTopLabelAndIconV12(
		&sb,
		topOriginX, topOriginY, w, d,
		o.Label, o.LabelLines, o.Icon, o.IconScale, o.IconAnchor, o.IconOffX, o.IconOffY,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	sb.WriteString(`</g></svg>`)
	return sb.String()
}

// sampleRoundedRectCW returns a screen-CW polyline of the rounded rectangle
// outline starting at (r, 0). Interior is on the RIGHT of motion.
func sampleRoundedRectCW(w, d, r float64) [][2]float64 {
	nEdge := 18
	nCorner := 12
	out := make([][2]float64, 0, 4*nEdge+4*nCorner+4)

	add := func(x, y float64) { out = append(out, [2]float64{x, y}) }

	// Top edge (r, 0) → (W-r, 0).
	for i := 0; i <= nEdge; i++ {
		t := float64(i) / float64(nEdge)
		add(r+t*(w-2*r), 0)
	}
	// Top-right corner: centre (W-r, r), θ from -π/2 to 0.
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		theta := -math.Pi/2 + t*math.Pi/2
		add((w-r)+r*math.Cos(theta), r+r*math.Sin(theta))
	}
	// Right edge (W, r) → (W, D-r).
	for i := 1; i <= nEdge; i++ {
		t := float64(i) / float64(nEdge)
		add(w, r+t*(d-2*r))
	}
	// Bottom-right corner: centre (W-r, D-r), θ from 0 to π/2.
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		theta := t * math.Pi / 2
		add((w-r)+r*math.Cos(theta), (d-r)+r*math.Sin(theta))
	}
	// Bottom edge (W-r, D) → (r, D).
	for i := 1; i <= nEdge; i++ {
		t := float64(i) / float64(nEdge)
		add((w-r)+t*(2*r-w), d)
	}
	// Bottom-left corner: centre (r, D-r), θ from π/2 to π.
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		theta := math.Pi/2 + t*math.Pi/2
		add(r+r*math.Cos(theta), (d-r)+r*math.Sin(theta))
	}
	// Left edge (0, D-r) → (0, r).
	for i := 1; i <= nEdge; i++ {
		t := float64(i) / float64(nEdge)
		add(0, (d-r)+t*(2*r-d))
	}
	// Top-left corner: centre (r, r), θ from π to 3π/2.
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		theta := math.Pi + t*math.Pi/2
		add(r+r*math.Cos(theta), r+r*math.Sin(theta))
	}
	return out
}

type roundedPr struct {
	topX, topY, botX, botY float64
}

// Helpers operate on the projected sample array (named pr for brevity).
func perimeterPath(p []roundedPr, bot bool) string {
	var sb strings.Builder
	if bot {
		fmt.Fprintf(&sb, "M %.2f,%.2f", p[0].botX, p[0].botY)
		for i := 1; i < len(p); i++ {
			fmt.Fprintf(&sb, " L %.2f,%.2f", p[i].botX, p[i].botY)
		}
	} else {
		fmt.Fprintf(&sb, "M %.2f,%.2f", p[0].topX, p[0].topY)
		for i := 1; i < len(p); i++ {
			fmt.Fprintf(&sb, " L %.2f,%.2f", p[i].topX, p[i].topY)
		}
	}
	sb.WriteString(" Z")
	return sb.String()
}

func sideWallPath(p []roundedPr, frontIdx []int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "M %.2f,%.2f", p[frontIdx[0]].topX, p[frontIdx[0]].topY)
	for _, idx := range frontIdx[1:] {
		fmt.Fprintf(&sb, " L %.2f,%.2f", p[idx].topX, p[idx].topY)
	}
	last := frontIdx[len(frontIdx)-1]
	fmt.Fprintf(&sb, " L %.2f,%.2f", p[last].botX, p[last].botY)
	for j := len(frontIdx) - 2; j >= 0; j-- {
		idx := frontIdx[j]
		fmt.Fprintf(&sb, " L %.2f,%.2f", p[idx].botX, p[idx].botY)
	}
	sb.WriteString(" Z")
	return sb.String()
}

func silhouetteSkirtPath(p []roundedPr, frontIdx []int) string {
	var sb strings.Builder
	first := frontIdx[0]
	last := frontIdx[len(frontIdx)-1]
	// Left vertical edge: top -> bot
	fmt.Fprintf(&sb, "M %.2f,%.2f L %.2f,%.2f",
		p[first].topX, p[first].topY,
		p[first].botX, p[first].botY,
	)
	// Bottom front-arc
	for _, idx := range frontIdx[1:] {
		fmt.Fprintf(&sb, " L %.2f,%.2f", p[idx].botX, p[idx].botY)
	}
	// Right vertical edge: bot -> top
	fmt.Fprintf(&sb, " L %.2f,%.2f", p[last].topX, p[last].topY)
	return sb.String()
}

func botFrontPath(p []roundedPr, frontIdx []int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "M %.2f,%.2f", p[frontIdx[0]].botX, p[frontIdx[0]].botY)
	for _, idx := range frontIdx[1:] {
		fmt.Fprintf(&sb, " L %.2f,%.2f", p[idx].botX, p[idx].botY)
	}
	return sb.String()
}

var _ = roundedPr{} // keep type comment around for readers
