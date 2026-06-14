package isotopo

import (
	"math"
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// anchorNames is the connector/annotation anchor vocabulary the resolver
// understands (see (*anchorResolver).world / .parse). Every side has a bare
// and a "-mid" spelling; "center" is an alias for the top-face centre. This is
// the single source for both validation (reject typos) and the capability
// contract (let the agent discover the set instead of guessing).
var anchorNames = []string{
	"top", "top-mid", "center",
	"bottom", "bottom-mid",
	"left", "left-mid", "right", "right-mid",
	"back", "back-mid", "front", "front-mid",
}

// anchorResolver resolves a connector endpoint reference ("partID" or
// "partID.anchor") to world / screen coordinates, exit normals and fan-out
// keys. It was lifted verbatim out of injectCompositeConnectors — a family of
// read-only closures over the parts index — so the anchor subsystem is a named,
// independently testable unit instead of ~200 lines inline in an 800-line
// function. All methods are pure reads of byID / tx / ty.
type anchorResolver struct {
	byID   map[string]partInfo
	tx, ty float64 // parts→screen origin translation (for screen())
}

func newAnchorResolver(infos []partInfo, tx, ty float64) *anchorResolver {
	byID := make(map[string]partInfo, len(infos))
	for _, p := range infos {
		if p.id != "" {
			byID[p.id] = p
		}
	}
	return &anchorResolver{byID: byID, tx: tx, ty: ty}
}

// parse splits "partID.anchor" into (id, anchor). Bare "partID" defaults to
// "top-mid".
func (ar *anchorResolver) parse(ref string) (id, anchor string) {
	dot := strings.Index(ref, ".")
	if dot < 0 {
		return ref, "top-mid"
	}
	return ref[:dot], ref[dot+1:]
}

// world returns the iso-world anchor coords for ref = "partID" or
// "partID.anchor". Anchors default to the top-face centre.
func (ar *anchorResolver) world(ref string) (wx, wy, wz float64, ok bool) {
	id, anchor := ar.parse(ref)
	p, found := ar.byID[id]
	if !found {
		return 0, 0, 0, false
	}
	wx, wy, wz = p.offWX+p.w/2, p.offWY+p.d/2, p.offWZ+p.h
	switch anchor {
	case "left-mid", "left":
		wx, wy = p.offWX, p.offWY+p.d/2
	case "right-mid", "right":
		wx, wy = p.offWX+p.w, p.offWY+p.d/2
	case "back-mid", "back":
		wx, wy = p.offWX+p.w/2, p.offWY
	case "front-mid", "front":
		wx, wy = p.offWX+p.w/2, p.offWY+p.d
	case "top-mid", "top", "center":
		// keep defaults
	case "bottom-mid", "bottom":
		wz = p.offWZ
	}
	return wx, wy, wz, true
}

// exit returns the unit outward-normal of an anchor in the iso world's (x, y)
// ground plane. top/bottom/center have no horizontal normal — caller falls
// back to the x-axis.
func (ar *anchorResolver) exit(ref string) (dx, dy float64) {
	_, anchor := ar.parse(ref)
	switch anchor {
	case "left-mid", "left":
		return -1, 0
	case "right-mid", "right":
		return 1, 0
	case "back-mid", "back":
		return 0, -1
	case "front-mid", "front":
		return 0, 1
	}
	return 1, 0
}

// faceMidZ returns the vertical middle (in world z) of the referenced part's
// side face. Used by the orthogonal router to pick a routing height that lies
// inside BOTH endpoints' side faces, so every segment of the path lies on a
// single horizontal world plane and projects to pure ±tan30° iso-axis slopes —
// i.e. it aligns with the TopoDSL grid lattice with zero off-axis tilt.
func (ar *anchorResolver) faceMidZ(ref string) float64 {
	id, _ := ar.parse(ref)
	p, found := ar.byID[id]
	if !found {
		return 0
	}
	return p.offWZ + p.h/2
}

// refineSilhouette adjusts a bbox-based side anchor (wx, wy) onto the actual
// visible silhouette of non-prismatic shapes:
//
//	circle / sphere: the silhouette at a given z is a disc of radius
//	    sqrt(r² − (z − cz)²) centred at the sphere centroid. Anchors
//	    slide inward when z is off the equator.
//	cloud:           the rendered outline insets from the bbox by
//	    leftX=0.04·w / rightX=0.96·w (matches sampleCloudOutline);
//	    back/front sit on the trunk's top/bottom edges.
//
// Other shapes are pass-through (bbox already matches silhouette).
func (ar *anchorResolver) refineSilhouette(ref string, wx, wy, z float64) (float64, float64) {
	id, anchor := ar.parse(ref)
	p, found := ar.byID[id]
	if !found {
		return wx, wy
	}
	// v3.2.1 — prism family: bbox face-midpoints sit OUTSIDE the inscribed
	// polygon (hexagon's left-mid is in empty canvas) or on a vertex, piling
	// arrowheads and detaching exits. Re-anchor on the base polygon along the
	// bbox-anchor direction.
	if sides := prismSidesFor(p.shape, p.sides); sides >= 3 {
		dx, dy := wx-(p.offWX+p.w/2), wy-(p.offWY+p.d/2)
		ax, ay := iso25d.PrismGroundAnchor(p.w, p.d, sides, dx, dy)
		return p.offWX + ax, p.offWY + ay
	}
	switch p.shape {
	case "circle":
		cx := p.offWX + p.w/2
		cy := p.offWY + p.d/2
		cz := p.offWZ + p.h/2
		r := math.Min(math.Min(p.w, p.d), p.h) / 2
		dz := z - cz
		if math.Abs(dz) >= r {
			return wx, wy
		}
		rXY := math.Sqrt(r*r - dz*dz)
		switch anchor {
		case "left", "left-mid":
			return cx - rXY, cy
		case "right", "right-mid":
			return cx + rXY, cy
		case "back", "back-mid":
			return cx, cy - rXY
		case "front", "front-mid":
			return cx, cy + rXY
		}
	case "cloud":
		leftX := 0.04 * p.w
		rightX := 0.96 * p.w
		horizonY := 0.10 * p.d // top of bumps row
		bottomY := 0.85 * p.d  // bottom of trunk
		switch anchor {
		case "left", "left-mid":
			return p.offWX + leftX, p.offWY + p.d/2
		case "right", "right-mid":
			return p.offWX + rightX, p.offWY + p.d/2
		case "back", "back-mid":
			return p.offWX + p.w/2, p.offWY + horizonY
		case "front", "front-mid":
			return p.offWX + p.w/2, p.offWY + bottomY
		}
	}
	return wx, wy
}

func (ar *anchorResolver) screen(ref string) (float64, float64, bool) {
	wx, wy, wz, ok := ar.world(ref)
	if !ok {
		return 0, 0, false
	}
	x, y := projectIso(wx, wy, wz)
	return x + ar.tx, y + ar.ty, true
}

// sideKey normalises "id.left-mid" and "id.left" to one key so multiple
// connectors touching the same side cluster together in the fan-out accounting.
func (ar *anchorResolver) sideKey(ref string) string {
	id, anchor := ar.parse(ref)
	switch anchor {
	case "left", "left-mid":
		anchor = "left"
	case "right", "right-mid":
		anchor = "right"
	case "back", "back-mid":
		anchor = "back"
	case "front", "front-mid":
		anchor = "front"
	case "top", "top-mid", "center":
		anchor = "top"
	case "bottom", "bottom-mid":
		anchor = "bottom"
	}
	return id + "/" + anchor
}

// auto resolves a bare ref (no ".anchor") to the side face FACING the other
// endpoint. v3.0 — bare refs used to pin to top-mid with a hard-coded +x exit
// normal: a target sitting to the LEFT still made the route walk +x for the
// 24-unit stub and double back, and every spoke into a hub converged on one
// point.
func (ar *anchorResolver) auto(ref, otherRef string) string {
	if strings.Contains(ref, ".") {
		return ref
	}
	p, ok := ar.byID[ref]
	if !ok {
		return ref
	}
	oid, _ := ar.parse(otherRef)
	o, ok2 := ar.byID[oid]
	if !ok2 {
		return ref
	}
	dx := (o.offWX + o.w/2) - (p.offWX + p.w/2)
	dy := (o.offWY + o.d/2) - (p.offWY + p.d/2)
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return ref + ".right"
		}
		return ref + ".left"
	}
	if dy >= 0 {
		return ref + ".front"
	}
	return ref + ".back"
}
