package iso25d

import (
	"strings"
	"testing"
)

// The adaptive contract: icons never overflow the face; labels wrap
// at word boundaries and shrink before any mid-word break; content
// that already fits is left byte-identical (Adjusted=false).

func TestFitLeavesFittingContentAlone(t *testing.T) {
	fit := fitTopContent(140, 140, []string{"GPU Pool"}, true, 0.4, 10)
	if fit.Adjusted {
		t.Fatalf("fitting content must not be adjusted")
	}
	if fit.FontSize != 10 || fit.IconSize != 56 {
		t.Fatalf("legacy values must pass through, got fs=%v icon=%v", fit.FontSize, fit.IconSize)
	}
}

func TestFitShrinksSingleWordInsteadOfBreaking(t *testing.T) {
	// "Database" at 16 doesn't fit a ~69-wide face; it must shrink,
	// not split mid-word.
	fit := fitTopContent(69.28, 69.28, []string{"Database"}, false, 0, 16)
	if !fit.Adjusted {
		t.Fatalf("expected adjustment")
	}
	if len(fit.Lines) != 1 || fit.Lines[0] != "Database" {
		t.Fatalf("single word must stay whole, got %q", fit.Lines)
	}
	if fit.FontSize >= 16 {
		t.Fatalf("font must shrink, got %v", fit.FontSize)
	}
	if lineWidth(fit.Lines[0], fit.FontSize) > 69.28-2*6 {
		t.Fatalf("still overflows after shrink")
	}
}

func TestFitWrapsAtWordBoundaries(t *testing.T) {
	fit := fitTopContent(120, 120, []string{"Customer Facing Dashboards"}, false, 0, 11)
	if !fit.Adjusted {
		t.Fatalf("expected adjustment")
	}
	for _, ln := range fit.Lines {
		if strings.Contains(ln, "Customer Facing Dashboards") {
			t.Fatalf("line not wrapped: %q", ln)
		}
		if lineWidth(ln, fit.FontSize) > 120-2*8.4+0.01 {
			t.Fatalf("wrapped line %q overflows", ln)
		}
	}
	if len(fit.Lines) < 2 {
		t.Fatalf("expected multiple lines, got %v", fit.Lines)
	}
}

func TestFitClampsIconOnTinyFace(t *testing.T) {
	fit := fitTopContent(40, 40, nil, true, 0.9, 10)
	if !fit.Adjusted {
		t.Fatalf("oversized icon must be adjusted")
	}
	if fit.IconSize > 40-2*6 {
		t.Fatalf("icon %v overflows 40-face", fit.IconSize)
	}
}

func TestFitIconYieldsHeightToText(t *testing.T) {
	long := []string{"Extremely Long Service Name That Wraps A Lot Of Times"}
	fit := fitTopContent(80, 80, long, true, 0.5, 12)
	if !fit.Adjusted {
		t.Fatalf("expected adjustment")
	}
	pad := 6.0
	blockBottom := fit.FirstLineY + float64(len(fit.Lines)-1)*fit.FontSize*lineHeightK + fit.FontSize*0.6
	if fit.IconTop < pad-0.01 || blockBottom > 80-pad+0.01 {
		t.Fatalf("block escapes face: iconTop=%v bottom=%v", fit.IconTop, blockBottom)
	}
}

func TestFitCJKWrapsByRune(t *testing.T) {
	fit := fitTopContent(100, 100, []string{"实时推理服务网关"}, false, 0, 14)
	if !fit.Adjusted {
		t.Fatalf("expected adjustment")
	}
	for _, ln := range fit.Lines {
		if lineWidth(ln, fit.FontSize) > 100-2*7+0.01 {
			t.Fatalf("CJK line %q overflows", ln)
		}
	}
}

func TestIconCatalogComplete(t *testing.T) {
	cat := IconCatalog()
	glyphs, brands, si := 0, 0, 0
	for _, ic := range cat {
		if ic.Description == "" {
			t.Fatalf("icon %s/%s has no description — add it to glyphDescs", ic.Kind, ic.Name)
		}
		if !strings.HasPrefix(ic.SVG, "<svg") {
			t.Fatalf("icon %s/%s has no renderable SVG", ic.Kind, ic.Name)
		}
		switch ic.Kind {
		case "glyph":
			glyphs++
		case "brand":
			brands++
		case "si":
			si++
		}
	}
	if glyphs != len(glyphIcons) || brands != len(brandBadges) || si != len(siIcons) {
		t.Fatalf("catalog incomplete: %d/%d glyphs, %d/%d brands, %d/%d si",
			glyphs, len(glyphIcons), brands, len(brandBadges), si, len(siIcons))
	}
	if si < 100 {
		t.Fatalf("Simple Icons pack suspiciously small: %d", si)
	}
}
