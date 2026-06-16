// Category D of the flexible-geometry plan: tapered prisms whose top face
// is a uniformly-scaled copy of the base (topScale=0 → apex, 0<topScale<1
// → frustum/truncated cone, topScale=1 → regular prism).
//
// Built-in aliases registered here:
//   cone     – n=32, topScale=0 (smooth circular cone)
//   pyramid  – n=4,  topScale=0 (square pyramid)
//   frustum  – n=32, topScale=0.5 (truncated cone, like an S3 bucket)
//   wedge    – n=4,  topScale=0 (right-triangle prism, tilted via params)
package iso25d

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// TaperedPrismShapeProvider implements ShapeProvider for tapered prisms.
type TaperedPrismShapeProvider struct{}

func (TaperedPrismShapeProvider) Names() []string {
	return []string{"cone", "pyramid", "frustum"}
}

// taperedResolveParams extracts sides and topScale from the params map.
// The caller sets these based on the shape name and geom.sides / geom.topScale.
func taperedResolveParams(params map[string]any) (sides int, topScale float64) {
	sides = 32
	topScale = 0.0
	if params != nil {
		if v, ok := params["sides"].(int); ok && v >= 3 {
			sides = v
		}
		if v, ok := params["topScale"].(float64); ok {
			topScale = v
		}
	}
	return
}

// taperedBase returns (bottomRing, topRing) in world XY.
// The bottom ring is the same regular n-gon as PrismShapeProvider.
// The top ring is uniformly scaled toward (w/2, d/2) by topScale.
func taperedBase(w, d float64, n int, topScale float64) (bottom, top [][2]float64) {
	bottom = prismBase(w, d, n)
	cx, cy := w/2, d/2
	top = make([][2]float64, n)
	for i, b := range bottom {
		top[i] = [2]float64{
			cx + topScale*(b[0]-cx),
			cy + topScale*(b[1]-cy),
		}
	}
	return
}

// taperedSideFaceNormal computes the outward world-space XY normal for the
// i-th side face and determines iso-camera visibility (camera at +X,+Y,+Z).
func taperedSideFaceNormal(base [][2]float64, cx, cy float64, i int) (nx, ny float64, visible bool) {
	n := len(base)
	a, b := base[i], base[(i+1)%n]
	// Perpendicular to edge, oriented away from centroid (same as PrismShapeProvider).
	ex, ey := b[0]-a[0], b[1]-a[1]
	nx, ny = ey, -ex
	mx := (a[0]+b[0])/2 - cx
	my := (a[1]+b[1])/2 - cy
	if nx*mx+ny*my < 0 {
		nx, ny = -nx, -ny
	}
	l := math.Hypot(nx, ny)
	if l == 0 {
		return 0, 0, false
	}
	nx, ny = nx/l, ny/l
	visible = nx+ny > 1e-9
	return
}

func (TaperedPrismShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	sides, topScale := taperedResolveParams(params)
	bottom, top := taperedBase(w, d, sides, topScale)
	cx, cy := w/2, d/2
	hasApex := topScale < 1e-6

	var faces []Face

	// Side faces.
	for i := 0; i < sides; i++ {
		j := (i + 1) % sides
		nx, ny, visible := taperedSideFaceNormal(bottom, cx, cy, i)

		var pts [][2]float64
		if hasApex {
			pts = [][2]float64{
				prismLocal(d, h, bottom[i][0], bottom[i][1], 0),
				prismLocal(d, h, bottom[j][0], bottom[j][1], 0),
				prismLocal(d, h, cx, cy, h),
			}
		} else {
			pts = [][2]float64{
				prismLocal(d, h, bottom[i][0], bottom[i][1], 0),
				prismLocal(d, h, bottom[j][0], bottom[j][1], 0),
				prismLocal(d, h, top[j][0], top[j][1], h),
				prismLocal(d, h, top[i][0], top[i][1], h),
			}
		}
		faces = append(faces, Face{
			Name:    fmt.Sprintf("side%d", i),
			Points:  pts,
			Normal:  [3]float64{nx, ny, 0},
			Visible: visible,
		})
	}

	// Painter order: farther faces first (same heuristic as PrismShapeProvider).
	sort.SliceStable(faces, func(i, j int) bool {
		if faces[i].Visible != faces[j].Visible {
			return !faces[i].Visible
		}
		// Use projected centroid screen-x+y as depth proxy.
		pi := faces[i].Points
		pj := faces[j].Points
		di := pi[0][0] + pi[0][1]
		dj := pj[0][0] + pj[0][1]
		return di < dj
	})

	// Top face (only when not a pure apex).
	if !hasApex {
		topPts := make([][2]float64, sides)
		for i := 0; i < sides; i++ {
			topPts[i] = prismLocal(d, h, top[i][0], top[i][1], h)
		}
		faces = append(faces, Face{
			Name:    "top",
			Points:  topPts,
			Normal:  [3]float64{0, 0, 1},
			Visible: true,
		})
	}

	for i := range faces {
		faces[i].ZOrder = i
	}
	return faces
}

