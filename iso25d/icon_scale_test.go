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

// TestIconOnlyLogoIsLarger guards the icon-only enlargement: a node with an icon
// and NO label renders its logo larger than the same-size node with a label,
// because there is no text sharing the face. Uses a small chip-sized face like
// the brand-logo tiles in dense scenes.
func TestIconOnlyLogoIsLarger(t *testing.T) {
	base := DefaultIsoBox()
	base.Width, base.Depth, base.Height = 66, 60, 18
	base.Icon = "iso://si/github"

	iconOnly := base
	iconOnly.Label = ""
	withLabel := base
	withLabel.Label = "GitHub"

	wOnly := iconImageWidth(t, RenderIsoBox(iconOnly))
	wLabel := iconImageWidth(t, RenderIsoBox(withLabel))
	if wOnly <= wLabel {
		t.Errorf("icon-only logo (%.1f) should be larger than icon+label logo (%.1f)", wOnly, wLabel)
	}
	// Should reflect the icon-only scale (0.72*60 = 43.2), clearly above the
	// 0.55 default (33).
	if wOnly < 40 {
		t.Errorf("icon-only logo width %.1f too small on a 66x60 chip; want >= 40", wOnly)
	}
}

func iconImageWidth(t *testing.T, svg string) float64 {
	t.Helper()
	m := regexp.MustCompile(`<image[^>]*width="([0-9.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatal("no icon <image> rendered")
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}
