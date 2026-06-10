package iso25d

import (
	"fmt"
	"math"
	"strings"
)

// Cylinder (database / queue / stored_data) shape implementation.
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

	// v2.4 — hatch/dots texture overlaid on the top ellipse.
	PatternKind    string
	PatternColor   string
	PatternSpacing float64
	PatternAngle   float64
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
	// v2.4 — texture overlay on the top ellipse.
	{
		var defs strings.Builder
		if patID := emitPatternDef(&defs, "cyl-pattern", o.PatternKind, o.PatternColor, o.PatternSpacing, o.PatternAngle); patID != "" {
			fmt.Fprintf(&sb, `<defs>%s</defs><ellipse data-face="top-pattern" cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" fill="url(#%s)" stroke="none"/>`,
				defs.String(), sx(topCx), sy(topCy), rx, ry, patID)
		}
	}

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


// applyCylinder lowers a ConvertOpts into an IsoCylinderOpts.
func applyCylinder(o ConvertOpts, c *IsoCylinderOpts) {
	c.PatternKind, c.PatternColor = o.PatternKind, o.PatternColor
	c.PatternSpacing, c.PatternAngle = o.PatternSpacing, o.PatternAngle
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

