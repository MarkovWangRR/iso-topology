package iso25d

import (
	"fmt"
	"math"
	"strings"
)

// ---------------------------------------------------------------------------
// Cylinder (database / queue / stored_data)
// ---------------------------------------------------------------------------

type IsoCylinderOpts struct {
	Radius float64
	Height float64

	TopFill   string
	LeftFill  string
	RightFill string

	Stroke      string
	StrokeWidth float64

	Label      string
	FontFamily string
	FontSize   float64
	FontColor  string
	FontWeight string

	Icon      string
	IconScale float64

	Margin float64

	Opacity         float64
	StrokeDasharray string
	Background      string
}

func DefaultIsoCylinder() IsoCylinderOpts {
	return IsoCylinderOpts{
		Radius: 80, Height: 110,
		TopFill:     "#FFD27F",
		LeftFill:    "#B8651F",
		RightFill:   "#E08A47",
		Stroke:      "#5C2A0A",
		StrokeWidth: 1.5,
		FontFamily:  "Helvetica Neue, Arial, sans-serif",
		FontSize:    16,
		FontColor:   "#3A1A06",
		FontWeight:  "600",
		IconScale:   0.45,
		Margin:      24,
	}
}

func RenderIsoCylinder(o IsoCylinderOpts) string {
	rx := o.Radius
	ry := rx * math.Tan(math.Pi/6)
	h := o.Height
	m := o.Margin

	topCx, topCy := 0.0, 0.0
	botCy := h

	minX := topCx - rx
	maxX := topCx + rx
	minY := topCy - ry
	maxY := botCy + ry

	W := (maxX - minX) + 2*m
	H := (maxY - minY) + 2*m
	tx, ty := -minX+m, -minY+m
	sx := func(x float64) float64 { return x + tx }
	sy := func(y float64) float64 { return y + ty }

	var sb strings.Builder
	sb.WriteString(svgHeader(W, H))
	openWrapper(&sb, W, H, o.Background, o.Opacity, o.StrokeDasharray, "")

	leftTopX, leftTopY := sx(-rx), sy(0)
	rightTopX, rightTopY := sx(rx), sy(0)
	leftBotX, leftBotY := sx(-rx), sy(h)
	rightBotX, rightBotY := sx(rx), sy(h)
	topFrontX, topFrontY := sx(0), sy(0+ry)
	botFrontX, botFrontY := sx(0), sy(h+ry)

	// Two non-overlapping body halves, each filled with its own pure colour.
	// Bounded by: outer vertical edge, front-bottom quarter arc, front
	// meridian (vertical line at x=0), front-top quarter arc.
	// Both halves carry no stroke — the silhouette is one separate path so
	// no seam appears on the front meridian.
	fmt.Fprintf(&sb,
		`<path data-face="side-left" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 0 %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 0 %.2f %.2f Z" fill="%s" stroke="none"/>`,
		leftTopX, leftTopY,
		leftBotX, leftBotY,
		rx, ry, botFrontX, botFrontY,
		topFrontX, topFrontY,
		rx, ry, leftTopX, leftTopY,
		o.LeftFill,
	)
	fmt.Fprintf(&sb,
		`<path data-face="side-right" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 1 %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 1 %.2f %.2f Z" fill="%s" stroke="none"/>`,
		rightTopX, rightTopY,
		rightBotX, rightBotY,
		rx, ry, botFrontX, botFrontY,
		topFrontX, topFrontY,
		rx, ry, rightTopX, rightTopY,
		o.RightFill,
	)

	// Body silhouette: left edge + full front-bottom arc + right edge.
	// Open path (no Z) so no horizontal line is drawn across the top — the
	// top ellipse drawn next provides the top silhouette via its own stroke.
	if strings.TrimSpace(o.Stroke) != "" && o.StrokeWidth > 0 {
		fmt.Fprintf(&sb,
			`<path data-face="outline" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 0 %.2f %.2f L %.2f %.2f" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round" stroke-linecap="round"/>`,
			leftTopX, leftTopY,
			leftBotX, leftBotY,
			rx, ry, rightBotX, rightBotY,
			rightTopX, rightTopY,
			o.Stroke, o.StrokeWidth,
		)
	}

	// Top ellipse.
	fmt.Fprintf(&sb,
		`<ellipse data-face="top" cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" fill="%s" stroke="%s" stroke-width="%.2f"/>`,
		sx(topCx), sy(topCy), rx, ry, o.TopFill, o.Stroke, o.StrokeWidth,
	)

	// Label + icon on the top circle, using the inscribed square (in 3D) as
	// the local frame so the icon/text sit cleanly inside the top ellipse.
	s := rx / cos30
	e := sx(topCx)
	f := sy(topCy) - s*sin30
	writeTopLabelAndIcon(
		&sb,
		e, f, s, s,
		o.Label, o.Icon, o.IconScale,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	closeWrapper(&sb)
	sb.WriteString(`</svg>`)
	return sb.String()
}

// ---------------------------------------------------------------------------
// Sphere (circle, person head)
// ---------------------------------------------------------------------------

type IsoSphereOpts struct {
	Radius      float64
	Highlight   string // top-lit colour
	Shadow      string // shaded colour
	Stroke      string
	StrokeWidth float64
	Label       string
	FontFamily  string
	FontSize    float64
	FontColor   string
	FontWeight  string
	Margin      float64

	Opacity         float64
	StrokeDasharray string
	Background      string
}

func DefaultIsoSphere() IsoSphereOpts {
	return IsoSphereOpts{
		Radius:      80,
		Highlight:   "#C2DAFF",
		Shadow:      "#2A4A7A",
		Stroke:      "#1D3A66",
		StrokeWidth: 1.5,
		FontFamily:  "Helvetica Neue, Arial, sans-serif",
		FontSize:    16,
		FontColor:   "#0B1F3A",
		FontWeight:  "600",
		Margin:      24,
	}
}

func RenderIsoSphere(o IsoSphereOpts) string {
	r := o.Radius
	m := o.Margin
	W := 2*r + 2*m
	H := 2*r + 2*m
	cx, cy := r+m, r+m

	var sb strings.Builder
	sb.WriteString(svgHeader(W, H))
	openWrapper(&sb, W, H, o.Background, o.Opacity, o.StrokeDasharray, "")
	gradID := "sphere-grad"
	fmt.Fprintf(&sb,
		`<defs><radialGradient id="%s" cx="35%%" cy="30%%" r="75%%"><stop offset="0%%" stop-color="%s"/><stop offset="100%%" stop-color="%s"/></radialGradient></defs>`,
		gradID, escapeAttr(o.Highlight), escapeAttr(o.Shadow),
	)
	fmt.Fprintf(&sb,
		`<circle data-face="sphere" cx="%.2f" cy="%.2f" r="%.2f" fill="url(#%s)" stroke="%s" stroke-width="%.2f"/>`,
		cx, cy, r, gradID, escapeAttr(o.Stroke), o.StrokeWidth,
	)
	if strings.TrimSpace(o.Label) != "" {
		fmt.Fprintf(&sb,
			`<text x="%.2f" y="%.2f" font-family="%s" font-size="%.2f" font-weight="%s" fill="%s" text-anchor="middle" dy=".35em">%s</text>`,
			cx, cy,
			escapeAttr(o.FontFamily), o.FontSize, escapeAttr(o.FontWeight),
			escapeAttr(o.FontColor), escapeXML(o.Label),
		)
	}
	closeWrapper(&sb)
	sb.WriteString(`</svg>`)
	return sb.String()
}

// ---------------------------------------------------------------------------
// Person (sphere head + box body)
// ---------------------------------------------------------------------------

type IsoPersonOpts struct {
	HeadRadius float64
	BodyWidth  float64
	BodyDepth  float64
	BodyHeight float64

	HeadHighlight string
	HeadShadow    string
	BodyTop       string
	BodyLeft      string
	BodyRight     string

	Stroke      string
	StrokeWidth float64

	Label      string
	FontFamily string
	FontSize   float64
	FontColor  string
	FontWeight string

	Margin float64

	// HeadStyle: "sphere" (default), "block" (C4-style boxy head).
	HeadStyle string

	Opacity         float64
	StrokeDasharray string
	Background      string
}

func DefaultIsoPerson() IsoPersonOpts {
	return IsoPersonOpts{
		HeadRadius: 36,
		BodyWidth:  120, BodyDepth: 120, BodyHeight: 95,
		HeadHighlight: "#FFD9B0", HeadShadow: "#A35F25",
		BodyTop: "#7FB3FF", BodyLeft: "#3A6FBA", BodyRight: "#5589D6",
		Stroke: "#1D3A66", StrokeWidth: 1.5,
		FontFamily: "Helvetica Neue, Arial, sans-serif",
		FontSize:   15, FontColor: "#0B1F3A", FontWeight: "600",
		Margin:    24,
		HeadStyle: "sphere",
	}
}

// RenderIsoPerson renders a friendly "user figure": rounded-cylinder
// torso ("shoulders") topped with a sphere head. Matches the iconic
// 2.5D user-avatar look — wider at the base, soft curves, no legs.
//
// HeadStyle "block" still works as a C4-style square head if requested.
func RenderIsoPerson(o IsoPersonOpts) string {
	rx := o.BodyWidth / 2
	ry := rx * sin30 / cos30 // iso ellipse y-radius
	h := o.BodyHeight
	headR := o.HeadRadius
	headGap := 4.0
	m := o.Margin
	if m <= 0 {
		m = 24
	}

	// Local frame: torso top ellipse centred at (0, 0). Bottom at (0, h).
	// Sphere head sits ABOVE the top ellipse with a small gap.
	headCx := 0.0
	headCy := -ry - headGap - headR

	// Local bbox extents.
	minX := -math.Max(rx, headR)
	maxX := math.Max(rx, headR)
	minY := headCy - headR
	maxY := h + ry

	W := (maxX - minX) + 2*m
	H := (maxY - minY) + 2*m
	tx := -minX + m
	ty := -minY + m
	sx := func(x float64) float64 { return x + tx }
	sy := func(y float64) float64 { return y + ty }

	var sb strings.Builder
	sb.WriteString(svgHeader(W, H))
	openWrapper(&sb, W, H, o.Background, o.Opacity, o.StrokeDasharray, "")

	// Defs: radial gradient for the sphere head.
	gradID := "person-head-grad"
	fmt.Fprintf(&sb,
		`<defs><radialGradient id="%s" cx="35%%" cy="30%%" r="75%%"><stop offset="0%%" stop-color="%s"/><stop offset="100%%" stop-color="%s"/></radialGradient></defs>`,
		gradID, escapeAttr(o.HeadHighlight), escapeAttr(o.HeadShadow),
	)

	// ── Body: stout iso cylinder (rounded shoulders) ─────────────────
	// Same scheme as RenderIsoCylinder — two non-overlapping front
	// halves filled with LeftFill / RightFill, plus a stroked outline
	// and a top ellipse.
	leftTopX, leftTopY := sx(-rx), sy(0)
	rightTopX, rightTopY := sx(rx), sy(0)
	leftBotX, leftBotY := sx(-rx), sy(h)
	rightBotX, rightBotY := sx(rx), sy(h)
	topFrontX, topFrontY := sx(0), sy(0+ry)
	botFrontX, botFrontY := sx(0), sy(h+ry)

	fmt.Fprintf(&sb,
		`<path data-face="body-left" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 0 %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 0 %.2f %.2f Z" fill="%s" stroke="none"/>`,
		leftTopX, leftTopY, leftBotX, leftBotY,
		rx, ry, botFrontX, botFrontY,
		topFrontX, topFrontY,
		rx, ry, leftTopX, leftTopY,
		o.BodyLeft,
	)
	fmt.Fprintf(&sb,
		`<path data-face="body-right" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 1 %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 1 %.2f %.2f Z" fill="%s" stroke="none"/>`,
		rightTopX, rightTopY, rightBotX, rightBotY,
		rx, ry, botFrontX, botFrontY,
		topFrontX, topFrontY,
		rx, ry, rightTopX, rightTopY,
		o.BodyRight,
	)

	if strings.TrimSpace(o.Stroke) != "" && o.StrokeWidth > 0 {
		fmt.Fprintf(&sb,
			`<path data-face="body-outline" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 0 %.2f %.2f L %.2f %.2f" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round" stroke-linecap="round"/>`,
			leftTopX, leftTopY,
			leftBotX, leftBotY,
			rx, ry, rightBotX, rightBotY,
			rightTopX, rightTopY,
			o.Stroke, o.StrokeWidth,
		)
	}

	// Top ellipse — the "shoulder ring".
	fmt.Fprintf(&sb,
		`<ellipse data-face="body-top" cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" fill="%s" stroke="%s" stroke-width="%.2f"/>`,
		sx(0), sy(0), rx, ry, o.BodyTop, escapeAttr(o.Stroke), o.StrokeWidth,
	)

	// ── Head ─────────────────────────────────────────────────────────
	if o.HeadStyle == "block" {
		// C4-style box head.
		hw := headR * 1.6
		hd := headR * 1.6
		hh := headR * 0.9
		mini := computeBoxGeom(hw, hd, hh, 0)
		anchorX := sx(0) - (mini.A[0]+mini.C[0])/2
		anchorY := sy(0) - headGap - mini.ViewH
		off := [2]float64{anchorX, anchorY}
		shift2 := func(p [2]float64) [2]float64 { return [2]float64{p[0] + off[0], p[1] + off[1]} }
		mE, mF, mG, mH := shift2(mini.E), shift2(mini.F), shift2(mini.G), shift2(mini.H)
		faceTag(&sb, "head-left", o.HeadShadow, o.Stroke, o.StrokeWidth, shift2(mini.D), mH, mG, shift2(mini.C))
		faceTag(&sb, "head-right", o.HeadShadow, o.Stroke, o.StrokeWidth, shift2(mini.B), mF, mG, shift2(mini.C))
		faceTag(&sb, "head-top", o.HeadHighlight, o.Stroke, o.StrokeWidth, mE, mF, mG, mH)
	} else {
		fmt.Fprintf(&sb,
			`<circle data-face="head" cx="%.2f" cy="%.2f" r="%.2f" fill="url(#%s)" stroke="%s" stroke-width="%.2f"/>`,
			sx(headCx), sy(headCy), headR, gradID,
			escapeAttr(o.Stroke), o.StrokeWidth,
		)
	}

	// Optional label on the body top ellipse.
	if strings.TrimSpace(o.Label) != "" {
		s := rx / cos30
		writeTopLabelAndIcon(
			&sb,
			sx(0), sy(0)-s*sin30, s, s,
			o.Label, "", 0,
			o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
		)
	}

	closeWrapper(&sb)
	sb.WriteString(`</svg>`)
	return sb.String()
}

// ---------------------------------------------------------------------------
// Hex prism (pointy-top hexagon extruded)
// ---------------------------------------------------------------------------

func RenderIsoText(o IsoBoxOpts) string {
	text := strings.TrimSpace(o.Label)
	if text == "" {
		var sb strings.Builder
		sb.WriteString(svgHeader(0, 0))
		sb.WriteString(`</svg>`)
		return sb.String()
	}
	fontSize := o.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}
	fontFamily := o.FontFamily
	if fontFamily == "" {
		fontFamily = "Inter, sans-serif"
	}
	fontWeight := o.FontWeight
	if fontWeight == "" {
		fontWeight = "700"
	}
	fontColor := o.FontColor
	if fontColor == "" {
		fontColor = "#1F2E50"
	}
	m := o.Margin
	if m <= 0 {
		m = 12
	}

	// Approximate text width: monospace fudge factor 0.55 (Inter-ish).
	// Add half-em of padding so the trailing glyph isn't flush against
	// the bbox edge after the matrix tilt.
	textW := float64(len(text))*fontSize*0.55 + fontSize*0.5

	// Local text bbox is (0..textW, −fontSize..0). After the iso
	// matrix(cos30 sin30 -cos30 sin30 originX originY), each local
	// corner (lx, ly) maps to (lx·cos30 − ly·cos30, lx·sin30 + ly·sin30)
	// relative to (originX, originY). Pick origin so all 4 projected
	// corners fall inside the standalone SVG with margin m.
	W := cos30*(textW+fontSize) + 2*m
	H := sin30*(textW+fontSize) + 2*m
	originX := m
	originY := m + sin30*fontSize

	var sb strings.Builder
	sb.WriteString(svgHeader(W, H))
	fmt.Fprintf(&sb,
		`<g data-face="iso-text" transform="matrix(%.4f %.4f %.4f %.4f %.2f %.2f)">`,
		cos30, sin30, -cos30, sin30, originX, originY,
	)
	fmt.Fprintf(&sb,
		`<text x="0" y="0" font-family="%s" font-size="%.2f" font-weight="%s" fill="%s">%s</text>`,
		escapeAttr(fontFamily), fontSize, escapeAttr(fontWeight), escapeAttr(fontColor), escapeXML(text),
	)
	sb.WriteString(`</g></svg>`)
	return sb.String()
}

