package isotopo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func leavesOf(rects []planRect) []planRect {
	var out []planRect
	for _, r := range rects {
		if !r.container {
			out = append(out, r)
		}
	}
	return out
}

// TestAB_RouterImprovesLayout is the A/B evidence that the scorecard-guided
// router beats the baseline dominant-axis router. It holds NODE POSITIONS
// FIXED (same applyLayout output) and varies only the routing decision, then
// scores both with the same metric kernel over the whole sample corpus.
//
// Run with -v to see the per-sample table:
//
//	go test -run TestAB_RouterImprovesLayout -v .
func TestAB_RouterImprovesLayout(t *testing.T) {
	files, _ := filepath.Glob("samples/topology/*/input.yaml")
	if len(files) == 0 {
		t.Skip("no topology samples found")
	}

	var baseX, optX, baseT, optT, baseB, optB int
	var baseL, optL float64
	improved, regressed := 0, 0

	t.Logf("%-18s | base cross/thru/bend | opt cross/thru/bend", "sample")
	t.Logf("%s", "-------------------+----------------------+--------------------")
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		doc, err := LoadInput(context.Background(), "yaml", data, LayoutDagre)
		if err != nil || doc.Scene() == nil {
			continue
		}
		scene := doc.Scene()
		rb, _, eb := buildPlanModelOpt(scene, doc.Theme, doc.Canvas, false)
		ro, _, eo := buildPlanModelOpt(scene, doc.Theme, doc.Canvas, true)
		b := evalGeom(leavesOf(rb), eb)
		o := evalGeom(leavesOf(ro), eo)

		baseX += b.Crossings
		optX += o.Crossings
		baseT += b.EdgesThroughNodes
		optT += o.EdgesThroughNodes
		baseB += b.TotalBends
		optB += o.TotalBends
		baseL += b.TotalEdgeLen
		optL += o.TotalEdgeLen

		bScore := b.Crossings + b.EdgesThroughNodes
		oScore := o.Crossings + o.EdgesThroughNodes
		switch {
		case oScore < bScore:
			improved++
		case oScore > bScore:
			regressed++
		}
		t.Logf("%-18s |      %2d / %2d / %2d   |     %2d / %2d / %2d",
			filepath.Base(filepath.Dir(f)),
			b.Crossings, b.EdgesThroughNodes, b.TotalBends,
			o.Crossings, o.EdgesThroughNodes, o.TotalBends)
	}
	t.Logf("%s", "-------------------+----------------------+--------------------")
	t.Logf("TOTAL  crossings base=%d opt=%d | through base=%d opt=%d | bends base=%d opt=%d | len base=%.0f opt=%.0f",
		baseX, optX, baseT, optT, baseB, optB, baseL, optL)
	t.Logf("samples improved=%d regressed=%d", improved, regressed)

	// Acceptance: the optimized router must not regress the corpus on the two
	// metrics that hurt readability most (crossings + tunnelling), and should
	// strictly improve at least one sample.
	if (optX + optT) > (baseX + baseT) {
		t.Fatalf("router REGRESSED corpus: base crossings+through=%d, opt=%d", baseX+baseT, optX+optT)
	}
	if regressed > 0 {
		t.Fatalf("router regressed %d sample(s) on crossings+through", regressed)
	}
	if improved == 0 {
		t.Fatalf("router improved no samples — no A/B evidence of benefit")
	}
}
