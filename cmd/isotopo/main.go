// Command isotopo — DSL → iso topology renderer.
//
// Usage:
//
//	isotopo render [--layout dagre|elk] <input.yaml|input.d2|-> <output-dir>
//	isotopo capabilities                       # structured JSON of supported features
//
// The render subcommand output directory is laid out as two tiers:
//
//	<out>/
//	├── topology.svg              # the full scene
//	├── topology.html             # embed snippet + editable DSL source
//	├── topology.<yaml|d2>        # source copy (for re-rendering)
//	└── nodes/
//	    ├── _index.html           # gallery of all per-part SVGs
//	    ├── <id>.svg              # standalone iso element
//	    ├── <id>.html             # embed snippet + per-part DSL fragment
//	    └── <id>.yaml             # per-part DSL fragment
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	isotopo "github.com/MarkovWangRR/iso-topology"
	"github.com/MarkovWangRR/iso-topology/internal/yamledit"
	"gopkg.in/yaml.v3"
)

// ── Detail editor (Studio right-click → Edit details) ────────────────────
//
// The form is SCHEMA-DRIVEN, not "show whatever scalar keys exist": each DSL
// key that matters for visual tuning is declared here ONCE with an English
// label, a one-line description, and an input type. detailSchema is the single
// source of truth — designing the DSL means deciding each key's semantics up
// front. Values are read from the element's current YAML (any depth, dotted
// path) and changes are written back in place (comment-preserving).

type schemaField struct {
	Path    string   `json:"key"`   // dotted YAML path; also the form key
	Label   string   `json:"label"` // English, human-readable
	Desc    string   `json:"desc"`  // one-line semantic description
	Type    string   `json:"type"`  // text | number | color | icon | choice
	Options []string `json:"options,omitempty"`
	Group   string   `json:"group,omitempty"`  // section header in the modal
	// Collapsed marks the field's group as collapsed by default in the modal
	// (read from the group's first field). Core groups stay open; advanced /
	// rarely-touched groups (Position, Gradients, Border, Effects) fold away so
	// the form has a clear primary→advanced hierarchy instead of one long dump.
	Collapsed bool `json:"collapsed,omitempty"`
	Inline    bool `json:"inline,omitempty"` // compact, laid out side-by-side
	Value     string `json:"value"`          // current value, filled per request
	// Eff is the EFFECTIVE resolved colour (theme→shape→preset cascade,
	// gradients flattened to their start colour) for a color field whose own
	// Value is empty — what the canvas actually shows. The form paints the
	// swatch with it so the picker matches reality, while Value stays empty so
	// an untouched field keeps inheriting. Only set for color-type fields.
	Eff string `json:"eff,omitempty"`
	// Empty is the muted placeholder for a field that has NEITHER an own value
	// NOR a resolved Eff — i.e. genuinely unset. It states what leaving it
	// blank means ("0", "1", "none", "auto"), so no field is ever a mystery
	// blank box.
	Empty string `json:"empty,omitempty"`
}

// nodeSchema / edgeSchema declare the editable, visually-impactful fields.
// shapeOptions is DERIVED from the capability report (the single source of
// truth for what the renderer accepts), so adding a shape to the engine makes
// it appear in the Studio picker automatically — no hand-maintained list to
// drift. Only `composite` (the scene container, never a node choice) is
// dropped, and iso_text is shown under its friendlier `text` alias.
func shapeOptions() []string {
	var out []string
	for _, s := range isotopo.CapabilityReport().Shapes {
		switch s.IsoName {
		case "composite":
			// scene container only — not a per-node shape
		case "iso_text":
			out = append(out, "text")
		default:
			out = append(out, s.IsoName)
		}
	}
	return out
}

// shapeClass buckets a shape by how it takes colour, so the detail form only
// offers controls that actually affect that shape:
//
//	faces   — top/left/right palette (box family, prisms, cylinder, group)
//	outline — dashed border only (boundary)
//	text    — label colour/size (iso_text)
//	fill    — a single surface colour (circle, cloud, person)
func shapeClass(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "boundary":
		return "outline"
	case "text", "iso_text", "title":
		return "text"
	case "circle", "oval", "cloud", "person", "c4-person", "c4_person", "sphere":
		return "fill"
	default: // rectangle/box/square, prism family, hexprism, cylinder, group, …
		return "faces"
	}
}

// grp stamps a Group name and collapsed-default onto a run of fields, so
// nodeSchema reads as an ordered list of labelled sections (primary groups
// open, advanced ones collapsed) rather than a flat field soup.
func grp(group string, collapsed bool, fs ...schemaField) []schemaField {
	for i := range fs {
		fs[i].Group = group
		fs[i].Collapsed = collapsed
	}
	return fs
}

// reusable option sets for choice fields.
var (
	dashOpts   = []string{"", "6 4", "1 5"}                   // solid | dashed | dotted
	weightOpts = []string{"", "400", "500", "600", "700", "800"}
	dirOpts    = []string{"", "down", "up", "left", "right", "diag"}
	alignOpts  = []string{"", "start", "center", "end"}
)

// emptyHint is the muted placeholder for a field with no own value and no
// resolved effective value — it states the consequence of leaving it blank, so
// the field never reads as "missing". Choice fields are skipped: their blank
// option tile (e.g. "Solid", "(none)") already shows the empty state.
func emptyHint(f schemaField) string {
	switch f.Path {
	case "offset.wx", "offset.wy", "offset.wz":
		return "0" // no manual nudge
	case "stack.count":
		return "1" // no replication
	case "stack.gap", "place.gap", "layout.gap", "layout.padding", "layout.cols":
		return "auto"
	case "geom.w", "geom.d", "geom.h", "geom.sides":
		return "" // size has shape-defaults not surfaced here; leave blank
	}
	switch f.Type {
	case "color":
		return "none"
	case "number":
		return "none" // an effect amount left at 0 is off
	case "text":
		if strings.HasPrefix(f.Path, "place.") {
			return "none"
		}
		return "" // label / preset / family — optional content, blank is self-evident
	}
	return ""
}

func isContainerShape(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "group", "boundary", "composite":
		return true
	}
	return false
}

