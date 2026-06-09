// Package isotopo's golden-file test suite.
//
// Every subdirectory of testdata/ is a self-contained example:
//
//	testdata/<name>/
//	├── input.{yaml|json|d2}   ← the source DSL
//	└── expected.svg           ← the frozen "golden" topology.svg
//
// The test renders input through the same pipeline as the `isotopo
// render` CLI and asserts the produced topology.svg matches the
// golden BYTE-FOR-BYTE.
//
// To intentionally update goldens after a deliberate change:
//
//	go test -update
//
// This is the agent's safety net — any change to render.go /
// translate.go / iso25d/* that drifts output gets caught here and
// the agent decides per-case whether the drift is intended.
package isotopo

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate testdata/*/expected.svg")

func TestGolden(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", name)
			data, ext, err := readInput(dir)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := loadGoldenDoc(ext, data)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			got := renderTopology(doc)

			goldenPath := filepath.Join(dir, "expected.svg")
			if *updateGolden {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal([]byte(got), want) {
				// Dump the actual output next to the golden so a human (or
				// agent) can `diff` / open both in a browser to inspect.
				actualPath := filepath.Join(dir, "actual.svg")
				_ = os.WriteFile(actualPath, []byte(got), 0o644)
				t.Errorf("topology.svg drift\n  golden: %s (%d bytes)\n  actual: %s (%d bytes)\nrun: go test -update -run %q to accept",
					goldenPath, len(want), actualPath, len(got), name)
			}
		})
	}
}

// readInput finds the single input.* file in dir and returns its bytes
// plus the lowercase extension (without leading dot).
func readInput(dir string) ([]byte, string, error) {
	for _, ext := range []string{"yaml", "d2", "json"} {
		p := filepath.Join(dir, "input."+ext)
		if data, err := os.ReadFile(p); err == nil {
			return data, ext, nil
		}
	}
	return nil, "", &os.PathError{Op: "read", Path: filepath.Join(dir, "input.*"), Err: os.ErrNotExist}
}

// loadGoldenDoc routes by extension like the CLI does (kept in-test so
// we don't depend on package main).
func loadGoldenDoc(ext string, data []byte) (*Document, error) {
	switch strings.ToLower(ext) {
	case "d2":
		diag, err := CompileD2(context.Background(), string(data), LayoutDagre)
		if err != nil {
			return nil, err
		}
		return Translate(diag, DefaultRenderOpts()), nil
	default:
		return Parse(data)
	}
}

// renderTopology mirrors the CLI's scene resolution.
func renderTopology(doc *Document) string {
	if scene := doc.Scene(); scene != nil {
		return RenderWithCanvas(scene, doc.Theme, doc.Canvas, doc.Annotations)
	}
	return ""
}
