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
	Name       string            `json:"name"`
	Where      string            `json:"where"`   // YAML location
	Syntax     string            `json:"syntax"`  // one-liner DSL form
	Purpose    string            `json:"purpose"`
	Fields     map[string]string `json:"fields"`  // field → semantics
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
		Version: "0.2.1",
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
	for _, iso := range []string{"rectangle", "cylinder", "circle", "cloud", "person", "iso_text", "composite", "group"} {
		if _, ok := by[iso]; !ok {
			by[iso] = &bucket{isoName: iso, hMul: 1.0, acceptedAs: map[string]struct{}{iso: {}}}
		}
	}
	notes := map[string]string{
		"composite": "container — holds parts: [] of CompositePart entries",
		"group":     "v2 primitive — translucent labeled substrate wrapping nested parts",
		"iso_text":  "flat text panel (low extrusion)",
		"cloud":     "free-form rounded outline; no per-face palette overrides",
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
			Syntax:  "{shape: group, parts: [...], label: \"…\", geom: {w, d, h}, offset: {wx, wy}}",
			Purpose: "Wrap N parts in a translucent labeled iso substrate. Children's offsets are interpreted relative to the group's offset.",
			Fields: map[string]string{
				"parts": "list of CompositePart (other groups OK — supports unlimited nesting)",
				"label": "rendered on the top face of the substrate (or screen-space if style.text.orient=screen)",
				"geom":  "substrate W × D footprint; H is the substrate thickness (low, default 6)",
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
			},
		},
		{
			Name:    "annotation",
			Where:   "document.annotations[*]",
			Syntax:  "{anchor: <part-id>, text: \"…\", side: top|right|bottom|left, distance: 60}",
			Purpose: "Screen-space callout pinned to a composite part. Multi-line text is supported via \\n.",
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
			Syntax:  "{from: <part-id>, to: <part-id>, routing: straight|orthogonal, arrow: none|triangle, label: \"…\"}",
			Purpose: "Directed line between two parts, optionally labeled and orthogonal-routed.",
			Fields: map[string]string{
				"from":    "source part id; \"id.anchor\" picks a specific face-centre (e.g. central.right-mid)",
				"to":      "destination part id (same anchor syntax)",
				"routing": "straight = single segment; orthogonal = U-shape along iso ground axes",
				"arrow":   "none = no head; triangle = filled arrowhead at the dst",
			},
		},
	}
}

func buildStyleKeyGroups() []StyleKeyGroup {
	return []StyleKeyGroup{
		{"palette", []string{"top", "left", "right"}},
		{"stroke", []string{"color", "width", "dash"}},
		{"text", []string{"family", "size", "weight", "color", "orient", "boxBg", "boxBorder"}},
		{"effects", []string{"opacity", "margin", "cornerRadius"}},
	}
}
