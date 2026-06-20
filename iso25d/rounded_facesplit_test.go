package iso25d

import (
	"regexp"
	"strings"
	"testing"
)

// faceSplit (opt-in) turns a rounded box's single continuous side band into two
// independently-filled walls so a rounded cube shades per-face like a sharp one.
// Default off must keep the single band (no change to existing diagrams).
func roundedOpts(split bool) IsoBoxOpts {
	o := DefaultIsoBox()
	o.Width, o.Depth, o.Height = 150, 150, 110
	o.CornerRadius = 14
	o.TopFill, o.LeftFill, o.RightFill = "#FFFFFF", "#E5484D", "#3E63DD"
	o.FaceSplit = split
	return o
}

func TestRoundedFaceSplitOffSingleBand(t *testing.T) {
	svg := RenderIsoBoxRounded(roundedOpts(false))
	if !strings.Contains(svg, `data-face="side"`) {
		t.Fatalf("default rounded box must render one continuous side band")
	}
	if strings.Contains(svg, `data-face="left"`) || strings.Contains(svg, `data-face="right"`) {
		t.Fatalf("faceSplit off must NOT emit per-face left/right walls")
	}
}

func TestRoundedFaceSplitOnPerFace(t *testing.T) {
	svg := RenderIsoBoxRounded(roundedOpts(true))
	if strings.Contains(svg, `data-face="side"`) {
		t.Fatalf("faceSplit on must replace the single band with two walls")
	}
	// Each wall carries its own fill: left → LeftFill, right → RightFill.
	left := regexp.MustCompile(`data-face="left" d="[^"]*" fill="([^"]*)"`).FindStringSubmatch(svg)
	right := regexp.MustCompile(`data-face="right" d="[^"]*" fill="([^"]*)"`).FindStringSubmatch(svg)
	if left == nil || right == nil {
		t.Fatalf("expected both data-face=\"left\" and data-face=\"right\" paths")
	}
	if left[1] != "#E5484D" {
		t.Errorf("left wall fill = %q, want #E5484D (LeftFill)", left[1])
	}
	if right[1] != "#3E63DD" {
		t.Errorf("right wall fill = %q, want #3E63DD (RightFill)", right[1])
	}
}
