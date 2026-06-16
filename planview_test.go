package isotopo

import (
	"context"
	"strings"
	"testing"
)

const planFixture = `nodes:
  scene:
    shape: composite
    parts:
      - id: lane
        shape: group
        label: "Lane"
        parts:
          - { id: a, shape: rectangle, geom: { w: 100, d: 80, h: 20 }, label: A }
          - { id: b, shape: rectangle, geom: { w: 100, d: 80, h: 20 }, label: B, offset: { wx: 160 } }
    connectors:
      - { from: a, to: b, arrow: triangle, label: hop }
`

func renderPlanFixture(t *testing.T, src string) string {
	t.Helper()
	doc, err := LoadInput(context.Background(), "yaml", []byte(src), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	doc.Canvas = &Canvas{Projection: "top"}
	svg := RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("plan render produced no SVG:\n%s", svg)
	}
	return svg
}

func TestRenderPlan_FlatFootprintsAndEdges(t *testing.T) {
	svg := renderPlanFixture(t, planFixture)
	// Both leaf labels and the lane label render as flat text.
	for _, want := range []string{">A<", ">B<", ">Lane<", ">hop<"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("plan view missing %q:\n%s", want, svg)
		}
	}
	// A connector path with an arrowhead marker is emitted.
	if !strings.Contains(svg, "marker-end=\"url(#planarrow)\"") {
		t.Fatal("plan view drew no arrowed connector")
	}
	// The plan renderer is flat: no isometric face transforms leak in.
	if strings.Contains(svg, "cos30") {
		t.Fatal("plan output unexpectedly contains iso projection math")
	}
}

func TestRenderPlan_DispatchedByCanvasProjection(t *testing.T) {
	doc, err := LoadInput(context.Background(), "yaml", []byte(planFixture), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	iso := RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
	doc.Canvas = &Canvas{Projection: "top"}
	top := RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
	if iso == top {
		t.Fatal("projection: top produced identical output to iso — not dispatched")
	}
}

func TestValidate_UnknownProjection(t *testing.T) {
	doc, err := LoadInput(context.Background(), "yaml",
		[]byte("canvas:\n  projection: side\nnodes:\n  scene:\n    shape: rectangle\n"), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var found bool
	for _, i := range Validate(doc) {
		if i.Path == "canvas.projection" && i.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatal("validator did not flag an unknown projection")
	}
}
