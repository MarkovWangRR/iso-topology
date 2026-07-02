package isotopo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSVGIntrinsicSize: the chrome fallback screenshots at the SVG's declared
// size — reading it wrong would crop or letterbox the capture.
func TestSVGIntrinsicSize(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.svg")
	if err := os.WriteFile(p, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="-41 -12 582 364" width="582" height="364"></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	w, h, err := svgIntrinsicSize(p)
	if err != nil || w != 582 || h != 364 {
		t.Fatalf("got %dx%d err=%v; want 582x364", w, h, err)
	}
	if err := os.WriteFile(p, []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svgIntrinsicSize(p); err == nil {
		t.Error("dimensionless svg must error, not guess")
	}
}

// TestRasterizeEndToEnd renders a real scene and rasterizes it with whatever
// backend this machine has (resvg / magick / chrome); skipped when none exists
// — but the error must then enumerate every install option, because a dead-end
// error is the exact friction Rasterize exists to remove.
func TestRasterizeEndToEnd(t *testing.T) {
	doc, err := Parse([]byte(`
nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, label: "A" }
`))
	if err != nil {
		t.Fatal(err)
	}
	svg := RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "t.svg")
	pngPath := filepath.Join(dir, "t.png")
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rasterize(svgPath, pngPath); err != nil {
		for _, opt := range []string{"resvg", "ImageMagick", "Chrome", "ISOTOPO_RASTERIZER"} {
			if !strings.Contains(err.Error(), opt) {
				t.Errorf("failure message must mention %s; got: %v", opt, err)
			}
		}
		t.Skipf("no rasterizer on this machine: %v", err)
	}
	info, err := os.Stat(pngPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("rasterize produced no png: %v", err)
	}
	// PNG magic.
	b, _ := os.ReadFile(pngPath)
	if len(b) < 8 || string(b[1:4]) != "PNG" {
		t.Error("output is not a png")
	}
}
