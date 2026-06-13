# Capabilities — v0.3.8

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
| **boundary** | `boundary` | 1.0 | v3.5 — a group whose substrate is a dashed OUTLINE-ONLY flat region (VPC / subnet / trust zone). Same nesting/layout/autosize as group; style.stroke restyles the dashes |
| **circle** | `circle`, `oval` | 1.0 |  |
| **cloud** | `cloud` | 0.8 | free-form rounded outline; no per-face palette overrides |
| **composite** | `composite` | 1.0 | container — holds parts: [] of CompositePart entries |
| **cylinder** | `cylinder`, `queue`, `stored-data`, `stored_data` | 1.0 |  |
| **diamond** | `diamond` | 0.7 | v3.2 — 4-gon prism, base rotated 22.5 deg so the projection is a real lozenge with two shaded walls: decision / routing semantics |
| **group** | `group` | 1.0 | v2 primitive — translucent labeled substrate wrapping nested parts |
| **hexprism** | `hexagon` | 0.7 | v3.2 — 6-gon prism: API gateway / middleware semantics |
| **iso_text** | `text` | 0.3 | flat text panel (low extrusion) |
| **octprism** | `octprism` | 1.0 | v3.2 — 8-gon prism: firewall (stop-sign) semantics |
| **person** | `c4-person`, `c4_person`, `person` | 1.2 |  |
| **prism** | `prism` | 1.0 | v3.2 — regular n-gon base x vertical extrude; geom.sides picks the base (default 6). Side walls shade left/right palette by facing. Prisms take gradients/patterns/strokes via style.faces (v3.3) and backglow (v3.3.1); Full effects parity with the box family as of v3.4 (dropShadow, grain, backglow). Connectors anchor on the true polygon edge. |
| **rectangle** | `callout`, `class`, `code`, `document`, `hierarchy`, `image`, `package`, `page`, `parallelogram`, `rectangle`, `sequence-diagram`, `sequence_diagram`, `sql-table`, `sql_table`, `square`, `step` | 1.0 |  |
| **triprism** | `triprism` | 1.0 | v3.2 — 3-gon prism: alert / one-way fan-out semantics |

## Composition primitives

### `group`

**Where:** `node.parts[*]`

**Syntax:** `{shape: group, layout: {mode: row|column|grid}, parts: [...], label: "…", geom: {h}}`

Wrap N parts in a translucent labeled iso substrate. Position children with layout (or per-child place) — the substrate then auto-sizes around them, so geom.w/d may be omitted. Child offsets are fine-tune deltas relative to the group.

| Field | Meaning |
|---|---|
| `geom` | h = substrate thickness (low, default 6); w/d only when overriding the auto-sized footprint |
| `label` | rendered on the top face of the substrate (or screen-space if style.text.orient=screen) |
| `layout` | auto-arrangement of the children — preferred over per-child coordinates |
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
| `padding` | v3.1 — uniform breathing margin in px around the final composition; use 40-80 for sparse hero shots |

### `annotation`

**Where:** `document.annotations[*] | node.annotations[*] (v3.0)`

**Syntax:** `{anchor: <part-id>, text: "…", side: top|right|bottom|left, distance: 60}`

Screen-space callout pinned to a composite part. Multi-line text is supported via \n. Placement is collision-managed: the box never crosses or touches any part projection (6px clearance) and never overlaps other text; side/distance are the FIRST candidate, and the box slides to the nearest free peripheral position when occupied.

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

**Syntax:** `{from: <part-id>, to: <part-id>, routing: straight|orthogonal|bezier, arrow: none|triangle, label: "…"}`

Directed line between two parts, optionally labeled and orthogonal-routed.

