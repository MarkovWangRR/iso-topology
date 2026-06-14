package isotopo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MarkovWangRR/iso-topology/yamledit"
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

// Field is one editable, visually-impactful DSL field — one row of the Studio
// detail form. Fields() returns a slice of these for an element, each with its
// current Value, an effective-value hint (Eff) for inherited/computed fields,
// and an Empty placeholder for genuinely-unset ones.
type Field struct {
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
	for _, s := range CapabilityReport().Shapes {
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
func grp(group string, collapsed bool, fs ...Field) []Field {
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
func emptyHint(f Field) string {
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

func formLayoutContainer(s string) bool {
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
func nodeSchema(shape string) []Field {
	class := shapeClass(shape)
	var f []Field

	// ── Content (open) ────────────────────────────────────────────────
	f = append(f, grp("Content", false,
		Field{Path: "label", Label: "Label", Desc: "Text rendered on the node", Type: "text"},
		Field{Path: "shape", Label: "Shape", Desc: "Geometric form of the node", Type: "choice", Options: shapeOptions()},
		Field{Path: "preset", Label: "Style preset", Desc: "Named style from theme.presets", Type: "text"},
		Field{Path: "icon", Label: "Icon", Desc: "iso://… ref, image URL, or pick a local file", Type: "icon"},
		Field{Path: "@iconColor", Label: "Icon color", Desc: "Tint for iso:// glyph/logo icons (blank = default ink)", Type: "color"},
	)...)

	// ── Size (open) ───────────────────────────────────────────────────
	size := []Field{
		{Path: "geom.w", Label: "Width", Type: "number", Inline: true},
		{Path: "geom.d", Label: "Depth", Type: "number", Inline: true},
		{Path: "geom.h", Label: "Height", Type: "number", Inline: true},
	}
	if strings.EqualFold(strings.TrimSpace(shape), "prism") {
		size = append(size, Field{Path: "geom.sides", Label: "Sides", Desc: "Polygon base sides", Type: "number", Inline: true})
	}
	f = append(f, grp("Size", false, size...)...)

	// ── Position (collapsed) — relative placement, fine offset, replicas.
	// Mostly driven by drag, but exposed for precise/declarative control.
	f = append(f, grp("Position", true,
		Field{Path: "place.rightOf", Label: "Right of", Desc: "Sibling id to sit to the right of", Type: "text", Inline: true},
		Field{Path: "place.leftOf", Label: "Left of", Type: "text", Inline: true},
		Field{Path: "place.above", Label: "Above", Type: "text", Inline: true},
		Field{Path: "place.behind", Label: "Behind", Type: "text", Inline: true},
		Field{Path: "place.inFrontOf", Label: "In front of", Type: "text", Inline: true},
		Field{Path: "place.gap", Label: "Place gap", Desc: "Gap in cells", Type: "number", Inline: true},
		Field{Path: "offset.wx", Label: "Offset X", Desc: "World-space fine-tune", Type: "number", Inline: true},
		Field{Path: "offset.wy", Label: "Offset Y", Type: "number", Inline: true},
		Field{Path: "offset.wz", Label: "Offset Z", Desc: "Lift (the axis the solver never sets)", Type: "number", Inline: true},
		Field{Path: "stack.count", Label: "Stack count", Desc: "Replica layers (1 = none)", Type: "number", Inline: true},
		Field{Path: "stack.gap", Label: "Stack gap", Desc: "Z-step between replicas", Type: "number", Inline: true},
	)...)

	// ── Layout (collapsed) — only for containers that arrange children. ─
	if formLayoutContainer(shape) {
		f = append(f, grp("Layout", true,
			Field{Path: "layout.mode", Label: "Mode", Desc: "Auto-arrange children", Type: "choice",
				Options: []string{"", "row", "column", "grid", "ring", "auto"}},
			Field{Path: "layout.gap", Label: "Gap", Desc: "Spacing between children, cells", Type: "number", Inline: true},
			Field{Path: "layout.cols", Label: "Columns", Desc: "Grid mode only", Type: "number", Inline: true},
			Field{Path: "layout.padding", Label: "Padding", Desc: "Inner margin, cells", Type: "number", Inline: true},
			Field{Path: "layout.align", Label: "Align", Desc: "Cross-axis alignment", Type: "choice", Options: alignOpts},
		)...)
	}

	// ── Fill / Gradients / Border — shape-class dependent ──────────────
	switch class {
	case "outline": // boundary — dashed region, border only
		f = append(f, grp("Border", false,
			Field{Path: "style.stroke.color", Label: "Border color", Desc: "Outline color (CSS color)", Type: "color"},
			Field{Path: "style.stroke.width", Label: "Border width", Type: "number", Inline: true},
			Field{Path: "style.stroke.dash", Label: "Border style", Desc: "Solid, dashed, or dotted", Type: "choice", Options: dashOpts, Inline: true},
		)...)
	case "text": // iso_text — no body; label IS the element (handled in Label below)
	case "fill": // circle / cloud / person — single surface colour
		f = append(f, grp("Fill", false,
			Field{Path: "style.palette.top", Label: "Fill color", Desc: "Surface fill (CSS color)", Type: "color"},
		)...)
		f = append(f, grp("Border", true,
			Field{Path: "style.stroke.color", Label: "Color", Type: "color", Inline: true},
			Field{Path: "style.stroke.width", Label: "Width", Type: "number", Inline: true},
			Field{Path: "style.stroke.dash", Label: "Dash", Type: "choice", Options: dashOpts, Inline: true},
		)...)
	default: // faces — box family, prisms, cylinder, group
		f = append(f, grp("Fill", false,
			Field{Path: "style.palette.top", Label: "Top", Type: "color", Inline: true},
			Field{Path: "style.palette.left", Label: "Left", Type: "color", Inline: true},
			Field{Path: "style.palette.right", Label: "Right", Type: "color", Inline: true},
		)...)
		f = append(f, grp("Gradients", true,
			Field{Path: "style.palette.topGradient.from", Label: "Top from", Type: "color", Inline: true},
			Field{Path: "style.palette.topGradient.to", Label: "Top to", Type: "color", Inline: true},
			Field{Path: "style.palette.topGradient.dir", Label: "Top dir", Type: "choice", Options: dirOpts, Inline: true},
			Field{Path: "style.palette.leftGradient.from", Label: "Left from", Type: "color", Inline: true},
			Field{Path: "style.palette.leftGradient.to", Label: "Left to", Type: "color", Inline: true},
			Field{Path: "style.palette.leftGradient.dir", Label: "Left dir", Type: "choice", Options: dirOpts, Inline: true},
			Field{Path: "style.palette.rightGradient.from", Label: "Right from", Type: "color", Inline: true},
			Field{Path: "style.palette.rightGradient.to", Label: "Right to", Type: "color", Inline: true},
			Field{Path: "style.palette.rightGradient.dir", Label: "Right dir", Type: "choice", Options: dirOpts, Inline: true},
		)...)
		f = append(f, grp("Border", true,
			Field{Path: "style.stroke.color", Label: "Color", Type: "color", Inline: true},
			Field{Path: "style.stroke.width", Label: "Width", Type: "number", Inline: true},
			Field{Path: "style.stroke.dash", Label: "Dash", Type: "choice", Options: dashOpts, Inline: true},
		)...)
	}

	// ── Label (open) — the caption every shape paints on its face. ─────
	f = append(f, grp("Label", false,
		Field{Path: "style.text.color", Label: "Text color", Desc: "Caption color (CSS color)", Type: "color", Inline: true},
		Field{Path: "style.text.size", Label: "Font size", Desc: "Caption size in px", Type: "number", Inline: true},
		Field{Path: "style.text.weight", Label: "Weight", Desc: "Font weight", Type: "choice", Options: weightOpts, Inline: true},
		Field{Path: "style.text.family", Label: "Font family", Desc: "CSS font stack", Type: "text"},
		Field{Path: "style.text.orient", Label: "Orientation", Desc: "iso (on-face) or screen (flat below)", Type: "choice", Options: []string{"", "iso", "screen"}},
	)...)

	// ── Effects (collapsed) — lighting, texture, transparency. Solids
	// only; text/outline have no volume to glow, shadow or texture. ─────
	if class == "faces" || class == "fill" {
		eff := []Field{}
		if class == "faces" {
			eff = append(eff, Field{Path: "style.effects.cornerRadius", Label: "Corner radius", Desc: "Rounds vertical edges", Type: "number", Inline: true})
		}
		eff = append(eff,
			Field{Path: "style.effects.opacity", Label: "Opacity", Desc: "0–1 whole-part transparency", Type: "number", Inline: true},
			Field{Path: "style.effects.blur", Label: "Blur", Desc: "Gaussian blur px — fog/ghost", Type: "number", Inline: true},
			Field{Path: "style.effects.dropShadow.color", Label: "Shadow color", Desc: "Drop shadow under the silhouette", Type: "color", Inline: true},
			Field{Path: "style.effects.dropShadow.dx", Label: "Shadow dx", Type: "number", Inline: true},
			Field{Path: "style.effects.dropShadow.dy", Label: "Shadow dy", Type: "number", Inline: true},
			Field{Path: "style.effects.dropShadow.blur", Label: "Shadow blur", Type: "number", Inline: true},
			Field{Path: "style.effects.backglow.color", Label: "Glow color", Desc: "Soft halo behind the part", Type: "color", Inline: true},
			Field{Path: "style.effects.backglow.radius", Label: "Glow radius", Type: "number", Inline: true},
			Field{Path: "style.effects.backglow.opacity", Label: "Glow opacity", Type: "number", Inline: true},
			Field{Path: "style.effects.pattern.kind", Label: "Pattern", Desc: "Top-face texture", Type: "choice", Options: []string{"", "hatch", "dots", "grid"}, Inline: true},
			Field{Path: "style.effects.pattern.color", Label: "Pattern color", Type: "color", Inline: true},
			Field{Path: "style.effects.pattern.spacing", Label: "Pattern spacing", Type: "number", Inline: true},
			Field{Path: "style.effects.pattern.angle", Label: "Pattern angle", Desc: "Degrees (hatch)", Type: "number", Inline: true},
		)
		f = append(f, grp("Effects", true, eff...)...)
	}
	return f
}

func canvasSchema() []Field {
	out := grp("Background", false,
		Field{Path: "background", Label: "Fill color", Desc: "Canvas fill behind the diagram (CSS color)", Type: "color"},
		Field{Path: "grid", Label: "Grid pattern", Desc: "Background texture", Type: "choice", Options: []string{"none", "iso", "dots", "hatch", "solid"}},
		Field{Path: "gridColor", Label: "Grid color", Desc: "Grid/texture line color (CSS color)", Type: "color"},
		Field{Path: "gridStep", Label: "Grid step", Desc: "Grid cell size in world units (blank = default)", Type: "number"},
	)
	out = append(out, grp("Layout", false,
		Field{Path: "padding", Label: "Padding", Desc: "Outer breathing margin around the scene, px", Type: "number"},
	)...)
	return out
}

func edgeSchema() []Field {
	out := grp("Connection", false,
		Field{Path: "from", Label: "From", Desc: "Source anchor — node id or node.face", Type: "text"},
		Field{Path: "to", Label: "To", Desc: "Target anchor — node id or node.face", Type: "text"},
	)
	out = append(out, grp("Routing", false,
		Field{Path: "routing", Label: "Routing", Desc: "Path style between endpoints", Type: "choice", Options: []string{"orthogonal", "straight", "bezier"}},
		Field{Path: "arrow", Label: "Arrowhead", Desc: "Marker at the target end", Type: "choice", Options: []string{"none", "triangle"}, Inline: true},
		Field{Path: "elbow", Label: "Elbow bias", Desc: "Orthogonal turn order", Type: "choice", Options: []string{"", "xFirst", "yFirst"}, Inline: true},
	)...)
	out = append(out, grp("Line", false,
		Field{Path: "stroke.color", Label: "Color", Desc: "Stroke color (CSS color)", Type: "color", Inline: true},
		Field{Path: "stroke.width", Label: "Width", Type: "number", Inline: true},
		Field{Path: "stroke.dash", Label: "Dash", Desc: "Solid, dashed, or dotted", Type: "choice", Options: dashOpts, Inline: true},
	)...)
	out = append(out, grp("Label", true,
		Field{Path: "label", Label: "Text", Desc: "Text rendered mid-route", Type: "text"},
		Field{Path: "labelColor", Label: "Text color", Type: "color", Inline: true},
		Field{Path: "labelBg", Label: "Background", Type: "color", Inline: true},
		Field{Path: "labelFontSize", Label: "Font size", Type: "number", Inline: true},
	)...)
	return out
}

// Fields returns the editable schema for one element (kind ∈ node|edge|canvas)
// with each field's current value, effective-value hint, and empty placeholder
// filled from the DSL. It is the contract behind Studio's /api/fields, lifted
// into the public library so a render service or a WASM build can produce the
// same detail form. Stateless: a pure function of (format, src, kind, id, ci).
func Fields(format string, src []byte, kind, id string, ci int) ([]Field, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}
	var subtree map[string]interface{}
	var fields []Field
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
		return nil, fmt.Errorf("unknown kind %q (want node|edge|canvas)", kind)
	}
	if subtree == nil {
		return nil, fmt.Errorf("%s %q not found in source", kind, id)
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
		if doc, err := LoadInput(context.Background(), format, src, LayoutDagre); err == nil {
			if eff := ResolvePartStyle(doc, id); eff != nil {
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
			if gw, gd, gh, okg := ResolvePartGeom(doc, id); okg {
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
	return fields, nil
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
func effValue(st *Style, path string) string {
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
