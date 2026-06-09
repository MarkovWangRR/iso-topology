# Style and Theme

Every iso part has a `Style` that controls its visual presentation —
fills, stroke, label typography, opacity, etc. The `Theme` block on a
Document provides defaults; per-part `style:` overrides them.

## The four style sub-blocks

```yaml
style:
  palette: {...}       # face fills (top / left / right)
  stroke:  {...}       # silhouette stroke
  text:    {...}       # label typography
  effects: {...}       # opacity, margin, corner radius
```

Each sub-block is optional and merges independently — set just the
fields you want to change.

### `palette`

Iso parts have three visible faces (top, left, right):

```yaml
palette:
  top:   "#FFFFFF"     # top face fill (lit-from-above)
  left:  "#7E94B5"     # left face (in shadow)
  right: "#A6B7D2"     # right face (mid-tone)
```

A clean palette uses three values on a single hue at different
lightness. Skip the fields you don't want to set; the theme defaults
fill in.

For shapes without three distinct faces (cylinder, sphere, cloud,
person), `top` and `left`/`right` cover the lit and shadowed sides
respectively.

### `stroke`

```yaml
stroke:
  color: "#1F2937"     # CSS color
  width: 1.2           # SVG stroke-width
  dash:  "4 3"         # SVG stroke-dasharray syntax (or omit for solid)
```

Set `width: 0` to hide the silhouette entirely (useful for soft cards).

### `text`

Controls the label rendered on the top face (or screen-space, depending
on `orient`):

```yaml
text:
  family: "Inter, sans-serif"
  size:   12               # px
  weight: "600"            # CSS font-weight string
  color:  "#1F2937"
  orient: iso              # iso (default) | screen
  boxBg:     "#FFFFFF"     # used only with orient: screen
  boxBorder: "#1F2937"
```

`orient: iso` paints the label on the iso-tilted top face. `orient:
screen` paints a horizontal label box BELOW the part's projected
bounding box — useful for parts that need a readable caption even
when the iso angle would make on-face text hard to read.

### `effects`

```yaml
effects:
  opacity: 0.9            # 0-1, multiplied across the whole part
  margin:  4              # padding inside the iso box (for content shapes)
  cornerRadius: 8         # rounded corners on box-family parts
```

## The Theme block

A `Theme` is a `Style` plus optional per-shape-type overrides:

```yaml
theme:
  # Defaults for every part:
  palette: {top: "#FFFFFF", left: "#7E94B5", right: "#A6B7D2"}
  stroke:  {color: "#1F2937", width: 1.2}
  text:    {family: Inter, size: 12, weight: "600", color: "#1F2937"}

  # Per-shape-type overrides (only when shape matches):
  shapes:
    cylinder:
      palette: {top: "#DEE1EB", left: "#0D32B2", right: "#1641C5"}
    person:
      palette: {top: "#EBC8A9", left: "#8E673E", right: "#A77F58"}
    cloud:
      palette: {top: "#FFFFFF", left: "#A5B8DC", right: "#C2CFE6"}
      stroke:  {color: "#5E6791"}
```

## Three-layer cascade

For any part, the renderer computes its final style by merging three
sources in order. Later layers override earlier ones; a missing field
inherits from the layer above:

```
1. theme            (document-level defaults)
2. theme.shapes[s]  (shape-specific defaults — optional)
3. part.style       (per-part overrides — optional)
```

Each merge is field-level. For example, if the theme says
`palette.top: "#FFF"` and the part says only `palette.left: "#888"`,
the final part keeps the white top and gains the gray left.

### Worked example

```yaml
theme:
  palette: {top: "#FFFFFF", left: "#7E94B5", right: "#A6B7D2"}
  shapes:
    cylinder:
      palette: {top: "#DEE1EB", left: "#0D32B2", right: "#1641C5"}

nodes:
  scene:
    shape: composite
    parts:
      - id: db
        shape: cylinder           # picks up the cylinder shape defaults
        style:
          palette: {top: "#FCD34D"}   # ⇐ only override top; keep blue sides
        ...
```

Final palette for `db`:
- top:   `#FCD34D` (from part.style — wins)
- left:  `#0D32B2` (from theme.shapes.cylinder — wins over theme)
- right: `#1641C5` (from theme.shapes.cylinder)

## Stroke and effects cascade the same way

Every sub-block follows the same rule. You can layer a theme that
sets only `stroke.color` with a per-shape default that sets only
`effects.cornerRadius` with a per-part override of `palette.top` —
all four sub-blocks merge independently.

## When you don't set a theme

Skip `theme:` entirely and parts use built-in defaults (white top,
gray sides, dark stroke, Inter font). This is fine for prototyping;
introduce a theme block when you want brand-consistent output.

## When you want NO style at all

Set `style: null` on a part to start from raw defaults, ignoring the
theme. Rarely needed — usually you want some inheritance.

## See also

- [DSL_YAML.md](../reference/dsl-yaml.md) — where `style` blocks slot into a Document
- [DSL_D2.md](../reference/dsl-d2.md) — how d2 styles translate (d2 has its own
  theme system; we map a subset of its fill/stroke into our cascade)
- `isotopo capabilities | jq '.style_keys'` — programmatic source of
  truth for every field name listed above
