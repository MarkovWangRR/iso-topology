package isotopo

import "testing"

// The renderer clamps a cloud up to a 200×140×24 floor (iso25d.ShapeMinDims).
// The layout solver must reserve that SAME space, or an auto-sized group wraps
// the authored size while the renderer paints the larger one and the cloud
// overflows its slot. These tests pin the root fix.

func TestCloudFootprintReservesRenderedFloor(t *testing.T) {
	cloud := &CompositePart{ID: "c", Shape: "cloud", Geom: &Geom{W: 124, D: 86, H: 46}}
	w, d := partFootprint(cloud)
	if w < 200 || d < 140 {
		t.Fatalf("cloud footprint must reserve the rendered floor (>=200x140), got %gx%g", w, d)
	}
	// A non-clamped shape is untouched.
	rect := &CompositePart{ID: "r", Shape: "rectangle", Geom: &Geom{W: 124, D: 86, H: 24}}
	if rw, rd := partFootprint(rect); rw != 124 || rd != 86 {
		t.Fatalf("non-clamped shape footprint must equal authored geom, got %gx%g", rw, rd)
	}
}

func TestCloudInAutoGroupGrowsToWrapIt(t *testing.T) {
	src := `
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        shape: group
        layout: { mode: column, gap: 0.8 }
        parts:
          - { id: cl, shape: cloud,     geom: { w: 124, d: 86, h: 46 } }
          - { id: r,  shape: rectangle, geom: { w: 90,  d: 70, h: 24 } }
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	scene := doc.Nodes["scene"]
	applyLayout(scene, doc.Canvas)

	var g *CompositePart
	for _, p := range scene.Parts {
		if p.ID == "g" {
			g = p
		}
	}
	if g == nil || g.Geom == nil {
		t.Fatalf("group g not resolved: %+v", g)
	}
	// The auto-sized slab must be wide enough to contain the cloud's rendered
	// 200 width (plus padding), not just the authored 124.
	if g.Geom.W < 200 {
		t.Fatalf("auto-group must grow to wrap the cloud's rendered 200 width, got W=%g", g.Geom.W)
	}
}
