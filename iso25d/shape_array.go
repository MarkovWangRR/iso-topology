package iso25d

import (
	"fmt"
	"strings"
)

// ArrayShapeProvider — G-category: N×M×K tiled cell grid.
// Each cell is a small box. Painter order: back-left-bottom cells first.
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

func (ArrayShapeProvider) Footprint(w, d, h float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(ArrayShapeProvider{})
}

// arrayParams resolves count and gap from params and shapeName defaults.
func arrayParams(shapeName string, params map[string]any) (cX, cY, cZ int, gap float64) {
	cX, cY, cZ = 3, 3, 1
	gap = 6
	switch shapeName {
	case "array1d":
		cY, cZ = 1, 1
	case "array3d":
		cZ = 3
	}
	if params != nil {
		if v, ok := params["countX"].(int); ok && v > 0 {
			cX = v
		}
		if v, ok := params["countX"].(float64); ok && v > 0 {
			cX = int(v)
		}
		if v, ok := params["countY"].(int); ok && v > 0 {
			cY = v
		}
		if v, ok := params["countY"].(float64); ok && v > 0 {
			cY = int(v)
		}
		if v, ok := params["countZ"].(int); ok && v > 0 {
			cZ = v
		}
		if v, ok := params["countZ"].(float64); ok && v > 0 {
			cZ = int(v)
		}
		if v, ok := params["gap"].(float64); ok && v >= 0 {
			gap = v
		}
		if v, ok := params["gap"].(int); ok && v >= 0 {
			gap = float64(v)
		}
	}
	return
}

// RenderIsoArray renders a tiled N×M×K grid of small iso boxes.
// The grid's overall bounding box is o.Width × o.Depth × o.Height.
func RenderIsoArray(o IsoBoxOpts, shapeName string) string {
	cX, cY, cZ, gap := arrayParams(shapeName, o.Params)

	// Compute cell dimensions from grid parameters
	gapXTotal := gap * float64(cX-1)
	gapYTotal := gap * float64(cY-1)
	gapZTotal := gap * float64(cZ-1)
	cellW := (o.Width - gapXTotal) / float64(cX)
	cellD := (o.Depth - gapYTotal) / float64(cY)
	cellH := (o.Height - gapZTotal) / float64(cZ)
	if cellW <= 0 {
		cellW = 4
	}
	if cellD <= 0 {
		cellD = 4
	}
	if cellH <= 0 {
		cellH = 4
	}

	// The array's total world dimensions
	arrayW := o.Width
	arrayD := o.Depth
	arrayH := o.Height

	g := computeBoxGeom(arrayW, arrayD, arrayH, o.Margin)

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "arr-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "arr-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "arr-grain", o.GrainIntensity, o.GrainScale)
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

	// proj projects a world (x,y,z) point using prismLocal with the array's bounding box.
	proj := func(x, y, z float64) [2]float64 {
		p := prismLocal(arrayD, arrayH, x, y, z)
		return [2]float64{p[0] + m, p[1] + m}
	}

	// Painter order: k=bottom..top, j=back..front, i=left..right
	// Within each cell: left face, right face, top face
	for k := 0; k < cZ; k++ {
		for j := cY - 1; j >= 0; j-- {
			for i := 0; i < cX; i++ {
				offX := float64(i) * (cellW + gap)
				offY := float64(j) * (cellD + gap)
				offZ := float64(k) * (cellH + gap)

				x0, y0, z0 := offX, offY, offZ
				x1, y1, z1 := offX+cellW, offY+cellD, offZ+cellH

				// Front-left wall: y=y1 (screen-left in this projection), spans x.
				leftPts := [4][2]float64{
					proj(x0, y1, z0),
					proj(x0, y1, z1),
					proj(x1, y1, z1),
					proj(x1, y1, z0),
				}
				writeFace(&sb, fmt.Sprintf("cell-%d-%d-%d-left", i, j, k),
					o.LeftFill, stroke, sw*0.7, "", 0, leftPts[:]...)

				// Front-right wall: x=x1 (screen-right), spans y. The old code drew
				// x=x0 — a hidden BACK face — leaving every exposed cell's right
				// side open; x=x1 is the camera-facing wall that closes the cell.
				rightPts := [4][2]float64{
					proj(x1, y0, z0),
					proj(x1, y0, z1),
					proj(x1, y1, z1),
					proj(x1, y1, z0),
				}
				writeFace(&sb, fmt.Sprintf("cell-%d-%d-%d-right", i, j, k),
					o.RightFill, stroke, sw*0.7, "", 0, rightPts[:]...)

				// Top face: z=z1, corners at (x0,y0), (x1,y0), (x1,y1), (x0,y1)
				topPts := [4][2]float64{
					proj(x0, y0, z1),
					proj(x1, y0, z1),
					proj(x1, y1, z1),
					proj(x0, y1, z1),
				}
				writeFace(&sb, fmt.Sprintf("cell-%d-%d-%d-top", i, j, k),
					o.TopFill, stroke, sw*0.7, "", 0, topPts[:]...)

				_ = x1
				_ = y0
				_ = z0
			}
		}
	}

	if grainID != "" {
		sb.WriteString(`</g>`)
	}

	// Label at center top of the entire array
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
