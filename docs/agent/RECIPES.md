# DSL recipes

Task-indexed quick reference. "I want to express X — which primitive
do I reach for?" Each recipe is copy-paste ready.

Both humans and agents read this. Agents should treat it as the
authoritative mapping from natural-language intent to DSL primitive.

For full field grammar see [`../reference/dsl-yaml.md`](../reference/dsl-yaml.md)
and [`../reference/dsl-d2.md`](../reference/dsl-d2.md). For the
machine-readable inventory call `isotopo capabilities` or read
[`CAPABILITIES.md`](CAPABILITIES.md).

## Layout & composition

### I want N replicas of one element

```yaml
- id: pod
  shape: rectangle
  geom: {w: 100, d: 100, h: 36}
  stack: {count: 3, gap: 10}
```

`count` = total layers, `gap` = world-Z step. Top layer's id is
`pod~2`, second from top `pod~1`, bottom keeps `pod`.

### I want to wrap N elements in a labeled container

YAML:

```yaml
- id: vpc
  shape: group
  geom:   {w: 480, d: 280, h: 6}
  label:  "AWS VPC"
  parts:
    - {id: api,  shape: rectangle, offset: {wx: 40,  wy: 40}, geom: {w: 120, d: 80, h: 30}, label: API}
    - {id: db,   shape: cylinder,  offset: {wx: 220, wy: 40}, geom: {w: 100, d: 100, h: 36}, label: DB}
```

Or `.d2`:

```d2
vpc: AWS VPC {
  api: API
  db: Database { shape: cylinder }
}
```

Children's offsets are interpreted relative to the group.

### I want a nested container hierarchy (cluster > region > pod)

YAML — nest `parts:` arbitrarily deep:

```yaml
- id: cluster
  shape: group
  geom: {w: 700, d: 400, h: 6}
  label: Cluster
  parts:
    - id: region_a
      shape: group
      geom: {w: 300, d: 300, h: 6}
      offset: {wx: 30, wy: 30}
      label: Region A
      parts:
        - {id: pod_a, shape: rectangle, offset: {wx: 30, wy: 30}, geom: {w: 100, d: 100, h: 30}}
```

Or `.d2` — same dot-path semantics:

```d2
cluster: Cluster {
  region_a: Region A {
    pod_a: Pod
  }
}
```

### I want to lay out a graph automatically

Use `.d2` instead of YAML. iso-topology runs `dagre` by default
(polyline edges) or `elk` for orthogonal right-angle routing.

```bash
isotopo render --layout elk scene.d2 ./out
```

## Annotations & connectors

### I want a labeled callout pinned to a part

Document-level `annotations:` block:

```yaml
annotations:
  - anchor: pod~2
    side: top
    text: "p95 < 100ms\nrolling update"
    fontSize: 11
```

Sides: `top` | `right` | `bottom` | `left`. Multi-line via `\n`.
Default leader-line distance is 60 world units; override with
`distance: 90`.

### I want a directed arrow between two parts

```yaml
connectors:
  - {from: api, to: db, arrow: triangle, label: "query"}
```

For specific face-centers: `from: api.right-mid`, `to: db.top-back`.

### I want orthogonal (Manhattan) routing

```yaml
connectors:
  - {from: api, to: db, routing: orthogonal, arrow: triangle}
```

The default is `straight`. Orthogonal renders as a U-shape along the
iso ground axes — reads like a PCB trace.

### I want a dashed connector

```yaml
connectors:
  - {from: a, to: b, stroke: {color: "#1F2937", width: 1, dash: "4 3"}}
```

`dash` uses SVG `stroke-dasharray` syntax: `"4 3"` = 4 on, 3 off.

## Shapes & styling

### I want a database / queue / stored-data icon

All three lower to the iso cylinder:

```yaml
- {id: db,    shape: cylinder,    label: "Postgres"}
- {id: queue, shape: queue,       label: "Job Queue"}
- {id: data,  shape: stored_data, label: "Blob"}
```

### I want a user / actor icon

```yaml
- {id: user, shape: person, geom: {w: 30, d: 30, h: 50}}
```

