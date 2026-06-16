package isotopo

import (
	"sort"
	"strings"

	"oss.terrastruct.com/d2/d2target"
)

func ptrFloat(v float64) *float64 { return &v }

// resolveColor maps d2's neutral-default theme variable codes (e.g. "B1",
// "N5", "AA4") to concrete hex colours. d2 returns these codes verbatim
// in d2target.Shape.Fill / Stroke; we dereference them here so our iso
// renderer only sees real CSS colours.
var d2ThemeColors = map[string]string{
	"N1": "#0A0F25", "N2": "#676C7E", "N3": "#9499AB", "N4": "#CFD2DD",
	"N5": "#DEE1EB", "N6": "#EEF1F8", "N7": "#FFFFFF",
	"B1": "#0D32B2", "B2": "#0D32B2", "B3": "#E3E9FD",
	"B4": "#E3E9FD", "B5": "#EDF0FD", "B6": "#F7F8FE",
	"AA2": "#4A6FF3", "AA4": "#EDF0FD", "AA5": "#F7F8FE",
	"AB4": "#EDF0FD", "AB5": "#F7F8FE",
}

func resolveColor(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	if strings.HasPrefix(c, "#") {
		return c
	}
	if hex, ok := d2ThemeColors[c]; ok {
		return hex
	}
	return c
}

// d2ShapeProfile is the lowered iso shape plus per-shape extrusion hint
// (multiplier on the base IsoHeight) used to make the auto-laid-out
// scene read correctly without requiring the author to specify heights.
type d2ShapeProfile struct {
	iso       string  // one of: rectangle | cylinder | circle | cloud | person | iso_text
	heightMul float64 // 0 < x < 1 = flat panel; > 1 = tall pillar
}

// d2ShapeCatalog maps every d2 dsl shape string to its iso lowering.
// Sources: d2target/d2target.go DSL_SHAPE_TO_SHAPE_TYPE in v0.7.1.
// Anything not listed falls back to rectangle with the default height.
//
// The catalog is intentionally generous on aliases (lowercased, snake
// and hyphen variants) because agent-authored .d2 can use any.
var d2ShapeCatalog = map[string]d2ShapeProfile{
	// flat documents and labels — render as low panels
	"text":     {"iso_text", 0.3},
	"code":     {"rectangle", 0.3},
	"image":    {"rectangle", 0.3},
	"document": {"rectangle", 0.4},
	"page":     {"rectangle", 0.5},

	// data containers — cylinder family
	"cylinder":    {"cylinder", 1.0},
	"queue":       {"cylinder", 1.0},
	"stored_data": {"cylinder", 1.0},
	"stored-data": {"cylinder", 1.0},

	// round / oval — sphere
	"circle": {"circle", 1.0},
	"oval":   {"circle", 0.8},

	// people
	"person":    {"person", 1.2},
	"c4-person": {"person", 1.2},
	"c4_person": {"person", 1.2},

	// nature / external
	"cloud": {"cloud", 0.8},

	// polygon family — we don't have native polygons yet, so render as
	// rectangle. Stamping with the d2 type means future polygon support
	// can light up retroactively without rewriting this map.
	"diamond":       {"diamond", 0.7},
	"hexagon":       {"hexprism", 0.7},
	"parallelogram": {"diamond", 0.6},
	"step":          {"rectangle", 0.6},
	"callout":       {"rectangle", 0.5},

	// tapered / revolution bodies
	"cone":    {"cone", 0.8},
	"pyramid": {"pyramid", 0.8},
	"frustum": {"frustum", 0.9},
	"dome":    {"dome", 0.7},
	"capsule": {"capsule", 1.1},
	"wedge":   {"wedge", 0.7},

	// structural / sql_table / class — paint as tall rectangles so the
	// "stack of rows" look reads on the side face
	"class":     {"rectangle", 1.4},
	"sql_table": {"rectangle", 1.4},
	"sql-table": {"rectangle", 1.4},

	// container-ish — treated as box, container detection happens at the
	// graph-walking stage (Step 4), not at the leaf-shape level
	"package":          {"rectangle", 1.0},
	"rectangle":        {"rectangle", 1.0},
	"square":           {"rectangle", 1.0},
	"sequence_diagram": {"rectangle", 1.0},
	"sequence-diagram": {"rectangle", 1.0},
	"hierarchy":        {"rectangle", 1.0},
}

// mapD2Shape lowers a d2 shape-type string to an iso (shape, heightMul)
// pair. Unknown types fall back to a plain rectangle at base height.
func mapD2Shape(t string) (string, float64) {
	key := strings.ToLower(strings.TrimSpace(t))
	if p, ok := d2ShapeCatalog[key]; ok {
		return p.iso, p.heightMul
	}
	return "rectangle", 1.0
}

