package isotopo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSamplesTextContrast is the repo-wide gate from issue #5: every bundled
// sample must render its labels with legible text-vs-fill contrast. The tool's
// own validator (VisualContrastIssues) enforces a >= 3.0 ratio; this test fails
// if ANY sample regresses below it, so a low-contrast scene can never ship as an
// example again (the flagship clickhouse-hub had shipped at ratio 2.08).
//
// Scope: the "low contrast between top fill X and text Y" class (label
// legibility). The separate "nearly indistinguishable from canvas background"
// check (white-card-on-light-canvas designs that rely on borders/shadows for
// separation) is intentionally out of scope here and tracked separately.
func TestSamplesTextContrast(t *testing.T) {
	for _, category := range []string{"node", "topology"} {
		root := filepath.Join("samples", category)
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := category + "/" + e.Name()
			t.Run(name, func(t *testing.T) {
				dir := filepath.Join(root, e.Name())
				data, ext, err := readInput(dir)
				if err != nil {
					t.Skipf("no input: %v", err)
				}
				doc, err := loadGoldenDoc(ext, data)
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				for _, iss := range VisualContrastIssues(doc) {
					if strings.Contains(iss.Message, "low contrast between top fill") {
						t.Errorf("%s: %s — %s", name, iss.Path, iss.Message)
					}
				}
			})
		}
	}
}
