package isotopo

import (
	"fmt"
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func ptr64(f float64) *float64 { return &f }

func hasWarning(issues []Issue, substr string) bool {
	for _, iss := range issues {
		if iss.Severity == SeverityWarning && strings.Contains(iss.Message, substr) {
			return true
		}
	}
	return false
}

// ── 1a. top fill vs text contrast ────────────────────────────────────────────

func TestVisualContrast_TextLowContrast(t *testing.T) {
	// Fill #E0E0E0 (light grey) vs text #D0D0D0 (nearly same) → ratio well below 3.0
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "box",
						Shape: "rectangle",
						Label: "Hi", // fill-vs-text contrast only applies to a labelled face
						Style: &Style{
							Palette: &Palette{Top: "#E0E0E0"},
							Text:    &Text{Color: "#D0D0D0"},
						},
					},
				},
			},
		},
	}
	issues := VisualContrastIssues(doc)
	if !hasWarning(issues, "low contrast between top fill") {
		t.Errorf("expected low-contrast warning, got: %v", issues)
	}
}

// ── 1b. node vs canvas background ────────────────────────────────────────────

func TestVisualContrast_NodeVsBackground(t *testing.T) {
	// Node fill nearly same as canvas background → ratio < 1.5
	doc := &Document{
		Canvas: &Canvas{Background: "#F8FAFC"},
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "box",
						Shape: "rectangle",
						Style: &Style{
							Palette: &Palette{Top: "#F8FAFC"},
						},
					},
				},
			},
		},
	}
	issues := VisualContrastIssues(doc)
	if !hasWarning(issues, "indistinguishable from canvas background") {
		t.Errorf("expected background contrast warning, got: %v", issues)
	}
}

// ── 1c. group vs child contrast ──────────────────────────────────────────────

func TestVisualContrast_GroupVsChild(t *testing.T) {
	// Group fill #AAAAAA, child fill #AAAAAB — nearly identical → ratio < 1.3
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "grp",
						Shape: "group",
						Style: &Style{Palette: &Palette{Top: "#AAAAAA"}},
						Parts: []*CompositePart{
							{
								ID:    "child",
								Shape: "rectangle",
								Style: &Style{Palette: &Palette{Top: "#AAAAAB"}},
							},
						},
					},
				},
			},
		},
	}
	issues := VisualContrastIssues(doc)
	if !hasWarning(issues, "low contrast against group fill") {
		t.Errorf("expected group vs child contrast warning, got: %v", issues)
	}
}

// ── 2. label truncation ───────────────────────────────────────────────────────

func TestLabelIssues_Truncation(t *testing.T) {
	// geom.W = 1, so availPx ≈ 1*40*0.9*0.95 = 34px; label "ABCDEFGHIJ" = 70px
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "p1",
						Shape: "rectangle",
						Label: "ABCDEFGHIJ", // 10 chars * 7 = 70px > 34px
						Geom:  &Geom{W: 1},
					},
				},
			},
		},
	}
	issues := labelIssues(doc)
	if !hasWarning(issues, "label may be truncated") {
		t.Errorf("expected truncation warning, got: %v", issues)
	}
}

// ── 3. label too long ────────────────────────────────────────────────────────

func TestLabelIssues_TooLong(t *testing.T) {
	longLabel := strings.Repeat("A", 41)
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "p1",
						Shape: "rectangle",
						Label: longLabel,
					},
				},
			},
		},
	}
	issues := labelIssues(doc)
	if !hasWarning(issues, "label is long") {
		t.Errorf("expected long-label warning, got: %v", issues)
	}
}

// ── 4. out-degree ≥ 5 ────────────────────────────────────────────────────────

func TestOutDegreeIssues(t *testing.T) {
	connectors := make([]*Connector, 5)
	for i := range connectors {
		connectors[i] = &Connector{From: "hub", To: "dst"}
	}
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape:      "composite",
				Connectors: connectors,
			},
		},
	}
	issues := outDegreeIssues(doc)
	if !hasWarning(issues, "outgoing connectors") {
		t.Errorf("expected out-degree warning, got: %v", issues)
	}
}

// ── 5. nesting depth > 3 ─────────────────────────────────────────────────────

