package iso25d

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// TaperedPrismShapeProvider handles cone, pyramid, and frustum shapes.
// A tapered prism has a base polygon at z=0 and a smaller (or zero-size)
// top polygon at z=h.
type TaperedPrismShapeProvider struct{}

func (TaperedPrismShapeProvider) Names() []string {
	return []string{"cone", "pyramid", "frustum"}
}

func taperedBase(w, d float64, n int) [][2]float64 {
	cx, cy := w/2, d/2
	rx, ry := w/2, d/2
	th0 := -math.Pi / 2
	if n == 4 {
		th0 += math.Pi / 8
	}
	out := make([][2]float64, n)
	for k := 0; k < n; k++ {
		th := th0 + 2*math.Pi*float64(k)/float64(n)
		out[k] = [2]float64{cx + rx*math.Cos(th), cy + ry*math.Sin(th)}
	}
	return out
}

// taperedParams returns (n sides, topScale) for each kind.
func taperedParams(kind string) (int, float64) {
	switch kind {
	case "cone":
		return 8, 0.0
	case "pyramid":
		return 4, 0.0
	case "frustum":
		return 8, 0.5
	default:
		return 8, 0.0
	}
}

func (TaperedPrismShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	kind := "cone"
	if params != nil {
		if k, ok := params["kind"].(string); ok {
			kind = k
		}
	}
	n, topScale := taperedParams(kind)
	base := taperedBase(w, d, n)
	cx, cy := w/2, d/2

	var faces []Face
	for k := 0; k < n; k++ {
		a, b := base[k], base[(k+1)%n]
		// outward normal
		nx, ny := b[1]-a[1], a[0]-b[0]
		mx, my := (a[0]+b[0])/2-cx, (a[1]+b[1])/2-cy
		if nx*mx+ny*my < 0 {
			nx, ny = -nx, -ny
		}
		l := math.Hypot(nx, ny)
		if l == 0 {
			continue
		}
		nx, ny = nx/l, ny/l
		visible := nx+ny > 1e-9

		// top vertices: scaled toward center
		ta := [2]float64{cx + (a[0]-cx)*topScale, cy + (a[1]-cy)*topScale}
		tb := [2]float64{cx + (b[0]-cx)*topScale, cy + (b[1]-cy)*topScale}

		var pts [][2]float64
		if topScale < 1e-9 {
			// triangle (apex)
			apex := [2]float64{cx, cy}
			pts = [][2]float64{
				prismLocal(d, h, a[0], a[1], 0),
				prismLocal(d, h, apex[0], apex[1], h),
				prismLocal(d, h, b[0], b[1], 0),
			}
		} else {
			pts = [][2]float64{
				prismLocal(d, h, a[0], a[1], 0),
				prismLocal(d, h, ta[0], ta[1], h),
				prismLocal(d, h, tb[0], tb[1], h),
				prismLocal(d, h, b[0], b[1], 0),
			}
		}
		faces = append(faces, Face{
			Name:    fmt.Sprintf("side%d", k),
			Points:  pts,
			Normal:  [3]float64{nx, ny, 0},
			Visible: visible,
		})
	}

	sort.SliceStable(faces, func(i, j int) bool {
		if faces[i].Visible != faces[j].Visible {
			return !faces[i].Visible
		}
		di := faces[i].Points[0][0] + faces[i].Points[0][1]
		dj := faces[j].Points[0][0] + faces[j].Points[0][1]
		return di < dj
	})

	if topScale >= 1e-9 {
		top := Face{
			Name:    "top",
			Normal:  [3]float64{0, 0, 1},
			Visible: true,
		}
		for _, v := range base {
			tv := [2]float64{cx + (v[0]-cx)*topScale, cy + (v[1]-cy)*topScale}
			top.Points = append(top.Points, prismLocal(d, h, tv[0], tv[1], h))
		}
		faces = append(faces, top)
	}
	for i := range faces {
		faces[i].ZOrder = i
	}
	return faces
}

func (TaperedPrismShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	kind := "cone"
	if params != nil {
		if k, ok := params["kind"].(string); ok {
			kind = k
		}
	}
	n, topScale := taperedParams(kind)
	base := taperedBase(w, d, n)
	cx, cy := w/2, d/2
	pts := make([][2]float64, 0, 2*n+1)
	for _, v := range base {
		pts = append(pts, prismLocal(d, h, v[0], v[1], 0))
		tv := [2]float64{cx + (v[0]-cx)*topScale, cy + (v[1]-cy)*topScale}
		pts = append(pts, prismLocal(d, h, tv[0], tv[1], h))
	}
	if topScale < 1e-9 {
		pts = append(pts, prismLocal(d, h, cx, cy, h))
	}
	return convexHull(pts)
}

func (TaperedPrismShapeProvider) ContentAnchor() string { return "top" }

func (TaperedPrismShapeProvider) ContentRectFor(w, d, h float64, params map[string]any) ContentRect {
	kind := "cone"
	if params != nil {
		if k, ok := params["kind"].(string); ok {
			kind = k
		}
	}
	n, _ := taperedParams(kind)
	base := taperedBase(w, d, n)
	return largestInscribedRect(base, w, d)
}

func (TaperedPrismShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(TaperedPrismShapeProvider{})
}

// RenderIsoTaperedPrism renders a cone, pyramid, or frustum.
func RenderIsoTaperedPrism(o IsoBoxOpts, kind string) string {
	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)
	params := map[string]any{"kind": kind}
	var prov TaperedPrismShapeProvider

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "tapered-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "tapered-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "tapered-grain", o.GrainIntensity, o.GrainScale)
		if defs.Len() > 0 {
			fmt.Fprintf(&sb, `<defs>%s</defs>`, defs.String())
		}
	}
	if strings.TrimSpace(o.BackglowColor) != "" && o.BackglowRadius > 0 {
		var gdefs, halo strings.Builder
		sil := prov.Silhouette(o.Width, o.Depth, o.Height, params)
		hpts := make([][2]float64, len(sil))
		for k, q := range sil {
			hpts[k] = [2]float64{q[0] + o.Margin, q[1] + o.Margin}
		}
		emitBackglowHalo(&halo, &gdefs, "tapered-backglow", o.BackglowColor, o.BackglowRadius, o.BackglowOpacity, hpts)
		fmt.Fprintf(&sb, `<defs>%s</defs>%s`, gdefs.String(), halo.String())
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
