package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Render's exit code must be trustworthy for an agent gating on it:
// empty scenes and validation errors fail (code 3), warnings do not,
// and a clean scene succeeds (code 0). See renderFile's contract.
func TestRenderFileExitCode(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want int
	}{
		{
			name: "empty_scene",
			yaml: "nodes: {}\n",
			want: 3, // produced no scene — must not be a silent exit 0
		},
		{
			name: "validation_error",
			yaml: "nodes:\n  scene:\n    shape: composite\n    parts:\n      - {id: a, shape: notarealshape}\n",
			want: 3, // unknown shape is an error — must not be a silent exit 0
		},
		{
			name: "valid_scene",
			yaml: "nodes:\n  scene:\n    shape: composite\n    parts:\n      - {id: a, shape: rectangle, label: A}\n      - {id: b, shape: cylinder, label: B, place: {rightOf: a}}\n",
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in := filepath.Join(dir, "scene.yaml")
			if err := os.WriteFile(in, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			code, err := renderFile(in, filepath.Join(dir, "out"))
			if err != nil {
				t.Fatalf("renderFile returned I/O error: %v", err)
			}
			if code != tc.want {
				t.Errorf("exit code = %d, want %d", code, tc.want)
			}
		})
	}
}

// A scene that only triggers warnings (e.g. an over-long label) must
// still render successfully — warnings never fail the exit code.
func TestRenderFileWarningStaysZero(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "scene.yaml")
	yaml := "nodes:\n  scene:\n    shape: composite\n    parts:\n      - {id: a, shape: rectangle, label: \"this is a very very very long label that should warn\"}\n"
	if err := os.WriteFile(in, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	code, err := renderFile(in, filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("renderFile returned I/O error: %v", err)
	}
	if code != 0 {
		t.Errorf("warning-only scene exit code = %d, want 0", code)
	}
}