// RenderIsoCloud extrudes a cloud silhouette into the iso top plane.
//
// Geometry: the cloud is sampled as a closed polyline in local (x, y) coords,
// projected at z=0 (bottom face) and z=h (top face). Front-facing perimeter
// segments are detected with the rule (dy > dx) in local coords (the iso
// view direction is (1,1,1); a segment whose outward normal has positive
// dot product with the view direction is front-facing). Each maximal run
// of consecutive front-facing segments is drawn as one side-wall polygon.
// Top face is drawn last as a polyline path so it cleanly covers all
// hidden-side walls and back-perimeter seams.
func RenderIsoCloud(o IsoBoxOpts) string {
	w, d, h, m := o.Width, o.Depth, o.Height, o.Margin

	outline := sampleCloudOutline(w, d)
	n := len(outline)

	type vp struct{ topX, topY, botX, botY float64 }
	proj := make([]vp, n)
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for i, p := range outline {
		sx, sy := project(p[0], p[1], 0)
		proj[i] = vp{topX: sx, topY: sy - h, botX: sx, botY: sy}
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
	W := maxX - minX + 2*m
	H := maxY - minY + 2*m

	var sb strings.Builder
	sb.WriteString(svgHeader(W, H))
	openWrapper(&sb, W, H, o.Background, o.Opacity, o.StrokeDasharray, "")

	// Per-segment front-facing test.
	visible := make([]bool, n)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		dx := outline[j][0] - outline[i][0]
		dy := outline[j][1] - outline[i][1]
		visible[i] = dy > dx
	}

	// Walk visible runs and draw one polygon per run (fill only, no stroke
	// so adjacent run boundaries don't double-stroke each other).
	//
	// The (dy > dx) test catches the steep descent tail of each bump as
	// "visible", producing 2–5 segment "fin" runs under bumps 1–3. We drop
	// runs shorter than minRunPoints — the real bottom-arc run is 30+
	// segments so the threshold cleanly separates noise from signal.
	const minRunPoints = 7
	type run struct{ idx []int }
	runs := []run{}
	for i := 0; i < n; i++ {
		if !visible[i] || visible[(i-1+n)%n] {
			continue
		}
		r := run{idx: []int{i}}
		j := i
		for visible[j] {
			j = (j + 1) % n
			r.idx = append(r.idx, j)
			if j == i {
				break
			}
		}
		if len(r.idx) < minRunPoints {
			continue
		}
		runs = append(runs, r)
	}

	for _, r := range runs {
		var ptsSB strings.Builder
		for _, k := range r.idx {
			fmt.Fprintf(&ptsSB, "%.2f,%.2f ", proj[k].topX+tx, proj[k].topY+ty)
		}
		for k := len(r.idx) - 1; k >= 0; k-- {
			fmt.Fprintf(&ptsSB, "%.2f,%.2f ", proj[r.idx[k]].botX+tx, proj[r.idx[k]].botY+ty)
		}
		fmt.Fprintf(&sb,
			`<polygon data-face="side" points="%s" fill="%s" stroke="none"/>`,
			strings.TrimSpace(ptsSB.String()), o.LeftFill,
		)
	}

	// Per-run silhouette outline: vertical-down at run start, along bottom
	// edge, vertical-up at run end. Single open path so no internal seams.
	if strings.TrimSpace(o.Stroke) != "" && o.StrokeWidth > 0 {
		for _, r := range runs {
			var pathSB strings.Builder
			first := r.idx[0]
			last := r.idx[len(r.idx)-1]
			fmt.Fprintf(&pathSB,
				"M %.2f %.2f L %.2f %.2f",
				proj[first].topX+tx, proj[first].topY+ty,
				proj[first].botX+tx, proj[first].botY+ty,
			)
			for k := 1; k < len(r.idx); k++ {
				fmt.Fprintf(&pathSB,
					" L %.2f %.2f",
					proj[r.idx[k]].botX+tx, proj[r.idx[k]].botY+ty,
				)
			}
			fmt.Fprintf(&pathSB,
				" L %.2f %.2f",
				proj[last].topX+tx, proj[last].topY+ty,
			)
			fmt.Fprintf(&sb,
				`<path data-face="outline" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round" stroke-linecap="round"/>`,
				pathSB.String(), escapeAttr(o.Stroke), o.StrokeWidth,
			)
		}
	}

	// Top face: polyline path filled with TopFill.
	var topPath strings.Builder
	fmt.Fprintf(&topPath, "M %.2f %.2f", proj[0].topX+tx, proj[0].topY+ty)
	for i := 1; i < n; i++ {
		fmt.Fprintf(&topPath, " L %.2f %.2f", proj[i].topX+tx, proj[i].topY+ty)
	}
	topPath.WriteString(" Z")

	if strings.TrimSpace(o.Stroke) != "" && o.StrokeWidth > 0 {
		fmt.Fprintf(&sb,
			`<path data-face="top" d="%s" fill="%s" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"/>`,
			topPath.String(), o.TopFill, escapeAttr(o.Stroke), o.StrokeWidth,
		)
	} else {
		fmt.Fprintf(&sb,
			`<path data-face="top" d="%s" fill="%s" stroke="none"/>`,
			topPath.String(), o.TopFill,
		)
	}

	// Label + icon on top face (matrix transform tilts content onto iso plane).
	writeTopLabelAndIcon(
		&sb,
		tx, ty-h, w, d,
		o.Label, o.Icon, o.IconScale,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	closeWrapper(&sb)
	sb.WriteString(`</svg>`)
	return sb.String()
}

// sampleCloudOutline returns a CW (in screen y-down) polyline around the
// cloud silhouette, modelled on a classic icon-style cloud: small left
// bump + tall wide middle bump + small right bump, joined to a substantial
// rectangular trunk with rounded bottom corners. The 3-bump layout gives
// recognisable variation; the trunk gives the silhouette body so the
// cloud reads as "fluffy" rather than "pinched arc".
func sampleCloudOutline(w, d float64) [][2]float64 {
	out := [][2]float64{}

	horizonY := 0.50 * d
	bottomY := 0.85 * d
	leftX := 0.04 * w
	rightX := 0.96 * w
	cornerR := 0.06 * d

	// Bump layout: [leftX, rightX, peakY] in local frame. Bump 1 small,
	// bump 2 big-and-tall, bump 3 small (matches reference cloud icon).
	bumps := [3][3]float64{
		{leftX, 0.26 * w, 0.32 * d},
		{0.26 * w, 0.74 * w, 0.06 * d},
		{0.74 * w, rightX, 0.24 * d},
	}

	nPerBump := 16
	for bi, b := range bumps {
		bLeft, bRight, bPeak := b[0], b[1], b[2]
		cx := (bLeft + bRight) / 2
		rx := (bRight - bLeft) / 2
		ry := horizonY - bPeak
		startI := 0
		if bi > 0 {
			startI = 1
		}
		for i := startI; i <= nPerBump; i++ {
			t := float64(i) / float64(nPerBump)
			angle := math.Pi - t*math.Pi
			x := cx + rx*math.Cos(angle)
			y := horizonY - ry*math.Sin(angle)
			out = append(out, [2]float64{x, y})
		}
	}

	// Right vertical edge of the trunk.
	nSide := 6
	for i := 1; i <= nSide; i++ {
		t := float64(i) / float64(nSide)
		y := horizonY + t*(bottomY-cornerR-horizonY)
		out = append(out, [2]float64{rightX, y})
	}

	// Right-bottom rounded corner (quarter circle).
	nCorner := 6
	cxR := rightX - cornerR
	cyR := bottomY - cornerR
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		angle := -t * math.Pi / 2
		x := cxR + cornerR*math.Cos(angle)
		y := cyR - cornerR*math.Sin(angle)
		out = append(out, [2]float64{x, y})
	}

	// Flat bottom from right-corner-end to left-corner-start.
	nBot := 16
	for i := 1; i <= nBot; i++ {
		t := float64(i) / float64(nBot)
		x := (rightX - cornerR) + t*((leftX+cornerR)-(rightX-cornerR))
		out = append(out, [2]float64{x, bottomY})
	}

	// Left-bottom rounded corner.
	cxL := leftX + cornerR
	cyL := bottomY - cornerR
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		angle := -math.Pi/2 - t*math.Pi/2
		x := cxL + cornerR*math.Cos(angle)
		y := cyL - cornerR*math.Sin(angle)
		out = append(out, [2]float64{x, y})
	}

	// Left vertical edge.
	for i := 1; i <= nSide; i++ {
		t := float64(i) / float64(nSide)
		y := (bottomY - cornerR) + t*(horizonY-(bottomY-cornerR))
		out = append(out, [2]float64{leftX, y})
	}

	return out
}

