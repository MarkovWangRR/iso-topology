package iso25d

import (
	"fmt"
	"strings"
)

// RevolveShapeProvider handles dome, torus, and capsule shapes.
// These are approximated using BoxShapeProvider faces as a fallback.
type RevolveShapeProvider struct{}

func (RevolveShapeProvider) Names() []string {
	return []string{"dome", "torus", "capsule"}
}

func (RevolveShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	return BoxShapeProvider{}.Faces(w, d, h, params)
}

func (RevolveShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	return BoxShapeProvider{}.Silhouette(w, d, h, params)
}

func (RevolveShapeProvider) ContentAnchor() string { return "top" }

func (RevolveShapeProvider) ContentRectFor(w, d, h float64, params map[string]any) ContentRect {
	return BoxShapeProvider{}.ContentRectFor(w, d, h, params)
}

func (RevolveShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(RevolveShapeProvider{})
}

// RenderIsoRevolve renders dome, torus, or capsule shapes.
func RenderIsoRevolve(o IsoBoxOpts, kind string) string {
	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)
	var prov RevolveShapeProvider

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "revolve-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "revolve-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "revolve-grain", o.GrainIntensity, o.GrainScale)
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
	params := map[string]any{"kind": kind}
	for _, f := range prov.Faces(o.Width, o.Depth, o.Height, params) {
		if !f.Visible {
			continue
		}
		fill := o.TopFill
		switch f.Name {
		case "left":
			fill = o.LeftFill
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

	cr := prov.ContentRectFor(o.Width, o.Depth, o.Height, params)
	ox, oy := project(cr.X, cr.Y, o.Height)
	writeTopLabelAndIconV12(
		&sb,
		ox+g.Tx, oy+g.Ty, cr.W, cr.H,
		o.Label, o.LabelLines, o.Icon, o.IconScale, o.IconAnchor, o.IconOffX, o.IconOffY,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	if o.OutlineColor != "" && o.OutlineWidth > 0 {
		sil := prov.Silhouette(o.Width, o.Depth, o.Height, params)
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
