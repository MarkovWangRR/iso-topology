package iso25d

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// CustomPathShapeProvider handles custom_path shapes defined by an SVG-like path.
type CustomPathShapeProvider struct{}

func (CustomPathShapeProvider) Names() []string { return []string{"custom_path"} }

// parseMLZPath parses a simplified "M x,y L x,y ... Z" path string.
func parseMLZPath(s string) [][2]float64 {
	var pts [][2]float64
	parts := strings.Fields(strings.ToUpper(s))
	for i := 0; i < len(parts); i++ {
		tok := parts[i]
		if tok == "M" || tok == "L" {
			if i+1 >= len(parts) {
				break
			}
			i++
			coords := strings.Split(parts[i], ",")
			if len(coords) == 2 {
				x, ex := strconv.ParseFloat(coords[0], 64)
				y, ey := strconv.ParseFloat(coords[1], 64)
				if ex == nil && ey == nil {
					pts = append(pts, [2]float64{x, y})
				}
			}
		} else if tok == "Z" {
			break
		} else {
			// Try parsing as "x,y" directly
			coords := strings.Split(tok, ",")
			if len(coords) == 2 {
				x, ex := strconv.ParseFloat(coords[0], 64)
				y, ey := strconv.ParseFloat(coords[1], 64)
				if ex == nil && ey == nil {
					pts = append(pts, [2]float64{x, y})
				}
			}
		}
	}
	return pts
}

// normalizeOutline scales points to fit within [0,w] x [0,d].
func normalizeOutline(pts [][2]float64, w, d float64) [][2]float64 {
	if len(pts) == 0 {
		return pts
	}
	minX, minY := pts[0][0], pts[0][1]
	maxX, maxY := pts[0][0], pts[0][1]
	for _, p := range pts[1:] {
		if p[0] < minX {
			minX = p[0]
		}
		if p[0] > maxX {
			maxX = p[0]
		}
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}
	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1e-9 {
		rangeX = 1
	}
	if rangeY < 1e-9 {
		rangeY = 1
	}
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		out[i] = [2]float64{
			(p[0]-minX)/rangeX*w,
			(p[1]-minY)/rangeY*d,
		}
	}
	return out
}

// centroid2D computes the centroid of a polygon.
func centroid2D(pts [][2]float64) (float64, float64) {
	cx, cy := 0.0, 0.0
	for _, p := range pts {
		cx += p[0]
		cy += p[1]
	}
	return cx / float64(len(pts)), cy / float64(len(pts))
}

// extrudeOutline creates side faces for each edge + top face.
// Visibility is determined by outward edge normal dot (1,1).
func extrudeOutline(outline [][2]float64, w, d, h float64) []Face {
	n := len(outline)
	cx, cy := centroid2D(outline)
	var faces []Face
	for k := 0; k < n; k++ {
		a, b := outline[k], outline[(k+1)%n]
		nx, ny := b[1]-a[1], a[0]-b[0]
		mx, my := (a[0]+b[0])/2-cx, (a[1]+b[1])/2-cy
		if nx*mx+ny*my < 0 {
			nx, ny = -nx, -ny
		}
		l := math.Hypot(nx, ny)
		if l < 1e-9 {
			continue
		}
		nx, ny = nx/l, ny/l
		visible := nx+ny > 1e-9
		faces = append(faces, Face{
			Name: fmt.Sprintf("side%d", k),
			Points: [][2]float64{
				prismLocal(d, h, a[0], a[1], 0),
				prismLocal(d, h, a[0], a[1], h),
				prismLocal(d, h, b[0], b[1], h),
				prismLocal(d, h, b[0], b[1], 0),
			},
			Normal:  [3]float64{nx, ny, 0},
			Visible: visible,
		})
	}
	// top face
	top := Face{
		Name:    "top",
		Normal:  [3]float64{0, 0, 1},
		Visible: true,
	}
	for _, v := range outline {
		top.Points = append(top.Points, prismLocal(d, h, v[0], v[1], h))
	}
	faces = append(faces, top)
	for i := range faces {
		faces[i].ZOrder = i
	}
	return faces
}

func (CustomPathShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	path := ""
	if params != nil {
		if p, ok := params["path"].(string); ok {
			path = p
		}
	}
	pts := parseMLZPath(path)
	if len(pts) < 3 {
		return BoxShapeProvider{}.Faces(w, d, h, params)
	}
	outline := normalizeOutline(pts, w, d)
	return extrudeOutline(outline, w, d, h)
}

func (CustomPathShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	path := ""
	if params != nil {
		if p, ok := params["path"].(string); ok {
			path = p
		}
	}
	pts := parseMLZPath(path)
	if len(pts) < 3 {
		return BoxShapeProvider{}.Silhouette(w, d, h, params)
	}
	outline := normalizeOutline(pts, w, d)
	all := make([][2]float64, 0, len(outline)*2)
	for _, v := range outline {
		all = append(all, prismLocal(d, h, v[0], v[1], 0))
		all = append(all, prismLocal(d, h, v[0], v[1], h))
	}
	return convexHull(all)
}

func (CustomPathShapeProvider) ContentAnchor() string { return "top" }

func (CustomPathShapeProvider) ContentRectFor(w, d, h float64, _ map[string]any) ContentRect {
	return ContentRect{X: w * 0.1, Y: d * 0.1, W: w * 0.8, H: d * 0.8}
}

func (CustomPathShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(CustomPathShapeProvider{})
}

// RenderIsoCustomPath renders a custom path shape.
func RenderIsoCustomPath(o IsoBoxOpts, params map[string]any) string {
	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)
	var prov CustomPathShapeProvider

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "custom-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "custom-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "custom-grain", o.GrainIntensity, o.GrainScale)
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
	for _, f := range prov.Faces(o.Width, o.Depth, o.Height, params) {
		if !f.Visible {
			continue
		}
		fill := o.TopFill
		if f.Name != "top" {
			if f.Normal[0] >= f.Normal[1] {
				fill = o.RightFill
			} else {
				fill = o.LeftFill
			}
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
