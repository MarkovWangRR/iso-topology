package isotopo

//go:generate go run ./tools/gen-docs

import "sort"

// Capabilities is the agent-facing, structured description of what
// this build of isotopo can express. Emitted by the `isotopo
// capabilities` subcommand as JSON. Lets an agent generate valid DSL
// without reading source code.
//
// The capabilities are derived from runtime data (d2 shape catalog,
// registered primitives, layout engines) so they cannot drift from
// what the renderer actually accepts.
type Capabilities struct {
	Version    string          `json:"version"`
	Inputs     []InputFormat   `json:"inputs"`
	Layouts    []LayoutCap     `json:"layouts"`
	Shapes     []ShapeCap      `json:"shapes"`
	Primitives []PrimitiveCap  `json:"primitives"`
	StyleKeys  []StyleKeyGroup `json:"style_keys"`
}

// InputFormat describes one file extension the CLI accepts.
type InputFormat struct {
	Extension   string `json:"extension"`
	Description string `json:"description"`
	Layout      string `json:"layout"` // "auto" | "manual"
}

// LayoutCap describes one layout engine for .d2 input.
type LayoutCap struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	EdgeStyle   string `json:"edge_style"` // human-readable
}

// ShapeCap describes one shape the iso renderer can lower.
//
// AcceptedAs lists every alias an agent can use (d2 dsl + native iso
// names + snake/hyphen variants). HeightHint is the default extrusion
// multiplier — agents don't usually need to think about it but it's
// surfaced so they can pick when the answer should be "flat" or "tall".
type ShapeCap struct {
	IsoName    string   `json:"iso_name"`
	AcceptedAs []string `json:"accepted_as"`
	HeightHint float64  `json:"height_hint"`
	Notes      string   `json:"notes,omitempty"`
}

// PrimitiveCap describes one DSL composition primitive (group / stack
// / annotation / canvas-grid / ...) — the recipe an agent uses to
// express layered, repeated, or annotated structure.
type PrimitiveCap struct {
	Name    string            `json:"name"`
	Where   string            `json:"where"`  // YAML location
	Syntax  string            `json:"syntax"` // one-liner DSL form
	Purpose string            `json:"purpose"`
	Fields  map[string]string `json:"fields"` // field → semantics
}

// StyleKeyGroup describes one style sub-block (Palette, Stroke, Text,
// Effects) so an agent can fill them out correctly.
type StyleKeyGroup struct {
	Block  string   `json:"block"`
	Fields []string `json:"fields"`
}

// CapabilityReport returns the live capability set for this build.
// Pure function — no IO, no globals.
func CapabilityReport() Capabilities {
	return Capabilities{
		Version: "0.4.4",
		Inputs: []InputFormat{
			{".yaml", "hand-authored iso composite with precise placement", "manual"},
			{".json", "same shape as .yaml but JSON-encoded", "manual"},
			{".d2", "d2 graph source; isotopo runs auto-layout then translates", "auto"},
		},
		Layouts: []LayoutCap{
			{"dagre", "default; fast, natural-bend polyline edges", "polyline"},
			{"elk", "ELK; orthogonal right-angle edges with obstacle avoidance", "orthogonal"},
		},
		Shapes:     buildShapeCaps(),
		Primitives: buildPrimitiveCaps(),
		StyleKeys:  buildStyleKeyGroups(),
	}
}

