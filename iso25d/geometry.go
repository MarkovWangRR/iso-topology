// M1 of the flexible-geometry plan: the geometry layer's vocabulary.
//
// A shape's job narrows to "given w/d/h, produce named faces" — pure
// math, no color. Everything downstream (surfaces, effects, content,
// connector anchors, layout footprints) consumes this description
// instead of poking at renderer internals.
//
// During the migration the providers run IN PARALLEL with the legacy
// string renderers: Convert2DTo25D still emits bytes the old way, and
// parity tests assert provider geometry matches the emitted polygons
// point-for-point. The renderer swap happens per-shape once a
// provider's geometry is proven.
package iso25d

import (
	"math"
	"sort"
)

// Face is one projected polygon of a shape under the iso camera.
type Face struct {
	Name   string       // "top" | "left" | "right" | custom
	Points [][2]float64 // screen coords, clockwise, local frame (pre-margin)
	Normal [3]float64   // world-space outward normal
	ZOrder int          // painter order, lower paints first
	// Visible is the backface-culling verdict under the fixed camera.
	Visible bool
}

// ContentRect is the largest axis-aligned rectangle (in the face's
// LOCAL unprojected coordinates) guaranteed to lie inside the face —
// where icons and labels may be placed.
type ContentRect struct {
	X, Y, W, H float64
}

// ShapeProvider is the registration unit for one shape family.
type ShapeProvider interface {
	// Names returns every shape name (and alias) this provider serves.
	Names() []string
	// Faces computes the projected faces for the given world dims.
	// params carries shape-specific knobs (e.g. sides for prisms).
	Faces(w, d, h float64, params map[string]any) []Face
	// Silhouette returns the outer screen outline (local frame, same
	// origin as Faces points) — the obstacle/clip polygon consumed by
	// connector endpoint clipping and arrowhead placement.
	Silhouette(w, d, h float64, params map[string]any) [][2]float64
	// ContentAnchor names the face that carries icon + label.
	ContentAnchor() string
	// ContentRectFor returns the inscribed content rectangle of the
	// content-anchor face in that face's local (unprojected) units.
	ContentRectFor(w, d, h float64, params map[string]any) ContentRect
	// Footprint is the ground-plane bbox used by the layout solver.
	Footprint(w, d, h float64) (fw, fd float64)
}

var shapeRegistry = map[string]ShapeProvider{}

// RegisterShape adds a provider under all its names. Later
// registrations win — external packages may override built-ins.
func RegisterShape(p ShapeProvider) {
	for _, name := range p.Names() {
		shapeRegistry[name] = p
	}
}

// LookupShape returns the provider for a shape name, or nil.
func LookupShape(name string) ShapeProvider { return shapeRegistry[name] }

// RegisteredShapeNames returns the sorted name list (for capabilities).
func RegisteredShapeNames() []string {
	out := make([]string, 0, len(shapeRegistry))
	for k := range shapeRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// BoxShapeProvider — the reference implementation. Its geometry must
// match RenderIsoBox's emitted polygons point-for-point (parity test).
// ---------------------------------------------------------------------------

type BoxShapeProvider struct{}

func (BoxShapeProvider) Names() []string {
	return []string{"rectangle", "box", "square"}
}

// Faces mirrors computeBoxGeom's projection exactly: local origin at
// (d·cos30, h) — the projected world-origin corner — so points line up
// with the legacy renderer after its -minX/-minY+margin shift.
func (BoxShapeProvider) Faces(w, d, h float64, _ map[string]any) []Face {
	p := func(x, y, z float64) [2]float64 {
		return [2]float64{(x-y)*cos30 + d*cos30, (x+y)*sin30 - z + h}
	}
	top := [][2]float64{p(0, 0, h), p(w, 0, h), p(w, d, h), p(0, d, h)}
	left := [][2]float64{p(0, d, h), p(w, d, h), p(w, d, 0), p(0, d, 0)}
	right := [][2]float64{p(w, 0, h), p(w, d, h), p(w, d, 0), p(w, 0, 0)}
	return []Face{
		{Name: "left", Points: left, Normal: [3]float64{0, 1, 0}, ZOrder: 0, Visible: true},
		{Name: "right", Points: right, Normal: [3]float64{1, 0, 0}, ZOrder: 1, Visible: true},
		{Name: "top", Points: top, Normal: [3]float64{0, 0, 1}, ZOrder: 2, Visible: true},
	}
}

// Silhouette is the classic 6-corner hexagon of an iso box.
func (BoxShapeProvider) Silhouette(w, d, h float64, _ map[string]any) [][2]float64 {
	p := func(x, y, z float64) [2]float64 {
		return [2]float64{(x-y)*cos30 + d*cos30, (x+y)*sin30 - z + h}
	}
	return [][2]float64{
		p(0, 0, h), p(w, 0, h), p(w, 0, 0),
		p(w, d, 0), p(0, d, 0), p(0, d, h),
	}
}

func (BoxShapeProvider) ContentAnchor() string { return "top" }

func (BoxShapeProvider) ContentRectFor(w, d, h float64, _ map[string]any) ContentRect {
	// The whole top face is usable; fitTopContent applies its own pad.
	return ContentRect{X: 0, Y: 0, W: w, H: d}
}

func (BoxShapeProvider) Footprint(w, d, _ float64) (float64, float64) { return w, d }

func init() {
	RegisterShape(BoxShapeProvider{})
}

// isoSlopeOK reports whether a screen edge runs along an iso ground
// axis (slope ±tan30°) or is vertical — the only legal directions for
// prism-family side-wall edges. Exported for geomtest.
func isoSlopeOK(dx, dy float64) bool {
	if math.Abs(dx) < 1e-6 {
		return true // vertical edge
	}
	slope := math.Abs(dy / dx)
	return math.Abs(slope-sin30/cos30) < 1e-6
}