// RenderIsoDiamondFlat draws a flat 2D rotated-square diamond with a soft
// drop shadow. We split the diamond into two halves (lit/shaded) so it still
// reads as a faceted gem rather than a 2D sticker.
type ConvertOpts struct {
	Width, Depth, Height float64
	Sides                int // polygon sides (3+); only used by shape=polygon

	TopFill, LeftFill, RightFill string
	Stroke                       string
	StrokeWidth                  float64

	// Per-face stroke overrides (v1.2). Empty string / 0 width = inherit.
	TopStroke, LeftStroke, RightStroke                struct{ Color, Dash string; Width float64 }
	// Per-face opacity overrides (v1.2). 0 = inherit base Opacity.
	TopOpacity, LeftOpacity, RightOpacity float64
	// Linear-gradient fills per face (v1.2). Non-nil overrides the matching
	// solid fill.
	TopGradient, LeftGradient, RightGradient *FaceGradient

	Label      string
	LabelLines []string // multi-line override; falls back to splitting Label by \n
	FontFamily string
	FontSize   float64
	FontColor  string
	FontWeight string

	Icon       string
	IconScale  float64
	IconAnchor string  // center | topLeft | topRight | bottomLeft | bottomRight
	IconOffX   float64 // fraction of W (0..1)
	IconOffY   float64 // fraction of D (0..1)

	Margin       float64
	CornerRadius float64 // box family: clipped corner radius in local units

	// Drop shadow filter (v1.2). Zero color = disabled.
	ShadowDx, ShadowDy, ShadowBlur float64
	ShadowColor                    string

	// Structural payload for class / sql_table / sequence_diagram.
	Rows      []string
	RowColors []string
	Header    string

	// Direction overrides for callout / step.
	TailDir  string // callout: down | up | left | right
	ArrowDir string // step: right | left

	Opacity         float64
	StrokeDasharray string
	Background      string

	// v1.3 — material + variant knobs.
	Wireframe       bool
	BackglowColor   string
	BackglowRadius  float64
	BackglowOpacity float64
	GrainIntensity  float64
	GrainScale      float64
	PatternKind     string  // hatch | dots | grid
	PatternColor    string
	PatternSpacing  float64
	PatternAngle    float64
	Cells           [][]string
	CellColors      [][]string
}

