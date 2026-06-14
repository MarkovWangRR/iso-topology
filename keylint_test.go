package isotopo

import (
	"strings"
	"testing"
)

// E — misspelled keys (the largest silent-failure class) are flagged with the
// owning type and a nearest-field suggestion.
func TestUnknownKeyIssues(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { with: 80, d: 80, h: 30 }, style: { palett: { top: "#fff" } } }
`
	issues := UnknownKeyIssues([]byte(src))
	want := map[string]string{"with": "w", "palett": "palette"}
	for key, suggest := range want {
		var found *Issue
		for i := range issues {
			if strings.Contains(issues[i].Message, `"`+key+`"`) {
				found = &issues[i]
				break
			}
		}
		if found == nil {
			t.Errorf("expected an unknown-key warning for %q, got %+v", key, issues)
			continue
		}
		if found.Severity != SeverityWarning {
			t.Errorf("%q should be a warning, got %s", key, found.Severity)
		}
		if found.Suggest != suggest {
			t.Errorf("%q suggestion = %q, want %q", key, found.Suggest, suggest)
		}
	}
}

// A clean document produces no unknown-key noise.
func TestUnknownKeyIssuesCleanSilent(t *testing.T) {
	src := `nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, style: { palette: { top: "#fff" } } }
`
	if issues := UnknownKeyIssues([]byte(src)); len(issues) != 0 {
		t.Errorf("clean doc should produce no unknown-key warnings, got %+v", issues)
	}
}

// Non-YAML / unparseable input is a no-op (Parse owns syntax errors).
func TestUnknownKeyIssuesNonYAML(t *testing.T) {
	if issues := UnknownKeyIssues([]byte("user -> api -> db")); len(issues) != 0 {
		t.Errorf("d2-ish input should yield no key lint, got %+v", issues)
	}
}
