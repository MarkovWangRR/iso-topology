---
name: draw-iso-diagram
description: Generate design-grade 2.5D isometric architecture diagrams from text DSL using the isotopo CLI. Use when the user asks for an architecture diagram, system topology visual, isometric/2.5D illustration, infra or AI-pipeline graphic, or wants a diagram they can commit and diff. Produces SVG (plus per-element fragments) from YAML or d2 source.
---

# Draw an isometric architecture diagram

You are driving `isotopo`, a deterministic renderer that turns text
DSL into 2.5D isometric SVG. You never compute coordinates — you
declare relations and the solver does geometry.

## 0. Preflight

```bash
isotopo capabilities >/dev/null 2>&1 || go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest
```

Read `isotopo capabilities` once per session — it is the complete,
current inventory of shapes, primitives, and style keys you may emit.

## 1. Choose the input mode

- Plain box-and-arrow graph, no styling demands → write **`.d2`**
  (`a -> b: label`, nested `{}` for containers) and let auto-layout
  do everything.
- Composed scene (hero element, boards, stairs, styled marketing
  visual) → write **`.yaml`** and follow the rules below.

## 2. Find a scene to imitate

Pick the fixture whose description best matches the task from
[docs/agent/SAMPLES.md](https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/docs/agent/SAMPLES.md)
and mirror its structure. The gallery scenes (`ai-platform`,
`llm-serving`, `rag-pipeline`, `training-compute`) are the quality
bar and are all coordinate-free.

## 3. Author the YAML — positioning rules (hard requirements)

- NEVER hand-compute coordinates. Pick ONE anchor part (no `place`),
  chain everything else off it with
  `place: {rightOf|leftOf|inFrontOf|behind: <sibling-id>, gap: N}`;
  arrange container children with
  `layout: {mode: row|column|grid, gap: N}`. Gaps are in CELLS
  (1 cell = canvas `gridStep`, default 40 world units).
- A stair = each tile `{rightOf: prev, inFrontOf: prev}`. A board =
  one `group` with `layout: {mode: grid}` — omit its `geom.w/d`, the
  substrate auto-sizes. A ring = satellites placed on the four sides
  and four diagonals of the anchor.
- `offset` is a fine-tune delta only; `offset.wz` lifts a part
  vertically (the one axis the solver never touches).
- Sizes stay semantic: hero 150–220, standard boxes ~90–120 with
  h 30–60, thin tiles h 8–20.

Reuse styles with `theme.presets` (define `hero` / `tile` / `ghost`
once, parts say `preset: tile`) instead of repeating style blocks or
YAML anchors. Full visual playbook: `docs/guides/scene-design.md`.

Visual quality rules (what makes the output look professional):

- ONE hero per scene: gradient flanks + `backglow` + `dropShadow`.
  Everything else stays quiet. One accent hue per scene.
- Top faces carry an icon AND a short caption — not paragraphs.
  `icon: "iso://glyph/<name>"` (gpu, model, agent, chat, vector,
  lake, stream, warehouse, etl, database, shield, key, gauge, …);
  `/light` variant on dark tops, `/RRGGBB` for accent color.
- Connectors: `routing: orthogonal` for architecture flow (rides the
  iso grid), `bezier` + `dash` for async links. Hairline widths
  (1–2) look better than thick pipes.
- Dark scenes: near-black canvas (`#0A0A0B`), `grid: iso` with a
  whisper grid color, white or neon glyphs. Light scenes: off-white
  canvas, ink glyphs.

## 4. Validate until clean

```bash
isotopo validate scene.yaml
# exit 0 = clean, 2 = warnings only, 3 = errors
```

Issues are JSONPath-located and carry a `suggest` value — apply it
and re-run. Overlap warnings name the exact colliding pair: raise
that `place` gap or rearrange. Do not render while exit code is 3.

## 5. Render and verify

```bash
isotopo render scene.yaml ./out
```

`out/topology.svg` is the scene; `out/topology.html` shows it next
to the editable source. If you can rasterize (headless Chrome), take
one screenshot sized from the SVG's viewBox and check: nothing
clipped, labels legible, one hero, connectors on-grid.

## Minimal scene template

```yaml
canvas: { background: "#F5F6F8" }
theme:
  text: { family: "Inter, system-ui, sans-serif", size: 10, weight: "600", color: "#3F3F46" }
nodes:
  scene:
    shape: composite
    parts:
      - id: hero
        shape: rectangle
        geom: { w: 170, d: 170, h: 24 }
        icon: "iso://glyph/sparkles/7C5CFC"
        label: "Core"
        style:
          palette:
            top: "#FFFFFF"
            leftGradient:  { from: "#7C3AED", to: "#4C1D95", dir: "down" }
            rightGradient: { from: "#8B5CF6", to: "#5B21B6", dir: "down" }
          effects:
            cornerRadius: 14
            backglow: { color: "#A78BFA", radius: 46, opacity: 0.4 }
      - id: satellite
        shape: rectangle
        place: { rightOf: hero, gap: 2.5 }
        geom: { w: 118, d: 118, h: 12 }
        icon: "iso://glyph/database"
        label: "Store"
        style:
          palette: { top: "#FFFFFF", left: "#E2E4EA", right: "#EEF0F4" }
          effects: { cornerRadius: 12 }
    connectors:
      - { from: hero, to: satellite, routing: orthogonal, arrow: triangle, stroke: { color: "#8B5CF6", width: 1.2 } }
```

## References (fetch on demand, don't preload)

- Full DSL grammar: `docs/reference/dsl-yaml.md`
- Task → primitive mapping: `docs/agent/RECIPES.md`
- Machine-readable schema: `docs/agent/schema/dsl.schema.json`
- MCP alternative to shelling out: `docs/agent/MCP.md`
