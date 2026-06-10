// gen-docs writes two agent-facing files from the live capability
// surface so they can't drift from code:
//
//	docs/agent/CAPABILITIES.md   markdown rendering of CapabilityReport()
//	docs/agent/schema/dsl.schema.json   JSON Schema describing the YAML DSL
//
// Both are committed to the repo (so they're readable on GitHub raw,
// usable by agents offline). Re-run after any DSL or shape catalog
// change. CI can guard against drift with `git diff --exit-code`.
//
// Usage:
//
//	go run ./tools/gen-docs
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	isotopo "github.com/MarkovWangRR/iso-topology"
)

func main() {
	if err := writeCapabilitiesMarkdown(); err != nil {
		fmt.Fprintln(os.Stderr, "CAPABILITIES.md:", err)
		os.Exit(1)
	}
	if err := writeDSLSchema(); err != nil {
		fmt.Fprintln(os.Stderr, "dsl.schema.json:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ok: wrote docs/agent/CAPABILITIES.md and docs/agent/schema/dsl.schema.json")
}

// writeCapabilitiesMarkdown renders CapabilityReport() into a
// human-skimmable markdown file. Same data as `isotopo capabilities`
// but committed so it's readable on GitHub without running the CLI.
func writeCapabilitiesMarkdown() error {
	cap := isotopo.CapabilityReport()
	var b strings.Builder

	fmt.Fprintf(&b, "# Capabilities — v%s\n\n", cap.Version)
	b.WriteString("Generated from `CapabilityReport()`. Do not edit by hand — run\n")
	b.WriteString("`go run ./tools/gen-docs` to regenerate after a code change.\n\n")
	b.WriteString("Same content as `isotopo capabilities` JSON, but markdown for skim-reading.\n\n")

	b.WriteString("## Input formats\n\n")
	b.WriteString("| Extension | Layout | Description |\n|---|---|---|\n")
	for _, in := range cap.Inputs {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", in.Extension, in.Layout, in.Description)
	}

	b.WriteString("\n## Layout engines (.d2 input)\n\n")
	b.WriteString("| Name | Edges | Description |\n|---|---|---|\n")
	for _, l := range cap.Layouts {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", l.Name, l.EdgeStyle, l.Description)
	}

	b.WriteString("\n## Shapes\n\n")
	b.WriteString("Each row is one iso shape with every DSL alias accepted. The\n")
	b.WriteString("height hint is the default extrusion multiplier — agents can\n")
	b.WriteString("override per-part via `geom.h`.\n\n")
	b.WriteString("| Iso shape | Accepted aliases | Height hint | Notes |\n|---|---|---|---|\n")
	for _, s := range cap.Shapes {
		aliases := "`" + strings.Join(s.AcceptedAs, "`, `") + "`"
		fmt.Fprintf(&b, "| **%s** | %s | %.1f | %s |\n",
			s.IsoName, aliases, s.HeightHint, s.Notes)
	}

	b.WriteString("\n## Composition primitives\n\n")
	for _, p := range cap.Primitives {
		fmt.Fprintf(&b, "### `%s`\n\n", p.Name)
		fmt.Fprintf(&b, "**Where:** `%s`\n\n", p.Where)
		fmt.Fprintf(&b, "**Syntax:** `%s`\n\n", p.Syntax)
		fmt.Fprintf(&b, "%s\n\n", p.Purpose)
		if len(p.Fields) > 0 {
			b.WriteString("| Field | Meaning |\n|---|---|\n")
			keys := make([]string, 0, len(p.Fields))
			for k := range p.Fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&b, "| `%s` | %s |\n", k, p.Fields[k])
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## Style keys\n\n")
	b.WriteString("Every field under each `style.*` sub-block.\n\n")
	b.WriteString("| Block | Fields |\n|---|---|\n")
	for _, g := range cap.StyleKeys {
		fmt.Fprintf(&b, "| `%s` | %s |\n", g.Block, "`"+strings.Join(g.Fields, "`, `")+"`")
	}

	b.WriteString("\n## See also\n\n")
	b.WriteString("- [`RECIPES.md`](RECIPES.md) — task → DSL primitive lookup\n")
	b.WriteString("- [`schema/dsl.schema.json`](schema/dsl.schema.json) — JSON Schema for local lint\n")
	b.WriteString("- [`../reference/dsl-yaml.md`](../reference/dsl-yaml.md) — full YAML grammar with examples\n")

	return os.WriteFile("docs/agent/CAPABILITIES.md", []byte(b.String()), 0o644)
}

// writeDSLSchema writes a JSON Schema (draft 2020-12) describing the
// YAML/JSON Document shape. Hand-written rather than reflected from
// struct tags so we can include semantic guidance (descriptions,
// enums, examples) that pure reflection can't produce.
//
// Agents can use this to lint candidate DSL locally before sending
// to `isotopo validate` — fewer network round-trips, faster loops.
func writeDSLSchema() error {
	cap := isotopo.CapabilityReport()

	// Collect shape names from CapabilityReport for the shape enum.
	allShapes := map[string]struct{}{}
	for _, s := range cap.Shapes {
		allShapes[s.IsoName] = struct{}{}
		for _, a := range s.AcceptedAs {
			allShapes[a] = struct{}{}
		}
	}
	shapeEnum := make([]string, 0, len(allShapes))
	for k := range allShapes {
		shapeEnum = append(shapeEnum, k)
	}
	sort.Strings(shapeEnum)

	schema := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://github.com/MarkovWangRR/iso-topology/blob/main/docs/agent/schema/dsl.schema.json",
		"title":       "iso-topology Document",
		"description": fmt.Sprintf("YAML/JSON DSL accepted by isotopo render. Generated for capabilities v%s.", cap.Version),
		"type":        "object",
		"required":    []string{"nodes"},
		"properties": map[string]any{
			"canvas":      canvasSchema(),
			"theme":       themeSchema(),
			"nodes":       nodesMapSchema(shapeEnum),
			"annotations": annotationsSchema(),
		},
	}

	enc, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("docs/agent/schema", 0o755); err != nil {
		return err
	}
	return os.WriteFile("docs/agent/schema/dsl.schema.json", append(enc, '\n'), 0o644)
}

func canvasSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Document-level backdrop (iso ground grid, solid color, etc.)",
		"properties": map[string]any{
			"background": map[string]any{"type": "string", "description": "Solid background color (CSS color)."},
			"grid":       map[string]any{"type": "string", "enum": []string{"iso", "dots", "hatch", "solid", "none"}, "description": "Backdrop pattern. iso = diamond rhombus lattice."},
			"gridColor":  map[string]any{"type": "string", "description": "Pattern stroke / dot color."},
			"gridStep":   map[string]any{"type": "number", "description": "Tile size in world units (default 40)."},
		},
	}
}

func themeSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Document-wide default Style plus optional per-shape-type overrides.",
		"properties": map[string]any{
			"palette": styleBlockSchema("Face fills (top/left/right), each optionally a {from, to, dir} gradient via topGradient/leftGradient/rightGradient.", []string{"top", "left", "right", "topGradient", "leftGradient", "rightGradient"}),
			"stroke":  styleBlockSchema("Silhouette stroke.", []string{"color", "width", "dash"}),
			"text":    styleBlockSchema("Label typography.", []string{"family", "size", "weight", "color", "orient", "boxBg", "boxBorder"}),
			"effects": styleBlockSchema("Visual modifiers. dropShadow {dx, dy, blur, color}; backglow {color, radius, opacity}; pattern {kind, color, spacing, angle}.", []string{"opacity", "margin", "cornerRadius", "dropShadow", "backglow", "pattern"}),
			"shapes": map[string]any{
				"type":                 "object",
				"description":          "Per-shape-type Style overrides. Keys are iso shape names (rectangle, cylinder, …).",
				"additionalProperties": styleSchema(),
			},
		},
	}
}

func styleBlockSchema(desc string, fields []string) map[string]any {
	props := map[string]any{}
	for _, f := range fields {
		props[f] = map[string]any{"description": "see reference/dsl-theme.md"}
	}
	return map[string]any{
		"type":        "object",
		"description": desc,
		"properties":  props,
	}
}

func styleSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Style block (Palette, Stroke, Text, Effects).",
		"properties": map[string]any{
			"palette": map[string]any{"type": "object"},
			"stroke":  map[string]any{"type": "object"},
			"text":    map[string]any{"type": "object"},
			"effects": map[string]any{"type": "object"},
		},
	}
}

func nodesMapSchema(shapeEnum []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Map of node id → Node. The 'scene' node (or the single node if there's only one) is treated as the topology root and picks up Canvas + Annotations.",
		"additionalProperties": nodeSchema(shapeEnum),
	}
}

func nodeSchema(shapeEnum []string) map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"shape"},
		"properties": map[string]any{
			"shape":      map[string]any{"type": "string", "enum": shapeEnum, "description": "One of the iso shapes from capabilities."},
			"geom":       geomSchema(),
			"style":      styleSchema(),
			"label":      map[string]any{"type": "string"},
			"icon":       map[string]any{"type": "string"},
			"content":    map[string]any{"type": "object"},
			"parts":      map[string]any{"type": "array", "items": compositePartSchema(shapeEnum), "description": "Children — only consulted when shape == composite or group."},
			"connectors": map[string]any{"type": "array", "items": connectorSchema(), "description": "Directed lines between parts (composite only)."},
			"gridStep":   map[string]any{"type": "number", "description": "Iso grid step for fossflow-style position: {i, j} placement; also the layout/place cell size."},
			"layout":     layoutSchema(),
		},
	}
}

