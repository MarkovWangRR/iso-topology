package isotopo

import (
	"fmt"
	"strings"
	"testing"
)

// validateSrc parses + validates a YAML source and returns the issues, failing
// the test if it doesn't even parse.
func validateSrc(t *testing.T, src string) []Issue {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Validate(doc)
}

// findIssue returns the first issue whose path ends with suffix (and, if msg is
// non-empty, whose message contains it).
func findIssue(issues []Issue, pathSuffix, msgContains string) *Issue {
	for i := range issues {
		if strings.HasSuffix(issues[i].Path, pathSuffix) &&
			(msgContains == "" || strings.Contains(issues[i].Message, msgContains)) {
			return &issues[i]
		}
	}
	return nil
}

const twoNodeScene = `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 } }
      - { id: b, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, place: { rightOf: a, gap: 1 } }
    connectors:
      - { from: %s, to: %s, arrow: %s, routing: %s }
`

func sceneWith(from, to, arrow, routing string) string {
	return fmt.Sprintf(twoNodeScene, from, to, arrow, routing)
}

// A1 — a bad anchor suffix on a connector endpoint must be flagged with a
// suggestion, not silently rendered at top-mid.
func TestValidateConnectorAnchorTypo(t *testing.T) {
	issues := validateSrc(t, sceneWith("a.lft", "b", "triangle", "orthogonal"))
	iss := findIssue(issues, ".from", "unknown anchor")
	if iss == nil {
		t.Fatalf("expected an anchor error on .from, got %+v", issues)
	}
	if iss.Suggest != "left" {
		t.Errorf("expected suggestion 'left', got %q", iss.Suggest)
	}
}

func TestValidateConnectorAnchorValid(t *testing.T) {
	for _, anchor := range []string{"left", "right-mid", "front", "back-mid", "top", "center"} {
		issues := validateSrc(t, sceneWith("a."+anchor, "b", "triangle", "orthogonal"))
		if iss := findIssue(issues, ".from", "unknown anchor"); iss != nil {
			t.Errorf("valid anchor %q wrongly flagged: %s", anchor, iss.Message)
		}
	}
}

// A2 — arrow/routing typos flagged with suggestions.
func TestValidateConnectorEnumTypos(t *testing.T) {
	issues := validateSrc(t, sceneWith("a", "b", "triangel", "orthagonal"))
	if iss := findIssue(issues, ".arrow", "unknown arrow"); iss == nil || iss.Suggest != "triangle" {
		t.Errorf("expected arrow error suggesting triangle, got %+v", iss)
	}
	if iss := findIssue(issues, ".routing", "unknown routing"); iss == nil || iss.Suggest != "orthogonal" {
		t.Errorf("expected routing error suggesting orthogonal, got %+v", iss)
	}
	// valid values produce no enum errors.
	clean := validateSrc(t, sceneWith("a", "b", "triangle", "bezier"))
	if iss := findIssue(clean, ".arrow", "unknown"); iss != nil {
		t.Errorf("valid arrow flagged: %s", iss.Message)
	}
	if iss := findIssue(clean, ".routing", "unknown"); iss != nil {
		t.Errorf("valid routing flagged: %s", iss.Message)
	}
}

// A bad part id should NOT also produce a second anchor error (no double-fault).
func TestValidateConnectorBadIdNoDoubleAnchorError(t *testing.T) {
	issues := validateSrc(t, sceneWith("ghost.lft", "b", "triangle", "orthogonal"))
	if iss := findIssue(issues, ".from", "unknown anchor"); iss != nil {
		t.Errorf("bad id should not also raise an anchor error, got: %s", iss.Message)
	}
	if iss := findIssue(issues, ".from", "unknown part id"); iss == nil {
		t.Error("expected an unknown-part-id error for ghost")
	}
}
