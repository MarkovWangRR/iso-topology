package iso25d

import (
	"fmt"
	"strings"
)

func init() { RegisterShape(ScreenShapeProvider{}) }

type ScreenShapeProvider struct{}

func (ScreenShapeProvider) Names() []string { return []string{"screen", "browser-panel"} }

func (ScreenShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	return (BoxShapeProvider{}).Faces(w, d, h, params)
}

func (ScreenShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	return (BoxShapeProvider{}).Silhouette(w, d, h, params)
}

func (ScreenShapeProvider) ContentAnchor() string { return "top" }

func (ScreenShapeProvider) ContentRectFor(w, d, h float64, params map[string]any) ContentRect {
	return (BoxShapeProvider{}).ContentRectFor(w, d, h, params)
}

func (ScreenShapeProvider) Footprint(w, d, h float64) (float64, float64) {
	return (BoxShapeProvider{}).Footprint(w, d, h)
}

func RenderIsoScreen(o IsoBoxOpts) string {
	if o.Depth <= 0 {
		o.Depth = 14
	}
	if o.Width <= 0 {
		o.Width = 100
	}
	if o.Height <= 0 {
		o.Height = 160
	}

	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "scr-blur", o.Blur)

	topFill, leftFill, rightFill, shadowID, patID, grainID := emitBoxDefs(&sb, &o)

	if strings.TrimSpace(o.BackglowColor) != "" && o.BackglowRadius > 0 {
		var gdefs strings.Builder
		sil := (BoxShapeProvider{}).Silhouette(o.Width, o.Depth, o.Height, nil)
		hpts := make([][2]float64, len(sil))
		for k, q := range sil {
			hpts[k] = [2]float64{q[0] + o.Margin, q[1] + o.Margin}
		}
		var halo strings.Builder
		emitBackglowHalo(&halo, &gdefs, "scr-backglow", o.BackglowColor, o.BackglowRadius, o.BackglowOpacity, hpts)
		fmt.Fprintf(&sb, `<defs>%s</defs>%s`, gdefs.String(), halo.String())
	}

	openWrapper(&sb, g.ViewW, g.ViewH, o.Background, o.Opacity, o.StrokeDasharray, shadowID)
	if grainID != "" {
		fmt.Fprintf(&sb, `<g filter="url(#%s)">`, grainID)
	}
	if o.Wireframe {
		topFill, leftFill, rightFill, patID = "none", "none", "none", ""
	}

	leftC, leftW, leftD := pickFaceStroke(o.Stroke, o.StrokeWidth, o.StrokeDasharray, o.LeftStroke)
	rightC, rightW, rightD := pickFaceStroke(o.Stroke, o.StrokeWidth, o.StrokeDasharray, o.RightStroke)
	topC, topW, topD := pickFaceStroke(o.Stroke, o.StrokeWidth, o.StrokeDasharray, o.TopStroke)

	pf := providerFacePoints(o.Width, o.Depth, o.Height, o.Margin)
	var faceDefs strings.Builder
	faceFill := func(name, fallback string) string {
		fs := surfaceFor(o.FaceSurfaces, name)
		if fs == nil || fs.Fill == nil {
			return fallback
		}
		if ref := emitFaceFill(&faceDefs, "", name, fs.Fill); ref != "" {
			return ref
		}
		return fallback
	}
	leftFill = faceFill("left", leftFill)
	rightFill = faceFill("right", rightFill)
	topFill = faceFill("top", topFill)
	if faceDefs.Len() > 0 {
		fmt.Fprintf(&sb, `<defs>%s</defs>`, faceDefs.String())
	}
	writeFace(&sb, "left", leftFill, leftC, leftW, leftD, 0, pf["left"]...)
	writeFace(&sb, "right", rightFill, rightC, rightW, rightD, 0, pf["right"]...)
	writeFace(&sb, "top", topFill, topC, topW, topD, 0, pf["top"]...)
	for _, name := range []string{"left", "right", "top"} {
		if fs := surfaceFor(o.FaceSurfaces, name); fs != nil && len(fs.Strokes) > 0 {
			writeFaceStrokeLayers(&sb, name, fs.Strokes, pf[name]...)
		}
	}
	if patID != "" {
		writeFace(&sb, "top-pattern", "url(#"+patID+")", "", 0, "", 0, pf["top"]...)
	}
	if grainID != "" {
		sb.WriteString(`</g>`)
	}

	// Label on the top face (same as box).
	writeTopLabelAndIconV12(
		&sb,
		g.E[0], g.E[1], o.Width, o.Depth,
		o.Label, o.LabelLines, o.Icon, o.IconScale, o.IconAnchor, o.IconOffX, o.IconOffY,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	if o.OutlineColor != "" && o.OutlineWidth > 0 {
		sil := (BoxShapeProvider{}).Silhouette(o.Width, o.Depth, o.Height, nil)
		for k := range sil {
			sil[k][0] += o.Margin
			sil[k][1] += o.Margin
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
