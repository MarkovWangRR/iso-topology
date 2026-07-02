package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestMCPAgentLoop drives the tools through the full self-correcting loop an
// agent runs — capabilities → validate → repair(compose) → evaluate →
// render — programmatically, asserting each step's contract. iso_snapshot is
// exercised when a rasterizer is available and skipped otherwise (CI runners
// vary), since its plumbing is shared with iso_render.
func TestMCPAgentLoop(t *testing.T) {
	// A messy but valid scene: scattered offsets (each within the compose
	// pass's bounded snap radius of some neighbour track), one low-contrast
	// label.
	dsl := `
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, offset: { wx: 13, wy: 27 },  geom: { w: 120, d: 90, h: 30 }, label: "Web" }
      - { id: b, shape: rectangle, offset: { wx: 431, wy: 88 }, geom: { w: 120, d: 90, h: 30 }, label: "API" }
      - { id: c, shape: rectangle, offset: { wx: 340, wy: 411 }, geom: { w: 120, d: 90, h: 30 }, label: "DB", style: { palette: { top: "#8A8A8A" }, text: { color: "#A5A5A5" } } }
`
	arg := func(v map[string]any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}

	// 1. capabilities — must list themes and roles for discovery.
	text, isErr := callTool("iso_capabilities", arg(map[string]any{}))
	if isErr || !strings.Contains(text, `"themes"`) || !strings.Contains(text, `"roles"`) {
		t.Fatalf("capabilities missing themes/roles (err=%v)", isErr)
	}

	// 2. validate — the contrast issue must be flagged AND marked as
	// something repair will fix... (validate itself reports; repairable
	// marking is a CLI concern — here we just need the issue visible).
	text, _ = callTool("iso_validate", arg(map[string]any{"dsl": dsl}))
	if !strings.Contains(text, "low contrast") {
		t.Fatalf("validate did not flag the planted contrast issue:\n%s", text)
	}

	// 3. repair with compose — returns REPAIRED dsl; fixes must cover both
	// the contrast defect and alignment snapping.
	text, isErr = callTool("iso_repair", arg(map[string]any{"dsl": dsl, "compose": true}))
	if isErr {
		t.Fatalf("repair errored: %s", text)
	}
	var rep struct {
		Fixes   []string `json:"fixes"`
		Changed bool     `json:"changed"`
		DSL     string   `json:"dsl"`
	}
	if err := json.Unmarshal([]byte(text), &rep); err != nil {
		t.Fatalf("repair returned non-JSON: %v", err)
	}
	if !rep.Changed || rep.DSL == "" {
		t.Fatal("repair reported no changes on a defective scene")
	}
	joined := strings.Join(rep.Fixes, "; ")
	if !strings.Contains(joined, "contrast") || !strings.Contains(joined, "aligned") {
		t.Fatalf("expected contrast + alignment fixes, got: %s", joined)
	}

	// 4. evaluate the repaired dsl — composition block present, defects zero.
	text, isErr = callTool("iso_evaluate", arg(map[string]any{"dsl": rep.DSL}))
	if isErr {
		t.Fatalf("evaluate errored: %s", text)
	}
	var ev struct {
		Readability struct {
			Overlaps int `json:"overlaps"`
			Tunnels  int `json:"tunnels"`
		} `json:"readability"`
		Composition struct {
			Score     float64 `json:"score"`
			Alignment float64 `json:"alignment"`
		} `json:"composition"`
	}
	if err := json.Unmarshal([]byte(text), &ev); err != nil {
		t.Fatalf("evaluate returned non-JSON: %v", err)
	}
	if ev.Composition.Score == 0 {
		t.Fatal("evaluate has no composition block")
	}
	if ev.Composition.Alignment < 0.99 {
		t.Errorf("repaired scene should be fully aligned, got %.2f", ev.Composition.Alignment)
	}
	if ev.Readability.Overlaps != 0 || ev.Readability.Tunnels != 0 {
		t.Errorf("repaired scene has defects: %+v", ev.Readability)
	}

	// 5. render the repaired dsl.
	text, isErr = callTool("iso_render", arg(map[string]any{"dsl": rep.DSL, "output_dir": t.TempDir()}))
	if isErr {
		t.Fatalf("render errored: %s", text)
	}
	if !strings.Contains(text, "topology.svg") {
		t.Fatalf("render did not report topology.svg:\n%s", text)
	}

	// 6. snapshot — only when a rasterizer exists on this machine.
	text, isErr = callTool("iso_snapshot", arg(map[string]any{"dsl": rep.DSL, "output_dir": t.TempDir()}))
	if isErr {
		if strings.Contains(text, "no rasterizer found") {
			t.Skipf("no rasterizer on this machine: %s", text)
		}
		t.Fatalf("snapshot errored: %s", text)
	}
	if !strings.Contains(text, ".png") {
		t.Fatalf("snapshot did not return a png path:\n%s", text)
	}
	fmt.Println("loop complete")
}