// nodeSchema is shape-aware AND ordered as a hierarchy: Content → Size →
// Position → Layout → Fill → Gradients → Border → Label → Effects. Primary
// groups (Content/Size/Fill/Label) open; the rest collapse by default. The
// colour/face sections depend on what the shape can render, so users never
// tune a knob that does nothing.
func nodeSchema(shape string) []schemaField {
	class := shapeClass(shape)
	var f []schemaField

	// ── Content (open) ────────────────────────────────────────────────
	f = append(f, grp("Content", false,
		schemaField{Path: "label", Label: "Label", Desc: "Text rendered on the node", Type: "text"},
		schemaField{Path: "shape", Label: "Shape", Desc: "Geometric form of the node", Type: "choice", Options: shapeOptions()},
		schemaField{Path: "preset", Label: "Style preset", Desc: "Named style from theme.presets", Type: "text"},
		schemaField{Path: "icon", Label: "Icon", Desc: "iso://… ref, image URL, or pick a local file", Type: "icon"},
		schemaField{Path: "@iconColor", Label: "Icon color", Desc: "Tint for iso:// glyph/logo icons (blank = default ink)", Type: "color"},
	)...)

	// ── Size (open) ───────────────────────────────────────────────────
	size := []schemaField{
		{Path: "geom.w", Label: "Width", Type: "number", Inline: true},
		{Path: "geom.d", Label: "Depth", Type: "number", Inline: true},
		{Path: "geom.h", Label: "Height", Type: "number", Inline: true},
	}
	if strings.EqualFold(strings.TrimSpace(shape), "prism") {
		size = append(size, schemaField{Path: "geom.sides", Label: "Sides", Desc: "Polygon base sides", Type: "number", Inline: true})
	}
	f = append(f, grp("Size", false, size...)...)

	// ── Position (collapsed) — relative placement, fine offset, replicas.
	// Mostly driven by drag, but exposed for precise/declarative control.
	f = append(f, grp("Position", true,
		schemaField{Path: "place.rightOf", Label: "Right of", Desc: "Sibling id to sit to the right of", Type: "text", Inline: true},
		schemaField{Path: "place.leftOf", Label: "Left of", Type: "text", Inline: true},
		schemaField{Path: "place.above", Label: "Above", Type: "text", Inline: true},
		schemaField{Path: "place.behind", Label: "Behind", Type: "text", Inline: true},
		schemaField{Path: "place.inFrontOf", Label: "In front of", Type: "text", Inline: true},
		schemaField{Path: "place.gap", Label: "Place gap", Desc: "Gap in cells", Type: "number", Inline: true},
		schemaField{Path: "offset.wx", Label: "Offset X", Desc: "World-space fine-tune", Type: "number", Inline: true},
		schemaField{Path: "offset.wy", Label: "Offset Y", Type: "number", Inline: true},
		schemaField{Path: "offset.wz", Label: "Offset Z", Desc: "Lift (the axis the solver never sets)", Type: "number", Inline: true},
		schemaField{Path: "stack.count", Label: "Stack count", Desc: "Replica layers (1 = none)", Type: "number", Inline: true},
		schemaField{Path: "stack.gap", Label: "Stack gap", Desc: "Z-step between replicas", Type: "number", Inline: true},
	)...)

	// ── Layout (collapsed) — only for containers that arrange children. ─
	if isContainerShape(shape) {
		f = append(f, grp("Layout", true,
			schemaField{Path: "layout.mode", Label: "Mode", Desc: "Auto-arrange children", Type: "choice",
				Options: []string{"", "row", "column", "grid", "ring", "auto"}},
			schemaField{Path: "layout.gap", Label: "Gap", Desc: "Spacing between children, cells", Type: "number", Inline: true},
			schemaField{Path: "layout.cols", Label: "Columns", Desc: "Grid mode only", Type: "number", Inline: true},
			schemaField{Path: "layout.padding", Label: "Padding", Desc: "Inner margin, cells", Type: "number", Inline: true},
			schemaField{Path: "layout.align", Label: "Align", Desc: "Cross-axis alignment", Type: "choice", Options: alignOpts},
		)...)
	}

	// ── Fill / Gradients / Border — shape-class dependent ──────────────
	switch class {
	case "outline": // boundary — dashed region, border only
		f = append(f, grp("Border", false,
			schemaField{Path: "style.stroke.color", Label: "Border color", Desc: "Outline color (CSS color)", Type: "color"},
			schemaField{Path: "style.stroke.width", Label: "Border width", Type: "number", Inline: true},
			schemaField{Path: "style.stroke.dash", Label: "Border style", Desc: "Solid, dashed, or dotted", Type: "choice", Options: dashOpts, Inline: true},
		)...)
	case "text": // iso_text — no body; label IS the element (handled in Label below)
	case "fill": // circle / cloud / person — single surface colour
		f = append(f, grp("Fill", false,
			schemaField{Path: "style.palette.top", Label: "Fill color", Desc: "Surface fill (CSS color)", Type: "color"},
		)...)
		f = append(f, grp("Border", true,
			schemaField{Path: "style.stroke.color", Label: "Color", Type: "color", Inline: true},
			schemaField{Path: "style.stroke.width", Label: "Width", Type: "number", Inline: true},
			schemaField{Path: "style.stroke.dash", Label: "Dash", Type: "choice", Options: dashOpts, Inline: true},
		)...)
	default: // faces — box family, prisms, cylinder, group
		f = append(f, grp("Fill", false,
			schemaField{Path: "style.palette.top", Label: "Top", Type: "color", Inline: true},
			schemaField{Path: "style.palette.left", Label: "Left", Type: "color", Inline: true},
			schemaField{Path: "style.palette.right", Label: "Right", Type: "color", Inline: true},
		)...)
		f = append(f, grp("Gradients", true,
			schemaField{Path: "style.palette.topGradient.from", Label: "Top from", Type: "color", Inline: true},
			schemaField{Path: "style.palette.topGradient.to", Label: "Top to", Type: "color", Inline: true},
			schemaField{Path: "style.palette.topGradient.dir", Label: "Top dir", Type: "choice", Options: dirOpts, Inline: true},
			schemaField{Path: "style.palette.leftGradient.from", Label: "Left from", Type: "color", Inline: true},
			schemaField{Path: "style.palette.leftGradient.to", Label: "Left to", Type: "color", Inline: true},
			schemaField{Path: "style.palette.leftGradient.dir", Label: "Left dir", Type: "choice", Options: dirOpts, Inline: true},
			schemaField{Path: "style.palette.rightGradient.from", Label: "Right from", Type: "color", Inline: true},
			schemaField{Path: "style.palette.rightGradient.to", Label: "Right to", Type: "color", Inline: true},
			schemaField{Path: "style.palette.rightGradient.dir", Label: "Right dir", Type: "choice", Options: dirOpts, Inline: true},
		)...)
		f = append(f, grp("Border", true,
			schemaField{Path: "style.stroke.color", Label: "Color", Type: "color", Inline: true},
			schemaField{Path: "style.stroke.width", Label: "Width", Type: "number", Inline: true},
			schemaField{Path: "style.stroke.dash", Label: "Dash", Type: "choice", Options: dashOpts, Inline: true},
		)...)
	}

	// ── Label (open) — the caption every shape paints on its face. ─────
	f = append(f, grp("Label", false,
		schemaField{Path: "style.text.color", Label: "Text color", Desc: "Caption color (CSS color)", Type: "color", Inline: true},
		schemaField{Path: "style.text.size", Label: "Font size", Desc: "Caption size in px", Type: "number", Inline: true},
		schemaField{Path: "style.text.weight", Label: "Weight", Desc: "Font weight", Type: "choice", Options: weightOpts, Inline: true},
		schemaField{Path: "style.text.family", Label: "Font family", Desc: "CSS font stack", Type: "text"},
		schemaField{Path: "style.text.orient", Label: "Orientation", Desc: "iso (on-face) or screen (flat below)", Type: "choice", Options: []string{"", "iso", "screen"}},
	)...)

	// ── Effects (collapsed) — lighting, texture, transparency. Solids
	// only; text/outline have no volume to glow, shadow or texture. ─────
	if class == "faces" || class == "fill" {
		eff := []schemaField{}
		if class == "faces" {
			eff = append(eff, schemaField{Path: "style.effects.cornerRadius", Label: "Corner radius", Desc: "Rounds vertical edges", Type: "number", Inline: true})
		}
		eff = append(eff,
			schemaField{Path: "style.effects.opacity", Label: "Opacity", Desc: "0–1 whole-part transparency", Type: "number", Inline: true},
			schemaField{Path: "style.effects.blur", Label: "Blur", Desc: "Gaussian blur px — fog/ghost", Type: "number", Inline: true},
			schemaField{Path: "style.effects.dropShadow.color", Label: "Shadow color", Desc: "Drop shadow under the silhouette", Type: "color", Inline: true},
			schemaField{Path: "style.effects.dropShadow.dx", Label: "Shadow dx", Type: "number", Inline: true},
			schemaField{Path: "style.effects.dropShadow.dy", Label: "Shadow dy", Type: "number", Inline: true},
			schemaField{Path: "style.effects.dropShadow.blur", Label: "Shadow blur", Type: "number", Inline: true},
			schemaField{Path: "style.effects.backglow.color", Label: "Glow color", Desc: "Soft halo behind the part", Type: "color", Inline: true},
			schemaField{Path: "style.effects.backglow.radius", Label: "Glow radius", Type: "number", Inline: true},
			schemaField{Path: "style.effects.backglow.opacity", Label: "Glow opacity", Type: "number", Inline: true},
			schemaField{Path: "style.effects.pattern.kind", Label: "Pattern", Desc: "Top-face texture", Type: "choice", Options: []string{"", "hatch", "dots", "grid"}, Inline: true},
			schemaField{Path: "style.effects.pattern.color", Label: "Pattern color", Type: "color", Inline: true},
			schemaField{Path: "style.effects.pattern.spacing", Label: "Pattern spacing", Type: "number", Inline: true},
			schemaField{Path: "style.effects.pattern.angle", Label: "Pattern angle", Desc: "Degrees (hatch)", Type: "number", Inline: true},
		)
		f = append(f, grp("Effects", true, eff...)...)
	}
	return f
}