// FaceGradient is the low-level twin of nodedsl.FaceGradient. Kept here so
// iso25d stays standalone (no cyclic import with the parent package).
type FaceGradient struct {
	From, To, Dir string
}

// Convert2DTo25D maps a 2D D2 node-type name to an iso SVG string. Every
// constant from d2target.go's shape list is handled; for shapes that don't
// have a natural 3D analogue, we use a sensible fallback (flat slab, low
// box, etc.).
func Convert2DTo25D(shapeType string, o ConvertOpts) string {
	switch strings.ToLower(strings.TrimSpace(shapeType)) {

	case "cylinder", "stored_data", "queue":
		c := DefaultIsoCylinder()
		applyCylinder(o, &c)
		return RenderIsoCylinder(c)

	case "circle":
		s := DefaultIsoSphere()
		applySphere(o, &s)
		return RenderIsoSphere(s)

	case "composite":
		// Composite parts are dispatched at the isotopo layer via
		// RenderComposite. Returning empty here is a safety net for
		// callers that bypass that layer.
		return ""

	case "person":
		p := DefaultIsoPerson()
		applyPerson(o, &p)
		p.HeadStyle = "sphere"
		return RenderIsoPerson(p)

	case "cloud":
		b := DefaultIsoBox()
		applyBox(o, &b)
		if b.Width < 200 {
			b.Width = 200
		}
		if b.Depth < 140 {
			b.Depth = 140
		}
		if b.Height < 24 {
			b.Height = 24
		}
		return RenderIsoCloud(b)

	case "iso_text", "title":
		b := DefaultIsoBox()
		applyBox(o, &b)
		return RenderIsoText(b)

	// rectangle / square / empty / unknown → plain iso box.
	case "rectangle", "square", "":
		b := DefaultIsoBox()
		applyBox(o, &b)
		if strings.ToLower(strings.TrimSpace(shapeType)) == "square" {
			s := math.Max(b.Width, b.Depth)
			b.Width, b.Depth = s, s
		}
		return RenderIsoBox(b)

	default:
		b := DefaultIsoBox()
		applyBox(o, &b)
		return RenderIsoBox(b)
	}
}

