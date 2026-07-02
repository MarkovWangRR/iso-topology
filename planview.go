package isotopo

import (
	"fmt"
	"math"
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// planview.go renders a composite scene as a flat TOP-DOWN (plan) view —
// the "projection: top" mode. Height (z) is dropped: every part becomes its
// footprint rectangle at world (wx, wy) sized (w, d), and connectors route
// orthogonally between footprint faces. It reads as a normal 2D node-and-edge
// diagram (a floor plan / flowchart), which is what you want when eyeballing
// the layout rather than the 3D form.
//
// This is a SEPARATE renderer from the isometric pipeline (render.go / iso25d):
// it shares the layout solver (applyLayout) and the style cascade (ResolveStyle)
// but emits its own flat SVG, so isometric output — and every golden file — is
// untouched.

const (
	planPad       = 28.0 // breathing margin around the content bbox (px)
	planRadius    = 6.0
	planFontTitle = 13.0
	planFontLabel = 11.0
)

type planRect struct {
	id    string
	label string
	x, y  float64 // absolute world corner
	w, d  float64
	z, h  float64 // world z-floor + height — kept only so the evaluator can
	// tell same-floor obstacles from things stacked above/below
	fill      string
	stroke    string
	textColor string
	container bool
	boundary  bool // dashed outline-only container
}

// planEdge is one connector resolved to a world-space orthogonal route between
// two footprints — the shared geometry the renderer draws and the evaluator
// measures.
type planEdge struct {
	ci       int // index in the scene's Connectors (matches data-connector)
	c        *Connector
	from, to planRect
	pts      [][2]float64 // world route, source→target
}

// buildPlanModel resolves a scene to its flat geometry with the scorecard-
// guided router (the better out-of-box default).
func buildPlanModel(n *Node, theme *Theme, canvas *Canvas) (rects []planRect, byID map[string]planRect, edges []planEdge) {
	return buildPlanModelOpt(n, theme, canvas, true)
}

// buildPlanModelOpt resolves footprint rects (planCollect order, containers
// before children), an id index, and the routed edges. With optimize=false it
// uses the baseline dominant-axis router; with optimize=true it picks, per edge,
// the candidate route that minimises the scorecard cost (tunnelling, crossings,
// bends, length) against the nodes and the edges placed so far. The two modes
// share everything else, so an A/B holds node positions fixed and varies only
// the routing decision.
func buildPlanModelOpt(n *Node, theme *Theme, canvas *Canvas, optimize bool) (rects []planRect, byID map[string]planRect, edges []planEdge) {
	if n.Shape == "composite" {
		applyLayout(n, canvas)
		planCollect(n.Parts, 0, 0, 0, theme, &rects)
	} else {
		st := ResolveStyleWithRole(theme, n.Shape, n.Role, n.Preset, n.Style)
		w, d, h := planDims(n.Shape, n.Geom)
		rects = append(rects, planRectFor(n.Label, n.Label, 0, 0, 0, w, d, h, n.Shape, st, false))
	}
	byID = map[string]planRect{}
	var leaves []planRect
	for _, r := range rects {
		if r.id != "" {
			byID[r.id] = r
		}
		if !r.container {
			leaves = append(leaves, r)
		}
	}
	var placed [][][2]float64
	for ci, c := range n.Connectors {
		if c == nil {
			continue
		}
		fr, okF := byID[connectorTarget(c.From)]
		to, okT := byID[connectorTarget(c.To)]
		if !okF || !okT {
			continue // dangling endpoint — Validate already flags it
		}
		var pts [][2]float64
		switch {
		case c.Routing == "straight":
			// A straight connector renders as a direct line, so it must be
			// SCORED as one — not the orthogonal staircase the router would pick.
			// Otherwise two crossing straight diagonals (or a line tunnelling a
			// node) read as clean, because the staircase avoids what the straight
			// line does not.
			pts = [][2]float64{
				{fr.x + fr.w/2, fr.y + fr.d/2},
				{to.x + to.w/2, to.y + to.d/2},
			}
		case optimize:
			pts = selectRoute(fr, to, leaves, placed)
		default:
			pts = planRoute(fr, to)
		}
		placed = append(placed, pts)
		edges = append(edges, planEdge{ci: ci, c: c, from: fr, to: to, pts: pts})
	}
	return
}

// planRouteCandidates enumerates the orthogonal 2-bend routes between two
// footprints: an x-first and a y-first staircase, each with the cross leg at
// the midpoint OR flush against the target. The selector scores these and keeps
// the cheapest; the baseline planRoute is just the dominant-axis midpoint pick.
func planRouteCandidates(fr, to planRect) [][][2]float64 {
	fcx, fcy := fr.x+fr.w/2, fr.y+fr.d/2
	tcx, tcy := to.x+to.w/2, to.y+to.d/2
	sx, ex := fr.x+fr.w, to.x
	if tcx < fcx {
		sx, ex = fr.x, to.x+to.w
	}
	sy, ey := fr.y+fr.d, to.y
	if tcy < fcy {
		sy, ey = fr.y, to.y+to.d
	}
	var c [][][2]float64
	for _, mx := range []float64{(sx + ex) / 2, ex} { // x-first staircases
		c = append(c, [][2]float64{{sx, fcy}, {mx, fcy}, {mx, tcy}, {ex, tcy}})
	}
	for _, my := range []float64{(sy + ey) / 2, ey} { // y-first staircases
		c = append(c, [][2]float64{{fcx, sy}, {fcx, my}, {tcx, my}, {tcx, ey}})
	}
	return c
}

// selectRoute picks the candidate route with the lowest scorecard cost given
// the node obstacles and the routes already placed this pass.
func selectRoute(fr, to planRect, nodes []planRect, placed [][][2]float64) [][2]float64 {
	best := planRoute(fr, to)
	bestCost := routeCost(best, fr, to, nodes, placed)
	for _, cand := range planRouteCandidates(fr, to) {
		if cost := routeCost(cand, fr, to, nodes, placed); cost < bestCost {
			best, bestCost = cand, cost
		}
	}
	return best
}

// routeCost is the per-edge slice of the scorecard: tunnelling a node is worst,
// then crossing an existing edge, then bends, then a small length tiebreak.
func routeCost(pts [][2]float64, fr, to planRect, nodes []planRect, placed [][][2]float64) float64 {
	cost := 0.0
	ez := edgeZLevel(fr, to)
	for _, r := range nodes {
		if r.id == fr.id || r.id == to.id || r.h <= planThinH ||
			!sameFloor(ez, r) || enclosesBoth(r, fr, to) {
			continue
		}
		if routeHitsRect(pts, r) {
			cost += 1000
		}
	}
	for _, pl := range placed {
		for a := 1; a < len(pts); a++ {
			for b := 1; b < len(pl); b++ {
				if _, ok := segInt(pts[a-1], pts[a], pl[b-1], pl[b]); ok {
					cost += 100
				}
			}
		}
	}
	cost += 3 * float64(planBends(pts))
	cost += 0.01 * planLen(pts)
	return cost
}

// RenderPlan emits the top-down plan SVG for a scene. A nil or empty scene
// yields "". Non-composite single nodes render as a single footprint.
func RenderPlan(n *Node, theme *Theme, canvas *Canvas, _ []*Annotation) string {
	return renderPlan(n, theme, canvas, nil)
}

// renderPlan does the work; a non-nil report overlays its findings (crossings,
// edges through nodes) in red on top of the diagram.
func renderPlan(n *Node, theme *Theme, canvas *Canvas, hl *PlanReport) string {
	if n == nil {
		return ""
	}
	rects, _, edges := buildPlanModel(n, theme, canvas)
	if len(rects) == 0 {
		return ""
	}

	// World bbox over every footprint.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range rects {
		minX, minY = math.Min(minX, r.x), math.Min(minY, r.y)
		maxX, maxY = math.Max(maxX, r.x+r.w), math.Max(maxY, r.y+r.d)
	}
	pad := planPad
	if canvas != nil && canvas.Padding > 0 {
		pad = canvas.Padding
	}
	tx := func(wx float64) float64 { return wx - minX + pad }
	ty := func(wy float64) float64 { return wy - minY + pad }
	W := (maxX - minX) + 2*pad
	H := (maxY - minY) + 2*pad

	var sb strings.Builder
	fmt.Fprintf(&sb,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.2f %.2f" width="%.2f" height="%.2f" font-family="Inter,system-ui,sans-serif">`,
		W, H, W, H)

	// Background + arrowhead marker.
	bg := "#ffffff"
	if canvas != nil && canvas.Background != "" {
		bg = canvas.Background
	}
	// Connector ink is a saturated dark slate that pops on the light lane
	// substrates and white boxes; a light CASING (drawn under each line, see
	// below) carries it across the dark gaps between lanes. This dark-line +
	// light-halo pair stays legible on every background the route crosses —
	// the iso author stroke colours, tuned for 3D faces, would vanish on one.
	edgeInk := "#27364b"
	edgeCasing := "#eef2f8"
	fmt.Fprintf(&sb, `<rect x="0" y="0" width="%.2f" height="%.2f" fill="%s"/>`, W, H, planEsc(bg))
	fmt.Fprintf(&sb, `<defs><marker id="planarrow" viewBox="0 0 10 10" refX="8.5" refY="5" markerWidth="9" markerHeight="9" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker></defs>`, edgeInk)

	drawRect := func(r planRect) {
		px, py := tx(r.x), ty(r.y)
		fill := r.fill
		dash := ""
		if r.boundary {
			dash = ` stroke-dasharray="6 4"`
			fill = "none"
		}
		fmt.Fprintf(&sb,
			`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.1f" ry="%.1f" fill="%s" stroke="%s" stroke-width="%.1f"%s/>`,
			px, py, r.w, r.d, planRadius, planRadius, planEsc(fill), planEsc(r.stroke), planLineW(r), dash)
		if r.label == "" {
			return
		}
		if r.container {
			// lane label hugs the top-left, inside the child-free padding strip.
			fmt.Fprintf(&sb,
				`<text x="%.2f" y="%.2f" font-size="%.1f" font-weight="600" fill="%s">%s</text>`,
				px+8, py+16, planFontTitle, planEsc(r.textColor), planEsc(r.label))
		} else {
			fmt.Fprintf(&sb,
				`<text x="%.2f" y="%.2f" font-size="%.1f" text-anchor="middle" dominant-baseline="central" fill="%s">%s</text>`,
				px+r.w/2, py+r.d/2, planFontLabel, planEsc(r.textColor), planEsc(r.label))
		}
	}

	// Paint order: substrates → connectors → leaf boxes → connector labels, so
	// routes tuck BEHIND the node boxes (no lines crossing labels) while the
	// pills stay on top. Containers come first in `rects` (planCollect order).
	for _, r := range rects {
		if r.container {
			drawRect(r)
		}
	}

	// Connectors: orthogonal face-to-face routes between footprints. Collect the
	// label placements to emit after the leaf boxes.
	type pill struct {
		x, y, w        float64
		bg, ink, label string
	}
	var pills []pill
	for _, e := range edges {
		c := e.c
		stroke := edgeInk
		casing := edgeCasing
		if c.Stroke != nil && c.Stroke.Color != "" {
			stroke = c.Stroke.Color
			// keep the halo opposite the ink so a custom colour still pops.
			if planIsDark(stroke) {
				casing = "#f1f5f9"
			} else {
				casing = "#1e293b"
			}
		}
		sw := 2.4 // bolder than iso hairlines so the flow reads at a glance
		if c.Stroke != nil && c.Stroke.Width != nil && *c.Stroke.Width > 0 {
			sw = *c.Stroke.Width
		}
		dash := ""
		if c.Stroke != nil && c.Stroke.Dash == "dashed" {
			dash = ` stroke-dasharray="6 5"`
		} else if c.Stroke != nil && c.Stroke.Dash == "dotted" {
			dash = ` stroke-dasharray="2 4"`
		}
		path := planPathStr(e.pts, tx, ty)
		mxw, myw := planMid(e.pts)
		mx, my := tx(mxw), ty(myw)
		head := ` marker-end="url(#planarrow)"`
		if c.Arrow == "none" {
			head = ""
		}
		// Casing first (a wider halo, no arrow), then the ink line on top — the
		// pair stays visible whether the segment runs over a light lane or a
		// dark gap. Dashed lines skip the casing so the gaps read cleanly.
		if dash == "" {
			fmt.Fprintf(&sb,
				`<path d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="round"/>`,
				path, planEsc(casing), sw+3)
		}
		fmt.Fprintf(&sb,
			`<path d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="round"%s%s/>`,
			path, planEsc(stroke), sw, dash, head)
		if c.Label != "" {
			lblBg, lblInk := "#ffffff", "#334155"
			if c.LabelBg != "" {
				lblBg = c.LabelBg
			}
			if c.LabelColor != "" {
				lblInk = c.LabelColor
			}
			wpx := 7.0*float64(len([]rune(c.Label))) + 10
			pills = append(pills, pill{x: mx - wpx/2, y: my - 8, w: wpx, bg: lblBg, ink: lblInk, label: c.Label})
		}
	}

	for _, r := range rects {
		if !r.container {
			drawRect(r)
		}
	}
	for _, p := range pills {
		fmt.Fprintf(&sb,
			`<rect x="%.2f" y="%.2f" width="%.2f" height="16" rx="4" ry="4" fill="%s" stroke="#e2e8f0" stroke-width="1"/>`+
				`<text x="%.2f" y="%.2f" font-size="10" text-anchor="middle" dominant-baseline="central" fill="%s">%s</text>`,
			p.x, p.y, p.w, planEsc(p.bg), p.x+p.w/2, p.y+8, planEsc(p.ink), planEsc(p.label))
	}

	// Evaluation overlay: redraw each through-node edge in red, then mark every
	// crossing with a red ✕, so layout problems jump out on the diagram.
	if hl != nil {
		for _, pe := range hl.ProblemEdges {
			fmt.Fprintf(&sb,
				`<path d="%s" fill="none" stroke="#e11d48" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" stroke-dasharray="5 4"/>`,
				planPathStr(pe.Pts, tx, ty))
		}
		for _, x := range hl.CrossingsAt {
			cx, cy := tx(x.X), ty(x.Y)
			fmt.Fprintf(&sb,
				`<path d="M%.2f %.2f L%.2f %.2f M%.2f %.2f L%.2f %.2f" stroke="#e11d48" stroke-width="2.4" stroke-linecap="round"/>`,
				cx-5, cy-5, cx+5, cy+5, cx-5, cy+5, cx+5, cy-5)
		}
	}

	sb.WriteString(`</svg>`)
	svg := sb.String()
	ar := 16.0 / 10.0
	if canvas != nil && canvas.AspectRatio != 0 {
		ar = canvas.AspectRatio
	}
	return ceilOuterDims(enforceAspectRatio(svg, ar))
}

