package isotopo

import (
	"os"
	"path/filepath"
	"testing"
)

func loadBench(t *testing.T, name string) *Document {
	t.Helper()
	data, err := os.ReadFile("samples/bench/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

// The projection-repair loop must eliminate a caption occlusion and raise R.
func TestRepair_FixesCaptionOcclusion(t *testing.T) {
	doc := loadBench(t, "bad-occlusion.yaml")
	before := Readability(doc)
	if before.Occlusions == 0 {
		t.Fatal("fixture should start with a caption occlusion")
	}
	_, iters := RepairScene(doc)
	after := Readability(doc)
	if iters == 0 {
		t.Fatal("repair should have run at least one iteration")
	}
	if after.Occlusions != 0 {
		t.Fatalf("repair must clear the caption occlusion, still %d", after.Occlusions)
	}
	if after.Score <= before.Score {
		t.Fatalf("repair must raise R: %.3f → %.3f", before.Score, after.Score)
	}
}

// Repair must be a strict no-op on an already-clean scene (0 iterations, so the
// rendered output is byte-identical — golden-safe to run unconditionally).
func TestRepair_NoOpOnClean(t *testing.T) {
	for _, name := range []string{"good-grid.yaml", "good-flow.yaml"} {
		doc := loadBench(t, name)
		if occ := Readability(doc).Occlusions; occ != 0 {
			t.Fatalf("%s is not clean (%d occlusions)", name, occ)
		}
		if _, iters := RepairScene(doc); iters != 0 {
			t.Fatalf("%s: repair must be a no-op on a clean scene, ran %d iters", name, iters)
		}
	}
}

// TestRepair_P1Gate is the Phase-1 acceptance gate: after the projection-repair
// loop, every benchmark scene must be free of occlusion AND overlap, and its
// readability must not have decreased.
func TestRepair_P1Gate(t *testing.T) {
	files, err := filepath.Glob("samples/bench/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no bench scenes: %v", err)
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
		before := Readability(doc).Score
		RepairScene(doc)
		after := Readability(doc)
		if after.Occlusions != 0 || after.Overlaps != 0 {
			t.Errorf("%s: not clean after repair (occl=%d overlap=%d)",
				filepath.Base(f), after.Occlusions, after.Overlaps)
		}
		if after.Score < before-1e-9 {
			t.Errorf("%s: repair lowered R (%.3f → %.3f)", filepath.Base(f), before, after.Score)
		}
	}
}

// Readability / EvaluateIso must NOT mutate the document (they solve a clone),
// so a measure-then-repair sequence still sees the original Layout declarations.
func TestEvaluate_DoesNotMutateDoc(t *testing.T) {
	doc := loadBench(t, "bad-occlusion.yaml")
	_ = Readability(doc) // would clear the group's Layout if it mutated
	_, iters := RepairScene(doc)
	if iters == 0 {
		t.Fatal("repair found nothing to do — evaluation mutated the doc's Layout")
	}
}
