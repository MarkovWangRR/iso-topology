package isotopo

import (
	"context"
	"strings"
	"testing"
)

func TestEvaluatePlan_TunnelDetectionAndAvoidance(t *testing.T) {
	// a → c runs straight across with b squarely in the corridor. The BASELINE
	// router drives straight through b (the evaluator must flag it); the
	// scorecard-guided router must find a detour that clears b.
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 0,   wy: 0 } }
      - { id: b, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 200, wy: 0 } }
      - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 400, wy: 0 } }
    connectors:
      - { from: a, to: c }
`
	doc, err := LoadInput(context.Background(), "yaml", []byte(src), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	scene := doc.Scene()

	rb, _, eb := buildPlanModelOpt(scene, doc.Theme, doc.Canvas, false)
	base := evalGeom(leavesOf(rb), eb)
	if base.EdgesThroughNodes != 1 || len(base.ProblemEdges) != 1 || base.ProblemEdges[0].Through[0] != "b" {
		t.Fatalf("baseline router should tunnel through b, got %+v", base)
	}

	ro, _, eo := buildPlanModelOpt(scene, doc.Theme, doc.Canvas, true)
	opt := evalGeom(leavesOf(ro), eo)
	if opt.EdgesThroughNodes != 0 {
		t.Fatalf("scorecard router should route around b, still tunnels: %+v", opt)
	}
}

func TestEvaluatePlan_IgnoresSubstrateAndStacks(t *testing.T) {
	// Two chips on a shared plate: the wire between them runs across the plate
	// (a PCB trace), which must NOT be flagged as tunnelling — the plate sits
	// below them (z) and encloses both (x,y).
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: plate, shape: rectangle, geom: { w: 300, d: 200, h: 12 }, offset: { wx: 0,   wy: 0,  wz: 0 } }
      - { id: a,     shape: rectangle, geom: { w: 50,  d: 50,  h: 20 }, offset: { wx: 20,  wy: 70, wz: 12 } }
      - { id: b,     shape: rectangle, geom: { w: 50,  d: 50,  h: 20 }, offset: { wx: 230, wy: 70, wz: 12 } }
    connectors:
      - { from: a, to: b }
`
	rep, err := EvaluatePlanText("yaml", []byte(src))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rep.EdgesThroughNodes != 0 {
		t.Fatalf("a→b across a shared plate must not be flagged as tunnelling, got %+v", rep)
	}
}

func TestEvaluatePlan_CleanLayoutScoresZero(t *testing.T) {
	// Two nodes side by side, one edge — no crossings, no tunnelling.
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 0,   wy: 0 } }
      - { id: b, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 200, wy: 0 } }
    connectors:
      - { from: a, to: b }
`
	rep, err := EvaluatePlanText("yaml", []byte(src))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rep.Crossings != 0 || rep.EdgesThroughNodes != 0 || rep.NodeOverlaps != 0 {
		t.Fatalf("clean layout should score zero, got %+v", rep)
	}
	if rep.FlowAxis != "horizontal" {
		t.Fatalf("a→b along x should read horizontal flow, got %q", rep.FlowAxis)
	}
}

func TestRenderPlanAnnotated_MarksProblems(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 0,   wy: 0 } }
      - { id: b, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 200, wy: 0 } }
      - { id: c, shape: rectangle, geom: { w: 60, d: 60, h: 10 }, offset: { wx: 400, wy: 0 } }
    connectors:
      - { from: a, to: c }
`
	doc, err := LoadInput(context.Background(), "yaml", []byte(src), LayoutDagre)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	scene := doc.Scene()

	// Clean render carries no red overlay.
	if strings.Contains(renderPlan(scene, doc.Theme, doc.Canvas, nil), "#e11d48") {
		t.Fatal("unannotated plan should have no red overlay")
	}
	// A report with a problem edge + crossing must paint red marks.
	hl := &PlanReport{
		ProblemEdges: []PlanEdgeIssue{{From: "a", To: "c", Through: []string{"b"},
			Pts: [][2]float64{{30, 30}, {430, 30}}}},
		CrossingsAt: []PlanPoint{{X: 200, Y: 30}},
	}
	if !strings.Contains(renderPlan(scene, doc.Theme, doc.Canvas, hl), "#e11d48") {
		t.Fatal("annotated plan missing the red problem overlay colour")
	}
	// The public helper returns both an SVG and a non-nil report.
	svg, rep := RenderPlanAnnotated(scene, doc.Theme, doc.Canvas)
	if rep == nil || !strings.Contains(svg, "<svg") {
		t.Fatal("RenderPlanAnnotated returned no svg/report")
	}
}