// planCollect walks the part tree, accumulating each part's ABSOLUTE world
// corner (parent corner + own resolved offset). applyLayout has already filled
// in computed footprints (auto-sized containers included) and parent-relative
// offsets, so a plain accumulation is exact.
func planCollect(parts []*CompositePart, ox, oy, oz float64, theme *Theme, out *[]planRect) {
	for _, p := range parts {
		if p == nil {
			continue
		}
		x, y, z := ox, oy, oz
		if p.Offset != nil {
			x += p.Offset.WX
			y += p.Offset.WY
			z += p.Offset.WZ
		}
		w, d, h := planDims(p.Shape, p.Geom)
		container := isContainerShape(p.Shape) || len(p.Parts) > 0
		st := ResolveStyleWithRole(theme, p.Shape, p.Role, p.Preset, p.Style)
		*out = append(*out, planRectFor(p.ID, p.Label, x, y, z, w, d, h, p.Shape, st, container))
		if len(p.Parts) > 0 {
			planCollect(p.Parts, x, y, z, theme, out)
		}
	}
}

func planDims(shape string, g *Geom) (w, d, h float64) {
	w, d, h = 140, 140, 80
	if g != nil {
		if g.W > 0 {
			w = g.W
		}
		if g.D > 0 {
			d = g.D
		}
		if g.H > 0 {
			h = g.H
		}
	}
	// Clamp up to the rendered floor (e.g. cloud) so the plan view and the
	// footprint-collision check measure the size that is actually drawn.
	mw, md, mh := iso25d.ShapeMinDims(shape)
	return math.Max(w, mw), math.Max(d, md), math.Max(h, mh)
}

