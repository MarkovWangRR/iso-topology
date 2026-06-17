package isotopo

// interact.go — server-side interaction model for Studio's direct-manipulation
// layer.
//
// Instead of having the browser reverse-engineer node geometry from projected
// SVG pixels (elementsFromPoint, getScreenCTM, polygon vertex math), the
// server emits a lightweight "interaction model" alongside every rendered SVG.
// The model carries each part's world-space AABB so the browser can do:
//
//   1. Reparent hit-test: which container does a dragged node's footprint
//      overlap most? (replaces the fragile pixel-based containerUnder)
//   2. Anchor display: where are the face-centre anchor points in world space?
//      (replaces the polygon-vertex scan that breaks on shapes without faces)
//
// The model is pure data derived from the resolved layout — no SVG rendering
// involved — so it adds negligible cost to the render round-trip.

// PartModel is the world-space geometry of one part after layout resolution.
// Coordinates are in world units (same space as offset.wx/wy/wz in the DSL).
type PartModel struct {
	ID        string        `json:"id"`
	Container bool          `json:"container"`
	X         float64       `json:"x"`
	Y         float64       `json:"y"`
	Z         float64       `json:"z,omitempty"`
	W         float64       `json:"w"`
	D         float64       `json:"d"`
	H         float64       `json:"h,omitempty"`
	Anchors   []AnchorPoint `json:"anchors,omitempty"`
}

// AnchorPoint is one named connection point on a part's surface.
// Name matches the anchor suffix used in connector from/to (e.g. "top",
// "left", "right", "bottom"). WX/WY are world coordinates.
type AnchorPoint struct {
	Name string  `json:"name"`
	WX   float64 `json:"wx"`
	WY   float64 `json:"wy"`
}

// BuildInteractionModel returns the world-space geometry for every part in the
// document's composite scene, after layout resolution. Returns nil when the
// document has no composite scene.
//
// The caller should embed the result in the render-response JSON under the
// key "model" so Studio can read it without a separate round-trip.
func BuildInteractionModel(doc *Document) []PartModel {
	if doc == nil {
		return nil
	}
	scene := doc.Scene()
	if scene == nil {
		return nil
	}
	// buildPlanModel runs applyLayout internally and returns the flat rect list
	// with absolute world coordinates — exactly what we need.
	rects, _, _ := buildPlanModel(scene, doc.Theme, doc.Canvas)
	if len(rects) == 0 {
		return nil
	}
	out := make([]PartModel, 0, len(rects))
	for _, r := range rects {
		if r.id == "" {
			continue
		}
		pm := PartModel{
			ID:        r.id,
			Container: r.container,
			X: r.x, Y: r.y, Z: r.z,
			W: r.w, D: r.d, H: r.h,
			Anchors: faceAnchors(r),
		}
		out = append(out, pm)
	}
	return out
}

// faceAnchors returns the four cardinal face-centre anchor points for a part.
// These are the same logical positions Studio's showHandles() was computing
// from SVG polygon vertices — now computed once in world space so the browser
// doesn't need to touch the DOM at all for anchor display.
//
// Convention (matches iso projection orientation):
//   top    — centre of the top (roof) face: (cx, y)        world
//   right  — centre of the right face:       (x+w, cy)
//   bottom — nadir (lowest visible point):   (cx, y+d)
//   left   — centre of the left face:         (x, cy)
func faceAnchors(r planRect) []AnchorPoint {
	cx := r.x + r.w/2
	cy := r.y + r.d/2
	return []AnchorPoint{
		{Name: "top",    WX: cx,      WY: r.y},
		{Name: "right",  WX: r.x + r.w, WY: cy},
		{Name: "bottom", WX: cx,      WY: r.y + r.d},
		{Name: "left",   WX: r.x,     WY: cy},
	}
}

// WorldDropTarget returns the id of the container that a dragged node (dragID)
// should be reparented into when dropped, based on world-space footprint
// overlap — or "" if no container qualifies (drop to scene root).
//
// Algorithm: among all containers in the model (excluding dragID and its
// subtree), find the one whose AABB overlaps the dragged node's AABB the
// most (by area). A container must overlap by at least minOverlapFrac of the
// dragged node's area to qualify, preventing accidental re-homes when a node
// merely grazes a container's edge.
//
// This replaces the pixel-based elementsFromPoint hit-test in Studio's
// containerUnder(), which failed whenever the container was smaller than the
// node or visually occluded.
func WorldDropTarget(model []PartModel, dragID string, minOverlapFrac float64) string {
	// Find the dragged node's AABB.
	var drag *PartModel
	for i := range model {
		if model[i].ID == dragID {
			drag = &model[i]
			break
		}
	}
	if drag == nil {
		return ""
	}
	dragArea := drag.W * drag.D
	if dragArea <= 0 {
		return ""
	}

	// Build a quick subtree set so we never drop a container into its own child.
	// (The server applyReparent also guards this, but we should not even try.)
	subtree := map[string]bool{dragID: true}
	// We don't have the tree structure in the flat model, so subtree detection
	// is done server-side. Here we just exclude the drag node itself.

	bestID := ""
	bestArea := 0.0
	for _, pm := range model {
		if !pm.Container || subtree[pm.ID] {
			continue
		}
		ov := overlapArea(drag.X, drag.Y, drag.W, drag.D, pm.X, pm.Y, pm.W, pm.D)
		if ov/dragArea >= minOverlapFrac && ov > bestArea {
			bestArea = ov
			bestID = pm.ID
		}
	}
	return bestID
}

// overlapArea returns the area of intersection of two axis-aligned rectangles.
// x,y is the min corner; w,d are the extents (all in world units).
func overlapArea(ax, ay, aw, ad, bx, by, bw, bd float64) float64 {
	ox := min2(ax+aw, bx+bw) - max2(ax, bx)
	oy := min2(ay+ad, by+bd) - max2(ay, by)
	if ox <= 0 || oy <= 0 {
		return 0
	}
	return ox * oy
}

func min2(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max2(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
