package isotopo

import (
	"math"
	"strings"
	"testing"
)

// Solver unit tests — the golden suite catches drift end-to-end; these
// pin the layout algebra itself (positions, auto-size, error paths) so
// a future change can't silently re-interpret a relation.

func fp(v float64) *float64 { return &v }

func part(id string, w, d, h float64) *CompositePart {
	return &CompositePart{ID: id, Shape: "rectangle", Geom: &Geom{W: w, D: d, H: h}}
}

func solveScene(t *testing.T, parts []*CompositePart) []Issue {
	t.Helper()
	n := &Node{Shape: "composite", Parts: parts}
	return applyLayout(n, &Canvas{GridStep: 40})
}

func at(t *testing.T, p *CompositePart, wx, wy, wz float64) {
	t.Helper()
	gx, gy, gz := 0.0, 0.0, 0.0
	if p.Offset != nil {
		gx, gy, gz = p.Offset.WX, p.Offset.WY, p.Offset.WZ
	}
	if math.Abs(gx-wx) > 0.01 || math.Abs(gy-wy) > 0.01 || math.Abs(gz-wz) > 0.01 {
		t.Fatalf("%s at (%.1f, %.1f, %.1f), want (%.1f, %.1f, %.1f)", p.ID, gx, gy, gz, wx, wy, wz)
	}
}

func TestPlaceRightOfAlignsCenter(t *testing.T) {
	a := part("a", 100, 100, 40)
	b := part("b", 60, 60, 20)
	b.Place = &Place{RightOf: "a", Gap: fp(2)}
	issues := solveScene(t, []*CompositePart{a, b})
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	// x = 100 + 2*40; y centers within a's depth: (100-60)/2.
	at(t, b, 180, 20, 0)
}

func TestPlacePerAxisGaps(t *testing.T) {
	a := part("a", 100, 100, 40)
	b := part("b", 100, 100, 40)
	b.Place = &Place{RightOf: "a", InFrontOf: "a", Gap: fp(1), GapX: fp(3), GapY: fp(0)}
	solveScene(t, []*CompositePart{a, b})
	at(t, b, 100+120, 100+0, 0)
}

func TestPlaceAboveSitsFlushAndCenters(t *testing.T) {
	base := part("base", 120, 120, 50)
	top := part("top", 60, 60, 20)
	top.Place = &Place{Above: "base"}
	issues := solveScene(t, []*CompositePart{base, top})
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	// z = base height; x/y centered on base footprint.
	at(t, top, 30, 30, 50)
}

func TestPlaceAboveChainsAndKeepsDelta(t *testing.T) {
	base := part("base", 100, 100, 40)
	mid := part("mid", 100, 100, 30)
	mid.Place = &Place{Above: "base"}
	top := part("top", 100, 100, 10)
	top.Place = &Place{Above: "mid"}
	top.Offset = &WorldPoint{WZ: 5} // author delta stacks on the solved z
	solveScene(t, []*CompositePart{base, mid, top})
	at(t, mid, 0, 0, 40)
	at(t, top, 0, 0, 75)
}

func TestPlaceDanglingRefSuggests(t *testing.T) {
	a := part("a", 100, 100, 40)
	b := part("b", 100, 100, 40)
	b.Place = &Place{RightOf: "aa"}
	issues := solveScene(t, []*CompositePart{a, b})
	if len(issues) == 0 || issues[0].Severity != SeverityError || issues[0].Suggest != "a" {
		t.Fatalf("want dangling-ref error suggesting %q, got %v", "a", issues)
	}
}

func TestPlaceCycleErrors(t *testing.T) {
	a := part("a", 100, 100, 40)
	b := part("b", 100, 100, 40)
	a.Place = &Place{RightOf: "b"}
	b.Place = &Place{RightOf: "a"}
	issues := solveScene(t, []*CompositePart{a, b})
	found := false
	for _, i := range issues {
		if i.Severity == SeverityError && strings.Contains(i.Message, "cycle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want cycle error, got %v", issues)
	}
}