// pickDim returns a positive value, falling back to defaultVal when v <= 0.
func pickDim(v, defaultVal float64) float64 {
	if v > 0 {
		return v
	}
	return defaultVal
}

// pickStructural lowers the DSL Header / Rows / RowColors into concrete
// values for RenderIsoBoxWithDividers. Falls back to label / defaults when
// the DSL didn't supply data.
func applyBox(o ConvertOpts, b *IsoBoxOpts) {
	if o.Width > 0 {
		b.Width = o.Width
	}
	if o.Depth > 0 {
		b.Depth = o.Depth
	}
	if o.Height > 0 {
		b.Height = o.Height
	}
	if o.TopFill != "" {
		b.TopFill = o.TopFill
	}
	if o.LeftFill != "" {
		b.LeftFill = o.LeftFill
	}
	if o.RightFill != "" {
		b.RightFill = o.RightFill
	}
	if o.Stroke != "" {
		b.Stroke = o.Stroke
	}
	if o.StrokeWidth > 0 {
		b.StrokeWidth = o.StrokeWidth
	}
	if o.Label != "" {
		b.Label = o.Label
	}
	if o.FontFamily != "" {
		b.FontFamily = o.FontFamily
	}
	if o.FontSize > 0 {
		b.FontSize = o.FontSize
	}
	if o.FontColor != "" {
		b.FontColor = o.FontColor
	}
	if o.FontWeight != "" {
		b.FontWeight = o.FontWeight
	}
	if o.Icon != "" {
		b.Icon = o.Icon
	}
	if o.IconScale > 0 {
		b.IconScale = o.IconScale
	}
	if o.Margin > 0 {
		b.Margin = o.Margin
	}
	if o.Opacity > 0 {
		b.Opacity = o.Opacity
	}
	if o.StrokeDasharray != "" {
		b.StrokeDasharray = o.StrokeDasharray
	}
	if o.Background != "" {
		b.Background = o.Background
	}
	// v1.2 wiring
	b.TopStroke = o.TopStroke
	b.LeftStroke = o.LeftStroke
	b.RightStroke = o.RightStroke
	b.TopOpacity = o.TopOpacity
	b.LeftOpacity = o.LeftOpacity
	b.RightOpacity = o.RightOpacity
	b.TopGradient = o.TopGradient
	b.LeftGradient = o.LeftGradient
	b.RightGradient = o.RightGradient
	b.LabelLines = o.LabelLines
	b.IconAnchor = o.IconAnchor
	b.IconOffX = o.IconOffX
	b.IconOffY = o.IconOffY
	b.CornerRadius = o.CornerRadius
	b.ShadowDx, b.ShadowDy, b.ShadowBlur, b.ShadowColor = o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor
	// v1.3
	b.Wireframe = o.Wireframe
	b.BackglowColor = o.BackglowColor
	b.BackglowRadius = o.BackglowRadius
	b.BackglowOpacity = o.BackglowOpacity
	b.GrainIntensity = o.GrainIntensity
	b.GrainScale = o.GrainScale
	b.PatternKind = o.PatternKind
	b.PatternColor = o.PatternColor
	b.PatternSpacing = o.PatternSpacing
	b.PatternAngle = o.PatternAngle
	b.Cells = o.Cells
	b.CellColors = o.CellColors
}

