# handdrawn-sketch

**References merged**: hand-drawn, green hand-drawn, sketch+tech, wireframe hand-drawn

## Style semantics
Human, approachable, low-fidelity draft feel -- like an early product-design whiteboard /
sketch. Even technical content reads as warm rather than cold. Difference from wireframe:
the lines have **grain / wobble**, usually on a **paper background** with a faint wash fill.

## Node dimensions
- **Typical structure**: single box / stacked skeleton, loose and possibly unaligned composition.
- **Fill**: hollow or a very light wash (a pale face close to the paper colour).
- **Border**: thin ink line width 1~1.2, ink colour (charcoal / dark green / indigo), solid; slightly darker than paper.
- **Corner radius**: small (0~3), with a slightly irregular hand feel.
- **Faces**: paper-white / pale; the key is layering **grain** (intensity 0.3~0.45).
- **Light**: none or very faint; no glow.
- **Form**: cubic, slightly slim.
- **Texture**: `grain` is the soul; a very faint hatch may indicate the shaded side.

## Style prompt
> Hand-drawn sketch isometric: paper background, thin ink stroke (width ~1.1, charcoal/dark-green),
> small corners with a hand feel; faces use a near-paper pale wash + film grain (intensity 0.35)
> for a pencil/silkscreen texture; no shadow, no glow.

## Canonical DSL template
```yaml
canvas: { background: "#FBF8F1", grid: "", padding: 70 }
theme:
  text: { color: "#3A352B", family: "Inter, sans-serif" }
  presets:
    sketch:
      palette: { top: "#FBFAF5", left: "#ECE8DD", right: "#E2DDCF" }
      stroke: { color: "#3A352B", width: 1.1 }
      effects: { cornerRadius: 3, grain: { intensity: 0.38, scale: 0.8 } }
    sketchWire:                      # wireframe hand-drawn: hollow + grain
      stroke: { color: "#2E3A30", width: 1.1 }
      effects: { wireframe: true, grain: { intensity: 0.4, scale: 0.8 } }
# Green hand-drawn variant: background "#EFF6EE", stroke "#2E5B3E"
```
