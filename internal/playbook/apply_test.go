package playbook

import (
	"os"
	"testing"

	isotopo "github.com/MarkovWangRR/iso-topology"
)

const testRoot = "../../samples/playbook"

func lum(hex string) float64 {
	r, g, b, _, _ := parseHex(hex)
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255
}

func TestDeriveFaces(t *testing.T) {
	m := Material{Space: "hsl", Light: "topRight", LitSideDL: -0.05, ShadeSideDL: -0.13, AoDL: 0.05}
	f := DeriveFaces("#FFFFFF", m)
	if f.Top != "#FFFFFF" {
		t.Errorf("top = %s, want #FFFFFF", f.Top)
	}
	// top brightest; right (lit) brighter than left (shadow)
	if lum(f.Top) <= lum(f.RightFrom) {
		t.Errorf("top must be brighter than the lit side")
	}
	if lum(f.RightFrom) <= lum(f.LeftFrom) {
		t.Errorf("right (lit) %s must be brighter than left (shadow) %s", f.RightFrom, f.LeftFrom)
	}
	// AO: within a side, the top edge is brighter than the base
	if lum(f.LeftFrom) <= lum(f.LeftTo) {
		t.Errorf("AO falloff: left top edge must be brighter than base")
	}
	// alpha is preserved
	f2 := DeriveFaces("#FFFFFFE6", m)
	if f2.LeftFrom[len(f2.LeftFrom)-2:] != "E6" {
		t.Errorf("alpha not preserved: %s", f2.LeftFrom)
	}
}

func loadLustre(t *testing.T) (*Manual, []byte) {
	t.Helper()
	m, err := LoadManual(testRoot, "lustre")
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.ReadFile(testRoot + "/_exemplar.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return m, st
}

func TestApplyDeterministic(t *testing.T) {
	m, st := loadLustre(t)
	a, err := Apply(st, m)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Apply(st, m)
	if string(a) != string(b) {
		t.Fatal("Apply is not deterministic")
	}
}

func TestApplyProducesValidIsotopo(t *testing.T) {
	m, st := loadLustre(t)
	out, err := Apply(st, m)
	if err != nil {
		t.Fatal(err)
	}
	// no leftover playbook-only keys
	if got := string(out); contains(got, "role:") {
		t.Errorf("Apply leaked a `role:` key into isotopo output")
	}
	doc, err := isotopo.Parse(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, i := range isotopo.Validate(doc) {
		if i.Severity == isotopo.SeverityError {
			t.Errorf("validate error: %s — %s", i.Path, i.Message)
		}
	}
}

func TestLintClean(t *testing.T) {
	if issues := Lint(testRoot, "lustre"); len(issues) != 0 {
		t.Errorf("lint lustre: %v", issues)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