`c4-person` is also accepted as an alias.

### I want a cloud / external system icon

```yaml
- {id: gcp, shape: cloud, label: "GCP"}
```

### I want a text-only label (no extruded box)

```yaml
- {id: title, shape: iso_text, label: "Production Topology"}
```

Low-extrusion panel, just text on the top face. Good for titles.

### I want flat / low / tall extrusion

Adjust `geom.h`. Conventions:

| Look | h | Use for |
|---|---|---|
| Paper-flat | 4–6 | substrates, ground layers |
| Standard | 30 | typical compute / API box |
| Tall | 60–80 | server rack, prominent element |

For `.d2` input, the height comes from `d2ShapeCatalog`'s
multiplier × base height. Override per-shape via the YAML path if
you need different heights.

### I want brand colors on a specific shape

```yaml
- id: api
  shape: rectangle
  style:
    palette: {top: "#3B82F6", left: "#1D4ED8", right: "#2563EB"}
    text:    {color: "#FFFFFF", weight: "600"}
```

For document-wide branding, put it in `theme:` once and every part
inherits:

```yaml
theme:
  palette: {top: "#FFFFFF", left: "#7E94B5", right: "#A6B7D2"}
  text:    {family: Inter, weight: "600", color: "#1F2937"}
```

See [`../reference/dsl-theme.md`](../reference/dsl-theme.md) for the
full three-layer cascade.

## Canvas / backdrop

### I want an iso ground grid

```yaml
canvas:
  background: "#FAFBFC"
  grid: iso
  gridColor: "#E2E6EE"
  gridStep: 36
```

### I want dots / hatch / solid instead

```yaml
canvas: {grid: dots, gridColor: "#CCCCCC"}
canvas: {grid: hatch, gridColor: "#E2E6EE"}
canvas: {grid: solid, background: "#FAFBFC"}
```

### I want a transparent / no backdrop

Omit `canvas:` entirely.

## Common composite patterns

### Stat card (icon + value + caption)

```yaml
- id: vite_stat
  shape: composite
  parts:
    - {id: icon,    shape: rectangle, offset: {wx: 0,   wy: 0}, geom: {w: 70, d: 70, h: 20}, label: "VITE"}
    - {id: value,   shape: iso_text,  offset: {wx: 130, wy: 0}, label: "121m+"}
    - {id: caption, shape: iso_text,  offset: {wx: 130, wy: 60}, label: "weekly npm"}
```

### Region / zone container

```yaml
- id: region
  shape: group
  geom: {w: 600, d: 380, h: 8}
  label: "us-east-1"
  style:
    palette: {top: "#EEF1F8", left: "#A6ADCB", right: "#C5CADE"}
  parts: [ ... ]
```

### Stacked replicas inside a worker node

```yaml
- id: worker
  shape: group
  label: "Worker Node"
  parts:
    - id: pod
      shape: rectangle
      offset: {wx: 40, wy: 40}
      stack: {count: 3, gap: 10}
```

## Validation

### I want my agent's output checked before render

```bash
isotopo validate scene.yaml
# exit 0 = clean, 2 = warnings only, 3 = errors
```

JSON output is JSONPath-located with "did you mean" suggestions —
apply them programmatically and re-validate. See
[`PROMPT_TEMPLATE.md`](PROMPT_TEMPLATE.md) for the agent-side
boilerplate.

## When you want behaviour we don't have

These are the recurring asks. If you hit one:

| Want | Today's workaround | Long-term |
|---|---|---|
| custom polygon (n-sided) | use rectangle with explicit palette | extending iso25d/ — see [`../guides/extending.md`](../guides/extending.md) |
| wireframe / line-art look | manual stroke + transparent palette | tracked: line-only render mode |
| radial / hub-and-spoke layout | place by hand in YAML | tracked: layout helper |
| inline SVG icons inside a box | none | tracked: icon embed |
| swimlane / sequence | author with `iso_text` rows | tracked: sequence_diagram lowering |

For everything else, search this file. If it's not here and not in
the references, it's probably a [missing primitive](../guides/extending.md).
