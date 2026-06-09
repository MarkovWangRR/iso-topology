package iso25d

import (
	"fmt"
	"math"
	"strings"
)

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
		// Dome-shaped body proportions: roughly 2× wider than tall so the
		// torso reads as "shoulders" rather than a column. Head radius is
		// ~0.3× body width, matching the classic user-avatar look.
		HeadRadius: 36,
		BodyWidth:  120, BodyDepth: 120, BodyHeight: 50,
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
	m := o.Margin
	if m <= 0 {
		m = 24
	}

	// Local frame: torso top ellipse centred at (0, 0). Bottom at (0, h).
	// Sphere head sits TANGENT to the top ellipse — its bottommost point
	// touches the top-ellipse's apex (the closest visual point of the
	// ellipse in iso projection) with zero gap.
	headCx := 0.0
	headCy := -ry - headR

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
		anchorY := sy(0) - mini.ViewH
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
