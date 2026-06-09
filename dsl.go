// Package nodedsl is a layered visual DSL for 2.5D iso node rendering.
//
// The shape (discrete enum) is orthogonal to the style (continuous palette
// of visual knobs). Style is structured into four sub-blocks — Palette,
// Stroke, Text, Effects — that mirror how design systems are typically
// organised. A Theme is just a Style; per-node Style is merged on top.
//
// Node             — one element on the canvas
// ├ Shape          — d2 shape type (rectangle | cylinder | hexagon | …)
// ├ Geom           — width / depth / height / sides (polygon only)
// ├ Style          — overrides this node only
// │  ├ Palette     — top / left / right face fills (+ optional gradient)
// │  ├ Stroke      — colour, width, dash, plus per-face overrides
// │  ├ Text        — family, size, weight, colour
// │  └ Effects     — opacity, margin, cornerRadius, dropShadow, per-face opacity
// ├ Label          — string written on the top face (\n splits to multiple lines)
// ├ Icon           — URL / data-URI; ScaleHint / Offset under style.icon
// ├ Content        — structural payload for class / sql_table / sequence_diagram
// └ Direction      — tail/arrow direction for callout / step
package isotopo

// Node is the public DSL root for a single 2.5D shape.
type Node struct {
	Shape     string     `yaml:"shape" json:"shape"`
	Geom      *Geom      `yaml:"geom,omitempty" json:"geom,omitempty"`
	Style     *Style     `yaml:"style,omitempty" json:"style,omitempty"`
	Label     string     `yaml:"label,omitempty" json:"label,omitempty"`
	Icon      string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Content   *Content   `yaml:"content,omitempty" json:"content,omitempty"`
	Direction *Direction `yaml:"direction,omitempty" json:"direction,omitempty"`

	// v1.3 — composite multi-element nodes. When Shape == "composite", Parts
	// is the painter-ordered list of sub-elements stacked at iso-world offsets.
	Parts []*CompositePart `yaml:"parts,omitempty" json:"parts,omitempty"`

	// v1.3 — custom named anchors (in iso-world coords). Eight corners + six
	// face-centres are auto-derived; entries here are additive overrides.
	Anchors map[string]*WorldPoint `yaml:"anchors,omitempty" json:"anchors,omitempty"`

	// v1.4 — composite-only first-class connectors between parts. Each
	// connector references "partID.anchor" on its From/To. Resolved at
	// render time using each part's iso-projected anchor positions.
	Connectors []*Connector `yaml:"connectors,omitempty" json:"connectors,omitempty"`

	// v1.4 — fossflow-style integer iso grid. When GridStep > 0, any
	// CompositePart with `position: {i, j}` is placed at world
	// (i*GridStep, j*GridStep, 0) before applying any free-form offset.
	GridStep float64 `yaml:"gridStep,omitempty" json:"gridStep,omitempty"`
}

// Connector is a directed line between two anchor points on different
// composite parts. Style follows EdgeDSL conventions (stroke colour /
// width / dash). The From/To strings have the form "partID" or
// "partID.anchor" — e.g. "central.right-mid" or just "central" (which
// resolves to the centre of that part).
type Connector struct {
	From    string  `yaml:"from" json:"from"`
	To      string  `yaml:"to" json:"to"`
	Stroke  *Stroke `yaml:"stroke,omitempty" json:"stroke,omitempty"`
	Arrow   string  `yaml:"arrow,omitempty" json:"arrow,omitempty"`     // none | triangle (v1.4)
	Label   string  `yaml:"label,omitempty" json:"label,omitempty"`
	LabelBg string  `yaml:"labelBg,omitempty" json:"labelBg,omitempty"`

	// v1.5 — routing strategy:
	//   "straight"   — single segment from anchor to anchor (default)
	//   "orthogonal" — U-shape bending along iso ground axes (world +x then
	//                   world +y then world +x), reads like a PCB / Manhattan
	//                   route in iso projection
	Routing string `yaml:"routing,omitempty" json:"routing,omitempty"`
}