// Translate converts a laid-out d2 diagram into an isotopo Document
// whose single composite "scene" node holds all d2 shapes as parts and
// all d2 connections as connectors. The (X, Y) layout coords become
// world (wx, wy) on the z=0 plane; per-shape extrusion is picked by
// the d2ShapeCatalog height multiplier.
//
// d2 source-level containers (foo.bar or "foo: { bar; }") become iso
// `group` CompositeParts, with children listed in their `parts`.
// Connector ids reference the dot-separated d2 path so nothing in the
// connector translation needs special-casing.
func Translate(diagram *d2target.Diagram, opts *RenderOpts) *Document {
	if opts == nil {
		opts = DefaultRenderOpts()
	}
	baseH := opts.IsoHeight
	if baseH <= 0 {
		baseH = 30
	}

	// Index every shape by absolute id and figure out parent/children
	// relationships from id prefixes.
	byID := make(map[string]*d2target.Shape, len(diagram.Shapes))
	for i := range diagram.Shapes {
		s := &diagram.Shapes[i]
		byID[s.ID] = s
	}
	childIDs := make(map[string][]string)
	parentID := make(map[string]string)
	for id := range byID {
		dot := strings.LastIndex(id, ".")
		if dot < 0 {
			continue
		}
		pid := id[:dot]
		if _, ok := byID[pid]; !ok {
			continue
		}
		parentID[id] = pid
		childIDs[pid] = append(childIDs[pid], id)
	}

	// buildPart turns a single d2 shape into a CompositePart, recursing
	// into children when the shape is also a container.
	var buildPart func(s *d2target.Shape) *CompositePart
	buildPart = func(s *d2target.Shape) *CompositePart {
		isContainer := len(childIDs[s.ID]) > 0
		// Children's Pos in d2target are absolute coords. To lift them
		// into our group's parts: list we need their offsets relative
		// to the parent shape's origin (since lowerCompositeParts adds
		// the parent's offset back during render).
		absX, absY := float64(s.Pos.X), float64(s.Pos.Y)
		style := &Style{Palette: &Palette{Top: resolveColor(s.Fill)}}
		if sc := resolveColor(s.Stroke); sc != "" {
			w := float64(s.StrokeWidth)
			style.Stroke = &Stroke{Color: sc, Width: &w}
		}
		if isContainer {
			// Container substrate: low extrusion so children read as
			// "sitting on top". Faded palette so contents stand out.
			containerStyle := &Style{
				Palette: &Palette{
					Top:   firstNonEmpty(resolveColor(s.Fill), "#EEF1F8"),
					Left:  "#C9D1E5",
					Right: "#D9DFF0",
				},
				Stroke: &Stroke{Color: firstNonEmpty(resolveColor(s.Stroke), "#7C8DB5"),
					Width: ptrFloat(1)},
				Effects: &Effects{CornerRadius: ptrFloat(16)},
			}
			cp := &CompositePart{
				ID:    s.ID,
				Shape: "group",
				Geom:  &Geom{W: float64(s.Width), D: float64(s.Height), H: 6},
				Offset: &WorldPoint{
					WX: absX, WY: absY, WZ: 0,
				},
				Label: s.Label,
				Style: containerStyle,
			}
			// Sort children deterministically by id so output is stable.
			ids := append([]string(nil), childIDs[s.ID]...)
			sort.Strings(ids)
			for _, cid := range ids {
				child := buildPart(byID[cid])
				if child == nil {
					continue
				}
				// Rebase child offset to parent-relative coords.
				if child.Offset != nil {
					child.Offset.WX -= absX
					child.Offset.WY -= absY
				}
				cp.Parts = append(cp.Parts, child)
			}
			return cp
		}

		iso, hMul := mapD2Shape(s.Type)
		return &CompositePart{
			ID:    s.ID,
			Shape: iso,
			Geom:  &Geom{W: float64(s.Width), D: float64(s.Height), H: baseH * hMul},
			Offset: &WorldPoint{
				WX: absX, WY: absY, WZ: 0,
			},
			Label: s.Label,
			Style: style,
		}
	}

	// Walk roots only — children are pulled in recursively by buildPart.
	rootIDs := make([]string, 0)
	for id := range byID {
		if _, hasParent := parentID[id]; !hasParent {
			rootIDs = append(rootIDs, id)
		}
	}
	sort.Strings(rootIDs)

	parts := make([]*CompositePart, 0, len(rootIDs))
	for _, id := range rootIDs {
		if p := buildPart(byID[id]); p != nil {
			parts = append(parts, p)
		}
	}

	connectors := make([]*Connector, 0, len(diagram.Connections))
	for _, c := range diagram.Connections {
		conn := &Connector{
			From:    c.Src,
			To:      c.Dst,
			Label:   c.Label,
			Routing: "orthogonal",
		}
		if sc := resolveColor(c.Stroke); sc != "" {
			w := float64(c.StrokeWidth)
			dash := ""
			if c.StrokeDash > 0 {
				// d2 returns a single float for dash gap; map to SVG syntax.
				dash = "6 4"
			}
			conn.Stroke = &Stroke{Color: sc, Width: &w, Dash: dash}
		}
		if string(c.DstArrow) != "" {
			conn.Arrow = "triangle"
		}
		connectors = append(connectors, conn)
	}

	scene := &Node{
		Shape:      "composite",
		Parts:      parts,
		Connectors: connectors,
	}
	// Auto-laid-out .d2 inputs come without a Canvas block, so the
	// resulting SVG would have no backdrop and look unfinished. Default
	// to the iso-grid ground plane — it lines up with the part footprints
	// because both use the same iso projection.
	return &Document{
		Canvas: &Canvas{
			Background: "#FAFBFC",
			Grid:       "iso",
			GridColor:  "#E2E6EE",
			GridStep:   40,
		},
		Nodes: map[string]*Node{"scene": scene},
	}
}
