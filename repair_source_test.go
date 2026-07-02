package isotopo

import (
	"strings"
	"testing"
)

// TestRepairSourcePersistsContrastFix: the contrast repair class round-trips
// into the source — the retinted label color lands in the YAML, comments
// survive, the persisted file re-validates clean, and repair is idempotent.
func TestRepairSourcePersistsContrastFix(t *testing.T) {
	src := []byte(`# scene comment
nodes:
  scene:
    shape: composite
    parts:
      # invisible label: mid-grey on mid-grey
      - { id: a, shape: rectangle, label: "Payments", style: { palette: { top: "#8A8A8A" }, text: { color: "#A5A5A5" } } }
`)
	out, fixes, err := RepairSource("yaml", src)
	if err != nil {
		t.Fatalf("RepairSource: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected a contrast fix, got none")
	}
	text := string(out)
	if !strings.Contains(text, "# scene comment") || !strings.Contains(text, "# invisible label") {
		t.Error("comments were not preserved")
	}
	if strings.Contains(text, "#A5A5A5") {
		t.Error("low-contrast text color still present in persisted source")
	}
	// The persisted source must parse and re-validate with no contrast issues.
	doc, err := Parse(out)
	if err != nil {
		t.Fatalf("persisted source does not parse: %v", err)
	}
	for _, is := range VisualContrastIssues(doc) {
		if strings.Contains(is.Message, "low contrast between top fill") {
			t.Errorf("persisted source still has contrast issue: %s", is.Message)
		}
	}
	// Idempotence: repairing the repaired source is a no-op.
	out2, fixes2, err := RepairSource("yaml", out)
	if err != nil {
		t.Fatalf("second RepairSource: %v", err)
	}
	if len(fixes2) != 0 {
		t.Errorf("repair not idempotent: second pass produced %d fixes: %v", len(fixes2), fixes2)
	}
	if string(out2) != string(out) {
		t.Error("second repair changed the source")
	}
}

// TestRepairSourcePersistsCaptionFix: the caption-occlusion class (children
// riding the group caption row) persists as a layout.padding bump on the group.
func TestRepairSourcePersistsCaptionFix(t *testing.T) {
	src := []byte(`nodes:
  scene:
    shape: composite
    parts:
      - id: grp
        shape: group
        label: "Services"
        layout: { mode: column, gap: 0.3 }
        parts:
          - { id: a, shape: rectangle, geom: { w: 104, d: 66, h: 18 }, label: "A" }
          - { id: b, shape: rectangle, geom: { w: 104, d: 66, h: 18 }, label: "B" }
`)
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(LabelOcclusionIssues(doc)) == 0 {
		t.Skip("fixture no longer produces a caption occlusion; adjust geometry")
	}
	out, fixes, err := RepairSource("yaml", src)
	if err != nil {
		t.Fatalf("RepairSource: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected a caption fix, got none")
	}
	if !strings.Contains(string(out), "padding:") {
		t.Errorf("expected a layout.padding write, got:\n%s", out)
	}
	rdoc, err := Parse(out)
	if err != nil {
		t.Fatalf("persisted source does not parse: %v", err)
	}
	if n := len(LabelOcclusionIssues(rdoc)); n != 0 {
		t.Errorf("persisted source still has %d occlusion(s)", n)
	}
}

// TestRepairSourceRejectsNonYAML: d2 geometry is generated, so there is
// nothing stable to write a repair into — must refuse, not corrupt.
func TestRepairSourceRejectsNonYAML(t *testing.T) {
	if _, _, err := RepairSource("d2", []byte("a -> b")); err == nil {
		t.Error("expected an error for d2 input")
	}
}

// TestMarkRepairable: issues that vanish after repair are flagged, ones that
// remain are not.
func TestMarkRepairable(t *testing.T) {
	before := []Issue{
		{Severity: SeverityWarning, Path: "a", Message: "fixed by repair"},
		{Severity: SeverityWarning, Path: "b", Message: "needs a hand edit"},
	}
	after := []Issue{
		{Severity: SeverityWarning, Path: "b", Message: "needs a hand edit"},
	}
	got := MarkRepairable(before, after)
	if !got[0].Repairable {
		t.Error("vanished issue not marked repairable")
	}
	if got[1].Repairable {
		t.Error("persisting issue wrongly marked repairable")
	}
}