// CompositePart is one sub-element of a composite node.
type CompositePart struct {
	ID      string      `yaml:"id,omitempty" json:"id,omitempty"` // v1.4 — referenced by Connectors
	Shape   string      `yaml:"shape" json:"shape"`
	Geom    *Geom       `yaml:"geom,omitempty" json:"geom,omitempty"`
	Style   *Style      `yaml:"style,omitempty" json:"style,omitempty"`
	Label   string      `yaml:"label,omitempty" json:"label,omitempty"`
	Icon    string      `yaml:"icon,omitempty" json:"icon,omitempty"`
	Content *Content    `yaml:"content,omitempty" json:"content,omitempty"`
	Offset  *WorldPoint `yaml:"offset,omitempty" json:"offset,omitempty"`

	// v1.4 — fossflow-style iso grid position. Applied if the parent
	// composite has GridStep > 0. Multiplied by GridStep, then any Offset
	// is added on top (mixing snap + free-form).
	Position *GridPos `yaml:"position,omitempty" json:"position,omitempty"`

	// v2 — replica stack. When Count > 1, the part is expanded into N
	// identical copies stacked vertically with the given Z gap; the
	// bottom copy keeps the declared id, the rest are suffixed
	// "<id>~1", "<id>~2", … so connectors can still reach a specific
	// layer. Models server racks / HA pods / replicas without forcing
	// the author (or an agent) to copy-paste N declarations.
	Stack *Stack `yaml:"stack,omitempty" json:"stack,omitempty"`

	// v2 — nested parts. Only honored when Shape == "group". The group
	// itself renders as a low-extrusion translucent substrate; each
	// nested part is laid down on top with its Offset interpreted
	// relative to the group's own offset.
	Parts []*CompositePart `yaml:"parts,omitempty" json:"parts,omitempty"`
}

// Stack tells the renderer to replicate a composite part vertically.
// Count = total number of layers (1 is a no-op). Gap = world Z step
// between layers. If Gap <= 0 we use the part's own H + a small margin
// so layers visually sit ON TOP of each other rather than overlap.
type Stack struct {
	Count int     `yaml:"count" json:"count"`
	Gap   float64 `yaml:"gap,omitempty" json:"gap,omitempty"`
}

// GridPos is an integer iso-grid coordinate.
type GridPos struct {
	I int `yaml:"i" json:"i"`
	J int `yaml:"j" json:"j"`
}

// WorldPoint is a point in iso world coords (wx, wy, wz). Used for both
// composite offsets and node anchors.
type WorldPoint struct {
	WX float64 `yaml:"wx,omitempty" json:"wx,omitempty"`
	WY float64 `yaml:"wy,omitempty" json:"wy,omitempty"`
	WZ float64 `yaml:"wz,omitempty" json:"wz,omitempty"`
}

// Geom is the local-frame box dimensions used by the iso projection. Sides
// is consumed only by `shape: polygon` — number of sides for the polygonal
// prism (3 = triangular, 5 = pentagon, 8 = octagon, etc.).
type Geom struct {
	W     float64 `yaml:"w,omitempty" json:"w,omitempty"`
	D     float64 `yaml:"d,omitempty" json:"d,omitempty"`
	H     float64 `yaml:"h,omitempty" json:"h,omitempty"`
	Sides int     `yaml:"sides,omitempty" json:"sides,omitempty"`
}

// Style groups every visual knob. Each sub-block is a pointer so the merge
// step can distinguish "explicitly empty" from "inherit from parent".
type Style struct {
	Palette *Palette `yaml:"palette,omitempty" json:"palette,omitempty"`
	Stroke  *Stroke  `yaml:"stroke,omitempty" json:"stroke,omitempty"`
	Text    *Text    `yaml:"text,omitempty" json:"text,omitempty"`
	Effects *Effects `yaml:"effects,omitempty" json:"effects,omitempty"`
}

// Palette assigns the three iso-face fills.
type Palette struct {
	Top   string `yaml:"top,omitempty" json:"top,omitempty"`
	Left  string `yaml:"left,omitempty" json:"left,omitempty"`
	Right string `yaml:"right,omitempty" json:"right,omitempty"`
}

// Stroke describes the wireframe. Dash uses SVG stroke-dasharray syntax.
type Stroke struct {
	Color string   `yaml:"color,omitempty" json:"color,omitempty"`
	Width *float64 `yaml:"width,omitempty" json:"width,omitempty"`
	Dash  string   `yaml:"dash,omitempty" json:"dash,omitempty"`
}

// Text controls the label rendered on the top face. Orient picks how
// the label is composed:
//
//	"" / "iso"   — default. Label sits inside the top face, tilted onto
//	                the iso plane via a matrix transform.
//	"screen"     — label is rendered as a horizontal box BELOW the part's
//	                projected bounding box. Background and border are read
//	                from BoxBg / BoxBorder; otherwise transparent.
//	                Composite-only: takes effect on CompositePart entries.
type Text struct {
	Family    string   `yaml:"family,omitempty" json:"family,omitempty"`
	Size      *float64 `yaml:"size,omitempty" json:"size,omitempty"`
	Weight    string   `yaml:"weight,omitempty" json:"weight,omitempty"`
	Color     string   `yaml:"color,omitempty" json:"color,omitempty"`
	Orient    string   `yaml:"orient,omitempty" json:"orient,omitempty"`
	BoxBg     string   `yaml:"boxBg,omitempty" json:"boxBg,omitempty"`
	BoxBorder string   `yaml:"boxBorder,omitempty" json:"boxBorder,omitempty"`
}