func canvasSchema() []schemaField {
	out := grp("Background", false,
		schemaField{Path: "background", Label: "Fill color", Desc: "Canvas fill behind the diagram (CSS color)", Type: "color"},
		schemaField{Path: "grid", Label: "Grid pattern", Desc: "Background texture", Type: "choice", Options: []string{"none", "iso", "dots", "hatch", "solid"}},
		schemaField{Path: "gridColor", Label: "Grid color", Desc: "Grid/texture line color (CSS color)", Type: "color"},
		schemaField{Path: "gridStep", Label: "Grid step", Desc: "Grid cell size in world units (blank = default)", Type: "number"},
	)
	out = append(out, grp("Layout", false,
		schemaField{Path: "padding", Label: "Padding", Desc: "Outer breathing margin around the scene, px", Type: "number"},
	)...)
	return out
}

func edgeSchema() []schemaField {
	out := grp("Connection", false,
		schemaField{Path: "from", Label: "From", Desc: "Source anchor — node id or node.face", Type: "text"},
		schemaField{Path: "to", Label: "To", Desc: "Target anchor — node id or node.face", Type: "text"},
	)
	out = append(out, grp("Routing", false,
		schemaField{Path: "routing", Label: "Routing", Desc: "Path style between endpoints", Type: "choice", Options: []string{"orthogonal", "straight", "bezier"}},
		schemaField{Path: "arrow", Label: "Arrowhead", Desc: "Marker at the target end", Type: "choice", Options: []string{"none", "triangle"}, Inline: true},
		schemaField{Path: "elbow", Label: "Elbow bias", Desc: "Orthogonal turn order", Type: "choice", Options: []string{"", "xFirst", "yFirst"}, Inline: true},
	)...)
	out = append(out, grp("Line", false,
		schemaField{Path: "stroke.color", Label: "Color", Desc: "Stroke color (CSS color)", Type: "color", Inline: true},
		schemaField{Path: "stroke.width", Label: "Width", Type: "number", Inline: true},
		schemaField{Path: "stroke.dash", Label: "Dash", Desc: "Solid, dashed, or dotted", Type: "choice", Options: dashOpts, Inline: true},
	)...)
	out = append(out, grp("Label", true,
		schemaField{Path: "label", Label: "Text", Desc: "Text rendered mid-route", Type: "text"},
		schemaField{Path: "labelColor", Label: "Text color", Type: "color", Inline: true},
		schemaField{Path: "labelBg", Label: "Background", Type: "color", Inline: true},
		schemaField{Path: "labelFontSize", Label: "Font size", Type: "number", Inline: true},
	)...)
	return out
}