func (TaperedPrismShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	sides, topScale := taperedResolveParams(params)
	bottom, top := taperedBase(w, d, sides, topScale)

	pts := make([][2]float64, 0, sides*2+1)
	for _, v := range bottom {
		pts = append(pts, prismLocal(d, h, v[0], v[1], 0))
	}
	if topScale < 1e-6 {
		pts = append(pts, prismLocal(d, h, w/2, d/2, h))
	} else {
		for _, v := range top {
			pts = append(pts, prismLocal(d, h, v[0], v[1], h))
		}
	}
	return convexHull(pts)
}

func (TaperedPrismShapeProvider) ContentAnchor() string { return "top" }

func (TaperedPrismShapeProvider) ContentRectFor(w, d, h float64, params map[string]any) ContentRect {
	sides, topScale := taperedResolveParams(params)
	if topScale < 1e-6 {
		// Cone/pyramid: content rect is centred in the base footprint.
		inset := 0.35
		return ContentRect{X: w * inset, Y: d * inset, W: w * (1 - 2*inset), H: d * (1 - 2*inset)}
	}
	_, top := taperedBase(w, d, sides, topScale)
	return largestInscribedRect(top, w, d)
}

func (TaperedPrismShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(TaperedPrismShapeProvider{})
}

// RenderIsoTaperedPrism draws a tapered n-gon prism with the full box-family
// surface vocabulary (palette, gradients, FaceSurface overrides, stroke, blur,
// backglow, grain, outline). topScale=0 gives a cone or pyramid; 0<topScale<1
// gives a frustum; topScale=1 degenerates to RenderIsoPrism.
func RenderIsoTaperedPrism(o IsoBoxOpts, sides int, topScale float64) string {
	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)
	params := map[string]any{"sides": sides, "topScale": topScale}
	var prov TaperedPrismShapeProvider

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "tap-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "tap-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "tap-grain", o.GrainIntensity, o.GrainScale)
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
		emitBackglowHalo(&halo, &gdefs, "tap-backglow", o.BackglowColor, o.BackglowRadius, o.BackglowOpacity, hpts)
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
	var faceDefs strings.Builder
	type pend struct {
		name string
		pts  [][2]float64
	}
	var stroked []pend

	for _, f := range prov.Faces(o.Width, o.Depth, o.Height, params) {
		if !f.Visible {
			continue
		}
		var fill string
		if f.Name == "top" {
			fill = o.TopFill
		} else {
			if f.Normal[0] >= f.Normal[1] {
				fill = o.RightFill
			} else {
				fill = o.LeftFill
			}
		}
		if fill == "" {
			fill = o.LeftFill
		}
		if fs := surfaceFor(o.FaceSurfaces, f.Name); fs != nil && fs.Fill != nil {
			if ref := emitFaceFill(&faceDefs, "", f.Name, fs.Fill); ref != "" {
				fill = ref
			}
		}
		pts := make([][2]float64, len(f.Points))
		for i, p := range f.Points {
			pts[i] = [2]float64{p[0] + m, p[1] + m}
		}
		writeFace(&sb, f.Name, fill, stroke, sw, "", 0, pts...)
		if fs := surfaceFor(o.FaceSurfaces, f.Name); fs != nil && len(fs.Strokes) > 0 {
			stroked = append(stroked, pend{f.Name, pts})
		}
	}
	if faceDefs.Len() > 0 {
		fmt.Fprintf(&sb, `<defs>%s</defs>`, faceDefs.String())
	}
	for _, sd := range stroked {
		fs := surfaceFor(o.FaceSurfaces, sd.name)
		writeFaceStrokeLayers(&sb, sd.name, fs.Strokes, sd.pts...)
	}

	if grainID != "" {
		sb.WriteString(`</g>`)
	}

	// Label / icon placement:
	// • cone/pyramid (topScale=0): content rect centred in the base footprint,
	//   projected from z=0 so text appears at the bottom of the shape.
	// • frustum: content rect on the (smaller) top face at z=h.
	cr := prov.ContentRectFor(o.Width, o.Depth, o.Height, params)
	var ox, oy float64
	if topScale < 1e-6 {
		// Project the centre of the base content rect from z=0.
		ox, oy = project(cr.X+cr.W/2, cr.Y+cr.H/2, 0)
		// Shift back from centre to top-left for writeTopLabelAndIconV12.
		ox -= cr.W / 2 * cos30
		oy -= cr.H / 2 * sin30
	} else {
		ox, oy = project(cr.X, cr.Y, o.Height)
	}
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
