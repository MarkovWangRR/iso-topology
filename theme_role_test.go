package isotopo

import (
	"strings"
	"testing"
)

// TestRoleCascadePrecedence pins the resolution order:
// theme defaults < shape defaults < role < preset < part style.
func TestRoleCascadePrecedence(t *testing.T) {
	theme := &Theme{
		Style: Style{Text: &Text{Color: "#111111", Family: "base-family"}},
		Roles: map[string]*Style{
			"chip": {Text: &Text{Color: "#222222", Weight: "600"}},
		},
		Presets: map[string]*Style{
			"special": {Text: &Text{Color: "#333333"}},
		},
	}
	// Role overrides theme default; inherits family from the base.
	st := ResolveStyleWithRole(theme, "rectangle", "chip", "", nil)
	if st.Text.Color != "#222222" || st.Text.Family != "base-family" || st.Text.Weight != "600" {
		t.Errorf("role layer wrong: %+v", st.Text)
	}
	// Preset overrides role.
	st = ResolveStyleWithRole(theme, "rectangle", "chip", "special", nil)
	if st.Text.Color != "#333333" {
		t.Errorf("preset must beat role, got %s", st.Text.Color)
	}
	if st.Text.Weight != "600" {
		t.Errorf("role fields not shadowed by preset must survive, got %+v", st.Text)
	}
	// Part style overrides everything.
	st = ResolveStyleWithRole(theme, "rectangle", "chip", "special", &Style{Text: &Text{Color: "#444444"}})
	if st.Text.Color != "#444444" {
		t.Errorf("part style must beat preset, got %s", st.Text.Color)
	}
	// Role-less resolution is unchanged (API compatibility).
	st = ResolveStyle(theme, "rectangle", "", nil)
	if st.Text.Color != "#111111" {
		t.Errorf("role-less resolution changed: %s", st.Text.Color)
	}
}

// TestThemeUseResolution: `theme: { use: name }` layers the built-in under
// the user's own theme (user wins), fills the canvas only when the doc has
// none, materialises role geoms without touching authored geometry, and is
// idempotent across re-parses (the RepairSource double-parse contract).
func TestThemeUseResolution(t *testing.T) {
	src := []byte(`
theme:
  use: clean-light
  roles:
    chip: { text: { color: "#FF0000" } }
nodes:
  scene:
    shape: composite
    parts:
      - { id: hero, role: hero, label: "Core" }
      - { id: c1, role: chip, label: "A" }
      - { id: c2, role: chip, geom: { w: 300, d: 200, h: 50 }, label: "B" }
`)
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Canvas filled from the theme (doc had none).
	if doc.Canvas == nil || doc.Canvas.Background != "#F3F5F9" {
		t.Fatalf("canvas not filled from theme: %+v", doc.Canvas)
	}
	// User's role override wins over the built-in's chip text color…
	st := ResolvePartStyle(doc, "c1")
	if st.Text.Color != "#FF0000" {
		t.Errorf("user role override lost: %s", st.Text.Color)
	}
	// …while un-overridden fields still come from the built-in.
	if st.Stroke == nil || st.Stroke.Color == "" {
		t.Error("built-in chip stroke lost in merge")
	}
	// Hero role resolves from the built-in untouched.
	if hs := ResolvePartStyle(doc, "hero"); hs.Text.Color != "#FFFFFF" {
		t.Errorf("built-in hero style missing: %+v", hs.Text)
	}
	// Role geom materialised for geomless parts; authored geom untouched.
	p1, p2 := findPart(doc, "c1"), findPart(doc, "c2")
	if p1.Geom == nil || p1.Geom.W != 96 {
		t.Errorf("role geom not applied: %+v", p1.Geom)
	}
	if p2.Geom.W != 300 {
		t.Errorf("authored geom overwritten: %+v", p2.Geom)
	}
	// Idempotence: re-parsing the same source yields the same resolved doc
	// (theme resolution must not accumulate).
	doc2, _ := Parse(src)
	if ResolvePartStyle(doc2, "c1").Text.Color != "#FF0000" {
		t.Error("second parse resolved differently")
	}
}