func applyCylinder(o ConvertOpts, c *IsoCylinderOpts) {
	if o.Width > 0 {
		c.Radius = o.Width / 2
	}
	if o.Height > 0 {
		c.Height = o.Height
	}
	if o.TopFill != "" {
		c.TopFill = o.TopFill
	}
	if o.LeftFill != "" {
		c.LeftFill = o.LeftFill
	}
	if o.RightFill != "" {
		c.RightFill = o.RightFill
	}
	if o.Stroke != "" {
		c.Stroke = o.Stroke
	}
	if o.StrokeWidth > 0 {
		c.StrokeWidth = o.StrokeWidth
	}
	if o.Label != "" {
		c.Label = o.Label
	}
	if o.FontFamily != "" {
		c.FontFamily = o.FontFamily
	}
	if o.FontSize > 0 {
		c.FontSize = o.FontSize
	}
	if o.FontColor != "" {
		c.FontColor = o.FontColor
	}
	if o.FontWeight != "" {
		c.FontWeight = o.FontWeight
	}
	if o.Icon != "" {
		c.Icon = o.Icon
	}
	if o.IconScale > 0 {
		c.IconScale = o.IconScale
	}
	if o.Margin > 0 {
		c.Margin = o.Margin
	}
	if o.Opacity > 0 {
		c.Opacity = o.Opacity
	}
	if o.StrokeDasharray != "" {
		c.StrokeDasharray = o.StrokeDasharray
	}
	if o.Background != "" {
		c.Background = o.Background
	}
}

