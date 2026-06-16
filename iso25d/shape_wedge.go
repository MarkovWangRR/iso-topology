package iso25d

import (
	"fmt"
	"strings"
)

// WedgeShapeProvider handles the wedge shape.
// Bottom: A(0,0,0) B(w,0,0) C(w,d,0) D(0,d,0)
// Top back edge at z=h: E(0,0,h) F(w,0,h)
// Front edge stays at z=0: C and D
type WedgeShapeProvider struct{}

func (WedgeShapeProvider) Names() []string { return []string{"wedge"} }

func (WedgeShapeProvider) Faces(w, d, h float64, _ map[string]any) []Face {
	// slope face: E(0,0,h), F(w,0,h), C(w,d,0), D(0,d,0)
	// Normal: reversed winding gives (0, +wh, +wd) → visible
	slope := Face{
		Name: "slope",
		Points: [][2]float64{
			prismLocal(d, h, 0, 0, h),
			prismLocal(d, h, w, 0, h),
			prismLocal(d, h, w, d, 0),
			prismLocal(d, h, 0, d, 0),
		},
		Normal:  [3]float64{0, 0.707, 0.707},
		Visible: true,
		ZOrder:  0,
	}
	// right triangle: B(w,0,0), C(w,d,0), F(w,0,h) — normal (1,0,0) → visible
	right := Face{
		Name: "right",
		Points: [][2]float64{
			prismLocal(d, h, w, 0, 0),
			prismLocal(d, h, w, d, 0),
			prismLocal(d, h, w, 0, h),
		},
		Normal:  [3]float64{1, 0, 0},
		Visible: true,
		ZOrder:  1,
	}
	// left triangle: A(0,0,0), E(0,0,h), D(0,d,0) — normal (-1,0,0) → NOT visible
	left := Face{
		Name: "left",
		Points: [][2]float64{
			prismLocal(d, h, 0, 0, 0),
			prismLocal(d, h, 0, 0, h),
			prismLocal(d, h, 0, d, 0),
		},
		Normal:  [3]float64{-1, 0, 0},
		Visible: false,
		ZOrder:  2,
	}
	// back wall: A(0,0,0), B(w,0,0), F(w,0,h), E(0,0,h) — normal (0,-1,0) → NOT visible
	back := Face{
		Name: "back",
		Points: [][2]float64{
			prismLocal(d, h, 0, 0, 0),
			prismLocal(d, h, w, 0, 0),
			prismLocal(d, h, w, 0, h),
			prismLocal(d, h, 0, 0, h),
		},
		Normal:  [3]float64{0, -1, 0},
		Visible: false,
		ZOrder:  3,
	}
	return []Face{left, back, slope, right}
}

func (WedgeShapeProvider) Silhouette(w, d, h float64, _ map[string]any) [][2]float64 {
	pts := [][2]float64{
		prismLocal(d, h, 0, 0, 0),
		prismLocal(d, h, w, 0, 0),
		prismLocal(d, h, w, d, 0),
		prismLocal(d, h, 0, d, 0),
		prismLocal(d, h, 0, 0, h),
		prismLocal(d, h, w, 0, h),
	}
	return convexHull(pts)
}

func (WedgeShapeProvider) ContentAnchor() string { return "top" }

func (WedgeShapeProvider) ContentRectFor(w, d, h float64, _ map[string]any) ContentRect {
	return ContentRect{X: w * 0.1, Y: d * 0.1, W: w * 0.8, H: d * 0.8}
}

func (WedgeShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(WedgeShapeProvider{})
}

// RenderIsoWedge renders a wedge shape.
func RenderIsoWedge(o IsoBoxOpts) string {
	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)
	var prov WedgeShapeProvider

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "wedge-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "wedge-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "wedge-grain", o.GrainIntensity, o.GrainScale)
		if defs.Len() > 0 {
			fmt.Fprintf(&sb, `<defs>%s</defs>`, defs.String())
		}
	}

	openWrapper(&sb, g.ViewW, g.ViewH, o.Background, o.Opacity, o.StrokeDasharray, shadowID)
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

	m := o.Margin
	for _, f := range prov.Faces(o.Width, o.Depth, o.Height, nil) {
		if !f.Visible {
			continue
		}
		fill := o.TopFill
		switch f.Name {
		case "slope":
			fill = o.TopFill
		case "right":
			fill = o.RightFill
		}
		pts := make([][2]float64, len(f.Points))
		for i, p := range f.Points {
			pts[i] = [2]float64{p[0] + m, p[1] + m}
		}
		writeFace(&sb, f.Name, fill, stroke, sw, "", 0, pts...)
	}

	if grainID != "" {
		sb.WriteString(`</g>`)
	}

	cr := prov.ContentRectFor(o.Width, o.Depth, o.Height, nil)
	ox, oy := project(cr.X, cr.Y, o.Height)
	writeTopLabelAndIconV12(
		&sb,
		ox+g.Tx, oy+g.Ty, cr.W, cr.H,
		o.Label, o.LabelLines, o.Icon, o.IconScale, o.IconAnchor, o.IconOffX, o.IconOffY,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	if o.OutlineColor != "" && o.OutlineWidth > 0 {
		sil := prov.Silhouette(o.Width, o.Depth, o.Height, nil)
		for k := range sil {
			sil[k][0] += m
			sil[k][1] += m
		}
		emitOutlineRing(&sb, sil, o.OutlineColor, o.OutlineWidth, o.OutlineDash, o.OutlineOpacity)
	}
	closeWrapper(&sb)
	if blurOn {
		sb.WriteString(`</g>`)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}
