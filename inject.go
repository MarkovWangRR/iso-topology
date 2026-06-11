// Post-render SVG layer injectors. Each one splices a new <g
// data-layer="…"> into the SVG produced by iso25d.RenderComposite —
// canvas backdrop, screen-space labels, composite connectors, and
// screen-space annotation callouts.
//
// They all operate on the SVG as a string (parse + edit + emit) so
// they're decoupled from iso25d's internal data structures and stay
// composable.
package isotopo

import (
	"fmt"
	"math"
	"strings"
)

// partsScreenOrigin projects every part's 8 world-bbox corners and
// returns the (tx, ty) translation that maps world-projected coords
// into the composite's screen space — the same math
// iso25d.RenderComposite applies internally, shared by every injector
// so they can't drift from each other.
func partsScreenOrigin(infos []partInfo) (tx, ty float64) {
	minX, minY := math.Inf(1), math.Inf(1)
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
			sx := c[0]*cos30 - c[1]*cos30
			sy := c[0]*sin30 + c[1]*sin30 - c[2]
			minX = math.Min(minX, sx)
			minY = math.Min(minY, sy)
		}
	}
	const pad = 12.0
	return -minX + pad, -minY + pad
}

func projectIso(wx, wy, wz float64) (float64, float64) {
	return wx*cos30 - wy*cos30, wx*sin30 + wy*sin30 - wz
}

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
func injectScreenLabels(svg string, infos []partInfo) (string, []screenRect) {
	project := projectIso
	any := false
	for _, p := range infos {
		if p.screenLabel != "" {
			any = true
			break
		}
	}
	if !any {
		return svg, nil
	}
	tx, ty := partsScreenOrigin(infos)

	// v2.8 — screen-space text contract: labels must not cross or
	// touch any part's projection, and prefer the picture's periphery.
	partRects := partScreenRects(infos)
	sceneCx, sceneCy := sceneCenter(partRects)
	obstacles := append([]screenRect(nil), partRects...)
	var placed []screenRect

	var sb strings.Builder
	sb.WriteString(`<g data-layer="screen-labels">`)
	maxLabelY := 0.0
	maxLabelX := 0.0
	minLabelX, minLabelY := math.Inf(1), math.Inf(1)
	for i, p := range infos {
		if p.screenLabel == "" {
			continue
		}
		// Preferred (legacy) spot: under the part's bottom-front corner.
		bottomFrontX, bottomFrontY := project(p.offWX+p.w/2, p.offWY+p.d, p.offWZ)
		cx := bottomFrontX + tx
		baseY := bottomFrontY + ty + 14 // 14px gap under the part

		text := p.screenLabel
		family := "Inter, sans-serif"
		fontSize := p.labelFontSize
		boxW := float64(len(text))*fontSize*0.58 + 16
		boxH := fontSize + 10

		// Own silhouette doesn't block the label hanging off its edge —
		// every OTHER part and already-placed label does.
		others := make([]screenRect, 0, len(obstacles))
		for j, o := range obstacles {
			if j == i {
				continue
			}
			others = append(others, o)
		}
		bx, by := placeTextBox(boxW, boxH, partRects[i], cx-boxW/2, baseY, sceneCx, sceneCy, others)
		cx, baseY = bx+boxW/2, by
		placed = append(placed, screenRect{bx, by, bx + boxW, by + boxH})
		obstacles = append(obstacles, screenRect{bx, by, bx + boxW, by + boxH})
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
		minLabelX = math.Min(minLabelX, cx-boxW/2)
		minLabelY = math.Min(minLabelY, baseY)
	}
	sb.WriteString(`</g>`)
	idx := strings.LastIndex(svg, "</svg>")
	if idx < 0 {
		return svg, placed
	}
	const pad = 12.0
	out := growViewBox(svg[:idx]+sb.String()+svg[idx:], maxLabelX+pad, maxLabelY+pad)
	// Periphery placement can spill past the LEFT/TOP origin too.
	if minLabelX < pad || minLabelY < pad {
		out = growViewBoxAround(out, minSvgRect{
			minX: minLabelX - pad, minY: minLabelY - pad,
			maxX: maxLabelX + pad, maxY: maxLabelY + pad,
		})
	}
	return out, placed
}

// growViewBox parses the leading <svg ...> tag and, if needed, expands
// viewBox/width/height so the post-hoc-inserted screen labels are not
// clipped. Idempotent — shrinks to a no-op when the current viewBox is

