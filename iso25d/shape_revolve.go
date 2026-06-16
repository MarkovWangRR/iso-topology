// Category C of the flexible-geometry plan: revolution bodies whose profile
// is a mother-line function r(z) revolved around the z-axis.
//
// The iso projection of a circle at height z is an ellipse with semi-axes
// (r·cos30, r·sin30) centred on the shape's world axis. We sample the
// ellipse and keep only the visible arc (the front half) to avoid painting
// hidden geometry on top of visible faces.
//
// Built-in aliases registered here:
//   dome     – quarter-circle profile (half-sphere cap)
//   torus    – circle profile centred away from z-axis
//   capsule  – cylinder with hemispherical caps
//
// Note: a smooth circular cone is already handled by TaperedPrismShapeProvider
// (n=32, topScale=0). RevolveShapeProvider provides the mathematically exact
// smooth version and adds dome / torus / capsule which cannot be expressed as
// simple tapered prisms.
package iso25d

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// RevolveProfile describes the mother-line of a revolution body.
// R(z) returns the world-space radius at height z ∈ [0, TotalH].
type RevolveProfile struct {
	TotalH  float64
	R       func(z float64) float64
	Slices  int // number of horizontal cross-sections to sample
}

// revolveProfiles maps shape names to profile constructors.
var revolveProfiles = map[string]func(w, d, h float64) RevolveProfile{
	"dome": func(w, d, h float64) RevolveProfile {
		r0 := math.Min(w, d) / 2
		return RevolveProfile{
			TotalH: h,
			R: func(z float64) float64 {
				// Quarter-circle profile: r(z) = r0·cos(z/h · π/2)
				t := (z / h) * (math.Pi / 2)
				return r0 * math.Cos(t)
			},
			Slices: 20,
		}
	},
	"torus": func(w, d, h float64) RevolveProfile {
		// Torus: tube radius = min(w,d)/4, ring radius = min(w,d)/4.
		// z spans the tube cross-section (full circle in z).
		rRing := math.Min(w, d) / 4
		rTube := math.Min(w, d) / 4
		return RevolveProfile{
			TotalH: h,
			R: func(z float64) float64 {
				// z is normalised to [-1, 1] across h.
				t := (z/h)*2 - 1
				return rRing + rTube*math.Cos(t*math.Pi)
			},
			Slices: 28,
		}
	},
	"capsule": func(w, d, h float64) RevolveProfile {
		r0 := math.Min(w, d) / 2
		capH := r0 * 0.6 // cap height
		return RevolveProfile{
			TotalH: h,
			R: func(z float64) float64 {
				if z <= capH {
					// Bottom cap: quarter-circle rising from apex to full radius.
					t := z / capH
					return r0 * math.Sin(t*math.Pi/2)
				}
				if z >= h-capH {
					// Top cap: quarter-circle falling from full radius to apex.
					t := (h - z) / capH
					return r0 * math.Sin(t*math.Pi/2)
				}
				return r0
			},
			Slices: 24,
		}
	},
}

// RevolveShapeProvider implements ShapeProvider for smooth revolution bodies.
type RevolveShapeProvider struct{}

func (RevolveShapeProvider) Names() []string {
	return []string{"dome", "torus", "capsule"}
}

// revolveSlices builds a set of horizontal elliptic cross-section rings.
// Each ring is a (z, r) pair where r is the world-space radius at that z.
func revolveSlices(prof RevolveProfile) [][2]float64 {
	n := prof.Slices
	if n < 4 {
		n = 16
	}
	rings := make([][2]float64, n+1)
	for i := 0; i <= n; i++ {
		z := prof.TotalH * float64(i) / float64(n)
		rings[i] = [2]float64{z, prof.R(z)}
	}
	return rings
}

// revolveProject maps a world-space point on the revolution body surface
// into the local iso frame (same origin convention as BoxShapeProvider).
// cx, cy = world-space centre; rx, ry = ellipse semi-axes at this height.
func revolveProject(d, h, cx, cy, rx, ry, ang, z float64) [2]float64 {
	wx := cx + rx*math.Cos(ang)
	wy := cy + ry*math.Sin(ang)
	return prismLocal(d, h, wx, wy, z)
}