| Field | Meaning |
|---|---|
| `arrow` | none = no head; triangle = filled arrowhead at the dst |
| `elbow` | v3.1 — orthogonal elbow bias: xFirst | yFirst (default: the axis the source face exits along) |
| `from` | source part id; "id.anchor" picks a specific face-centre (e.g. central.right-mid). Bare ids auto-pick the face FACING the other endpoint. |
| `labelBg` | pill background (default #FFFFFFEE) |
| `labelColor` | v3.1 — pill ink (default #1F2433); dim it for dark scenes |
| `labelFontSize` | v3.1 — pill font size (default 11) |
| `routing` | ALWAYS use orthogonal — every segment rides the iso ground axes, flush with the 2.5D grid (collinear endpoints collapse to one on-axis segment). straight/bezier cut across the grid and are reserved for non-iso freeform sketches. |
| `to` | destination part id (same anchor syntax) |

### `layout`

**Where:** `node.layout | node.parts[*].layout (groups)`

**Syntax:** `layout: {mode: row|column|grid|ring, cols: N, gap: 1, padding: 1, align: start|center|end}`

Auto-arrange a container's parts along the iso ground axes — the preferred way to position parts. No hand-computed coordinates: row marches along world +x, column along +y, grid wraps row-major after cols, ring puts the FIRST child at the centre and distributes the rest on a circle around it (hub-and-spoke in one rule). A layout group's geom.w/d may be omitted; the substrate auto-sizes around the arranged content.

| Field | Meaning |
|---|---|
| `align` | cross-axis alignment within each track (default center) |
| `cols` | grid only; default ceil(sqrt(n)) |
| `gap` | space between children (ring: hub-to-satellite clearance), in CELLS (1 cell = gridStep, default 40 world units); default 1 |
| `mode` | row | column | grid | ring (first child = hub, rest clockwise from the back) |
| `padding` | content inset from the container edge, in cells; defaults to gap |

### `place`

**Where:** `node.parts[*].place`

**Syntax:** `place: {rightOf: <sibling-id>, inFrontOf: <sibling-id>, above: <sibling-id>, gap: 1, gapX: 2, gapY: 0, align: start|center|end}`

Position a part relative to a SIBLING — the preferred way to compose free-standing scenes. One constraint per axis: rightOf/leftOf pins world x, inFrontOf/behind pins world y (front = toward viewer), above pins world z (the part sits flush ON TOP of the sibling, centred on its footprint — stacked plinths, ghost volumes, toppers). Unpinned ground axes align to the referenced sibling per align. Chains are solved topologically (a stair = each tile rightOf+inFrontOf its predecessor). offset degrades to a fine-tune delta.

| Field | Meaning |
|---|---|
| `above` | sibling id — flush on its top face (z = its top), x/y centred unless also pinned |
| `align` | alignment along the unconstrained axis (default center) |
| `behind` | sibling id — -y side (mutually exclusive with inFrontOf) |
| `gap` | distance from the sibling's footprint, in cells; default 1 |
| `gapX` | overrides gap on the x axis only |
| `gapY` | overrides gap on the y axis only |
| `inFrontOf` | sibling id — +y side, toward the viewer |
| `leftOf` | sibling id — -x side (mutually exclusive with rightOf) |
| `rightOf` | sibling id — this part sits on its +x side |

### `preset`

**Where:** `theme.presets + node.parts[*].preset`

**Syntax:** `theme: {presets: {<name>: <Style>}}  →  part: {preset: <name>}`

Named design-system style presets — define a Style once in the theme, reference it by name from any part. Merges between the per-shape defaults and the part's own style block, so parts can still override individual fields. The portable replacement for YAML anchors (works in JSON, validates with suggestions). Typical systems: hero / satellite / ghost.

| Field | Meaning |
|---|---|
| `preset` | preset name on a Node or CompositePart; unknown names are validation errors with a nearest-name suggestion |
| `theme.presets` | map of preset name to a full Style block (palette / stroke / text / effects) |

### `icon`

**Where:** `node.parts[*].icon`

**Syntax:** `icon: "iso://glyph/<name>[/light|/RRGGBB]" | "iso://si/<slug>[/light|/RRGGBB]" | "iso://brand/<name>"`

Put an icon on a part's top face instead of text — the preferred look for showcase scenes. REAL brand logos (iso://si/<slug>, ~150 vendored from Simple Icons, CC0): mysql, postgresql, redis, apachekafka, docker, kubernetes, terraform, openai, anthropic, pytorch, grafana, ...; same /light and /RRGGBB variants as glyphs; full list in docs/agent/ICONS.md. Glyphs (generic, stroke style): cloud, database, bolt, chart, globe, shield, lock, gear, cpu, code, layers, rocket, user, mobile, browser, search, bell, queue; default ink, /light = white (dark tops), /RRGGBB = custom. Brand aliases (iso://brand/<name>, resolve to the real logo in its brand color): spark, hadoop, mysql, postgresql, hive, pulsar, kafka, redis, mongo, kubernetes, docker, github, gcp, vite, rolldown, oxc. Unknown iso:// icons are validation errors with nearest-name suggestions. Any other icon value is treated as a URL / data-URI. Browsable index with previews: docs/agent/ICONS.md.

| Field | Meaning |
|---|---|
| `icon` | iso://glyph/<name>[/variant] | iso://brand/<name> | https://… | data:image/… |

## Style keys

Every field under each `style.*` sub-block.

| Block | Fields |
|---|---|
| `palette` | `top`, `left`, `right`, `topGradient {from, to, dir}`, `leftGradient {from, to, dir}`, `rightGradient {from, to, dir}` |
| `stroke` | `color`, `width`, `dash` |
| `faces (v3.3)` | `map of face name → {fill, strokes}; names: top|left|right (box family), top|side0..sideN-1 (prisms), "*" wildcard; outranks palette`, `fill {kind: solid|linearGradient|radialGradient|pattern, color, stops: [{offset 0..1, color}], angle (linear, degrees), cx/cy (radial 0..1), pattern {kind: hatch|dots, color, spacing, angle, projected}}`, `projected: true pins the pattern tile to the face's iso plane instead of screen space`, `strokes: [{color, width, dash, opacity}] — multiple re-traces per face, list order = paint order (put thin highlight lines after wide outlines)`, `note: a pattern fill REPLACES the base fill (transparent tile background); supported on the box family + prisms — cylinder/cloud/person/sphere keep palette only for now` |
| `text` | `family`, `size (a MAXIMUM — top-face labels auto-wrap at word boundaries and auto-shrink so they never overflow the face; icons are clamped to the face too)`, `weight`, `color`, `orient`, `boxBg`, `boxBorder` |
| `effects` | `opacity`, `margin`, `cornerRadius`, `dropShadow {dx, dy, blur, color}`, `backglow {color, radius, opacity}`, `blur (v3.7) — gaussian stdDev in px over the whole part; fog / ghost / de-emphasized layers`, `pattern {kind: hatch|dots, color, spacing, angle}`, `wireframe (bool — line-art: strokes only, no fills; ghost parts are exempt from overlap warnings)`, `grain {intensity 0..1, scale} (film-grain noise on the faces)` |

## See also

- [`RECIPES.md`](RECIPES.md) — task → DSL primitive lookup
- [`schema/dsl.schema.json`](schema/dsl.schema.json) — JSON Schema for local lint
- [`../reference/dsl-yaml.md`](../reference/dsl-yaml.md) — full YAML grammar with examples