func injectCompositeConnectors(svg string, conns []*Connector, infos []partInfo) string {
	project := projectIso
	tx, ty := partsScreenOrigin(infos)

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

	// v2.3 — connector geometry participates in the viewBox. The part
	// bbox alone is NOT enough: orthogonal stubs/staggers and bezier
	// control points can swing past the lowest part and get clipped at
	// the SVG edge. Track the extremes of every emitted waypoint and
	// grow the viewBox afterwards.
	cMinX, cMinY := math.Inf(1), math.Inf(1)
	cMaxX, cMaxY := math.Inf(-1), math.Inf(-1)
	trackPt := func(x, y float64) {
		cMinX = math.Min(cMinX, x)
		cMinY = math.Min(cMinY, y)
		cMaxX = math.Max(cMaxX, x)
		cMaxY = math.Max(cMaxY, y)
	}

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
		case "bezier":
			// v2.1 — single quadratic curve from c.From to c.To, control
			// point sits on the perpendicular bisector ¼ of the chord's
			// length away. Produces a natural S-free arc that reads as
			// "data flow" instead of the rigid L-corner of orthogonal.
			x1, y1, ok1 := anchorScreen(c.From)
			x2, y2, ok2 := anchorScreen(c.To)
			if !ok1 || !ok2 {
				continue
			}
			dx, dy := x2-x1, y2-y1
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > 0 {
				nx, ny := -dy/dist, dx/dist
				bend := dist * 0.25
				cx, cy := (x1+x2)/2+nx*bend, (y1+y2)/2+ny*bend
				pts = [][2]float64{{x1, y1}, {cx, cy}, {x2, y2}}
			} else {
				pts = [][2]float64{{x1, y1}, {x2, y2}}
			}
		default: // "straight" or empty
			x1, y1, ok1 := anchorScreen(c.From)
			x2, y2, ok2 := anchorScreen(c.To)
			if !ok1 || !ok2 {
				continue
			}
			pts = [][2]float64{{x1, y1}, {x2, y2}}
		}

		for _, p := range pts {
			trackPt(p[0], p[1])
		}

		// Emit path. Bezier routing emits a single quadratic curve
		// (M start Q ctrl end); everything else lays down a polyline.
		// Arrow + label still consume `pts[0]` and `pts[len-1]` so they
		// stay correct in both modes.
		var d strings.Builder
		if c.Routing == "bezier" && len(pts) == 3 {
			fmt.Fprintf(&d, "M %.2f,%.2f Q %.2f,%.2f %.2f,%.2f",
				pts[0][0], pts[0][1], pts[1][0], pts[1][1], pts[2][0], pts[2][1])
		} else {
			for i, p := range pts {
				if i == 0 {
					fmt.Fprintf(&d, "M %.2f,%.2f", p[0], p[1])
				} else {
					fmt.Fprintf(&d, " L %.2f,%.2f", p[0], p[1])
				}
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
			trackPt(mx-textW/2, my-10)
			trackPt(mx+textW/2, my+10)
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

	// Grow the viewBox to cover every connector waypoint (plus a pad
	// for stroke width + arrowheads) BEFORE splicing the layer in, so
	// routes that bend past the part bbox are never clipped.
	if cMinX <= cMaxX {
		const pad = 10.0
		svg = growViewBoxAround(svg, minSvgRect{
			minX: cMinX - pad, minY: cMinY - pad,
			maxX: cMaxX + pad, maxY: cMaxY + pad,
		})
	}

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

func injectAnnotations(svg string, anns []*Annotation, infos []partInfo, extraObstacles []screenRect) string {
	if len(anns) == 0 || len(infos) == 0 {
		return svg
	}
	project := projectIso
	tx, ty := partsScreenOrigin(infos)

	// v2.8 — same screen-space contract as labels: never cross or
	// touch a part; pick the most peripheral collision-free spot.
	partRects := partScreenRects(infos)
	sceneCx, sceneCy := sceneCenter(partRects)
	obstacles := append(append([]screenRect(nil), partRects...), extraObstacles...)

	byID := make(map[string]partInfo, len(infos))
	anchorRectByID := make(map[string]screenRect, len(infos))
	for i, p := range infos {
		if p.id != "" {
			byID[p.id] = p
			anchorRectByID[p.id] = partRects[i]
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

		posFor := func(s string, d float64) (float64, float64) {
			switch s {
			case "top":
				return ax - boxW/2, ay - d - boxH
			case "bottom":
				return ax - boxW/2, ay + d
			case "left":
				return ax - d - boxW, ay - boxH/2
			default: // right
				return ax + d, ay - boxH/2
			}
		}
		// Candidate order is SIDE-major for consistency: the author's
		// side is exhausted at every distance before any other side is
		// tried; fallback sides are ordered periphery-first.
		outX, outY := ax-sceneCx, ay-sceneCy
		sideScore := func(s string) float64 {
			switch s {
			case "top":
				return -outY
			case "bottom":
				return outY
			case "left":
				return -outX
			default:
				return outX
			}
		}
		sides := []string{side}
		for _, s2 := range []string{"top", "right", "bottom", "left"} {
			if s2 != side {
				sides = append(sides, s2)
			}
		}
		rest := sides[1:]
		for i := 0; i < len(rest); i++ {
			for j := i + 1; j < len(rest); j++ {
				if sideScore(rest[j]) > sideScore(rest[i])+1e-9 {
					rest[i], rest[j] = rest[j], rest[i]
				}
			}
		}
		bx, by := posFor(side, dist)
		found := false
		for _, s2 := range sides {
			for _, d := range []float64{dist, dist + 45, dist + 90, dist + 140} {
				cx2, cy2 := posFor(s2, d)
				c := screenRect{cx2, cy2, cx2 + boxW, cy2 + boxH}
				if !collides(c, obstacles) {
					bx, by, side = cx2, cy2, s2
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		obstacles = append(obstacles, screenRect{bx, by, bx + boxW, by + boxH})

		// Leader line: from the silhouette edge FACING the box to the
		// box's nearest edge — never across the node's own body.
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
		lx1, ly1 := leaderStart(anchorRectByID[a.Anchor], lx2, ly2)
		fmt.Fprintf(&sb,
			`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1" stroke-dasharray="3 3"/>`,
			lx1, ly1, lx2, ly2, escAttr(border),
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
