# neon-glow

**References merged**: tech-blue breathing, dark-purple tech, blue-bg tech

## Style semantics
Futuristic, high-energy, glowing in the dark. In a dark context the nodes **self-illuminate**,
with an edge stroke brighter than the face plus a strong halo. Conveys AI / compute / realtime /
frontier. Difference from frosted: **dark bg + high saturation + bright border** (not soft and translucent).

## Node dimensions
- **Typical structure**: a single hero + satellite nodes + a connector network; the hero is brightest.
- **Fill**: solid or gradient, **highly saturated** (electric blue / magenta-purple / cyan).
- **Border**: **a neon colour brighter than the face** (the key signature!) width 1.2~1.5, solid; reads like a glowing edge.
- **Corner radius**: small to mid (3~8).
- **Faces**: saturated dark, or bright->dark gradient; top face lifted.
- **Light**: **strong `backglow`** (radius 50~66, opacity 0.45~0.6); optional `outline` bright ring; connectors can use `stroke.gradient` to glow.
- **Form**: medium height.
- **Texture**: optional glowing grid lines; background uses `grid: dots` dark dots.

## Style prompt
> Neon-tech isometric: dark background (navy/dark-purple); nodes with highly saturated faces +
> a neon stroke brighter than the face (width 1.4), strong backglow, small-mid radius; the hero
> is brightest with an outline ring; connectors use glowing gradient lines; dark dotted background.

## Canonical DSL template
```yaml
canvas: { background: "#0B0E1A", grid: dots, gridColor: "#1B2138", padding: 70 }
theme:
  text: { color: "#C9D4F0" }
  presets:
    neon:
      palette: { top: "#5C6CC0", left: "#283056", right: "#3C4684" }
      stroke: { color: "#9FB0F0", width: 1.3 }     # brighter than the face = neon
      effects: { cornerRadius: 4, backglow: { color: "#6478E0", radius: 34, opacity: 0.45 } }
    neonHero:
      palette: { top: "#8B7BFF", left: "#3B2EA8", right: "#5B49DD" }
      stroke: { color: "#C7BCFF", width: 1.5 }
      effects: { cornerRadius: 5, backglow: { color: "#7C6BFF", radius: 62, opacity: 0.55 }, outline: { color: "#C7BCFF", width: 1.5, opacity: 0.8 } }
# Blue/purple/cyan variants just swap the hue; connectors stroke.gradient { from: hero colour, to: brighter }.
```
