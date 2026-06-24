package isotopo

import "testing"

// A palette that sets only `top` must render as a flat SHADED box (sides the
// same hue darker), never sprout the engine's default blue walls (the Q2 footgun).
func TestSideFacesDerivedFromTopNotBlue(t *testing.T) {
	n := &Node{Shape: "rectangle", Style: &Style{Palette: &Palette{Top: "#EFF2F6"}}}
	_, o := Flatten(n, nil)
	if o.LeftFill == "" || o.RightFill == "" {
		t.Fatalf("sides should be derived from top, got left=%q right=%q", o.LeftFill, o.RightFill)
	}
	if o.LeftFill == "#3A6FBA" || o.RightFill == "#5589D6" {
		t.Fatalf("sides must not be the default blue, got left=%q right=%q", o.LeftFill, o.RightFill)
	}
	// Derived sides are darker than the top (lower luminance).
	lum := func(hex string) float64 {
		r, g, b, ok := parseHex6(hex)
		if !ok {
			t.Fatalf("unparseable derived face %q", hex)
		}
		return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
	}
	if lum(o.LeftFill) >= lum("#EFF2F6") || lum(o.RightFill) >= lum("#EFF2F6") {
		t.Fatalf("derived sides must be darker than top, got left=%q right=%q", o.LeftFill, o.RightFill)
	}
}

// An explicit side fill or a side gradient must NOT be overwritten by derivation.
func TestExplicitSidesWin(t *testing.T) {
	n := &Node{Shape: "rectangle", Style: &Style{Palette: &Palette{Top: "#FFFFFF", Left: "#111111", Right: "#222222"}}}
	_, o := Flatten(n, nil)
	if o.LeftFill != "#111111" || o.RightFill != "#222222" {
		t.Fatalf("explicit sides must be preserved, got left=%q right=%q", o.LeftFill, o.RightFill)
	}
}

// Alpha tops (#RRGGBBAA) still derive (alpha dropped for the opaque side wall).
func TestAlphaTopDerives(t *testing.T) {
	n := &Node{Shape: "rectangle", Style: &Style{Palette: &Palette{Top: "#FFFFFFCC"}}}
	_, o := Flatten(n, nil)
	if o.LeftFill == "" || o.RightFill == "" {
		t.Fatalf("alpha top should still derive sides, got left=%q right=%q", o.LeftFill, o.RightFill)
	}
}
