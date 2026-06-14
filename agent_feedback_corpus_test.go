package isotopo

import (
	"strings"
	"testing"
)

// TestAgentMistakeCorpus is the guardrail for the product thesis: an agent's
// DSL accuracy depends on getting an actionable signal for every mistake it can
// make. Each row is a realistic agent error wrapped in an otherwise-valid
// scene; the loop must surface an issue whose message (and, where a fix exists,
// whose suggestion) points at it. A regression here means an agent would fail
// silently — the single most damaging failure mode for the platform.
//
// allFeedback mirrors what the CLI/MCP/serve paths report: structural Validate
// plus the source-level unknown-key lint.
func allFeedback(t *testing.T, src string) []Issue {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		// A hard parse error is itself a clear signal — model it as one issue.
		return []Issue{{Severity: SeverityError, Path: "$", Message: err.Error()}}
	}
	return append(Validate(doc), UnknownKeyIssues([]byte(src))...)
}

func TestAgentMistakeCorpus(t *testing.T) {
	// Each scene embeds exactly one mistake at <<>>; want* assert the signal.
	cases := []struct {
		name        string
		src         string
		wantMessage string // substring the surfaced issue must contain
		wantSuggest string // expected suggestion ("" = don't care)
	}{
		{
			name:        "unknown shape",
			src:         corpusScene(`{ id: a, shape: cilinder, geom: { w: 80, d: 80, h: 30 } }`),
			wantMessage: "unknown shape", wantSuggest: "cylinder",
		},
		{
			name:        "bad connector anchor",
			src:         corpusConn(`{ from: a.lft, to: b }`),
			wantMessage: "unknown anchor", wantSuggest: "left",
		},
		{
			name:        "bad arrow",
			src:         corpusConn(`{ from: a, to: b, arrow: triangel }`),
			wantMessage: "unknown arrow", wantSuggest: "triangle",
		},
		{
			name:        "bad routing",
			src:         corpusConn(`{ from: a, to: b, routing: orthagonal }`),
			wantMessage: "unknown routing", wantSuggest: "orthogonal",
		},
		{
			name:        "dangling connector ref",
			src:         corpusConn(`{ from: a, to: ghost }`),
			wantMessage: "unknown part id", wantSuggest: "",
		},
		{
			name:        "bad place relation",
			src:         corpusScene(`{ id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }`, `{ id: c, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, place: { below: a } }`),
			wantMessage: "no recognized relation", wantSuggest: "",
		},
		{
			name:        "misspelled geom key",
			src:         corpusScene(`{ id: a, shape: rectangle, geom: { with: 80, d: 80, h: 30 } }`),
			wantMessage: "unknown key", wantSuggest: "w",
		},
		{
			name:        "misspelled style block",
			src:         corpusScene(`{ id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, style: { palett: { top: "#fff" } } }`),
			wantMessage: "unknown key", wantSuggest: "palette",
		},
		{
			name:        "negative dimension",
			src:         corpusScene(`{ id: a, shape: rectangle, geom: { w: -5, d: 80, h: 30 } }`),
			wantMessage: "negative dimension", wantSuggest: "",
		},
		{
			name:        "unknown grid mode",
			src:         `canvas: { grid: dotz }` + "\n" + corpusScene(`{ id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }`),
			wantMessage: "unknown grid mode", wantSuggest: "dots",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := allFeedback(t, tc.src)
			var hit *Issue
			for i := range issues {
				if strings.Contains(issues[i].Message, tc.wantMessage) {
					hit = &issues[i]
					break
				}
			}
			if hit == nil {
				t.Fatalf("no feedback for mistake %q (want message containing %q)\n got: %+v",
					tc.name, tc.wantMessage, issues)
			}
			if tc.wantSuggest != "" && hit.Suggest != tc.wantSuggest {
				t.Errorf("%q: suggestion = %q, want %q", tc.name, hit.Suggest, tc.wantSuggest)
			}
		})
	}
}

// part wraps one or more part literals in a minimal valid composite scene.
func corpusScene(parts ...string) string {
	var b strings.Builder
	b.WriteString("nodes:\n  scene:\n    shape: composite\n    parts:\n")
	for _, p := range parts {
		b.WriteString("      - " + p + "\n")
	}
	return b.String()
}

// withConn builds a two-part scene plus one connector literal.
func corpusConn(conn string) string {
	return corpusScene(
		`{ id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }`,
		`{ id: b, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, place: { rightOf: a, gap: 1 } }`,
	) + "    connectors:\n      - " + conn + "\n"
}