// TestThemeCanvasRespectsAuthor: a doc that declares ANY canvas keeps it
// wholesale — the theme's canvas never partially overwrites author intent.
func TestThemeCanvasRespectsAuthor(t *testing.T) {
	doc, err := Parse([]byte(`
canvas: { background: "#123456" }
theme: { use: clickhouse-dark }
nodes:
  scene: { shape: composite, parts: [ { id: a, role: chip, label: "A" } ] }
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Canvas.Background != "#123456" {
		t.Errorf("author canvas overwritten: %s", doc.Canvas.Background)
	}
}

// TestUnknownThemeAndRoleValidation: typos surface with did-you-mean, and an
// unknown use renders unthemed rather than failing parse.
func TestUnknownThemeAndRoleValidation(t *testing.T) {
	doc, err := Parse([]byte(`
theme: { use: clickhose-dark }
nodes:
  scene: { shape: composite, parts: [ { id: a, role: cheep, label: "A" } ] }
`))
	if err != nil {
		t.Fatalf("unknown theme must not fail parse: %v", err)
	}
	issues := Validate(doc)
	var themeErr, roleWarn bool
	for _, is := range issues {
		if is.Path == "theme.use" && is.Severity == SeverityError && is.Suggest == "clickhouse-dark" {
			themeErr = true
		}
		if strings.HasSuffix(is.Path, ".role") && is.Suggest == "chip" {
			roleWarn = true
		}
	}
	if !themeErr {
		t.Errorf("unknown theme.use not reported with suggestion; issues: %+v", issues)
	}
	if !roleWarn {
		t.Errorf("unknown role not reported with suggestion; issues: %+v", issues)
	}
}

// TestBuiltinThemesWellFormed: every embedded theme parses, describes itself,
// styles all three roles with an explicit stroke (the canvas-separation
// guarantee), passes text-contrast against its own fills, and appears in
// capabilities.
func TestBuiltinThemesWellFormed(t *testing.T) {
	names := ThemeNames()
	if len(names) < 4 {
		t.Fatalf("expected >=4 built-in themes, got %v", names)
	}
	caps := CapabilityReport()
	capThemes := map[string]bool{}
	for _, tc := range caps.Themes {
		capThemes[tc.Name] = true
	}
	for _, name := range names {
		if ThemeDescription(name) == "" {
			t.Errorf("%s: missing description", name)
		}
		if !capThemes[name] {
			t.Errorf("%s: not listed in capabilities", name)
		}
		tf := builtinThemes[name]
		for _, role := range RoleNames {
			st := tf.Theme.Roles[role]
			if st == nil {
				t.Errorf("%s: role %q unstyled", name, role)
				continue
			}
			if st.Stroke == nil || st.Stroke.Color == "" {
				t.Errorf("%s/%s: no explicit stroke (canvas-separation guarantee)", name, role)
			}
		}
		// Contrast gate: a themed scene must produce zero text-contrast issues.
		doc, err := Parse([]byte(`
theme: { use: ` + name + ` }
nodes:
  scene:
    shape: composite
    parts:
      - { id: h, role: hero, label: "Core Service" }
      - id: t1
        role: tray
        shape: group
        label: "Category"
        layout: { mode: row, gap: 0.8 }
        parts:
          - { id: c1, role: chip, label: "Alpha" }
          - { id: c2, role: chip, label: "Beta" }
`))
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		for _, is := range VisualContrastIssues(doc) {
			if strings.Contains(is.Message, "low contrast between top fill") {
				t.Errorf("%s: %s", name, is.Message)
			}
		}
	}
}

// TestApplyThemeUnknown: the CLI-facing entry errors with a did-you-mean.
func TestApplyThemeUnknown(t *testing.T) {
	doc := &Document{}
	err := ApplyTheme(doc, "navvy-glass")
	if err == nil || !strings.Contains(err.Error(), "navy-glass") {
		t.Errorf("expected did-you-mean error, got %v", err)
	}
}
