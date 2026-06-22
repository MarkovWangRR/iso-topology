package playbook

import (
	"os"
	"testing"

	isotopo "github.com/MarkovWangRR/iso-topology"
	"github.com/MarkovWangRR/iso-topology/internal/llm"
)

func TestSynthesizeDeterministic(t *testing.T) {
	ex := &Extracted{Accent: "#29B5E8", Surface: "#FFFFFF", Ink: "#11567F",
		Radius: 10, Mood: []string{"clean", "bright"}, Domain: "data-platform",
		Icon: "brand", Corners: "sharp"}
	m := Synthesize("snowflake", ex)
	if m.Tokens["accent"] != "#29B5E8" {
		t.Errorf("accent not carried: %s", m.Tokens["accent"])
	}
	if m.Geometry.Radius != 5 { // corners:sharp → radius 5
		t.Errorf("radius = %v, want 5 (corners:sharp)", m.Geometry.Radius)
	}
	if m.Tokens["accent2"] == "" {
		t.Errorf("secondary accent token missing")
	}
	if len(m.Roles) != 8 {
		t.Errorf("expected the 8 standard roles, got %d", len(m.Roles))
	}
	// the synthesized manual must apply to valid isotopo
	st, _ := os.ReadFile(testRoot + "/_exemplar.yaml")
	out, err := Apply(st, m)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := isotopo.Parse(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, i := range isotopo.Validate(doc) {
		if i.Severity == isotopo.SeverityError {
			t.Errorf("synthesized style invalid: %s", i.Message)
		}
	}
}

func TestRefineAppliesVerdict(t *testing.T) {
	m := Synthesize("x", &Extracted{Accent: "#000000"})
	Refine(m, &Verdict{Accent: "#29B5E8", Radius: 12})
	if m.Tokens["accent"] != "#29B5E8" {
		t.Errorf("refine did not apply accent")
	}
	if m.Geometry.Radius != 12 {
		t.Errorf("refine did not apply radius")
	}
	// a light accent → dark ink
	if m.Tokens["accentInk"] != "#202124" {
		t.Errorf("accentInk for light accent = %s, want dark", m.Tokens["accentInk"])
	}
}

func TestPickInk(t *testing.T) {
	if pickInk("#101010") != "#FFFFFF" {
		t.Error("dark accent should get white ink")
	}
	if pickInk("#FFE000") != "#202124" {
		t.Error("light accent should get dark ink")
	}
}

// Live extraction is exercised by the flywheel e2e; here we only assert the
// client constructs and is gated when no key is present.
func TestLLMClientGating(t *testing.T) {
	if llm.New().Available() && os.Getenv("OPENAI_API_KEY") == "" {
		t.Error("client reports available without a key")
	}
}
