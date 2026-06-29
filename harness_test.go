package isotopo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two top-level group trays whose slabs collide must register in R's overlap
// term (not only validate) — the leaf-only count missed it — and the repair must
// separate them. (Dogfooding the Dremio diagram surfaced this gap.)
func TestOverlap_GroupSlabsCountedAndRepaired(t *testing.T) {
	src := `
canvas: { background: "#0E1726" }
nodes:
  scene:
    shape: composite
    parts:
      - id: g1
        shape: group
        label: "Group One"
        layout: { mode: column, gap: 0.8 }
        parts:
          - { id: a1, shape: rectangle, geom: { w: 120, d: 80, h: 24 }, label: "A1" }
          - { id: a2, shape: rectangle, geom: { w: 120, d: 80, h: 24 }, label: "A2" }
      - id: g2
        offset: { wx: 70, wy: 70 }
        shape: group
        label: "Group Two"
        layout: { mode: column, gap: 0.8 }
        parts:
          - { id: b1, shape: rectangle, geom: { w: 120, d: 80, h: 24 }, label: "B1" }
          - { id: b2, shape: rectangle, geom: { w: 120, d: 80, h: 24 }, label: "B2" }
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if before := Readability(doc).Overlaps; before == 0 {
		t.Fatal("colliding group slabs must count as an overlap in R")
	}
	RepairScene(doc)
	if after := Readability(doc).Overlaps; after != 0 {
		t.Fatalf("repair must separate colliding group slabs, still %d", after)
	}
}

// A child that overflows a deliberately-narrow author-fixed group geom must be
// classified as the group's OWN child (caption-ride), not misread as a neighbour
// occlusion "from an adjacent module" — the misclassification that flung the
// Dremio hero. The author's explicit dims are still respected (not grown); the
// classifier just uses footprint overlap instead of centre-inside.
func TestGroup_OverflowChildClassifiedInGroup(t *testing.T) {
	src := `
canvas: { background: "#0E1726" }
nodes:
  scene:
    shape: composite
    parts:
      - id: g
        geom: { d: 120 }
        shape: group
        label: "A Deliberately Long Group Caption Here"
        layout: { mode: column, gap: 0.8 }
        parts:
          - { id: c1, shape: rectangle, geom: { w: 170, d: 92, h: 24 }, label: "Child One" }
          - { id: c2, shape: rectangle, geom: { w: 170, d: 92, h: 24 }, label: "Child Two" }
          - { id: c3, shape: rectangle, geom: { w: 170, d: 92, h: 24 }, label: "Child Three" }
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var sawInGroup bool
	for _, is := range LabelOcclusionIssues(doc) {
		if strings.Contains(is.Message, "adjacent module") {
			t.Errorf("overflow child misclassified as neighbour: %s", is.Message)
		}
		if strings.Contains(is.Message, "its own child") {
			sawInGroup = true
		}
	}
	if !sawInGroup {
		t.Fatal("expected an in-group caption-ride for the overflowing child")
	}
}

// TestL2_PatchActionability is the L2 gate (docs/design/agent-loop-harness-plan.md):
// every machine-applicable patch the report emits must, when applied, clear the
// defect it targets — ≥95%. Exercised on the caption-ride class (the one the
// report patches today) across the corpus + real demos.
func TestL2_PatchActionability(t *testing.T) {
	scenes := []string{
		"samples/bench/bad-occlusion.yaml",
		"samples/bench/bad-both.yaml",
		"samples/topology/langchain-app/input.yaml",
		"samples/topology/data-fabric/input.yaml",
		"samples/topology/vpc-boundary/input.yaml",
	}
	emitted, cleared := 0, 0
	for _, f := range scenes {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		doc, err := Parse(data)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, d := range BuildRenderReport(doc).Defects {
			if d.Patch == nil {
				continue
			}
			emitted++
			fresh, _ := Parse(data)
			if !ApplyPatch(fresh, *d.Patch) {
				t.Errorf("%s: patch %+v hit no target", filepath.Base(f), *d.Patch)
				continue
			}
			want := d.Message
			if i := strings.Index(want, " — "); i >= 0 {
				want = want[:i]
			}
			if !occlusionMessages(fresh)[want] {
				cleared++
			} else {
				t.Logf("%s: patch did NOT clear %q", filepath.Base(f), want)
			}
		}
	}
	if emitted == 0 {
		t.Fatal("report emitted no patches — patch generation broken")
	}
	if rate := float64(cleared) / float64(emitted); rate < 0.95 {
		t.Fatalf("patch actionability %.0f%% (%d/%d) < 95%%", rate*100, cleared, emitted)
	}
	t.Logf("patch actionability: %d/%d", cleared, emitted)
}

// TestBIC_HandledClassesZero is the L1 gate (docs/design/agent-loop-harness-plan.md):
// a single repair pass — what the default `render` now does — must clear ALL
// handled-class defects (caption-rides + world-overlaps) on every synthetic
// corpus AND real samples/topology scene, driving the Blind-Iteration Count to 0.
// The one allowed residual is clickhouse-hub's neighbour-label screen-occlusion,
// a class not yet repaired (the Layer-4 target).
func TestBIC_HandledClassesZero(t *testing.T) {
	var files []string
	bench, _ := filepath.Glob("samples/bench/*.yaml")
	files = append(files, bench...)
	dirs, _ := filepath.Glob("samples/topology/*")
	for _, dir := range dirs {
		f := filepath.Join(dir, "input.yaml")
		if _, err := os.Stat(f); err == nil {
			files = append(files, f)
		}
	}
	if len(files) < 10 {
		t.Fatalf("expected the corpus + real demos, found %d scenes", len(files))
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		doc, err := Parse(data)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		RepairScene(doc)
		r := Readability(doc)
		if r.Occlusions != 0 || r.Overlaps != 0 {
			t.Errorf("%s: not accepted after one repair (occl=%d overlap=%d) — BIC>0",
				filepath.Base(filepath.Dir(f)), r.Occlusions, r.Overlaps)
		}
	}
}
