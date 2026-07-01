package isotopo

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestBadCaseIntake guards the issue #12 scaffolding: the bad-case issue form
// and the add-sample capture script must both be present and well-formed, so
// the "report → golden sample" pipeline stays wired up.
func TestBadCaseIntake(t *testing.T) {
	// 1. The issue form exists and parses as YAML with the required fields.
	const form = ".github/ISSUE_TEMPLATE/bad-case.yml"
	b, err := os.ReadFile(form)
	if err != nil {
		t.Fatalf("bad-case issue template missing (%s): %v", form, err)
	}
	var parsed struct {
		Name string `yaml:"name"`
		Body []struct {
			ID string `yaml:"id"`
		} `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("%s is not valid YAML: %v", form, err)
	}
	if parsed.Name == "" {
		t.Errorf("%s: missing top-level `name`", form)
	}
	hasDSL := false
	for _, f := range parsed.Body {
		if f.ID == "dsl" {
			hasDSL = true
		}
	}
	if !hasDSL {
		t.Errorf("%s: form must ask for the DSL source (a body field with id: dsl)", form)
	}

	// 2. The capture script exists and is executable.
	const script = "scripts/add-sample.sh"
	fi, err := os.Stat(script)
	if err != nil {
		t.Fatalf("capture script missing (%s): %v", script, err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("%s must be executable (chmod +x)", script)
	}
}