func TestNestingIssues(t *testing.T) {
	// scene(depth 0) → g1(1) → g2(2) → g3(3) → leaf(4) should trigger
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{
						ID:    "g1",
						Shape: "group",
						Parts: []*CompositePart{
							{
								ID:    "g2",
								Shape: "group",
								Parts: []*CompositePart{
									{
										ID:    "g3",
										Shape: "group",
										Parts: []*CompositePart{
											{ID: "leaf", Shape: "rectangle"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	issues := nestingIssues(doc)
	if !hasWarning(issues, "nesting depth") {
		t.Errorf("expected nesting depth warning, got: %v", issues)
	}
}

// ── 6. DiffIssues ────────────────────────────────────────────────────────────

func TestDiffIssues(t *testing.T) {
	a := Issue{Severity: SeverityError, Path: "nodes.scene.parts[x].shape", Message: "unknown shape"}
	b := Issue{Severity: SeverityWarning, Path: "nodes.scene.parts[y].label", Message: "label is long"}
	c := Issue{Severity: SeverityWarning, Path: "nodes.scene.parts[z].label", Message: "label may be truncated"}

	before := []Issue{a, b}
	after := []Issue{b, c}

	diff := DiffIssues(before, after)

	if len(diff.Fixed) != 1 || diff.Fixed[0].Message != a.Message {
		t.Errorf("Fixed: expected [%v], got %v", a, diff.Fixed)
	}
	if len(diff.New) != 1 || diff.New[0].Message != c.Message {
		t.Errorf("New: expected [%v], got %v", c, diff.New)
	}
	if len(diff.Remained) != 1 || diff.Remained[0].Message != b.Message {
		t.Errorf("Remained: expected [%v], got %v", b, diff.Remained)
	}
}

// ── no false positives for DiffIssues with empty lists ───────────────────────

func TestDiffIssues_Empty(t *testing.T) {
	diff := DiffIssues(nil, nil)
	if len(diff.Fixed) != 0 || len(diff.New) != 0 || len(diff.Remained) != 0 {
		t.Errorf("unexpected non-empty diff: %v", diff)
	}
}

// ── parseHex round-trip ───────────────────────────────────────────────────────

// ── 7. styleConsistencyIssues ────────────────────────────────────────────────

func mkClusterDoc(textColors []string, icons []string, preset string) *Document {
	parts := make([]*CompositePart, len(textColors))
	for i, tc := range textColors {
		parts[i] = &CompositePart{
			ID:    fmt.Sprintf("node%d", i),
			Shape: "rectangle",
			Label: "Label",
			Icon:  icons[i],
		}
		if preset != "" {
			parts[i].Preset = preset
		} else {
			// same fill+stroke bucket when no preset
			parts[i].Style = &Style{
				Palette: &Palette{Top: "#111827"},
				Stroke:  &Stroke{Color: "#22D3EE"},
				Text:    &Text{Color: tc},
			}
		}
		if preset != "" {
			parts[i].Style = &Style{Text: &Text{Color: tc}}
		}
	}
	return &Document{
		Nodes: map[string]*Node{
			"scene": {Shape: "composite", Parts: parts},
		},
	}
}

// Three nodes share the same preset; one has a different text color → outlier.
func TestStyleConsistency_TextColorOutlier(t *testing.T) {
	doc := mkClusterDoc(
		[]string{"#ffffff", "#ffffff", "#ff0000"},
		[]string{"iso://si/github/light", "iso://si/redis/light", "iso://si/redis/light"},
		"tool",
	)
	issues := styleConsistencyIssues(doc)
	if !hasWarning(issues, "text color") {
		t.Errorf("expected text color outlier warning, got: %v", issues)
	}
}

// All nodes have the same text color → no spurious warning.
func TestStyleConsistency_NoFalsePositive(t *testing.T) {
	doc := mkClusterDoc(
		[]string{"#ffffff", "#ffffff", "#ffffff"},
		[]string{"iso://si/github/light", "iso://si/redis/light", "iso://si/postgres/light"},
		"tool",
	)
	issues := styleConsistencyIssues(doc)
	if hasWarning(issues, "text color") {
		t.Errorf("unexpected text color warning when all colors match: %v", issues)
	}
}

// Majority of cluster uses /light icons; one uses /hex → outlier.
func TestStyleConsistency_IconSuffixOutlier(t *testing.T) {
	doc := mkClusterDoc(
		[]string{"#ffffff", "#ffffff", "#ffffff"},
		[]string{"iso://si/github/light", "iso://si/redis/light", "iso://si/postgres/3BE8F0"},
		"tool",
	)
	issues := styleConsistencyIssues(doc)
	if !hasWarning(issues, "icon suffix") {
		t.Errorf("expected icon suffix outlier warning, got: %v", issues)
	}
}

// Only 1 node in a cluster → no issue (cluster too small to vote).
func TestStyleConsistency_SingletonCluster(t *testing.T) {
	doc := &Document{
		Nodes: map[string]*Node{
			"scene": {
				Shape: "composite",
				Parts: []*CompositePart{
					{ID: "a", Shape: "rectangle", Label: "A", Preset: "glass",
						Style: &Style{Text: &Text{Color: "#ff0000"}}},
				},
			},
		},
	}
	issues := styleConsistencyIssues(doc)
	if hasWarning(issues, "text color") {
		t.Errorf("unexpected warning for singleton cluster: %v", issues)
	}
}

func TestParseHex(t *testing.T) {
	tests := []struct {
		in      string
		r, g, b uint8
		ok      bool
	}{
		{"#RRGGBB", 0, 0, 0, false},
		{"#E2E8F0", 0xE2, 0xE8, 0xF0, true},
		{"#fff", 0xFF, 0xFF, 0xFF, true},
		{"#000", 0x00, 0x00, 0x00, true},
		{"rgba(1,2,3)", 0, 0, 0, false},
		{"", 0, 0, 0, false},
	}
	for _, tc := range tests {
		r, g, b, ok := parseHex(tc.in)
		if ok != tc.ok || r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("parseHex(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)", tc.in, r, g, b, ok, tc.r, tc.g, tc.b, tc.ok)
		}
	}
}