func planRectFor(id, label string, x, y, z, w, d, h float64, shape string, st *Style, container bool) planRect {
	r := planRect{id: id, label: label, x: x, y: y, z: z, w: w, d: d, h: h, container: container}
	r.boundary = shape == "boundary"
	// Fill from the resolved top face (gradient → its start colour).
	fill := "#cfd8e3"
	if st != nil && st.Palette != nil {
		if st.Palette.Top != "" {
			fill = st.Palette.Top
		} else if st.Palette.TopGradient != nil && st.Palette.TopGradient.From != "" {
			fill = st.Palette.TopGradient.From
		}
	}
	r.fill = fill
	r.stroke = "#334155"
	if st != nil && st.Stroke != nil && st.Stroke.Color != "" {
		r.stroke = st.Stroke.Color
	}
	if container && !r.boundary {
		// translucent substrate so children read on top; lane label stays dark.
		r.fill = planTint(fill)
		r.textColor = "#475569"
	} else {
		// Leaf label: pick black/white by the footprint's luminance. The
		// theme's own text colour is tuned for the iso TOP face (often a dark
		// slab), so honouring it here would print invisible light-on-light.
		if planIsDark(r.fill) {
			r.textColor = "#f1f5f9"
		} else {
			r.textColor = "#0f172a"
		}
	}
	return r
}