// Effects holds compositional knobs that aren't tied to a single face.
type Effects struct {
	Opacity      *float64 `yaml:"opacity,omitempty" json:"opacity,omitempty"`
	Margin       *float64 `yaml:"margin,omitempty" json:"margin,omitempty"`
	CornerRadius *float64 `yaml:"cornerRadius,omitempty" json:"cornerRadius,omitempty"`
}

// Content is the structural payload for shapes whose visual is composed of
// rows or cells. Header lands on the top face; Rows are drawn on the
// front-right face with one row per band. Cells (v1.3) is a 2D grid drawn
// on the TOP face (iso-tilted via the top matrix).
type Content struct {
	Header    string     `yaml:"header,omitempty" json:"header,omitempty"`
	Rows      []string   `yaml:"rows,omitempty" json:"rows,omitempty"`
	RowColors []string   `yaml:"rowColors,omitempty" json:"rowColors,omitempty"`
	Cells     [][]string `yaml:"cells,omitempty" json:"cells,omitempty"`     // [row][col]
	CellColors [][]string `yaml:"cellColors,omitempty" json:"cellColors,omitempty"`
}

// Direction is the tail/arrow orientation for callout / step.
//
//	callout.tail ∈ {"down", "up", "left", "right"}    (default: "down")
//	step.arrow   ∈ {"right", "left"}                  (default: "right")
type Direction struct {
	Tail  string   `yaml:"tail,omitempty" json:"tail,omitempty"`
	Arrow string   `yaml:"arrow,omitempty" json:"arrow,omitempty"`
	Size  *float64 `yaml:"size,omitempty" json:"size,omitempty"`
}

// Theme is the document-level default Style plus optional per-shape-type
// defaults. Resolution order for any node:
//
//	theme (base)  →  theme.shapes[node.Shape]  →  node.Style
//
// Each step is a field-level merge: later non-empty fields override
// earlier ones; nil sub-blocks inherit.
type Theme struct {
	Style  `yaml:",inline" json:",inline"`
	Shapes map[string]*Style `yaml:"shapes,omitempty" json:"shapes,omitempty"`
}

// Canvas is composition-level metadata for a Document. v2 extended:
// `grid` selects an iso-aware backdrop ("iso" = diamond rhombus lattice,
// "dots" = vertex dots, "hatch" = single-axis stripes, "" = none) so
// authors can pick the right "ground" without computing pattern math.
type Canvas struct {
	Background string  `yaml:"background,omitempty" json:"background,omitempty"`
	Grid       string  `yaml:"grid,omitempty" json:"grid,omitempty"`
	GridColor  string  `yaml:"gridColor,omitempty" json:"gridColor,omitempty"`
	GridStep   float64 `yaml:"gridStep,omitempty" json:"gridStep,omitempty"`
}

// Annotation is a screen-space callout pinned to a composite part. It
// renders as a rounded text box (multi-line if Text contains '\n') with
// a thin leader line back to the anchor part's projected silhouette.
//
// Anchor is the id of a CompositePart inside the document's "scene"
// node. Side picks which direction the box sits relative to the
// anchor's projected centre: "top" | "right" | "bottom" | "left"
// (default "right"). Distance is the world-unit gap from the part
// silhouette to the box (default 60).
type Annotation struct {
	Anchor   string  `yaml:"anchor" json:"anchor"`
	Text     string  `yaml:"text" json:"text"`
	Side     string  `yaml:"side,omitempty" json:"side,omitempty"`
	Distance float64 `yaml:"distance,omitempty" json:"distance,omitempty"`
	Bg       string  `yaml:"bg,omitempty" json:"bg,omitempty"`
	Border   string  `yaml:"border,omitempty" json:"border,omitempty"`
	Color    string  `yaml:"color,omitempty" json:"color,omitempty"`
	FontSize float64 `yaml:"fontSize,omitempty" json:"fontSize,omitempty"`
}

// Document bundles a Canvas + Theme + Nodes. This is the canonical
// top-level shape of a NodeDSL .yaml or .json file.
type Document struct {
	Canvas      *Canvas          `yaml:"canvas,omitempty" json:"canvas,omitempty"`
	Theme       *Theme           `yaml:"theme,omitempty" json:"theme,omitempty"`
	Nodes       map[string]*Node `yaml:"nodes" json:"nodes"`
	Annotations []*Annotation    `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// Scene returns the document-level composite node that should pick up
// Canvas + Annotations layers when rendered. The canonical convention
// is a node named "scene" inside Nodes; if there is exactly one node
// and no "scene" key, that node is treated as the scene. Returns nil
// if the document is empty.
//
// Use this in preference to `doc.Nodes["scene"]` directly — the helper
// stays correct even for single-node documents that don't bother
// naming the entry "scene".
func (d *Document) Scene() *Node {
	if d == nil || len(d.Nodes) == 0 {
		return nil
	}
	if n := d.Nodes["scene"]; n != nil {
		return n
	}
	if len(d.Nodes) == 1 {
		for _, n := range d.Nodes {
			return n
		}
	}
	return nil
}
