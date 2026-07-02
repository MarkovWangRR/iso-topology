package isotopo

import (
	"os"
	"strings"
	"testing"
)

func compositionOf(t *testing.T, path string) *CompositionReport {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return EvaluateComposition(doc)
}

// sloppyScene: the deliberately-bad control — same class of topology as the
// gallery, but freehand-scattered offsets sharing no tracks and four clashing
// accent hues. If the metric cannot rank this below the gallery, it does not
// ship (the M2 calibration contract).
const sloppyScene = `
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, offset: { wx: 13, wy: 27 },  geom: { w: 120, d: 90, h: 30 }, label: "Web",    style: { palette: { top: "#E74C3C" } } }
      - { id: b, shape: rectangle, offset: { wx: 431, wy: 88 }, geom: { w: 120, d: 90, h: 30 }, label: "API",    style: { palette: { top: "#27AE60" } } }
      - { id: c, shape: rectangle, offset: { wx: 197, wy: 411 },geom: { w: 120, d: 90, h: 30 }, label: "DB",     style: { palette: { top: "#8E44AD" } } }
      - { id: d, shape: rectangle, offset: { wx: 689, wy: 302 },geom: { w: 120, d: 90, h: 30 }, label: "Cache",  style: { palette: { top: "#F39C12" } } }
      - { id: e, shape: rectangle, offset: { wx: 541, wy: 620 },geom: { w: 120, d: 90, h: 30 }, label: "Queue",  style: { palette: { top: "#2980B9" } } }
      - { id: f, shape: rectangle, offset: { wx: 88, wy: 743 }, geom: { w: 120, d: 90, h: 30 }, label: "Worker", style: { palette: { top: "#16A085" } } }
`

// TestCompositionCalibration is the ground-truth gate: the hand-tuned gallery
// scenes score in the top band, the sloppy control scores clearly below every
// one of them. Thresholds carry margin under the measured values (gallery
// 0.87–0.99, sloppy 0.56 at time of pinning).
func TestCompositionCalibration(t *testing.T) {
	gallery := []string{
		"samples/topology/clickhouse-hub/input.yaml",
		"samples/topology/langchain-app/input.yaml",
		"samples/topology/duckdb-handdrawn/input.yaml",
		"samples/topology/lakehouse-agent/input.yaml",
		"samples/topology/theme-clickhouse-dark/input.yaml",
	}
	minGallery := 1.0
	for _, p := range gallery {
		s := compositionOf(t, p).Score
		if s < 0.82 {
			t.Errorf("gallery scene %s scores %.2f; the metric must keep known-good in the top band (>= 0.82)", p, s)
		}
		if s < minGallery {
			minGallery = s
		}
	}

	doc, err := Parse([]byte(sloppyScene))
	if err != nil {
		t.Fatal(err)
	}
	sloppy := EvaluateComposition(doc)
	if sloppy.Score > 0.68 {
		t.Errorf("sloppy control scores %.2f; the metric must rank known-bad low (<= 0.68)", sloppy.Score)
	}
	if sloppy.Score >= minGallery-0.12 {
		t.Errorf("separation too small: sloppy %.2f vs worst gallery %.2f — the metric cannot distinguish good from bad", sloppy.Score, minGallery)
	}

	// The control's specific sins must be called out, located and actionable.
	var alignFinding, colorFinding bool
	for _, f := range sloppy.Findings {
		if f.Metric == "alignment" && strings.HasPrefix(f.Path, "nodes.scene.parts[") {
			alignFinding = true
		}
		if f.Metric == "color" && strings.Contains(f.Message, "accent hues") {
			colorFinding = true
		}
	}
	if !alignFinding {
		t.Error("sloppy control: no located alignment finding")
	}
	if !colorFinding {
		t.Error("sloppy control: no color-discipline finding")
	}
	if sloppy.AccentHues < 4 {
		t.Errorf("sloppy control has 6 clashing fills; expected >= 4 accent hue clusters, got %d", sloppy.AccentHues)
	}
}

// TestCompositionDeterministic: same doc, same report — the agent loop depends
// on stable feedback.
func TestCompositionDeterministic(t *testing.T) {
	data, err := os.ReadFile("samples/topology/langchain-app/input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var first *CompositionReport
	for i := 0; i < 5; i++ {
		doc, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		rep := EvaluateComposition(doc)
		if first == nil {
			first = rep
			continue
		}
		if rep.Score != first.Score || rep.Balance != first.Balance ||
			rep.Alignment != first.Alignment || rep.Rhythm != first.Rhythm ||
			rep.AccentHues != first.AccentHues || len(rep.Findings) != len(first.Findings) {
			t.Fatalf("non-deterministic composition report: run %d differs", i)
		}
	}
}

// TestCompositionHeroMetric: hero dominance appears only when a role:hero
// exists, rewards a big central hero, and flags a puny one.
func TestCompositionHeroMetric(t *testing.T) {
	noHero, _ := Parse([]byte(sloppyScene))
	if rep := EvaluateComposition(noHero); rep.HeroDominance != nil {
		t.Error("hero_dominance must be omitted when no role:hero exists")
	}

	small := `
theme: { use: clean-light }
nodes:
  scene:
    shape: composite
    parts:
      - { id: h,  role: hero, geom: { w: 40, d: 40, h: 10 }, offset: { wx: 0, wy: 0 }, label: "H" }
      - { id: c1, role: chip, offset: { wx: 300, wy: 0 },   label: "A" }
      - { id: c2, role: chip, offset: { wx: 300, wy: 200 }, label: "B" }
      - { id: c3, role: chip, offset: { wx: 300, wy: 400 }, label: "C" }
`
	doc, err := Parse([]byte(small))
	if err != nil {
		t.Fatal(err)
	}
	rep := EvaluateComposition(doc)
	if rep.HeroDominance == nil {
		t.Fatal("expected hero metric")
	}
	var heroFinding bool
	for _, f := range rep.Findings {
		if f.Metric == "hero" {
			heroFinding = true
		}
	}
	if !heroFinding {
		t.Error("tiny corner hero must produce a hero finding")
	}
}