func planLineW(r planRect) float64 {
	if r.container {
		return 1.4
	}
	return 1.2
}

// planRoute builds an orthogonal route between two footprints along the
// dominant axis and returns its corner points in WORLD coords, source→target.
// Both the renderer and the layout evaluator read these same points, so the
// metrics measure exactly what is drawn.
func planRoute(fr, to planRect) [][2]float64 {
	fcx, fcy := fr.x+fr.w/2, fr.y+fr.d/2
	tcx, tcy := to.x+to.w/2, to.y+to.d/2
	if math.Abs(tcx-fcx) >= math.Abs(tcy-fcy) {
		// horizontal-dominant: exit left/right faces, elbow at mid-x.
		sx, ex := fr.x+fr.w, to.x
		if tcx < fcx {
			sx, ex = fr.x, to.x+to.w
		}
		midx := (sx + ex) / 2
		return [][2]float64{{sx, fcy}, {midx, fcy}, {midx, tcy}, {ex, tcy}}
	}
	// vertical-dominant: exit top/bottom faces, elbow at mid-y.
	sy, ey := fr.y+fr.d, to.y
	if tcy < fcy {
		sy, ey = fr.y, to.y+to.d
	}
	midy := (sy + ey) / 2
	return [][2]float64{{fcx, sy}, {fcx, midy}, {tcx, midy}, {tcx, ey}}
}

