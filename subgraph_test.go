package isotopo

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// subgraphFixture: two standalone agents → a Runner container (with a nested
// child) → a standalone server. Exercises every selection mode: leaf node,
// container subtree, edge ref, auto-edge inclusion, and subsumption.
const subgraphFixture = `
nodes:
  scene:
    shape: composite
    layout: { mode: auto }
    parts:
      - id: agent_a
        shape: rectangle
        geom: { w: 80, d: 60, h: 24 }
        label: "Agent A"
      - id: agent_b
        shape: rectangle
        geom: { w: 80, d: 60, h: 24 }
        label: "Agent B"
      - id: runner
        shape: boundary
        label: "Runner"
        layout: { mode: row, gap: 1, padding: 1 }
        parts:
          - id: sandbox
            shape: rectangle
            geom: { w: 70, d: 50, h: 20 }
            label: "Sandbox"
          - id: reliab
            shape: rectangle
            geom: { w: 70, d: 50, h: 20 }
            label: "Reliability"
      - id: server
        shape: rectangle
        geom: { w: 90, d: 70, h: 26 }
        label: "Server"
    connectors:
      - { from: agent_a, to: runner, routing: orthogonal }
      - { from: agent_b, to: runner, routing: orthogonal }
      - { from: runner, to: server, routing: orthogonal }
`

func loadSubgraphDoc(t *testing.T) *Document {
	t.Helper()
	doc, err := LoadInput(context.Background(), "yaml", []byte(subgraphFixture), LayoutDagre)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return doc
}

func partIDsIn(svg string) []string {
	re := regexp.MustCompile(`data-part-id="([^"]*)"`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		seen[m[1]] = true
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func TestRenderSubgraphSingleNode(t *testing.T) {
	doc := loadSubgraphDoc(t)
	svg, err := RenderSubgraph(doc, []string{"server"})
	if err != nil {
		t.Fatalf("RenderSubgraph: %v", err)
	}
	if got := partIDsIn(svg); len(got) != 1 || got[0] != "server" {
		t.Fatalf("single-node preview parts = %v, want [server]", got)
	}
	if strings.Contains(svg, "data-connector") {
		t.Errorf("single isolated node should carry no connector")
	}
}

func TestRenderSubgraphContainerSubtree(t *testing.T) {
	doc := loadSubgraphDoc(t)
	svg, err := RenderSubgraph(doc, []string{"runner"})
	if err != nil {
		t.Fatalf("RenderSubgraph: %v", err)
	}
	got := partIDsIn(svg)
	for _, want := range []string{"runner", "sandbox", "reliab"} {
		if !contains(got, want) {
			t.Errorf("container preview missing %q (got %v)", want, got)
		}
	}
	// The container preview should NOT drag in unrelated standalone nodes.
	if contains(got, "agent_a") || contains(got, "server") {
		t.Errorf("container preview leaked unrelated nodes: %v", got)
	}
}

func TestRenderSubgraphEdgeRef(t *testing.T) {
	doc := loadSubgraphDoc(t)
	// edge:2 is runner→server — must pull in both endpoints and the wire.
	svg, err := RenderSubgraph(doc, []string{"edge:2"})
	if err != nil {
		t.Fatalf("RenderSubgraph: %v", err)
	}
	got := partIDsIn(svg)
	if !contains(got, "runner") || !contains(got, "server") {
		t.Errorf("edge preview missing an endpoint: %v", got)
	}
	if !strings.Contains(svg, "data-connector") {
		t.Errorf("edge preview should render the connector")
	}
}

func TestRenderSubgraphAutoIncludesEdge(t *testing.T) {
	doc := loadSubgraphDoc(t)
	// Selecting both endpoints of an existing edge must surface the wire.
	svg, err := RenderSubgraph(doc, []string{"runner", "server"})
	if err != nil {
		t.Fatalf("RenderSubgraph: %v", err)
	}
	if !strings.Contains(svg, "data-connector") {
		t.Errorf("two connected nodes should auto-include their edge")
	}
}

func TestRenderSubgraphSubsumesChild(t *testing.T) {
	doc := loadSubgraphDoc(t)
	// Selecting a container AND one of its children must equal the container
	// alone — the child is already shown by the subtree.
	full, _ := RenderSubgraph(doc, []string{"runner"})
	withChild, err := RenderSubgraph(doc, []string{"runner", "sandbox"})
	if err != nil {
		t.Fatalf("RenderSubgraph: %v", err)
	}
	if a, b := partIDsIn(full), partIDsIn(withChild); !equalStrings(a, b) {
		t.Errorf("subsumption broke: runner=%v vs runner+sandbox=%v", a, b)
	}
}

func TestRenderSubgraphErrors(t *testing.T) {
	doc := loadSubgraphDoc(t)
	cases := []struct {
		name string
		ids  []string
	}{
		{"empty", nil},
		{"unknown-id", []string{"ghost"}},
		{"bad-edge", []string{"edge:99"}},
		{"non-numeric-edge", []string{"edge:x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := RenderSubgraph(doc, c.ids); err == nil {
				t.Errorf("expected error for %v", c.ids)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
