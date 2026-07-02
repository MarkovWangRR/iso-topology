package isotopo

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Built-in named themes — the gallery looks distilled into reusable design
// systems. A scene opts in with `theme: { use: <name> }` (or the CLI's
// --theme flag) and gets a complete role system (hero / tray / chip), text
// defaults, sizing rhythm, and — when the doc declares no canvas — the
// matching canvas. Everything the user's own theme block sets overrides the
// built-in, so `use:` is a base layer, never a straitjacket.
//
//go:embed themes/*.yaml
var themeFS embed.FS

// themeFile is the on-disk shape of a built-in theme: the theme proper plus
// the canvas that completes the look (used only when the doc has none).
type themeFile struct {
	Description string  `yaml:"description"`
	Canvas      *Canvas `yaml:"canvas"`
	Theme       *Theme  `yaml:"theme"`
}

// RoleNames are the semantic slots themes style. Kept deliberately small:
//
//	hero — the focal element the diagram is about (one per scene)
//	tray — a category container (group/boundary) holding related parts
//	chip — a compact member tile inside a tray (logo/label card)
var RoleNames = []string{"hero", "tray", "chip"}

var builtinThemes = loadBuiltinThemes()

func loadBuiltinThemes() map[string]*themeFile {
	out := map[string]*themeFile{}
	entries, err := themeFS.ReadDir("themes")
	if err != nil {
		panic(fmt.Sprintf("isotopo: embedded themes unreadable: %v", err))
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".yaml")
		data, err := themeFS.ReadFile("themes/" + e.Name())
		if err != nil {
			panic(fmt.Sprintf("isotopo: embedded theme %s unreadable: %v", name, err))
		}
		tf := &themeFile{}
		if err := yaml.Unmarshal(data, tf); err != nil {
			panic(fmt.Sprintf("isotopo: embedded theme %s does not parse: %v", name, err))
		}
		out[name] = tf
	}
	return out
}