// planPathStr renders a world-space polyline to an SVG path in screen coords.
func planPathStr(pts [][2]float64, tx, ty func(float64) float64) string {
	var b strings.Builder
	for i, p := range pts {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.2f %.2f ", cmd, tx(p[0]), ty(p[1]))
	}
	return strings.TrimSpace(b.String())
}

// planMid returns the midpoint of a route's middle (cross) segment in world
// coords — where a connector label sits.
func planMid(pts [][2]float64) (x, y float64) {
	if len(pts) < 2 {
		return 0, 0
	}
	i := len(pts) / 2
	return (pts[i-1][0] + pts[i][0]) / 2, (pts[i-1][1] + pts[i][1]) / 2
}

// planTint lightens a hex colour toward white for substrate fills; non-hex
// inputs pass through (the renderer falls back to the raw value).
func planTint(hex string) string {
	r, g, b, ok := parseHex6(hex)
	if !ok {
		return hex
	}
	mix := func(c int) int { return c + (255-c)*72/100 } // 72% toward white
	return fmt.Sprintf("#%02X%02X%02X", mix(r), mix(g), mix(b))
}

// planIsDark reports whether a colour is dark enough to want light ink on top
// (relative luminance < 0.5). Non-hex inputs default to light (false).
func planIsDark(hex string) bool {
	r, g, b, ok := parseHex6(hex)
	if !ok {
		return false
	}
	lum := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
	return lum < 0.5
}

func parseHex6(s string) (r, g, b int, ok bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, false
	}
	var v int
	if _, err := fmt.Sscanf(s[1:], "%06x", &v); err != nil {
		return 0, 0, 0, false
	}
	return v >> 16 & 0xff, v >> 8 & 0xff, v & 0xff, true
}

func planEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
