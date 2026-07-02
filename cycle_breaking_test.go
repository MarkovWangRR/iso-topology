package isotopo

import "testing"

func autoDoc(edges [][2]string) *Node {
	ids := map[string]bool{}
	var parts []*CompositePart
	add := func(id string) {
		if !ids[id] {
			ids[id] = true
			parts = append(parts, &CompositePart{ID: id, Shape: "rectangle",
				Geom: &Geom{W: 120, D: 90, H: 30}, Label: id})
		}
	}
	var conns []*Connector
	for _, e := range edges {
		add(e[0])
		add(e[1])
		conns = append(conns, &Connector{From: e[0], To: e[1], Routing: "orthogonal"})
	}
	return &Node{Shape: "composite", Layout: &Layout{Mode: "auto"}, Parts: parts, Connectors: conns}
}

func offsetX(n *Node, id string) float64 {
	for _, p := range n.Parts {
		if p.ID == id {
			if p.Offset == nil {
				return 0
			}
			return p.Offset.WX
		}
	}
	return 0
}

// TestCycleBreakingKeepsFlowLayered: a multi-row flow with one long-cycle
// feedback arc is a FLOW, not a mesh — the feedback edge has a clear elbow
// route (it targets an off-spine node), so the layered left-to-right
// narrative must survive instead of being forfeited wholesale to the force
// placer, which is what a binary cyclic check did.
func TestCycleBreakingKeepsFlowLayered(t *testing.T) {
	n := autoDoc([][2]string{
		{"ingest", "route"},
		{"route", "fast"}, {"route", "slow"}, // rank 2 has two rows
		{"fast", "merge"}, {"slow", "merge"},
		{"merge", "serve"},
		{"serve", "slow"}, // feedback into the off-spine row — elbow-clear
	})
	applyLayout(n, nil)
	chain := []string{"ingest", "route", "fast", "merge", "serve"}
	for i := 1; i < len(chain); i++ {
		if !(offsetX(n, chain[i]) > offsetX(n, chain[i-1])) {
			t.Fatalf("flow not layered: x(%s)=%.0f !> x(%s)=%.0f",
				chain[i], offsetX(n, chain[i]), chain[i-1], offsetX(n, chain[i-1]))
		}
	}
	if tn := trialTunnels(n.Parts, n.Connectors, feedbackEdges(n.Parts, n.Connectors)); tn != 0 {
		t.Errorf("kept layered arrangement has %d unclearable edge(s); want 0", tn)
	}
}

// TestCycleBreakingSingleRowPipelineFallsBack documents today's honest limit:
// a single-file pipeline's feedback arc has no clear straight OR elbow route
// (every candidate degenerates onto the shared row and tunnels the chain), so
// the dispatcher must fall back to the force placer — which renders the cycle
// as a clean ring. When the router learns detours, this can flip to layered.
func TestCycleBreakingSingleRowPipelineFallsBack(t *testing.T) {
	n := autoDoc([][2]string{
		{"a", "b"}, {"b", "c"}, {"c", "d"}, {"d", "e"}, {"e", "b"},
	})
	applyLayout(n, nil)
	mono := true
	chain := []string{"a", "b", "c", "d", "e"}
	for i := 1; i < len(chain); i++ {
		if !(offsetX(n, chain[i]) > offsetX(n, chain[i-1])) {
			mono = false
		}
	}
	if mono {
		t.Error("single-row pipeline kept a layered arrangement whose feedback edge must tunnel; expected force fallback")
	}
	if tn := straightTunnelsAll(n); tn != 0 {
		t.Errorf("force fallback still tunnels: %d", tn)
	}
}

// TestCycleBreakingFallsBackToForceOnDenseGraphs: a dense near-mesh whose
// trial layered arrangement tunnels must roll back to the force placer —
// judged by route clearance, not a magic back-edge threshold (the bench mesh
// scores 5 real tunnels when layered vs 0 under force).
func TestCycleBreakingFallsBackToForceOnDenseGraphs(t *testing.T) {
	n := autoDoc([][2]string{
		{"a", "b"}, {"a", "c"}, {"b", "c"}, {"b", "d"},
		{"c", "d"}, {"c", "e"}, {"d", "e"}, {"e", "a"}, {"d", "a"},
	})
	applyLayout(n, nil)
	if tn := straightTunnelsAll(n); tn != 0 {
		t.Errorf("dense graph still tunnels after dispatch: %d; want 0 (force fallback)", tn)
	}
}

// TestReciprocalPairsKeepForce: A<->B pairs are bidirectional traffic
// (hub-and-spoke), not pipeline feedback — the layered trial must be skipped
// so hubs keep their radial force spread.
func TestReciprocalPairsKeepForce(t *testing.T) {
	n := autoDoc([][2]string{
		{"hub", "s1"}, {"hub", "s2"}, {"hub", "s3"}, {"hub", "s4"},
		{"s1", "hub"}, {"s2", "hub"},
	})
	back := feedbackEdges(n.Parts, n.Connectors)
	if !hasReciprocalBackEdge(n.Connectors, back) {
		t.Fatal("reciprocal back edges not detected")
	}
	applyLayout(n, nil)
	// Force spread: the spokes surround the hub rather than forming ranks —
	// at least one spoke must sit at x <= hub's x (a pure 2-rank layered
	// arrangement puts every spoke strictly right of the hub).
	left := false
	for _, s := range []string{"s1", "s2", "s3", "s4"} {
		if offsetX(n, s) <= offsetX(n, "hub") {
			left = true
		}
	}
	if !left {
		t.Error("hub-and-spoke was layered into ranks; expected radial force spread")
	}
}

// TestFeedbackEdgesDeterministic: the DFS back-edge set must be stable across
// runs (declared order drives everything downstream).
func TestFeedbackEdgesDeterministic(t *testing.T) {
	n := autoDoc([][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}, {"b", "d"}})
	first := feedbackEdges(n.Parts, n.Connectors)
	for i := 0; i < 10; i++ {
		got := feedbackEdges(n.Parts, n.Connectors)
		if len(got) != len(first) {
			t.Fatalf("non-deterministic back-edge count: %v vs %v", got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("non-deterministic back edges: %v vs %v", got, first)
			}
		}
	}
	if len(first) != 1 || first[0] != 2 {
		t.Errorf("expected exactly connector[2] (c->a) as the back edge, got %v", first)
	}
}

// straightTunnelsAll counts center-line tunnels over every edge — the sanity
// check that a force fallback actually spread the graph clean.
func straightTunnelsAll(n *Node) int {
	return trialTunnels(n.Parts, n.Connectors, nil)
}
