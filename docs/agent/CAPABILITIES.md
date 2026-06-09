# Capabilities — v0.2.1

Generated from `CapabilityReport()`. Do not edit by hand — run
`go run ./tools/gen-docs` to regenerate after a code change.

Same content as `isotopo capabilities` JSON, but markdown for skim-reading.

## Input formats

| Extension | Layout | Description |
|---|---|---|
| `.yaml` | manual | hand-authored iso composite with precise placement |
| `.json` | manual | same shape as .yaml but JSON-encoded |
| `.d2` | auto | d2 graph source; isotopo runs auto-layout then translates |

## Layout engines (.d2 input)

| Name | Edges | Description |
|---|---|---|
| `dagre` | polyline | default; fast, natural-bend polyline edges |
| `elk` | orthogonal | ELK; orthogonal right-angle edges with obstacle avoidance |

## Shapes

Each row is one iso shape with every DSL alias accepted. The
height hint is the default extrusion multiplier — agents can
override per-part via `geom.h`.

| Iso shape | Accepted aliases | Height hint | Notes |
|---|---|---|---|
| **circle** | `circle`, `oval` | 0.8 |  |
| **cloud** | `cloud` | 0.8 | free-form rounded outline; no per-face palette overrides |
| **composite** | `composite` | 1.0 | container — holds parts: [] of CompositePart entries |
| **cylinder** | `cylinder`, `queue`, `stored-data`, `stored_data` | 1.0 |  |
| **group** | `group` | 1.0 | v2 primitive — translucent labeled substrate wrapping nested parts |
| **iso_text** | `text` | 0.3 | flat text panel (low extrusion) |
| **person** | `c4-person`, `c4_person`, `person` | 1.2 |  |
| **rectangle** | `callout`, `class`, `code`, `diamond`, `document`, `hexagon`, `hierarchy`, `image`, `package`, `page`, `parallelogram`, `rectangle`, `sequence-diagram`, `sequence_diagram`, `sql-table`, `sql_table`, `square`, `step` | 0.7 |  |

## Composition primitives

### `group`

**Where:** `node.parts[*]`

**Syntax:** `{shape: group, parts: [...], label: "…", geom: {w, d, h}, offset: {wx, wy}}`

Wrap N parts in a translucent labeled iso substrate. Children's offsets are interpreted relative to the group's offset.

| Field | Meaning |
|---|---|
| `geom` | substrate W × D footprint; H is the substrate thickness (low, default 6) |
| `label` | rendered on the top face of the substrate (or screen-space if style.text.orient=screen) |
| `parts` | list of CompositePart (other groups OK — supports unlimited nesting) |

### `stack`

**Where:** `node.parts[*].stack`

**Syntax:** `stack: {count: N, gap: g}`

Replicate a part N times vertically with a Z-gap. Bottom copy keeps the id; the rest are suffixed "~k" so connectors can target a specific layer.

| Field | Meaning |
|---|---|
| `count` | total number of layers (1 is a no-op) |
| `gap` | world Z step between layers (default = part.H + 4 if omitted) |

### `canvas-grid`

**Where:** `document.canvas`

**Syntax:** `canvas: {background: "#FAFBFC", grid: iso, gridColor: "#E2E6EE", gridStep: 36}`

Paint an iso-aware backdrop under the scene. grid: iso draws a diamond rhombus lattice that aligns with the part projection.

| Field | Meaning |
|---|---|
| `background` | solid background color (also bleed color behind patterns) |
| `grid` | iso | dots | hatch | solid | none |
| `gridColor` | pattern stroke / dot color |
| `gridStep` | tile size in world units (default 40) |

### `annotation`

**Where:** `document.annotations[*]`

**Syntax:** `{anchor: <part-id>, text: "…", side: top|right|bottom|left, distance: 60}`

Screen-space callout pinned to a composite part. Multi-line text is supported via \n.

| Field | Meaning |
|---|---|
| `anchor` | id of the part to point at |
| `bg` | fill color (default white) |
| `border` | stroke color (default near-black) |
| `distance` | world-unit gap from part silhouette to box (default 60) |
| `side` | which direction the box sits relative to the part |
| `text` | multi-line text (split on \n) |

### `connector`

**Where:** `node.connectors[*]`

**Syntax:** `{from: <part-id>, to: <part-id>, routing: straight|orthogonal, arrow: none|triangle, label: "…"}`

Directed line between two parts, optionally labeled and orthogonal-routed.

| Field | Meaning |
|---|---|
| `arrow` | none = no head; triangle = filled arrowhead at the dst |
| `from` | source part id; "id.anchor" picks a specific face-centre (e.g. central.right-mid) |
| `routing` | straight = single segment; orthogonal = U-shape along iso ground axes |
| `to` | destination part id (same anchor syntax) |

## Style keys

Every field under each `style.*` sub-block.

| Block | Fields |
|---|---|
| `palette` | `top`, `left`, `right` |
| `stroke` | `color`, `width`, `dash` |
| `text` | `family`, `size`, `weight`, `color`, `orient`, `boxBg`, `boxBorder` |
| `effects` | `opacity`, `margin`, `cornerRadius` |

## See also

- [`RECIPES.md`](RECIPES.md) — task → DSL primitive lookup
- [`schema/dsl.schema.json`](schema/dsl.schema.json) — JSON Schema for local lint
- [`../reference/dsl-yaml.md`](../reference/dsl-yaml.md) — full YAML grammar with examples
