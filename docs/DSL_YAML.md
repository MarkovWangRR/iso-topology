# YAML composite DSL

The YAML DSL gives you **precise iso placement**: every part has an
explicit `(wx, wy, wz)` world coordinate. Use this when auto-layout
won't do — designer-controlled scenes, infographics, fixed templates.

For auto-layout from a textual graph, see [DSL_D2.md](DSL_D2.md)
instead.

JSON is also accepted (same shape, JSON encoding); the parser
sniffs the leading byte. We document YAML below.

## Document structure

```yaml
canvas:        # optional — page-level backdrop
  ...
theme:         # optional — document-wide default style
  ...
nodes:         # required — map of node id → Node
  scene:
    shape: composite
    parts:
      - ...
    connectors:
      - ...
annotations:   # optional — screen-space callouts
  - ...
```

Every full scene typically has exactly one node named **`scene`** with
`shape: composite`. That node holds all the visible parts. The library
also accepts a single-node document with any name — see
[USAGE.md](USAGE.md) for `doc.Scene()`.

## Canvas

```yaml
canvas:
  background: "#FAFBFC"   # solid color behind the scene
  grid: iso               # iso | dots | hatch | solid | none
  gridColor: "#E2E6EE"    # pattern stroke / dot color
  gridStep: 36            # tile size in world units (default 40)
```

`grid: iso` draws a dashed-diamond ground plane that aligns with the
iso projection — gives the scene a sense of place. Skip the field
entirely to get a transparent background.

## Theme

A theme is a default `Style` plus optional per-shape overrides:

```yaml
theme:
  palette:                    # default face colors for every part
    top:   "#FFFFFF"
    left:  "#7E94B5"
    right: "#A6B7D2"
  stroke: {color: "#1F2937", width: 1.2}
  text:   {family: Inter, size: 12, weight: "600", color: "#1F2937"}

  shapes:                     # per-shape-type overrides (optional)
    cylinder:
      palette: {top: "#FFC", left: "#A82", right: "#D04"}
    person:
      palette: {top: "#EBC8A9", left: "#8E673E", right: "#A77F58"}
```

Resolution order for any part: **theme.Style → theme.shapes[shape] →
part.Style** (each step overrides matching fields, nils inherit).
Full Style field reference: [DSL_THEME.md](DSL_THEME.md).

## Node

A `Node` is the top-level element. The most useful shape is
`composite` (a container holding `Parts` + optional `Connectors`):

```yaml
nodes:
  scene:
    shape: composite
    parts:
      - {id: a, shape: rectangle, geom: {w: 100, d: 100, h: 30}, label: A}
    connectors:
      - {from: a, to: b, arrow: triangle}
```

A non-composite Node (just `shape: cylinder` etc.) renders a single
iso element — convenient for sticker-sheet style outputs.

## CompositePart

One entry inside `nodes.scene.parts`:

```yaml
- id: api
  shape: rectangle
  geom:
    w: 140       # iso world X extent
    d: 90        # iso world Y extent (depth into screen)
    h: 30        # iso world Z extent (extrusion height)
  offset:
    wx: 200      # iso world X position
    wy: 80       # iso world Y position
    wz: 0        # iso world Z position (height above ground)
  label: "API Gateway"
  icon: "https://…/icon.svg"   # optional embedded icon
  style:
    palette: {top: "#3B82F6"}
    text:    {color: "#FFFFFF"}
```

The world axes form a left-handed iso frame:
- **wx** runs to the right and down (after projection),
- **wy** runs to the left and down,
- **wz** is vertical screen up.

For a beginner, just remember: bigger wx + wy = further back in the
scene. Bigger wz = taller.

## Shapes

Native iso shapes (see `isotopo capabilities`):

| Shape | Looks like |
|---|---|
| `rectangle` | iso box with top/left/right faces |
| `cylinder` | iso cylinder; also covers `queue`, `stored_data` |
| `circle` | sphere |
| `cloud` | rounded blob; flat top with depth |
| `person` | cylinder + sphere head |
| `iso_text` | low-extrusion label panel |
| `composite` | container; needs `parts` |
| `group` | composition primitive — see below |

## Composition primitives

### `group` — labeled translucent container

