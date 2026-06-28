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

// Force-directed placement must materially beat longest-path layering on the
// dense mesh, where longest-path tunnels 6 edges.
func TestForceLayout_ReducesMeshTunnelling(t *testing.T) {
	doc := loadBench(t, "mesh.yaml")
	if thru := Readability(doc).Tunnels; thru >= 6 {
		t.Fatalf("force-directed should cut mesh tunnelling below the longest-path baseline of 6, got %d", thru)
	}
}
