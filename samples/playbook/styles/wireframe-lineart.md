# wireframe-lineart

**References merged**: black line-art, pure-black line-art, green-bg line-art

## Style semantics
The restraint of an engineering blueprint / technical schematic. **Structure-first, all
volume removed**: a node is just its outline skeleton, faces are hollow. Good for stressing
system relationships, technical rigour, understated professionalism; the background colour
(white / black / dark green) sets the temperature.

## Node dimensions
- **Typical structure**: a single box or stacked diamond/cube skeleton; often many nodes + many edges forming a network.
- **Fill**: hollow (`wireframe`), faces uncoloured.
- **Border**: **the only visual** -- a very thin line, width 1 (0.8~1.2), mostly solid; colour = ink, high-contrast vs background (white bg -> near-black `#1A1A1A`, black bg -> near-white `#EDEDED`, green bg -> dark green).
- **Corner radius**: sharp or minimal (0~2).
- **Faces**: none (hollow). An optional very faint dot pattern may hint at a surface.
- **Light**: no drop shadow, no glow (line-art must avoid volume).
- **Form**: cubic, slightly slim; nodes shrink when the network is dense.
- **Texture**: optional very faint `pattern dots`.

## Style prompt
> Pure-wireframe isometric skeleton: all faces hollow, only a thin outline, stroke width 1,
> solid lines, ink colour in strong contrast with the background; sharp corners; no fill, no
> shadow, no glow; solid background (white/black/dark-green) with a dots grid; express
> structure with many nodes + thin connectors.

## Canonical DSL template
```yaml
canvas: { background: "#FFFFFF", grid: dots, gridColor: "#E6E6E6", padding: 70 }
theme:
  presets:
    wire:
      stroke: { color: "#1A1A1A", width: 1 }
      effects: { wireframe: true, cornerRadius: 2 }
# Black-bg variant: canvas.background "#0A0A0A", stroke.color "#EDEDED"
# Green-bg variant: canvas.background "#0E2A1E", stroke.color "#BFEFD6"
# Usage: nodes use preset: wire; connectors stroke { color: same ink, width: 0.8 }
```