Wrap N parts in a translucent rounded substrate with a corner label.
Children's `offset` is interpreted **relative to the group's offset**.

```yaml
- id: cluster
  shape: group
  offset: {wx: 100, wy: 60}
  geom:   {w: 500, d: 320, h: 6}   # H is the substrate thickness
  label:  "Kubernetes Cluster"
  parts:
    - id: api
      shape: rectangle
      offset: {wx: 40, wy: 40}     # 40,40 inside the cluster
      ...
    - id: worker
      shape: rectangle
      offset: {wx: 200, wy: 40}
      ...
```

Groups nest unboundedly — a group inside a group inside a group works.

### `stack` — auto-replicate N times

Stamp a part N times vertically with a Z-gap. The bottom copy keeps
the declared id; layers above are suffixed `~1`, `~2`, …:

```yaml
- id: pods
  shape: rectangle
  geom: {w: 110, d: 110, h: 36}
  stack: {count: 3, gap: 10}     # 3 layers, 10 units apart in Z
  label: "Pods"
```

Connectors and annotations can target a specific layer:

```yaml
- {from: ingress, to: pods}      # bottom layer
- {from: ingress, to: pods~2}    # top layer
```

If `gap` is omitted, the renderer defaults to `part.geom.h + 4` so
layers visually sit atop each other.

### `connectors` — directed lines between parts

```yaml
nodes:
  scene:
    shape: composite
    parts: [...]
    connectors:
      - from: api               # part id (or "id.anchor" — see below)
        to:   db
        routing: orthogonal     # straight (default) | orthogonal
        arrow:   triangle       # none (default) | triangle
        label:   "query"
        stroke:  {color: "#1F2937", width: 1.2, dash: "4 3"}
```

Anchor selectors: `"api.right-mid"`, `"api.top-back"`, etc. — pick a
specific face-center on the source/target part. Default is the part
centroid.

### `annotations` — screen-space callouts

Document-level field (NOT inside a node):

```yaml
annotations:
  - anchor: api                  # id of a part to point at
    side:   right                # top | right | bottom | left
    distance: 60                 # world-unit gap from part silhouette
    text: "p95 < 100ms\nregion edge"
    bg:     "#FFFFFF"
    border: "#1F2937"
    fontSize: 11
```

The renderer draws a rounded text box with a dashed leader line back
to the anchor part. Multi-line text is supported via `\n`.

## Worked example

```yaml
canvas:
  background: "#FAFBFC"
  grid: iso
  gridColor: "#E2E6EE"

theme:
  palette: {top: "#FFFFFF", left: "#7E94B5", right: "#A6B7D2"}
  stroke:  {color: "#1F2937", width: 1.2}
  text:    {family: Inter, size: 12, weight: "600", color: "#1F2937"}

nodes:
  scene:
    shape: composite
    parts:
      - id: cluster
        shape: group
        offset: {wx: 0, wy: 0}
        geom:   {w: 480, d: 280, h: 6}
        label:  "Cluster"
        parts:
          - id: api
            shape: rectangle
            offset: {wx: 40, wy: 40}
            geom:   {w: 120, d: 80, h: 30}
            label:  "API"
          - id: pods
            shape: rectangle
            offset: {wx: 220, wy: 40}
            geom:   {w: 100, d: 100, h: 30}
            label:  "Pods"
            stack:  {count: 3, gap: 10}
    connectors:
      - {from: api, to: pods, routing: orthogonal, arrow: triangle}

annotations:
  - anchor: pods~2
    side:   right
    text:   "3 replicas"
    fontSize: 11
```

Render with:

```bash
isotopo render scene.yaml ./out
open ./out/topology.html
```

## Round-trip-able per-part fragments

For every CompositePart with an `id`, the CLI emits
`out/nodes/<id>.yaml` — a self-contained Document with just that one
part. Drop it back into `isotopo render` to get the standalone iso of
that single element. Useful for building icon libraries from a scene.

See [OUTPUTS.md](OUTPUTS.md) for the full output structure.

## Validating before render

```bash
isotopo validate scene.yaml
```

Catches: unknown shape names ("did you mean cylinder?"), annotation
anchors that don't resolve, missing connector targets, bad grid mode
names. JSON output with paths is ready for agent self-correction.
