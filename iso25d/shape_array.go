package iso25d

import (
	"fmt"
	"strings"
)

func init() { RegisterShape(ArrayShapeProvider{}) }

type ArrayShapeProvider struct{}

func (ArrayShapeProvider) Names() []string {
	return []string{"array1d", "array2d", "array3d"}
}

func (ArrayShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	return (BoxShapeProvider{}).Faces(w, d, h, params)
}

func (ArrayShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	return (BoxShapeProvider{}).Silhouette(w, d, h, params)
}

func (ArrayShapeProvider) ContentAnchor() string { return "top" }

func (ArrayShapeProvider) ContentRectFor(w, d, h float64, params map[string]any) ContentRect {
	return (BoxShapeProvider{}).ContentRectFor(w, d, h, params)
}

func (ArrayShapeProvider) Footprint(w, d, h float64) (float64, float64) {
	return (BoxShapeProvider{}).Footprint(w, d, h)
}

func arrayResolveParams(shapeName string, params map[string]any) (cX, cY, cZ int, gap float64) {
	cX, cY, cZ = 3, 3, 1
	gap = 6
	switch shapeName {
	case "array1d":
		cY, cZ = 1, 1
	case "array3d":
		cZ = 3
	}
	intParam := func(key string) (int, bool) {
		if params == nil {
			return 0, false
		}
		if v, ok := params[key].(int); ok && v > 0 {
			return v, true
		}
		if v, ok := params[key].(float64); ok && v > 0 {
			return int(v), true
		}
		return 0, false
	}
	if v, ok := intParam("countX"); ok {
		cX = v
	}
	if v, ok := intParam("countY"); ok {
		cY = v
	}
	if v, ok := intParam("countZ"); ok {
		cZ = v
	}
	if params != nil {
		if v, ok := params["gap"].(float64); ok && v >= 0 {
			gap = v
		}
		if v, ok := params["gap"].(int); ok && v >= 0 {
			gap = float64(v)
		}
	}
	return
}

func RenderIsoArray(o IsoBoxOpts, shapeName string) string {
	cX, cY, cZ, gap := arrayResolveParams(shapeName, o.Params)

	gapXTotal := gap * float64(cX-1)
	gapYTotal := gap * float64(cY-1)
	gapZTotal := gap * float64(cZ-1)
	cellW := (o.Width - gapXTotal) / float64(cX)
	cellD := (o.Depth - gapYTotal) / float64(cY)
	cellH := (o.Height - gapZTotal) / float64(cZ)
	if cellW <= 4 {
		cellW = 4
	}
	if cellD <= 4 {
		cellD = 4
	}
	if cellH <= 4 {
		cellH = 4
	}

	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "arr-blur", o.Blur)

	var defs strings.Builder
	shadowID := ""
	if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
		shadowID = "arr-shadow"
		emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
	}
	grainID := emitGrainFilter(&defs, "arr-grain", o.GrainIntensity, o.GrainScale)
	if defs.Len() > 0 {
		fmt.Fprintf(&sb, `<defs>%s</defs>`, defs.String())
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

	proj := func(x, y, z float64) [2]float64 {
		p := prismLocal(o.Depth, o.Height, x, y, z)
		return [2]float64{p[0] + m, p[1] + m}
	}

	for k := 0; k < cZ; k++ {
		for j := cY - 1; j >= 0; j-- {
			for i := 0; i < cX; i++ {
				offX := float64(i) * (cellW + gap)
				offY := float64(j) * (cellD + gap)
				offZ := float64(k) * (cellH + gap)

				x0, y0, z0 := offX, offY, offZ
				x1, y1, z1 := offX+cellW, offY+cellD, offZ+cellH

				leftPts := [][2]float64{
					proj(x0, y0, z0), proj(x0, y0, z1),
					proj(x0, y1, z1), proj(x0, y1, z0),
				}
				writeFace(&sb, fmt.Sprintf("cell-%d-%d-%d-left", i, j, k),
					o.LeftFill, stroke, sw*0.7, "", 0, leftPts...)

				rightPts := [][2]float64{
					proj(x0, y1, z0), proj(x0, y1, z1),
					proj(x1, y1, z1), proj(x1, y1, z0),
				}
				writeFace(&sb, fmt.Sprintf("cell-%d-%d-%d-right", i, j, k),
					o.RightFill, stroke, sw*0.7, "", 0, rightPts...)

				topPts := [][2]float64{
					proj(x0, y0, z1), proj(x1, y0, z1),
					proj(x1, y1, z1), proj(x0, y1, z1),
				}
				writeFace(&sb, fmt.Sprintf("cell-%d-%d-%d-top", i, j, k),
					o.TopFill, stroke, sw*0.7, "", 0, topPts...)

				_, _, _ = x1, y0, z0
			}
		}
	}

	if grainID != "" {
		sb.WriteString(`</g>`)
	}

	writeTopLabelAndIconV12(
		&sb,
		g.E[0], g.E[1], o.Width, o.Depth,
		o.Label, o.LabelLines, o.Icon, o.IconScale, o.IconAnchor, o.IconOffX, o.IconOffY,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	closeWrapper(&sb)
	if blurOn {
		sb.WriteString(`</g>`)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}
