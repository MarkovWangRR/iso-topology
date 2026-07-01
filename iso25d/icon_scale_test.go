package iso25d

import (
	"regexp"
	"strconv"
	"testing"
)

// TestDefaultIconScaleLegible guards issue #7: the default top-face icon must
// render large enough to be identifiable. On a 140-unit face the icon width is
// face*scale; at the historical 0.4 that was 56, too small for brand logos.
// The raised default (0.55) yields 77. Assert the rendered icon is comfortably
// above the old size so a regression back toward 0.4 fails here.
func TestDefaultIconScaleLegible(t *testing.T) {
	if defaultIconScale < 0.5 {
		t.Fatalf("defaultIconScale regressed to %.2f; want >= 0.5 for legible icons", defaultIconScale)
	}
	o := DefaultIsoBox()
	o.Icon = "iso://si/github"
	o.Label = ""
	svg := RenderIsoBox(o) // 140x140 face
	m := regexp.MustCompile(`<image[^>]*width="([0-9.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatal("no icon <image> rendered")
	}
	w, _ := strconv.ParseFloat(m[1], 64)
	// 0.4*140 = 56 (old). Require clearly bigger than the old default.
	if w < 70 {
		t.Errorf("default icon width %.1f on a 140 face is too small (old default was 56); want >= 70", w)
	}
}
