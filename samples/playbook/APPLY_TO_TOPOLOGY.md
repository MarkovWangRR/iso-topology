# Applying a node style across a whole topology

Picking a family gives you a NODE grammar. A diagram has more than nodes: a canvas, a hero,
regular nodes, groups/boundaries, and connectors. To make the style cohere across the whole
picture, map the family onto EVERY element with these rules.

## The rule set
1. **One family per diagram.** Never mix families. Variation comes from *intensity within the
   family*, not from switching styles.
2. **Canvas comes from the family** (background + grid + gridColor). Dark-context families
   (neon-glow) need a dark canvas; light families need a light one — the node grammar assumes it.
3. **Every node uses the family preset.** Map the diagram's roles to family preset variants:
   - regular service/source/sink -> the base preset
   - hero -> the family's *intensified* variant (bigger radius / stronger glow / accent hue) —
     still the SAME family, it's the only node allowed to be louder
   - a few secondary nodes -> the family's dark/contrast variant for rhythm
4. **Groups / boundaries get a family-consistent tray** (see table).
5. **Connectors get a family-consistent edge** (see table).
6. **Palette discipline**: <=2-3 hues total; the hero may carry the one accent hue.

## Per-family mapping (canvas / node / hero / group tray / connector)

| family | canvas | regular node | hero (intensified) | group tray | connector |
|---|---|---|---|---|---|
| wireframe-lineart | white/black/green + `grid: dots` | `wireframe`, thin ink stroke | thicker stroke (1.6) or accent ink | `boundary` dashed ink outline | thin ink, width 0.8, solid |
| handdrawn-sketch | paper `#FBF8F1`, no grid | pale wash + `grain` | accent wash + heavier ink | translucent paper tray, ink dashed | thin ink, width 1, slight dash |
| ascii-dot | light grey, no grid | light + `pattern dots` | denser/larger dot tile | light tray w/ dots border | thin dark, width 1, dotted |
| solid-flat | white, no grid | two-tone solid + same-hue-darker border | accent two-tone, small radius | light dotted tray (`pattern dots`) | thin same-ink line, width 1.4, solid |
| gradient-face | neutral/warm, no grid | per-face gradient + faceSplit | bigger radius + soft dropShadow | translucent slab, faint border | accent line w/ `stroke.gradient` |
| frosted-glass | pale cool, no grid | translucent + soft backglow, no border | stronger backglow + larger | very faint translucent tray, no border | pale line, width 1, soft (optional dash) |
| neon-glow | dark navy + `grid: dots` | saturated + neon (brighter) border + backglow | `outline` ring + strong backglow + accent | dark translucent slab, faint neon edge | glowing `stroke.gradient`, width 1.4 |
| soft-rounded-card | light/neutral + `grid: iso` | large radius + faceSplit + soft dropShadow | accent card + backglow + bigger radius | translucent rounded tray, light dashed border | soft grey line, width 1.5, dash "1 6" |

## Worked example — weave `neon-glow` onto a 3-group platform
```yaml
canvas: { background: "#0B0E1A", grid: dots, gridColor: "#1B2138", padding: 70 }   # 1 family canvas
theme:
  text: { color: "#C9D4F0" }
  presets:                                                                          # 2 family presets
    neon:     { palette: { top: "#5C6CC0", left: "#283056", right: "#3C4684" }, stroke: { color: "#9FB0F0", width: 1.3 }, effects: { cornerRadius: 4, backglow: { color: "#6478E0", radius: 34, opacity: 0.45 } } }
    neonHero: { palette: { top: "#8B7BFF", left: "#3B2EA8", right: "#5B49DD" }, stroke: { color: "#C7BCFF", width: 1.5 }, effects: { cornerRadius: 5, backglow: { color: "#7C6BFF", radius: 62, opacity: 0.55 }, outline: { color: "#C7BCFF", width: 1.5, opacity: 0.8 } } }
    tray:     { palette: { top: "#141B33" }, stroke: { color: "#3A4570", width: 1, dash: "4 3" }, effects: { cornerRadius: 8 } }
nodes:
  scene:
    shape: composite
    parts:
      - { id: core, preset: neonHero, shape: rectangle, geom: { w: 170, d: 150, h: 34 }, label: "Core" }   # hero -> intensified
      - { id: ingest, preset: tray, shape: group, place: { leftOf: core, gapX: 3 }, layout: { mode: column, gap: 0.7 },
          parts: [ { id: a, preset: neon, shape: rectangle, geom: { w: 110, d: 70, h: 20 }, label: "A" },
                   { id: b, preset: neon, shape: rectangle, geom: { w: 110, d: 70, h: 20 }, label: "B" } ] }   # nodes -> base preset
    connectors:
      - { from: a, to: core, routing: orthogonal, arrow: triangle, stroke: { color: "#6478E0", width: 1.4, gradient: { from: "#6478E0", to: "#C7BCFF" } } }  # edge -> glowing gradient
```
## Self-check before shipping (does the style actually show?)
After render, confirm the family's `signature` (from `styles/index.json`) is visibly present:
- neon-glow -> dark canvas + borders brighter than faces + glow halos
- frosted-glass -> see-through faces + soft glow + no hard borders
- wireframe-lineart -> hollow faces, lines only
- solid-flat -> flat two-tone + crisp same-hue-darker edges
If the signature is absent, the style was not actually applied — fix before shipping. Fallback
family when nothing matches: `solid-flat`.
