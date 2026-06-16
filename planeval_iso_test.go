package isotopo

import (
	"context"
	"testing"
)

const isoRouteFixture = `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 20 }, offset: { wx: 0,   wy: 0 } }
      - { id: b, shape: rectangle, geom: { w: 80, d: 80, h: 20 }, offset: { wx: 300, wy: 200 } }
    connectors:
      - { from: a, to: b, routing: orthogonal }
`

func TestEvaluateIso_ParsesRealRoutes(t *testing.T) {
	doc, err := LoadInput(context.Background(), "yaml", []byte(isoRouteFixture), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	scene := doc.Scene()

	// The engine must emit a real WORLD route for the orthogonal connector.
	routes := isoRealRoutes(scene, doc.Theme, doc.Canvas)
	if r, ok := routes[0]; !ok || len(r) < 2 {
		t.Fatalf("expected a parsed iso route for connector 0, got %v", routes)
	}

	// EvaluateIso scores that real route (1 edge, finite metrics).
	rep := EvaluateIso(scene, doc.Theme, doc.Canvas)
	if rep.Nodes != 2 || rep.Edges != 1 {
		t.Fatalf("expected 2 nodes / 1 edge, got %d / %d", rep.Nodes, rep.Edges)
	}
	if rep.TotalEdgeLen <= 0 {
		t.Fatalf("real-route edge length should be positive, got %v", rep.TotalEdgeLen)
	}
}

func TestEvaluateIsoText_Errors(t *testing.T) {
	if _, err := EvaluateIsoText("yaml", []byte("not: a scene\n")); err == nil {
		t.Fatal("expected an error for a document with no scene")
	}
}