// schemaWithValues returns the kind's schema with each field's Value read
// from the element's current YAML.
func schemaWithValues(src, lang, kind, id string, ci int) ([]schemaField, bool) {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		return nil, false
	}
	var subtree map[string]interface{}
	var fields []schemaField
	switch kind {
	case "node":
		subtree = yamledit.FindNodeMap(root, id)
		shape, _ := subtree["shape"].(string) // nil subtree → "" → face schema; guarded below
		fields = nodeSchema(shape)
	case "edge":
		conns := yamledit.FindConnectors(root)
		if ci >= 0 && ci < len(conns) {
			subtree, _ = conns[ci].(map[string]interface{})
		}
		fields = edgeSchema()
	case "canvas":
		// canvas may be absent — still show the schema so the user can add a
		// background/grid from scratch (the editor creates the block on save).
		subtree, _ = root["canvas"].(map[string]interface{})
		if subtree == nil {
			subtree = map[string]interface{}{}
		}
		fields = canvasSchema()
	default:
		return nil, false
	}
	if subtree == nil {
		return nil, false
	}
	for i := range fields {
		// "@iconColor" is synthetic — the tint lives in the icon ref suffix
		// (iso://glyph|si/<name>/RRGGBB), not a YAML key. Surface the current
		// suffix so the picker reflects the existing colour.
		if fields[i].Path == "@iconColor" {
			fields[i].Value = iconColorOf(yamledit.ReadPath(subtree, "icon"))
			continue
		}
		fields[i].Value = yamledit.ReadPath(subtree, fields[i].Path)
	}
	// For a node, fill the EFFECTIVE value of any inherited (empty) style.*
	// field by resolving the full cascade — colours, sizes, weights, radii,
	// the lot — so EVERY tab reflects what the canvas paints (shown as an
	// "inherits …" hint) rather than a blank that wrongly reads as "unset".
	if kind == "node" {
		if doc, err := loadDocument(lang, []byte(src)); err == nil {
			if eff := isotopo.ResolvePartStyle(doc, id); eff != nil {
				for i := range fields {
					if fields[i].Value != "" || !strings.HasPrefix(fields[i].Path, "style.") {
						continue
					}
					fields[i].Eff = effValue(eff, fields[i].Path)
				}
			}
			// Auto-sized containers (a group with no authored geom.w/d) get their
			// SOLVER-COMPUTED size as the hint, so empty W/D reads as "auto 380",
			// not "missing". Height too, when defaulted.
			if gw, gd, gh, okg := isotopo.ResolvePartGeom(doc, id); okg {
				for i := range fields {
					if fields[i].Value != "" {
						continue
					}
					switch fields[i].Path {
					case "geom.w":
						fields[i].Eff = fstrv(gw)
					case "geom.d":
						fields[i].Eff = fstrv(gd)
					case "geom.h":
						fields[i].Eff = fstrv(gh)
					}
				}
			}
		}
	}
	// Edges have no style cascade, but the renderer draws every connector with
	// active DEFAULTS when unset — a grey orthogonal line. Surface those so an
	// empty routing/stroke reads as "default orthogonal", not "missing".
	// (Keep in sync with the connector defaults in inject.go.)
	if kind == "edge" {
		edgeDefaults := map[string]string{
			"routing":      "orthogonal",
			"arrow":        "none",
			"stroke.color": "#7A8390",
			"stroke.width": "1.4",
		}
		for i := range fields {
			if fields[i].Value != "" {
				continue
			}
			if d, ok := edgeDefaults[fields[i].Path]; ok {
				fields[i].Eff = d
			}
		}
	}
	// Final pass: any field still with no value AND no effective hint gets a
	// muted "what blank means" placeholder, so nothing is a mystery blank box.
	for i := range fields {
		if fields[i].Value == "" && fields[i].Eff == "" {
			fields[i].Empty = emptyHint(fields[i])
		}
	}
	return fields, true
}