func compositePartSchema(shapeEnum []string) map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"shape"},
		"properties": map[string]any{
			"id":       map[string]any{"type": "string", "description": "Stable id — referenced by connectors and annotations."},
			"shape":    map[string]any{"type": "string", "enum": shapeEnum},
			"geom":     geomSchema(),
			"style":    styleSchema(),
			"label":    map[string]any{"type": "string"},
			"icon":     map[string]any{"type": "string"},
			"content":  map[string]any{"type": "object"},
			"offset":   worldPointSchema(),
			"position": map[string]any{"type": "object", "properties": map[string]any{"i": map[string]any{"type": "integer"}, "j": map[string]any{"type": "integer"}}},
			"stack":    stackSchema(),
			"parts":    map[string]any{"type": "array", "description": "Nested parts — only honored when shape == group."},
			"place":    placeSchema(),
			"layout":   layoutSchema(),
		},
	}
}

func placeSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Declarative position relative to a SIBLING part — preferred over offset. rightOf/leftOf pin world x, inFrontOf/behind pin world y (front = toward viewer). Chains solve topologically; offset degrades to a fine-tune delta.",
		"properties": map[string]any{
			"rightOf":   map[string]any{"type": "string", "description": "Sibling part id — this part sits on its +x side."},
			"leftOf":    map[string]any{"type": "string", "description": "Sibling part id — -x side. Mutually exclusive with rightOf."},
			"inFrontOf": map[string]any{"type": "string", "description": "Sibling part id — +y side (toward viewer)."},
			"behind":    map[string]any{"type": "string", "description": "Sibling part id — -y side. Mutually exclusive with inFrontOf."},
			"gap":       map[string]any{"type": "number", "description": "Distance from the sibling's footprint in CELLS (1 cell = gridStep, default 40 world units). Default 1."},
			"align":     map[string]any{"type": "string", "enum": []string{"start", "center", "end"}, "description": "Alignment along the unconstrained axis (default center)."},
		},
	}
}

func layoutSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Auto-arrange this container's parts along the iso ground axes — no hand-computed coordinates. On a group, geom.w/d may be omitted (substrate auto-sizes around content).",
		"required":    []string{"mode"},
		"properties": map[string]any{
			"mode":    map[string]any{"type": "string", "enum": []string{"row", "column", "grid"}, "description": "row = world +x, column = world +y, grid = row-major wrap."},
			"cols":    map[string]any{"type": "integer", "description": "Grid mode only. Default ceil(sqrt(n))."},
			"gap":     map[string]any{"type": "number", "description": "Space between children in cells. Default 1."},
			"padding": map[string]any{"type": "number", "description": "Content inset from container edge in cells. Defaults to gap."},
			"align":   map[string]any{"type": "string", "enum": []string{"start", "center", "end"}, "description": "Cross-axis alignment within each track (default center)."},
		},
	}
}

func geomSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"w":     map[string]any{"type": "number", "description": "Iso world X extent."},
			"d":     map[string]any{"type": "number", "description": "Iso world Y (depth) extent."},
			"h":     map[string]any{"type": "number", "description": "Iso world Z (height) extrusion."},
			"sides": map[string]any{"type": "integer", "description": "Polygon sides (polygon shape only)."},
		},
	}
}

func worldPointSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"wx": map[string]any{"type": "number"},
			"wy": map[string]any{"type": "number"},
			"wz": map[string]any{"type": "number"},
		},
	}
}

func stackSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"count"},
		"properties": map[string]any{
			"count": map[string]any{"type": "integer", "minimum": 1, "description": "Total number of layers."},
			"gap":   map[string]any{"type": "number", "description": "World-Z step between layers. Defaults to part.geom.h + 4 if omitted."},
		},
	}
}

func connectorSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"from", "to"},
		"properties": map[string]any{
			"from":    map[string]any{"type": "string", "description": "Source part id (or id.anchor for a specific face center)."},
			"to":      map[string]any{"type": "string", "description": "Destination part id (same syntax)."},
			"label":   map[string]any{"type": "string"},
			"labelBg": map[string]any{"type": "string"},
			"arrow":   map[string]any{"type": "string", "enum": []string{"none", "triangle"}},
			"routing": map[string]any{"type": "string", "enum": []string{"straight", "orthogonal", "bezier"}, "description": "orthogonal bends along iso ground axes (grid-aligned); bezier = soft quadratic arc."},
			"stroke": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"color": map[string]any{"type": "string"},
					"width": map[string]any{"type": "number"},
					"dash":  map[string]any{"type": "string", "description": "SVG stroke-dasharray syntax e.g. \"4 3\"."},
				},
			},
		},
	}
}

func annotationsSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":     "object",
			"required": []string{"anchor", "text"},
			"properties": map[string]any{
				"anchor":   map[string]any{"type": "string", "description": "Id of the part the callout points at."},
				"text":     map[string]any{"type": "string", "description": "Multi-line via \\n."},
				"side":     map[string]any{"type": "string", "enum": []string{"top", "right", "bottom", "left"}},
				"distance": map[string]any{"type": "number"},
				"bg":       map[string]any{"type": "string"},
				"border":   map[string]any{"type": "string"},
				"color":    map[string]any{"type": "string"},
				"fontSize": map[string]any{"type": "number"},
			},
		},
	}
}