// ThemeNames lists the built-in themes, sorted, for capabilities/docs/agents.
func ThemeNames() []string {
	names := make([]string, 0, len(builtinThemes))
	for n := range builtinThemes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ThemeDescription returns the one-line description of a built-in theme.
func ThemeDescription(name string) string {
	if tf, ok := builtinThemes[name]; ok {
		return tf.Description
	}
	return ""
}

// resolveThemeUse layers the named built-in theme UNDER the document's own
// theme block and fills the canvas when the doc declares none. Runs at parse
// time so every consumer (render, validate, evaluate, edit ops) sees the same
// resolved document. An unknown name is left in place — Validate reports it
// with a did-you-mean, and the scene renders unthemed rather than failing.
func resolveThemeUse(doc *Document) {
	if doc == nil || doc.Theme == nil || doc.Theme.Use == "" {
		return
	}
	tf, ok := builtinThemes[doc.Theme.Use]
	if !ok {
		return // surfaced by Validate
	}
	doc.Theme = mergeThemes(tf.Theme, doc.Theme)
	if doc.Canvas == nil && tf.Canvas != nil {
		cv := *tf.Canvas
		doc.Canvas = &cv
	}
}

// mergeThemes layers `over` (the user's theme) on top of `base` (the named
// built-in): style fields merge with user-wins precedence, maps merge per key
// with user entries winning. Use is preserved so Validate/agents can still
// see which base was requested (idempotent: re-merging is a no-op because the
// merged theme already contains the base).
func mergeThemes(base, over *Theme) *Theme {
	if base == nil {
		return over
	}
	if over == nil {
		cp := *base
		return &cp
	}
	out := &Theme{Use: over.Use}
	if m := MergeStyle(&base.Style, &over.Style); m != nil {
		out.Style = *m
	}
	out.Shapes = mergeStyleMap(base.Shapes, over.Shapes)
	out.Presets = mergeStyleMap(base.Presets, over.Presets)
	out.Roles = mergeStyleMap(base.Roles, over.Roles)
	out.RoleGeoms = mergeGeomMap(base.RoleGeoms, over.RoleGeoms)
	return out
}

func mergeStyleMap(base, over map[string]*Style) map[string]*Style {
	if base == nil && over == nil {
		return nil
	}
	out := map[string]*Style{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if prev, ok := out[k]; ok {
			out[k] = MergeStyle(prev, v)
			continue
		}
		out[k] = v
	}
	return out
}

func mergeGeomMap(base, over map[string]*Geom) map[string]*Geom {
	if base == nil && over == nil {
		return nil
	}
	out := map[string]*Geom{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// applyRoleDefaults materialises what a role implies, at parse time, so
// layout, validation, and rendering all see the same document:
//
//   - shape: a role-carrying part needs no `shape:` — hero/chip default to
//     rectangle, tray to group — so a topology can be pure ids+roles+labels;
//   - geom: the theme's sizing rhythm (theme.roleGeoms) fills parts that
//     declare no geometry of their own — authored geometry is never touched;
//   - icon ink: a built-in icon with no color suffix is invisible on a dark
//     top face (the exact rule VisualContrastIssues warns about); role parts
//     opt into the design system, so the suffix is added for them
//     automatically and the same topology re-skins across light and dark
//     themes without editing every icon reference.
func applyRoleDefaults(doc *Document) {
	if doc == nil || doc.Theme == nil {
		return
	}
	theme := doc.Theme
	canInfer := theme.Roles["chip"] != nil && theme.Roles["tray"] != nil
	var walk func(parts []*CompositePart)
	walk = func(parts []*CompositePart) {
		for _, p := range parts {
			if p == nil {
				continue
			}
			// Role inference: a part with NO authored styling (no role, no
			// preset, no hand-written style) under a role-carrying theme is
			// better served by the design system than by engine defaults —
			// containers read as trays, leaves as chips. Translator-generated
			// styles (the .d2 path fabricates fill/stroke for every shape)
			// count as unstyled and are displaced, so `--theme` restyles a
			// plain d2 graph completely; any authored style still wins.
			if canInfer && p.Role == "" && p.Preset == "" && (p.Style == nil || p.styleGenerated) {
				p.Style = nil
				p.styleGenerated = false
				if isContainerShape(p.Shape) {
					p.Role = "tray"
				} else {
					p.Role = "chip"
				}
			}
			if p.Role != "" {
				if p.Shape == "" {
					if p.Role == "tray" {
						p.Shape = "group"
					} else {
						p.Shape = "rectangle"
					}
				}
				if p.Geom == nil && !isContainerShape(p.Shape) {
					if g := theme.RoleGeoms[p.Role]; g != nil {
						cp := *g
						p.Geom = &cp
					}
				}
				// Group captions are injected by the lowering pass from the part's
				// RAW style (it has no theme access), so a container role's text
				// color must be materialised onto the part for the caption to
				// match the theme (default blue-grey captions on a paper theme —
				// caught by the verification matrix).
				if isContainerShape(p.Shape) {
					if rs := theme.Roles[p.Role]; rs != nil && rs.Text != nil && rs.Text.Color != "" {
						if p.Style == nil {
							p.Style = &Style{}
						}
						if p.Style.Text == nil {
							p.Style.Text = &Text{}
						}
						if p.Style.Text.Color == "" {
							p.Style.Text.Color = rs.Text.Color
						}
					}
				}
				if p.Icon != "" && builtinIconNoSuffix(p.Icon) {
					eff := ResolveStyleWithRole(theme, p.Shape, p.Role, p.Preset, p.Style)
					if lum, ok := lumOf(topFillColor(eff)); ok && lum < 0.18 {
						p.Icon += "/light"
					}
				}
			}
			walk(p.Parts)
		}
	}
	for _, n := range doc.Nodes {
		if n != nil {
			walk(n.Parts)
		}
	}
}

// builtinIconNoSuffix reports whether the icon is an iso:// builtin carrying
// no explicit color — the case whose default ink disappears on dark fills.
func builtinIconNoSuffix(uri string) bool {
	isBuiltin := strings.HasPrefix(uri, "iso://glyph/") ||
		strings.HasPrefix(uri, "iso://si/") ||
		strings.HasPrefix(uri, "iso://brand/")
	if !isBuiltin {
		return false
	}
	return !strings.Contains(uri, "/light") && !iconHasColorSuffix(uri)
}

// ApplyTheme layers the named built-in theme under whatever theme the
// document already carries — the programmatic form of `theme: { use: name }`,
// used by the CLI's --theme flag (which is how a plain .d2 graph gets the
// full design system). Errors on unknown names with a did-you-mean.
func ApplyTheme(doc *Document, name string) error {
	if doc == nil {
		return fmt.Errorf("theme: nil document")
	}
	if _, ok := builtinThemes[name]; !ok {
		sugg := nearest(name, ThemeNames())
		if sugg != "" {
			return fmt.Errorf("unknown theme %q (did you mean %q? available: %s)", name, sugg, strings.Join(ThemeNames(), ", "))
		}
		return fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(ThemeNames(), ", "))
	}
	if doc.Theme == nil {
		doc.Theme = &Theme{}
	}
	doc.Theme.Use = name
	// The flag is an explicit "give me this look": the theme owns the canvas
	// (unlike in-DSL `use:` where an authored canvas wins) and displaces
	// translator-generated edge colors so connectors take the neutral default
	// instead of d2's palette.
	doc.Canvas = nil
	for _, n := range doc.Nodes {
		if n == nil {
			continue
		}
		for _, c := range n.Connectors {
			if c != nil && c.strokeGenerated {
				c.Stroke = nil
				c.strokeGenerated = false
			}
		}
	}
	resolveThemeUse(doc)
	applyRoleDefaults(doc)
	return nil
}
