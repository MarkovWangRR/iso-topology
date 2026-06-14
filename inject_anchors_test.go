package isotopo

import (
	"math"
	"testing"
)

// Two unit-square parts side by side: a at origin, b two units to the +x.
func testResolver() *anchorResolver {
	infos := []partInfo{
		{id: "a", shape: "rectangle", w: 10, d: 10, h: 10, offWX: 0, offWY: 0, offWZ: 0},
		{id: "b", shape: "rectangle", w: 10, d: 10, h: 10, offWX: 30, offWY: 0, offWZ: 0},
	}
	return newAnchorResolver(infos, 100, 50)
}

func TestAnchorParse(t *testing.T) {
	ar := testResolver()
	if id, an := ar.parse("a"); id != "a" || an != "top-mid" {
		t.Errorf(`parse("a") = %q,%q; want a,top-mid`, id, an)
	}
	if id, an := ar.parse("a.left"); id != "a" || an != "left" {
		t.Errorf(`parse("a.left") = %q,%q; want a,left`, id, an)
	}
}

func TestAnchorWorld(t *testing.T) {
	ar := testResolver()
	// bare ref → top-face centre (wz = offWZ + h)
	wx, wy, wz, ok := ar.world("a")
	if !ok || wx != 5 || wy != 5 || wz != 10 {
		t.Errorf(`world("a") = %v,%v,%v,%v; want 5,5,10,true`, wx, wy, wz, ok)
	}
	// left face → x at the left edge, y at mid-depth
	wx, wy, _, _ = ar.world("a.left")
	if wx != 0 || wy != 5 {
		t.Errorf(`world("a.left") x,y = %v,%v; want 0,5`, wx, wy)
	}
	if _, _, _, ok := ar.world("ghost"); ok {
		t.Error("unknown id should resolve not-ok")
	}
}

func TestAnchorExit(t *testing.T) {
	ar := testResolver()
	cases := map[string][2]float64{
		"a.left": {-1, 0}, "a.right": {1, 0}, "a.back": {0, -1}, "a.front": {0, 1},
		"a": {1, 0}, // top has no horizontal normal → x-axis fallback
	}
	for ref, want := range cases {
		dx, dy := ar.exit(ref)
		if dx != want[0] || dy != want[1] {
			t.Errorf("exit(%q) = %v,%v; want %v", ref, dx, dy, want)
		}
	}
}

func TestAnchorAutoFacesOtherEndpoint(t *testing.T) {
	ar := testResolver()
	// b sits to a's +x, so a's bare ref auto-resolves to its right face,
	// and b's to its left face — they point AT each other.
	if got := ar.auto("a", "b"); got != "a.right" {
		t.Errorf(`auto("a","b") = %q; want a.right`, got)
	}
	if got := ar.auto("b", "a"); got != "b.left" {
		t.Errorf(`auto("b","a") = %q; want b.left`, got)
	}
	// an explicit anchor is left untouched.
	if got := ar.auto("a.front", "b"); got != "a.front" {
		t.Errorf(`auto should not override an explicit anchor, got %q`, got)
	}
}

func TestAnchorSideKeyNormalises(t *testing.T) {
	ar := testResolver()
	// "left" and "left-mid" must collapse to one fan-out bucket.
	if ar.sideKey("a.left") != ar.sideKey("a.left-mid") {
		t.Error("left / left-mid should share a side key")
	}
	if ar.sideKey("a.top") != "a/top" {
		t.Errorf("sideKey = %q; want a/top", ar.sideKey("a.top"))
	}
}

func TestAnchorScreenTranslates(t *testing.T) {
	ar := testResolver()
	// screen() applies the iso projection then the (tx,ty)=(100,50) origin.
	wx, wy, wz, _ := ar.world("a")
	px, py := projectIso(wx, wy, wz)
	sx, sy, ok := ar.screen("a")
	if !ok || math.Abs(sx-(px+100)) > 1e-9 || math.Abs(sy-(py+50)) > 1e-9 {
		t.Errorf("screen(a) = %v,%v; want %v,%v", sx, sy, px+100, py+50)
	}
}
