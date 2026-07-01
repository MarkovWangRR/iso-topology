package isotopo

import (
	"strings"
	"testing"
)

// TestProjectionTopRendersPlan is the issue #9 guard: the 2D top-down view must
// be a real, reachable alternative renderer, not iso. The same scene rendered
// with canvas.projection "top" must produce the flat plan output (orthogonal
// plan arrows, footprint rects) and must NOT emit isometric side faces; the
// default iso render of the same scene must emit those faces. This keeps the
// promoted "documentation mode" working as a distinct projection.
func TestProjectionTopRendersPlan(t *testing.T) {
	data, ext, err := readInput("samples/topology/plan-view-2d")
	if err != nil {
		t.Fatalf("read 2D sample: %v", err)
	}
	doc, err := loadGoldenDoc(ext, data)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	scene := doc.Scene()
	if scene == nil {
		t.Fatal("sample has no scene")
	}

	// Flat plan view (as declared by the sample's canvas.projection: top).
	plan := RenderWithCanvas(scene, doc.Theme, doc.Canvas, doc.Annotations)
	if !strings.Contains(plan, "planarrow") {
		t.Error("projection:top output is missing the plan-view orthogonal arrow marker")
	}
	if strings.Contains(plan, `data-face="left"`) || strings.Contains(plan, `data-face="right"`) {
		t.Error("projection:top output leaked isometric side faces — it should be flat")
	}

	// Same scene, isometric: must have the 3D faces the plan view drops.
	isoCanvas := *doc.Canvas
	isoCanvas.Projection = "iso"
	iso := RenderWithCanvas(scene, doc.Theme, &isoCanvas, doc.Annotations)
	if !strings.Contains(iso, `data-face="left"`) {
		t.Error("iso render of the same scene is missing isometric side faces")
	}
}