// buildShapeCaps inverts d2ShapeCatalog (the canonical source of truth
// for which strings get accepted) into one entry per iso shape, with
// every alias collected and sorted.
func buildShapeCaps() []ShapeCap {
	type bucket struct {
		isoName    string
		hMul       float64
		acceptedAs map[string]struct{}
	}
	by := map[string]*bucket{}
	for d2name, prof := range d2ShapeCatalog {
		b, ok := by[prof.iso]
		if !ok {
			b = &bucket{isoName: prof.iso, hMul: prof.heightMul, acceptedAs: map[string]struct{}{}}
			by[prof.iso] = b
		}
		b.acceptedAs[d2name] = struct{}{}
	}
	// Add native iso names that aren't in the d2 catalog.
	for _, iso := range []string{"rectangle", "cylinder", "circle", "cloud", "person", "iso_text", "composite", "group",
		"prism", "diamond", "triprism", "hexprism", "octprism", "boundary"} {
		if _, ok := by[iso]; !ok {
			by[iso] = &bucket{isoName: iso, hMul: 1.0, acceptedAs: map[string]struct{}{iso: {}}}
		}
	}
	notes := map[string]string{
		"composite": "container — holds parts: [] of CompositePart entries",
		"group":     "v2 primitive — translucent labeled substrate wrapping nested parts",
		"boundary":  "v3.5 — a group whose substrate is a dashed OUTLINE-ONLY flat region (VPC / subnet / trust zone). Same nesting/layout/autosize as group; style.stroke restyles the dashes",
		"iso_text":  "flat text panel (low extrusion)",
		"cloud":     "free-form rounded outline; no per-face palette overrides",
		"prism":     "v3.2 — regular n-gon base x vertical extrude; geom.sides picks the base (default 6). Side walls shade left/right palette by facing. Prisms take gradients/patterns/strokes via style.faces (v3.3) and backglow (v3.3.1); Full effects parity with the box family as of v3.4 (dropShadow, grain, backglow). Connectors anchor on the true polygon edge.",
		"diamond":   "v3.2 — 4-gon prism, base rotated 22.5 deg so the projection is a real lozenge with two shaded walls: decision / routing semantics",
		"triprism":  "v3.2 — 3-gon prism: alert / one-way fan-out semantics",
		"hexprism":  "v3.2 — 6-gon prism: API gateway / middleware semantics",
		"octprism":  "v3.2 — 8-gon prism: firewall (stop-sign) semantics",
	}
	out := make([]ShapeCap, 0, len(by))
	for _, b := range by {
		ids := make([]string, 0, len(b.acceptedAs))
		for k := range b.acceptedAs {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		out = append(out, ShapeCap{
			IsoName:    b.isoName,
			AcceptedAs: ids,
			HeightHint: b.hMul,
			Notes:      notes[b.isoName],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IsoName < out[j].IsoName })
	return out
}

func buildPrimitiveCaps() []PrimitiveCap {
	return []PrimitiveCap{
		{
			Name:    "group",
			Where:   "node.parts[*]",
			Syntax:  "{shape: group, layout: {mode: row|column|grid}, parts: [...], label: \"…\", geom: {h}}",
			Purpose: "Wrap N parts in a translucent labeled iso substrate. Position children with layout (or per-child place) — the substrate then auto-sizes around them, so geom.w/d may be omitted. Child offsets are fine-tune deltas relative to the group.",
			Fields: map[string]string{
				"parts":  "list of CompositePart (other groups OK — supports unlimited nesting)",
				"layout": "auto-arrangement of the children — preferred over per-child coordinates",
				"label":  "rendered on the top face of the substrate (or screen-space if style.text.orient=screen)",
				"geom":   "h = substrate thickness (low, default 6); w/d only when overriding the auto-sized footprint",
			},
		},
		{
			Name:    "stack",
			Where:   "node.parts[*].stack",
			Syntax:  "stack: {count: N, gap: g}",
			Purpose: "Replicate a part N times vertically with a Z-gap. Bottom copy keeps the id; the rest are suffixed \"~k\" so connectors can target a specific layer.",
			Fields: map[string]string{
				"count": "total number of layers (1 is a no-op)",
				"gap":   "world Z step between layers (default = part.H + 4 if omitted)",
			},
		},
		{
			Name:    "canvas-grid",
			Where:   "document.canvas",
			Syntax:  "canvas: {background: \"#FAFBFC\", grid: iso, gridColor: \"#E2E6EE\", gridStep: 36}",
			Purpose: "Paint an iso-aware backdrop under the scene. grid: iso draws a diamond rhombus lattice that aligns with the part projection.",
			Fields: map[string]string{
				"background": "solid background color (also bleed color behind patterns)",
				"grid":       "iso | dots | hatch | solid | none",
				"gridColor":  "pattern stroke / dot color",
				"gridStep":   "tile size in world units (default 40)",
				"padding":    "v3.1 — uniform breathing margin in px around the final composition; use 40-80 for sparse hero shots",
			},
		},
		{
			Name:    "annotation",
			Where:   "document.annotations[*] | node.annotations[*] (v3.0)",
			Syntax:  "{anchor: <part-id>, text: \"…\", side: top|right|bottom|left, distance: 60}",
			Purpose: "Screen-space callout pinned to a composite part. Multi-line text is supported via \\n. Placement is collision-managed: the box never crosses or touches any part projection (6px clearance) and never overlaps other text; side/distance are the FIRST candidate, and the box slides to the nearest free peripheral position when occupied.",
			Fields: map[string]string{
				"anchor":   "id of the part to point at",
				"text":     "multi-line text (split on \\n)",
				"side":     "which direction the box sits relative to the part",
				"distance": "world-unit gap from part silhouette to box (default 60)",
				"bg":       "fill color (default white)",
				"border":   "stroke color (default near-black)",
			},
		},
		{
			Name:    "connector",
			Where:   "node.connectors[*]",
			Syntax:  "{from: <part-id>, to: <part-id>, routing: straight|orthogonal|bezier, arrow: none|triangle, label: \"…\"}",
			Purpose: "Directed line between two parts, optionally labeled and orthogonal-routed.",
			Fields: map[string]string{
				"from":          "source part id; \"id.anchor\" picks a specific face-centre (e.g. central.right-mid). Bare ids auto-pick the face FACING the other endpoint.",
				"to":            "destination part id (same anchor syntax)",
				"routing":       "ALWAYS use orthogonal — every segment rides the iso ground axes, flush with the 2.5D grid (collinear endpoints collapse to one on-axis segment). straight/bezier cut across the grid and are reserved for non-iso freeform sketches.",
				"arrow":         "none = no head; triangle = filled arrowhead at the dst",
				"bend":          "v4.4 — {wx, wy} world offset on the route's interior so a Studio edge-drag shifts the line while both endpoints stay docked",
				"elbow":         "v3.1 — orthogonal elbow bias: xFirst | yFirst (default: the axis the source face exits along)",
				"labelBg":       "pill background (default #FFFFFFEE)",
				"labelColor":    "v3.1 — pill ink (default #1F2433); dim it for dark scenes",
				"labelFontSize": "v3.1 — pill font size (default 11)",
			},
		},
		{
			Name:    "layout",
			Where:   "node.layout | node.parts[*].layout (groups)",
			Syntax:  "layout: {mode: row|column|grid|ring, cols: N, gap: 1, padding: 1, align: start|center|end}",
			Purpose: "Auto-arrange a container's parts along the iso ground axes — the preferred way to position parts. No hand-computed coordinates: row marches along world +x, column along +y, grid wraps row-major after cols, ring puts the FIRST child at the centre and distributes the rest on a circle around it (hub-and-spoke in one rule). A layout group's geom.w/d may be omitted; the substrate auto-sizes around the arranged content.",
			Fields: map[string]string{
				"mode":    "row | column | grid | ring | auto. auto (v4.2) is connector-driven: declare parts + connectors with NO place/coords and the engine layers the graph into a balanced iso flow — per-part style (faces/prisms/effects) is untouched. The other modes arrange a chosen structure.",
				"cols":    "grid only; default ceil(sqrt(n))",
				"gap":     "space between children (ring: hub-to-satellite clearance), in CELLS (1 cell = gridStep, default 40 world units); default 1",
				"padding": "content inset from the container edge, in cells; defaults to gap",
				"align":   "cross-axis alignment within each track (default center)",
			},
		},
		{
			Name:    "place",
			Where:   "node.parts[*].place",
			Syntax:  "place: {rightOf: <sibling-id>, inFrontOf: <sibling-id>, above: <sibling-id>, gap: 1, gapX: 2, gapY: 0, align: start|center|end}",
			Purpose: "Position a part relative to a SIBLING — the preferred way to compose free-standing scenes. One constraint per axis: rightOf/leftOf pins world x, inFrontOf/behind pins world y (front = toward viewer), above pins world z (the part sits flush ON TOP of the sibling, centred on its footprint — stacked plinths, ghost volumes, toppers). Unpinned ground axes align to the referenced sibling per align. Chains are solved topologically (a stair = each tile rightOf+inFrontOf its predecessor). offset degrades to a fine-tune delta.",
			Fields: map[string]string{
				"rightOf":   "sibling id — this part sits on its +x side",
				"leftOf":    "sibling id — -x side (mutually exclusive with rightOf)",
				"inFrontOf": "sibling id — +y side, toward the viewer",
				"behind":    "sibling id — -y side (mutually exclusive with inFrontOf)",
				"above":     "sibling id — flush on its top face (z = its top), x/y centred unless also pinned",
				"gap":       "distance from the sibling's footprint, in cells; default 1",
				"gapX":      "overrides gap on the x axis only",
				"gapY":      "overrides gap on the y axis only",
				"align":     "alignment along the unconstrained axis (default center)",
			},
		},
		{
			Name:    "preset",
			Where:   "theme.presets + node.parts[*].preset",
			Syntax:  "theme: {presets: {<name>: <Style>}}  →  part: {preset: <name>}",
			Purpose: "Named design-system style presets — define a Style once in the theme, reference it by name from any part. Merges between the per-shape defaults and the part's own style block, so parts can still override individual fields. The portable replacement for YAML anchors (works in JSON, validates with suggestions). Typical systems: hero / satellite / ghost.",
			Fields: map[string]string{
				"theme.presets": "map of preset name to a full Style block (palette / stroke / text / effects)",
				"preset":        "preset name on a Node or CompositePart; unknown names are validation errors with a nearest-name suggestion",
			},
		},
		{
			Name:    "icon",
			Where:   "node.parts[*].icon",
			Syntax:  "icon: \"iso://glyph/<name>[/light|/RRGGBB]\" | \"iso://si/<slug>[/light|/RRGGBB]\" | \"iso://brand/<name>\"",
			Purpose: "Put an icon on a part's top face instead of text — the preferred look for showcase scenes. REAL brand logos (iso://si/<slug>, ~150 vendored from Simple Icons, CC0): mysql, postgresql, redis, apachekafka, docker, kubernetes, terraform, openai, anthropic, pytorch, grafana, ...; same /light and /RRGGBB variants as glyphs; full list in docs/agent/ICONS.md. Glyphs (generic, stroke style): cloud, database, bolt, chart, globe, shield, lock, gear, cpu, code, layers, rocket, user, mobile, browser, search, bell, queue; default ink, /light = white (dark tops), /RRGGBB = custom. Brand aliases (iso://brand/<name>, resolve to the real logo in its brand color): spark, hadoop, mysql, postgresql, hive, pulsar, kafka, redis, mongo, kubernetes, docker, github, gcp, vite, rolldown, oxc. Unknown iso:// icons are validation errors with nearest-name suggestions. Any other icon value is treated as a URL / data-URI. Browsable index with previews: docs/agent/ICONS.md.",
			Fields: map[string]string{
				"icon": "iso://glyph/<name>[/variant] | iso://brand/<name> | https://… | data:image/…",
			},
		},
	}
}

func buildStyleKeyGroups() []StyleKeyGroup {
	return []StyleKeyGroup{
		{"palette", []string{
			"top", "left", "right",
			"topGradient {from, to, dir}", "leftGradient {from, to, dir}", "rightGradient {from, to, dir}",
		}},
		{"stroke", []string{"color", "width", "dash"}},
		{"faces (v3.3)", []string{
			"map of face name → {fill, strokes}; names: top|left|right (box family), top|side0..sideN-1 (prisms), \"*\" wildcard; outranks palette",
			"fill {kind: solid|linearGradient|radialGradient|pattern, color, stops: [{offset 0..1, color}], angle (linear, degrees), cx/cy (radial 0..1), pattern {kind: hatch|dots, color, spacing, angle, projected}}",
			"projected: true pins the pattern tile to the face's iso plane instead of screen space",
			"strokes: [{color, width, dash, opacity}] — multiple re-traces per face, list order = paint order (put thin highlight lines after wide outlines)",
			"note: a pattern fill REPLACES the base fill (transparent tile background); supported on the box family, prisms, and cylinders (top/left/right) — cloud/person/sphere keep palette only for now",
		}},
		{"text", []string{"family", "size (a MAXIMUM — top-face labels auto-wrap at word boundaries and auto-shrink so they never overflow the face; icons are clamped to the face too)", "weight", "color", "orient", "boxBg", "boxBorder"}},
		{"effects", []string{
			"opacity", "margin", "cornerRadius",
			"dropShadow {dx, dy, blur, color}",
			"backglow {color, radius, opacity}",
			"blur (v3.7) — gaussian stdDev in px over the whole part; fog / ghost / de-emphasized layers",
			"outline {color, width, dash, opacity} (v3.8) — accent ring along the part's full silhouette (selection / emphasis), distinct from per-face stroke; hugs the true outline of every shape",
			"pattern {kind: hatch|dots, color, spacing, angle}",
			"wireframe (bool — line-art: strokes only, no fills; ghost parts are exempt from overlap warnings)",
			"grain {intensity 0..1, scale} (film-grain noise on the faces)",
		}},
	}
}