func TestLayoutRowAutosizesGroup(t *testing.T) {
	g := &CompositePart{
		ID: "g", Shape: "group",
		Layout: &Layout{Mode: "row", Gap: fp(1)},
		Parts:  []*CompositePart{part("c1", 80, 80, 10), part("c2", 80, 120, 10)},
	}
	solveScene(t, []*CompositePart{g})
	// padding defaults to gap (1 cell = 40): content 80+40+80 wide,
	// 120 deep → group 280 × 200.
	if g.Geom.W != 280 || g.Geom.D != 200 {
		t.Fatalf("group autosize = %v × %v, want 280 × 200", g.Geom.W, g.Geom.D)
	}
	// c1 center-aligned in the 120 track: y = 40 + 20.
	at(t, g.Parts[0], 40, 60, 0)
	at(t, g.Parts[1], 160, 40, 0)
}

func TestLayoutRingGeometry(t *testing.T) {
	hub := part("hub", 100, 100, 40)
	sats := []*CompositePart{
		part("s1", 60, 60, 10), part("s2", 60, 60, 10),
		part("s3", 60, 60, 10), part("s4", 60, 60, 10),
	}
	n := &Node{
		Shape:  "composite",
		Layout: &Layout{Mode: "ring", Gap: fp(1)},
		Parts:  append([]*CompositePart{hub}, sats...),
	}
	if issues := applyLayout(n, &Canvas{GridStep: 40}); len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	// Radius = (100+60)/2 + 40 = 120. First satellite is straight back
	// (-y) of the hub centre; all four sit at exactly R from it.
	hubCx := hub.Offset.WX + 50
	hubCy := hub.Offset.WY + 50
	for i, s := range sats {
		cx := s.Offset.WX + 30
		cy := s.Offset.WY + 30
		dist := math.Hypot(cx-hubCx, cy-hubCy)
		if math.Abs(dist-120) > 0.01 {
			t.Fatalf("satellite %d at distance %.2f, want 120", i+1, dist)
		}
	}
	if math.Abs((sats[0].Offset.WY+30)-(hubCy-120)) > 0.01 {
		t.Fatalf("first satellite should sit straight behind the hub")
	}
}

func TestOverlapWarningNamesPair(t *testing.T) {
	a := part("a", 100, 100, 40)
	b := part("b", 100, 100, 40)
	b.Place = &Place{RightOf: "a", Gap: fp(0)}
	b.Offset = &WorldPoint{WX: -50} // delta drags b back over a
	issues := solveScene(t, []*CompositePart{a, b})
	found := false
	for _, i := range issues {
		if i.Severity == SeverityWarning && strings.Contains(i.Message, `"b" overlaps sibling "a"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("want overlap warning naming the pair, got %v", issues)
	}
}

func TestPureOffsetDocumentsUntouched(t *testing.T) {
	a := part("a", 100, 100, 40)
	a.Offset = &WorldPoint{WX: 123, WY: 456}
	solveScene(t, []*CompositePart{a})
	at(t, a, 123, 456, 0)
}

// ── v2.5 preset cascade ──────────────────────────────────────────────

func TestPresetCascade(t *testing.T) {
	w := 2.0
	theme := &Theme{
		Style: Style{Palette: &Palette{Top: "#AAA", Left: "#BBB"}},
		Presets: map[string]*Style{
			"hero": {
				Palette: &Palette{Top: "#FFF"},
				Stroke:  &Stroke{Color: "#111", Width: &w},
			},
		},
	}
	// preset overrides theme; node style overrides preset.
	got := ResolveStyle(theme, "rectangle", "hero", &Style{Stroke: &Stroke{Color: "#222"}})
	if got.Palette.Top != "#FFF" || got.Palette.Left != "#BBB" {
		t.Fatalf("palette merge wrong: %+v", got.Palette)
	}
	if got.Stroke.Color != "#222" || got.Stroke.Width == nil || *got.Stroke.Width != 2 {
		t.Fatalf("stroke merge wrong: %+v", got.Stroke)
	}
}

func TestValidateUnknownPresetSuggests(t *testing.T) {
	doc := &Document{
		Theme: &Theme{Presets: map[string]*Style{"satellite": {}}},
		Nodes: map[string]*Node{
			"scene": {Shape: "composite", Parts: []*CompositePart{
				{ID: "a", Shape: "rectangle", Preset: "satelite"},
			}},
		},
	}
	issues := Validate(doc)
	found := false
	for _, i := range issues {
		if i.Severity == SeverityError && i.Suggest == "satellite" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want unknown-preset error suggesting satellite, got %v", issues)
	}
}
