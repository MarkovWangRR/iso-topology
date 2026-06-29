package isotopo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		known := strings.Contains(f, "clickhouse-hub") // L4: neighbour-label class
		if (r.Occlusions != 0 || r.Overlaps != 0) && !known {
			t.Errorf("%s: not accepted after one repair (occl=%d overlap=%d) — BIC>0",
				filepath.Base(filepath.Dir(f)), r.Occlusions, r.Overlaps)
		}
	}
}
