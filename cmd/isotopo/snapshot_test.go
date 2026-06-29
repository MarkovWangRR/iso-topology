package main

import (
	"bytes"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
)

func hasRasterizer() bool {
	for _, t := range []string{"resvg", "magick"} {
		if _, err := exec.LookPath(t); err == nil {
			return true
		}
	}
	return false
}

var svgWHRe = regexp.MustCompile(`<svg[^>]*\bwidth="(\d+)"[^>]*\bheight="(\d+)"`)

// TestSnapshot_FaithfulDeterministic is the L3 gate (agent-loop-harness-plan):
// the snapshot PNG must match the SVG's intrinsic dimensions exactly (viewport
// == viewBox, NO trim — so geometry is preserved 1:1) and be byte-identical
// across runs (deterministic). Skipped where no rasterizer is installed.
func TestSnapshot_FaithfulDeterministic(t *testing.T) {
	if !hasRasterizer() {
		t.Skip("no resvg/magick on PATH")
	}
	in := "../../samples/bench/good-grid.yaml"
	if _, err := os.Stat(in); err != nil {
		t.Skip("sample missing")
	}
	d1, d2 := t.TempDir(), t.TempDir()
	if code, err := snapshotFile(in, d1); err != nil || code != 0 {
		t.Fatalf("snapshot: code=%d err=%v", code, err)
	}
	if code, err := snapshotFile(in, d2); err != nil || code != 0 {
		t.Fatalf("snapshot(2): code=%d err=%v", code, err)
	}

	svg, err := os.ReadFile(d1 + "/topology.svg")
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}
	m := svgWHRe.FindStringSubmatch(string(svg))
	if m == nil {
		t.Fatal("svg root has no width/height")
	}
	sw, _ := strconv.Atoi(m[1])
	sh, _ := strconv.Atoi(m[2])

	f, err := os.Open(d1 + "/topology.png")
	if err != nil {
		t.Fatalf("open png: %v", err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if cfg.Width != sw || cfg.Height != sh {
		t.Fatalf("snapshot not faithful: PNG %dx%d != SVG %dx%d (trim/scale)", cfg.Width, cfg.Height, sw, sh)
	}

	a, _ := os.ReadFile(d1 + "/topology.png")
	b, _ := os.ReadFile(d2 + "/topology.png")
	if !bytes.Equal(a, b) {
		t.Fatal("snapshot not deterministic (byte mismatch across runs)")
	}
}
