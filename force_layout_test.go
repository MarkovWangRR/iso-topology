package isotopo

import "testing"

// Graph-class detection must route cyclic graphs to the force-directed placer
// and leave DAGs on longest-path layering.
func TestGraphClass_DetectsCycles(t *testing.T) {
	mesh := loadBench(t, "mesh.yaml").Scene()
	if !graphIsCyclic(mesh.Parts, mesh.Connectors) {
		t.Error("the mesh (cyclic) must be detected as cyclic")
	}
	dag := loadBench(t, "good-flow.yaml").Scene()
	if graphIsCyclic(dag.Parts, dag.Connectors) {
		t.Error("good-flow (a DAG) must NOT be detected as cyclic")
	}
}

// Adaptive-spread force-directed placement must clear ALL tunnelling on the
// dense mesh (longest-path tunnels 6), without regressing the sparse hub, which
// must keep its compact base spread (it never tunnels, so never escalates).
func TestForceLayout_ClearsMeshTunnelling(t *testing.T) {
	if thru := Readability(loadBench(t, "mesh.yaml")).Tunnels; thru != 0 {
		t.Fatalf("adaptive force-directed must clear mesh tunnelling, got %d", thru)
	}
	hub := Readability(loadBench(t, "hub.yaml"))
	if hub.Tunnels != 0 || hub.Crossings != 0 {
		t.Fatalf("hub should stay clean (thru=%d cross=%d)", hub.Tunnels, hub.Crossings)
	}
	if hub.Score < 0.78 {
		t.Fatalf("hub must keep its compact ring (R≈0.79), got %.3f — over-spread?", hub.Score)
	}
}