func applySphere(o ConvertOpts, s *IsoSphereOpts) {
	if o.Width > 0 {
		s.Radius = o.Width / 2
	}
	if o.TopFill != "" {
		s.Highlight = o.TopFill
	}
	if o.LeftFill != "" {
		s.Shadow = o.LeftFill
	}
	if o.Stroke != "" {
		s.Stroke = o.Stroke
	}
	if o.StrokeWidth > 0 {
		s.StrokeWidth = o.StrokeWidth
	}
	if o.Label != "" {
		s.Label = o.Label
	}
	if o.FontFamily != "" {
		s.FontFamily = o.FontFamily
	}
	if o.FontSize > 0 {
		s.FontSize = o.FontSize
	}
	if o.FontColor != "" {
		s.FontColor = o.FontColor
	}
	if o.FontWeight != "" {
		s.FontWeight = o.FontWeight
	}
	if o.Margin > 0 {
		s.Margin = o.Margin
	}
	if o.Opacity > 0 {
		s.Opacity = o.Opacity
	}
	if o.StrokeDasharray != "" {
		s.StrokeDasharray = o.StrokeDasharray
	}
	if o.Background != "" {
		s.Background = o.Background
	}
}

func applyPerson(o ConvertOpts, p *IsoPersonOpts) {
	if o.Width > 0 {
		p.BodyWidth = o.Width
		p.HeadRadius = o.Width * 0.27
	}
	if o.Depth > 0 {
		p.BodyDepth = o.Depth
	}
	if o.Height > 0 {
		p.BodyHeight = o.Height
	}
	if o.TopFill != "" {
		p.BodyTop = o.TopFill
		// Head highlight reuses the body's top fill so the figure reads as
		// one unified colour family. Sphere gradient already shades it.
		p.HeadHighlight = o.TopFill
	}
	if o.LeftFill != "" {
		p.BodyLeft = o.LeftFill
		p.HeadShadow = o.LeftFill
	}
	if o.RightFill != "" {
		p.BodyRight = o.RightFill
	}
	if o.Stroke != "" {
		p.Stroke = o.Stroke
	}
	if o.StrokeWidth > 0 {
		p.StrokeWidth = o.StrokeWidth
	}
	if o.Label != "" {
		p.Label = o.Label
	}
	if o.FontFamily != "" {
		p.FontFamily = o.FontFamily
	}
	if o.FontSize > 0 {
		p.FontSize = o.FontSize
	}
	if o.FontColor != "" {
		p.FontColor = o.FontColor
	}
	if o.FontWeight != "" {
		p.FontWeight = o.FontWeight
	}
	if o.Margin > 0 {
		p.Margin = o.Margin
	}
	if o.Opacity > 0 {
		p.Opacity = o.Opacity
	}
	if o.StrokeDasharray != "" {
		p.StrokeDasharray = o.StrokeDasharray
	}
	if o.Background != "" {
		p.Background = o.Background
	}
}