func (RevolveShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
	name := ""
	if n, ok := params["name"].(string); ok {
		name = n
	}
	builder, ok := revolveProfiles[name]
	if !ok {
		builder = revolveProfiles["dome"]
	}
	prof := builder(w, d, h)
	rings := revolveSlices(prof)
	cx, cy := w/2, d/2

	// For each adjacent pair of rings, build a "band" face.
	// The front arc (visible from iso camera) sweeps roughly π radians.
	// We approximate the arc as a polygon strip.
	const angStep = math.Pi / 16 // 32 points per full circle; visible half = 16 pts

	var faces []Face
	for i := 0; i+1 < len(rings); i++ {
		z0, r0 := rings[i][0], rings[i][1]
		z1, r1 := rings[i+1][0], rings[i+1][1]
		if r0 < 1e-6 && r1 < 1e-6 {
			continue
		}
		// Build the visible (front) arc band polygon.
		// In iso, the front of a circle is the arc where screen-dy/da < 0
		// i.e. the arc pointing toward the viewer (+x,+y,+z direction).
		// That corresponds to world angles roughly [π/4, π+π/4] = [45°, 225°].
		startAng := math.Pi/4 + math.Pi // 225°
		// We sweep from startAng forward by π radians to cover the front arc.
		// rx = r*cos30 (iso x scale), ry = r*sin30 (iso y scale)
		rx0, ry0 := r0*cos30, r0*sin30
		rx1, ry1 := r1*cos30, r1*sin30

		var pts [][2]float64
		// Bottom ring forward arc (left→right in screen).
		ang := startAng
		for ang <= startAng+math.Pi+angStep/2 {
			pts = append(pts, revolveProject(d, h, cx, cy, rx0, ry0, ang, z0))
			ang += angStep
		}
		// Top ring (right→left).
		ang = startAng + math.Pi
		for ang >= startAng-angStep/2 {
			pts = append(pts, revolveProject(d, h, cx, cy, rx1, ry1, ang, z1))
			ang -= angStep
		}

		if len(pts) < 3 {
			continue
		}

		// Depth proxy: screen x+y of centroid.
		sumD := 0.0
		for _, p := range pts {
			sumD += p[0] + p[1]
		}
		bandName := fmt.Sprintf("band%d", i)
		faces = append(faces, Face{
			Name:    bandName,
			Points:  pts,
			Normal:  [3]float64{0.707, 0.707, 0},
			Visible: true,
			ZOrder:  int(sumD / float64(len(pts))),
		})
	}

	// Top cap (disc if top radius > 0).
	if topR := rings[len(rings)-1][1]; topR > 1e-3 {
		rx, ry := topR*cos30, topR*sin30
		var topPts [][2]float64
		for k := 0; k <= 32; k++ {
			ang := 2 * math.Pi * float64(k) / 32
			topPts = append(topPts, revolveProject(d, h, cx, cy, rx, ry, ang, h))
		}
		faces = append(faces, Face{
			Name:    "top",
			Points:  topPts,
			Normal:  [3]float64{0, 0, 1},
			Visible: true,
			ZOrder:  1<<20,
		})
	}

	sort.Slice(faces, func(i, j int) bool {
		return faces[i].ZOrder < faces[j].ZOrder
	})
	return faces
}

func (RevolveShapeProvider) Silhouette(w, d, h float64, params map[string]any) [][2]float64 {
	name := ""
	if n, ok := params["name"].(string); ok {
		name = n
	}
	builder, ok := revolveProfiles[name]
	if !ok {
		builder = revolveProfiles["dome"]
	}
	prof := builder(w, d, h)
	rings := revolveSlices(prof)
	cx, cy := w/2, d/2

	var pts [][2]float64
	for _, ring := range rings {
		z, r := ring[0], ring[1]
		rx, ry := r*cos30, r*sin30
		// Add 4 cardinal points per ring (enough for convex hull).
		for _, ang := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
			pts = append(pts, revolveProject(d, h, cx, cy, rx, ry, ang, z))
		}
	}
	return convexHull(pts)
}

func (RevolveShapeProvider) ContentAnchor() string { return "top" }

func (RevolveShapeProvider) ContentRectFor(w, d, h float64, params map[string]any) ContentRect {
	name := ""
	if n, ok := params["name"].(string); ok {
		name = n
	}
	builder, ok := revolveProfiles[name]
	if !ok {
		builder = revolveProfiles["dome"]
	}
	prof := builder(w, d, h)
	topR := prof.R(h)
	inset := 0.4
	if topR < 1e-3 {
		// Apex: use base footprint with heavy inset.
		inset = 0.35
		return ContentRect{X: w * inset, Y: d * inset, W: w * (1 - 2*inset), H: d * (1 - 2*inset)}
	}
	rW := topR * 2
	rD := topR * 2
	return ContentRect{X: (w - rW) / 2 * inset, Y: (d - rD) / 2 * inset, W: rW * (1 - inset), H: rD * (1 - inset)}
}

func (RevolveShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(RevolveShapeProvider{})
}

// RenderIsoRevolve renders a revolution body shape.
// params must include "name" → one of "dome", "torus", "capsule".
func RenderIsoRevolve(o IsoBoxOpts, shapeName string) string {
	g := computeBoxGeom(o.Width, o.Depth, o.Height, o.Margin)
	params := map[string]any{"name": shapeName}
	var prov RevolveShapeProvider

	var sb strings.Builder
	sb.WriteString(svgHeader(g.ViewW, g.ViewH))
	blurOn := emitBlurOpen(&sb, "rev-blur", o.Blur)

	shadowID, grainID := "", ""
	{
		var defs strings.Builder
		if strings.TrimSpace(o.ShadowColor) != "" && (o.ShadowDx != 0 || o.ShadowDy != 0 || o.ShadowBlur > 0) {
			shadowID = "rev-shadow"
			emitDropShadowFilter(&defs, shadowID, o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor)
		}
		grainID = emitGrainFilter(&defs, "rev-grain", o.GrainIntensity, o.GrainScale)
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
		emitBackglowHalo(&halo, &gdefs, "rev-backglow", o.BackglowColor, o.BackglowRadius, o.BackglowOpacity, hpts)
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
		fill := o.LeftFill
		if f.Name == "top" {
			fill = o.TopFill
		}
		if fill == "" {
			fill = o.LeftFill
		}
		if fs := surfaceFor(o.FaceSurfaces, f.Name); fs != nil && fs.Fill != nil {
			var defs strings.Builder
			if ref := emitFaceFill(&defs, "", f.Name, fs.Fill); ref != "" {
				fill = ref
				if defs.Len() > 0 {
					fmt.Fprintf(&sb, `<defs>%s</defs>`, defs.String())
				}
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