// fstr renders an optional float (nil → ""); fstrv renders a plain float,
// treating 0 as unset so a default-zero knob doesn't show a noisy "inherits 0".
func fstr(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}
func fstrv(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// fnum formats a float INCLUDING zero — used for the parameters of an effect
// that is already present (e.g. a drop shadow's dx:0), where 0 is a real value
// to show, not "unset". Dropping it would leave a half-filled effect.
func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// effValue maps ANY style.* dotted path to its resolved value in a merged
// Style — colours, numbers, weights, dirs, the lot — flattening a face gradient
// to its start colour for the solid swatch. The form shows this as the
// "inherits …" hint on inherited fields so EVERY tab reflects what the canvas
// paints, not just the colour fields. Returns "" when nothing is set.
func effValue(st *isotopo.Style, path string) string {
	if st == nil {
		return ""
	}
	switch path {
	// ── palette (solid; gradient flattens to its start colour) ──
	case "style.palette.top":
		if st.Palette != nil {
			if g := st.Palette.TopGradient; g != nil {
				return g.From
			}
			return st.Palette.Top
		}
	case "style.palette.left":
		if st.Palette != nil {
			if g := st.Palette.LeftGradient; g != nil {
				return g.From
			}
			return st.Palette.Left
		}
	case "style.palette.right":
		if st.Palette != nil {
			if g := st.Palette.RightGradient; g != nil {
				return g.From
			}
			return st.Palette.Right
		}
	case "style.palette.topGradient.from":
		if st.Palette != nil && st.Palette.TopGradient != nil {
			return st.Palette.TopGradient.From
		}
	case "style.palette.topGradient.to":
		if st.Palette != nil && st.Palette.TopGradient != nil {
			return st.Palette.TopGradient.To
		}
	case "style.palette.topGradient.dir":
		if st.Palette != nil && st.Palette.TopGradient != nil {
			return st.Palette.TopGradient.Dir
		}
	case "style.palette.leftGradient.from":
		if st.Palette != nil && st.Palette.LeftGradient != nil {
			return st.Palette.LeftGradient.From
		}
	case "style.palette.leftGradient.to":
		if st.Palette != nil && st.Palette.LeftGradient != nil {
			return st.Palette.LeftGradient.To
		}
	case "style.palette.leftGradient.dir":
		if st.Palette != nil && st.Palette.LeftGradient != nil {
			return st.Palette.LeftGradient.Dir
		}
	case "style.palette.rightGradient.from":
		if st.Palette != nil && st.Palette.RightGradient != nil {
			return st.Palette.RightGradient.From
		}
	case "style.palette.rightGradient.to":
		if st.Palette != nil && st.Palette.RightGradient != nil {
			return st.Palette.RightGradient.To
		}
	case "style.palette.rightGradient.dir":
		if st.Palette != nil && st.Palette.RightGradient != nil {
			return st.Palette.RightGradient.Dir
		}
	// ── stroke ──
	case "style.stroke.color":
		if st.Stroke != nil {
			return st.Stroke.Color
		}
	case "style.stroke.width":
		if st.Stroke != nil {
			return fstr(st.Stroke.Width)
		}
	case "style.stroke.dash":
		if st.Stroke != nil {
			return st.Stroke.Dash
		}
	// ── text ──
	case "style.text.color":
		if st.Text != nil {
			return st.Text.Color
		}
	case "style.text.size":
		if st.Text != nil {
			return fstr(st.Text.Size)
		}
	case "style.text.weight":
		if st.Text != nil {
			return st.Text.Weight
		}
	case "style.text.family":
		if st.Text != nil {
			return st.Text.Family
		}
	case "style.text.orient":
		if st.Text != nil && st.Text.Orient != "" {
			return st.Text.Orient
		}
		return "iso" // render default: label sits on the iso top face
	// ── effects ──
	case "style.effects.opacity":
		if st.Effects != nil && st.Effects.Opacity != nil {
			return fstr(st.Effects.Opacity)
		}
		return "1" // render default: fully opaque
	case "style.effects.blur":
		if st.Effects != nil {
			return fstr(st.Effects.Blur)
		}
	case "style.effects.cornerRadius":
		if st.Effects != nil {
			return fstr(st.Effects.CornerRadius)
		}
	case "style.effects.dropShadow.color":
		if st.Effects != nil && st.Effects.DropShadow != nil {
			return st.Effects.DropShadow.Color
		}
	case "style.effects.dropShadow.dx":
		if st.Effects != nil && st.Effects.DropShadow != nil {
			return fnum(st.Effects.DropShadow.Dx)
		}
	case "style.effects.dropShadow.dy":
		if st.Effects != nil && st.Effects.DropShadow != nil {
			return fnum(st.Effects.DropShadow.Dy)
		}
	case "style.effects.dropShadow.blur":
		if st.Effects != nil && st.Effects.DropShadow != nil {
			return fnum(st.Effects.DropShadow.Blur)
		}
	case "style.effects.backglow.color":
		if st.Effects != nil && st.Effects.Backglow != nil {
			return st.Effects.Backglow.Color
		}
	case "style.effects.backglow.radius":
		if st.Effects != nil && st.Effects.Backglow != nil {
			return fnum(st.Effects.Backglow.Radius)
		}
	case "style.effects.backglow.opacity":
		if st.Effects != nil && st.Effects.Backglow != nil {
			return fnum(st.Effects.Backglow.Opacity)
		}
	case "style.effects.pattern.kind":
		if st.Effects != nil && st.Effects.Pattern != nil {
			return st.Effects.Pattern.Kind
		}
	case "style.effects.pattern.color":
		if st.Effects != nil && st.Effects.Pattern != nil {
			return st.Effects.Pattern.Color
		}
	case "style.effects.pattern.spacing":
		if st.Effects != nil && st.Effects.Pattern != nil {
			return fnum(st.Effects.Pattern.Spacing)
		}
	case "style.effects.pattern.angle":
		if st.Effects != nil && st.Effects.Pattern != nil {
			return fnum(st.Effects.Pattern.Angle)
		}
	}
	return ""
}

// spliceIconColor folds a tint into an icon ref: only iso glyph/logo refs
// carry a "/RRGGBB" (or "/light") colour suffix, so it returns "" for URLs,
// data URIs, local paths and brand aliases (nothing to tint). A blank hex
// strips the suffix, reverting to the icon's default ink.
func spliceIconColor(icon, hex string) string {
	icon = strings.TrimSpace(icon)
	if !strings.HasPrefix(icon, "iso://glyph/") && !strings.HasPrefix(icon, "iso://si/") {
		return ""
	}
	base := icon
	if i := strings.LastIndex(base, "/"); i > len("iso://") {
		if suf := base[i+1:]; suf == "light" || isHex6(suf) {
			base = base[:i]
		}
	}
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if hex == "" {
		return base
	}
	return base + "/" + strings.ToUpper(hex)
}

// iconColorOf returns the "#RRGGBB" tint currently encoded in an icon ref,
// or "" if the ref has no hex colour suffix.
func iconColorOf(icon string) string {
	icon = strings.TrimSpace(icon)
	if i := strings.LastIndex(icon, "/"); i > len("iso://") {
		if suf := icon[i+1:]; isHex6(suf) {
			return "#" + strings.ToUpper(suf)
		}
	}
	return ""
}

func isHex6(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// CLI flag state — set by parseFlags. Layout is consulted only for .d2
// inputs (YAML/JSON have no layout step).
var (
	flagLayout = "dagre"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "render":
		args, err := parseFlags(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			usage()
			os.Exit(2)
		}
		if len(args) != 2 {
			usage()
			os.Exit(2)
		}
		if err := renderFile(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "capabilities":
		if err := emitCapabilities(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "serve":
		args, err := parseFlags(os.Args[2:])
		if err != nil || len(args) != 1 {
			usage()
			os.Exit(2)
		}
		if err := serveFile(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "validate":
		if len(os.Args) != 3 {
			usage()
			os.Exit(2)
		}
		code, err := validateFile(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		os.Exit(code)
	default:
		usage()
		os.Exit(2)
	}
}

// validateFile parses the input and runs Validate, emitting structured
// JSON of any issues. Exit code: 0 = clean, 2 = warnings only, 3 = any
// errors. Designed for agent CI loops.
func validateFile(in string) (int, error) {
	var data []byte
	var err error
	if in == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(in)
	}
	if err != nil {
		return 1, err
	}
	sourceLang, _ := classifyInput(in)
	doc, err := loadDocument(sourceLang, data)
	if err != nil {
		// Parse failure is a single hard error — emit as one Issue so
		// agents have a uniform JSON contract.
		out := map[string]any{
			"issues": []any{
				map[string]any{
					"severity": "error",
					"path":     "$",
					"message":  fmt.Sprintf("parse failed: %s", err),
				},
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 3, nil
	}
	issues := isotopo.Validate(doc)
	if sourceLang != "d2" {
		issues = append(issues, isotopo.UnknownKeyIssues(data)...)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"issues": issues})
	for _, i := range issues {
		if i.Severity == isotopo.SeverityError {
			return 3, nil
		}
	}
	if len(issues) > 0 {
		return 2, nil
	}
	return 0, nil
}

// emitCapabilities writes the structured capability report as
// pretty-printed JSON. Agents call this at startup to learn what
// shapes, primitives, layouts, and style keys are available without
// reading source.
func emitCapabilities(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(isotopo.CapabilityReport())
}

// parseFlags strips known flags from argv and returns the positional
// remainder. Kept hand-rolled (no `flag` package) because the CLI has
// exactly one optional flag and we want it to appear in any position.
func parseFlags(argv []string) ([]string, error) {
	positional := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--layout":
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("--layout requires a value")
			}
			flagLayout = argv[i+1]
			i++
		case strings.HasPrefix(a, "--layout="):
			flagLayout = strings.TrimPrefix(a, "--layout=")
		default:
			positional = append(positional, a)
		}
	}
	switch flagLayout {
	case "dagre", "elk":
	default:
		return nil, fmt.Errorf("invalid --layout %q (want dagre|elk)", flagLayout)
	}
	return positional, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  isotopo render [--layout dagre|elk] <input.yaml|input.d2|-> <output-dir>
  isotopo capabilities

input formats:
  .yaml / .json  manual iso composite (precise placement)
  .d2            d2 graph source (auto-layout)

flags:
  --layout dagre  natural-bend polyline edges (default)
  --layout elk    orthogonal right-angle edges with obstacle avoidance

subcommands:
  render         render an input file to <output-dir>
  capabilities   emit structured JSON of supported shapes, primitives,
                 layouts, and style keys — intended for agents to read
                 before generating DSL
  validate <in>  parse + structural validate the input file. Emits JSON
                 of issues with paths and "did you mean" suggestions.
                 exit: 0 = clean, 2 = warnings only, 3 = errors
  serve <in>     local live-preview server (default :8731, override with
                 ISOTOPO_PORT): the interactive topology.html with hover
                 source-mapping, zoom/pan, edit-to-re-render against an
                 in-browser COPY (the input file is never written),
                 SVG/PNG export of the edited canvas, and the per-node
                 gallery at /nodes/`)
}

func renderFile(in, outDir string) error {
	var data []byte
	var err error
	if in == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(in)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	nodesDir := filepath.Join(outDir, "nodes")
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		return err
	}

	sourceLang, sourceExt := classifyInput(in)
	doc, err := loadDocument(sourceLang, data)
	if err != nil {
		return err
	}

	topologySVG := renderTopologySVG(doc)
	if err := writeFile(filepath.Join(outDir, "topology.svg"), []byte(topologySVG)); err != nil {
		return err
	}

	sourceFilename := "topology" + sourceExt
	if err := writeFile(filepath.Join(outDir, sourceFilename), data); err != nil {
		return err
	}

	absCopy, err := filepath.Abs(filepath.Join(outDir, sourceFilename))
	if err != nil {
		absCopy = sourceFilename
	}
	topologyHTML := isotopo.TopologyHTML(topologySVG, string(data), sourceLang, absCopy)
	if err := writeFile(filepath.Join(outDir, "topology.html"), []byte(topologyHTML)); err != nil {
		return err
	}

	parts := isotopo.RenderParts(doc)
	frags := isotopo.PartFragments(doc)
	ids := isotopo.PartIDs(doc)
	for _, id := range ids {
		svg := parts[id]
		if svg == "" {
			continue
		}
		if err := writeFile(filepath.Join(nodesDir, id+".svg"), []byte(svg)); err != nil {
			return err
		}
		var fragYAML []byte
		if f := frags[id]; f != nil {
			fragYAML, err = isotopo.MarshalFragmentYAML(f)
			if err != nil {
				return err
			}
			if err := writeFile(filepath.Join(nodesDir, id+".yaml"), fragYAML); err != nil {
				return err
			}
		}
		nodeHTML := isotopo.NodeHTML(id, svg, string(fragYAML))
		if err := writeFile(filepath.Join(nodesDir, id+".html"), []byte(nodeHTML)); err != nil {
			return err
		}
	}

	indexHTML := isotopo.NodesIndexHTML(ids)
	if err := writeFile(filepath.Join(nodesDir, "_index.html"), []byte(indexHTML)); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "rendered %d node(s) into %s\n", len(ids), outDir)
	return nil
}

// renderTopologySVG returns the topology-level scene SVG. Resolution
// is delegated to doc.Scene() so the CLI stays in sync with the
// library's scene rules.
func renderTopologySVG(doc *isotopo.Document) string {
	if scene := doc.Scene(); scene != nil {
		return isotopo.RenderWithCanvas(scene, doc.Theme, doc.Canvas, doc.Annotations)
	}
	return ""
}

// loadDocument routes the input through the right parser based on file
// extension. YAML/JSON go through the composite parser directly; .d2 is
// compiled + auto-laid-out by d2lib first, then translated to a
// composite document. The layout engine comes from the --layout CLI
// flag (default dagre).
func loadDocument(lang string, data []byte) (*isotopo.Document, error) {
	engine := isotopo.LayoutDagre
	if flagLayout == "elk" {
		engine = isotopo.LayoutELK
	}
	return isotopo.LoadInput(context.Background(), lang, data, engine)
}

// classifyInput maps an input path to (sourceLang, fileExtension). The
// language drives loadDocument; the extension drives the
// `topology.<ext>` source-copy filename so users can re-render later.
func classifyInput(path string) (lang, ext string) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".d2":
		return "d2", ".d2"
	case ".json":
		return "json", ".json"
	default:
		return "yaml", ".yaml"
	}
}

func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// serveFile starts a local preview server for one input file: GET /
// renders the CURRENT file content into the interactive topology.html,
// and POST /api/render renders whatever source the page's editor holds
// — a copy living in the browser; the original file is never written.
func serveFile(in string) error {
	if _, err := os.Stat(in); err != nil {
		return err
	}
	sourceLang, _ := classifyInput(in)
	absIn, err := filepath.Abs(in)
	if err != nil {
		absIn = in
	}

	port := os.Getenv("ISOTOPO_PORT")
	if port == "" {
		port = "8731"
	}

	render := func(lang string, data []byte) (string, []isotopo.Issue, error) {
		doc, err := loadDocument(lang, data)
		if err != nil {
			return "", []isotopo.Issue{{Severity: isotopo.SeverityError, Path: "$", Message: err.Error()}}, nil
		}
		issues := isotopo.Validate(doc)
		if lang != "d2" {
			issues = append(issues, isotopo.UnknownKeyIssues(data)...)
		}
		for _, i := range issues {
			if i.Severity == isotopo.SeverityError {
				return "", issues, nil
			}
		}
		svg := renderTopologySVG(doc)
		if svg == "" {
			// BUG3 (cross-test suite): a doc with zero nodes parses and
			// validates clean, then renders nothing — the page showed the
			// stale badge with an EMPTY issues panel. Say why instead.
			issues = append(issues, isotopo.Issue{
				Severity: isotopo.SeverityError, Path: "$",
				Message: "document renders no scene — it has no nodes (or the scene resolves empty)",
			})
		}
		return svg, issues, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/render", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		lang := r.URL.Query().Get("format")
		if lang == "" {
			lang = sourceLang
		}
		svg, issues, err := render(lang, body)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"svg": svg, "issues": issues})
	})
	// v4.4 — drag-to-edit: the editor copy + a move op come in, the
	// server text-edits the YAML (preserving comments/formatting), then
	// renders. kind=node writes an absolute offset on the part; kind=edge
	// writes a bend delta on the ci-th connector. Returns {yaml, svg,
	// issues} so Studio updates editor AND canvas in one round-trip.
	mux.HandleFunc("POST /api/move", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		q := r.URL.Query()
		atof := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
		// dwx, dwy are a WORLD-space drag DELTA; the server resolves the
		// target's current position and adds the delta to get an absolute
		// value, so dragging a pure-auto node (no coords yet) works.
		dwx, dwy := atof(q.Get("dwx")), atof(q.Get("dwy"))
		// snap>0 rounds the dragged node's final offset to a grid step.
		snapStep := atof(q.Get("snap"))
		snap := func(v float64) float64 {
			if snapStep > 0 {
				return math.Round(v/snapStep) * snapStep
			}
			return v
		}
		src := string(body)
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
		}
		doc, derr := loadDocument(lang, body)
		if derr != nil {
			http.Error(w, derr.Error(), 422)
			return
		}
		var out string
		var ok bool
		switch q.Get("kind") {
		case "node":
			// drawio model: the FIRST manual move freezes the whole scene
			// into explicit coordinates and drops auto-layout, so the
			// engine never re-decides positions again — no unexpected
			// jumps. Every later move just nudges that one node.
			if isotopo.SceneNeedsFreeze(doc) {
				offs := isotopo.ResolveAllOffsets(doc)
				out = yamledit.FreezeLayoutText(src)
				ids := make([]string, 0, len(offs))
				for id := range offs {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				for _, id := range ids {
					o := offs[id]
					wx, wy, wz := o[0], o[1], o[2]
					if id == q.Get("id") {
						wx = snap(wx + dwx)
						wy = snap(wy + dwy)
					}
					out, ok = yamledit.UpsertInlineKey(out, yamledit.FindPartIDLine(out, id), "offset", wx, wy, wz)
				}
			} else {
				cx, cy, cz, found := isotopo.ResolvePartOffset(doc, q.Get("id"))
				if !found {
					http.Error(w, "part not found", 422)
					return
				}
				out, ok = yamledit.UpsertInlineKey(src, yamledit.FindPartIDLine(src, q.Get("id")), "offset", snap(cx+dwx), snap(cy+dwy), cz)
			}
		case "edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			// v4.6 — Studio computes the new interior waypoint list in world
			// coords (drawio-style per-segment edit) and posts it as wp; the
			// server just serialises it. Falls back to the legacy single-corner
			// bend when wp is absent (non-orthogonal routes / older clients).
			if wp := q.Get("wp"); wp != "" {
				var raw [][2]float64
				if err := json.Unmarshal([]byte(wp), &raw); err != nil {
					http.Error(w, "bad wp: "+err.Error(), 400)
					return
				}
				out, ok = yamledit.UpsertInlineList(src, yamledit.FindConnectorLine(src, ci), "waypoints", raw)
			} else {
				bx, by := isotopo.ConnectorBend(doc, ci)
				out, ok = yamledit.UpsertInlineKey(src, yamledit.FindConnectorLine(src, ci), "bend", bx+dwx, by+dwy, 0)
			}
		default:
			http.Error(w, "kind must be node|edge", 400)
			return
		}
		if !ok {
			http.Error(w, "target not found in source", 422)
			return
		}
		svg, issues, _ := render(lang, []byte(out))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"yaml": out, "svg": svg, "issues": issues})
	})
	// v4.7 — detail editor. /api/fields returns the scalar fields actually
	// present in a node/edge's YAML, each with a semantic label, for the
	// right-click "edit details" modal. /api/edit writes the user's changes
	// back (comment-preserving) and re-renders.
	targetLine := func(src string, q url.Values) (int, bool) {
		switch q.Get("kind") {
		case "node":
			return yamledit.FindPartIDLine(src, q.Get("id")), true
		case "edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			return yamledit.FindConnectorLine(src, ci), true
		case "canvas":
			return yamledit.FindCanvasLine(src), true
		}
		return -1, false
	}
	mux.HandleFunc("POST /api/fields", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		src := string(body)
		q := r.URL.Query()
		ci, _ := strconv.Atoi(q.Get("ci"))
		fields, ok := schemaWithValues(src, sourceLang, q.Get("kind"), q.Get("id"), ci)
		if !ok {
			http.Error(w, "target not found in source", 422)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"fields": fields})
	})
	mux.HandleFunc("POST /api/edit", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		q := r.URL.Query()
		var changes map[string]string
		if err := json.Unmarshal([]byte(q.Get("f")), &changes); err != nil {
			http.Error(w, "bad f: "+err.Error(), 400)
			return
		}
		out := string(body)
		// Editing the canvas when no `canvas:` block exists yet: create an
		// empty one at the top so the writes below have somewhere to land.
		if q.Get("kind") == "canvas" && yamledit.FindCanvasLine(out) < 0 {
			out = "canvas: {}\n" + out
		}
		// "@iconColor" is a synthetic field: the icon tint lives in the icon
		// ref suffix, not a style key. Fold it into an `icon` write, based on
		// the icon edited in this same submit if present, else the node's
		// current ref. Non-glyph icons can't be tinted → drop it silently.
		if q.Get("kind") == "node" {
			if hex, ok := changes["@iconColor"]; ok {
				base := changes["icon"]
				if base == "" {
					var root map[string]interface{}
					if yaml.Unmarshal([]byte(out), &root) == nil {
						if nm := yamledit.FindNodeMap(root, q.Get("id")); nm != nil {
							base, _ = nm["icon"].(string)
						}
					}
				}
				if spliced := spliceIconColor(base, hex); spliced != "" {
					changes["icon"] = spliced
				}
				delete(changes, "@iconColor")
			}
		}
		// Re-find the target after each edit: a write can shift line numbers.
		for key, val := range changes {
			line, ok := targetLine(out, q)
			if !ok {
				http.Error(w, "kind must be node|edge|canvas", 400)
				return
			}
			if line < 0 {
				http.Error(w, "target not found in source", 422)
				return
			}
			out, _ = yamledit.SetField(out, line, strings.Split(key, "."), val)
		}
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
		}
		svg, issues, _ := render(lang, []byte(out))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"yaml": out, "svg": svg, "issues": issues})
	})
	// v4.8 — structural ops: delete a node (+its connectors) or edge, or
	// duplicate a node. Comment-preserving text surgery on the posted source.
	mux.HandleFunc("POST /api/op", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		q := r.URL.Query()
		src := string(body)
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
		}
		var out string
		ok := false
		switch q.Get("op") + ":" + q.Get("kind") {
		case "add:node":
			out, ok = yamledit.AddPart(src)
		case "delete:node":
			out, ok = yamledit.DeletePart(src, q.Get("id"))
		case "delete:edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			if s, e, found := yamledit.ConnectorItemRange(src, ci); found {
				out, ok = yamledit.DeleteLineRange(src, s, e), true
			}
		case "duplicate:node":
			ox, oy := 40.0, 40.0
			if doc, derr := loadDocument(lang, body); derr == nil {
				if cx, cy, _, found := isotopo.ResolvePartOffset(doc, q.Get("id")); found {
					ox, oy = cx+40, cy+40
				}
			}
			out, ok = yamledit.DuplicatePart(src, q.Get("id"), ox, oy)
		default:
			http.Error(w, "op must be add|delete|duplicate", 400)
			return
		}
		if !ok {
			http.Error(w, "target not found in source", 422)
			return
		}
		svg, issues, _ := render(lang, []byte(out))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"yaml": out, "svg": svg, "issues": issues})
	})
	// The per-part gallery the footer links to. The render command writes
	// these as files; serve answers them on the fly from the CURRENT file
	// content so the link works in live mode too.
	mux.HandleFunc("GET /nodes/", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(in)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		doc, err := loadDocument(sourceLang, data)
		if err != nil {
			http.Error(w, err.Error(), 422)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/nodes/")
		if name == "" || name == "_index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(isotopo.NodesIndexHTML(isotopo.PartIDs(doc))))
			return
		}
		ext := filepath.Ext(name)
		id := strings.TrimSuffix(name, ext)
		switch ext {
		case ".svg":
			svg := isotopo.RenderParts(doc)[id]
			if svg == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write([]byte(svg))
		case ".yaml":
			f := isotopo.PartFragments(doc)[id]
			if f == nil {
				http.NotFound(w, r)
				return
			}
			y, err := isotopo.MarshalFragmentYAML(f)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.Write(y)
		case ".html":
			svg := isotopo.RenderParts(doc)[id]
			if svg == "" {
				http.NotFound(w, r)
				return
			}
			var fragYAML []byte
			if f := isotopo.PartFragments(doc)[id]; f != nil {
				fragYAML, _ = isotopo.MarshalFragmentYAML(f)
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(isotopo.NodeHTML(id, svg, string(fragYAML))))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /topology.svg", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(in)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		svg, _, _ := render(sourceLang, data)
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(svg))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(in)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		svg, _, _ := render(sourceLang, data)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Never let the browser serve a stale Studio page — the template
		// changes between builds and a cached copy would run old JS (the
		// source of the "still teleports after I fixed it" reports).
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Write([]byte(isotopo.TopologyHTML(svg, string(data), sourceLang, absIn)))
	})

	fmt.Printf("isotopo Studio · %s\nopen:  http://localhost:%s\nedits in the browser are a copy — %s is never written\n", in, port, in)
	return http.ListenAndServe("localhost:"+port, mux)
}
