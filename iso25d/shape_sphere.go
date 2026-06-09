package iso25d

import (
	"fmt"
	"strings"
)

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

