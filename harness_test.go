package isotopo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
