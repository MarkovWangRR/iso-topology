# gradient-face

**References merged**: red-gradient, dark frosted gradient

## Style semantics
Modern, dynamic, warm/cool tension. Each face is a **linear gradient** (light->dark), more
"juicy" and voluminous than flat colour -- common for hero / landing-page key art.
Red-orange = heat, blue-purple = tech.

## Node dimensions
- **Typical structure**: a single block or a **stacked slab tower** (layered gradients look great).
- **Fill**: solid, but each face is a gradient.
- **Border**: **faint or none** (let the gradient lead); if present, use the face's dark-end colour, width 1.
- **Corner radius**: small to mid (6~10), with `faceSplit:true` so a rounded box lights each side independently.
- **Faces**: `topGradient/leftGradient/rightGradient`, top bright->mid, sides mid->dark (all dir: down).
- **Light**: optional soft shadow; generally no strong glow (distinguishes from neon).
- **Form**: mid to tall; for a slab tower each layer is flat but many are stacked.
- **Texture**: none.

## Style prompt
> Gradient-face isometric: each face a linear gradient (light->dark, dir down), top brightest,
> sides darker, faint or no border, small-mid radius + faceSplit; can stack into a tower of
> layered gradients; soft shadow optional, no strong glow; neutral/warm background.

## Canonical DSL template
```yaml
canvas: { background: "#F4F1EA", grid: "", padding: 80 }
theme:
  presets:
    grad:
      palette:
        topGradient:   { from: "#FF8A4C", to: "#E23B3B", dir: down }
        leftGradient:  { from: "#C42A2A", to: "#8E1B1B", dir: down }
        rightGradient: { from: "#E5402F", to: "#B62121", dir: down }
      stroke: { color: "#7E1414", width: 1 }
      effects: { cornerRadius: 8, faceSplit: true }
# Blue-purple tech variant: swap from/to to #6EA0FF->#2E5BDC (top) / #274C9E->#15265E (left), etc.
# Slab tower: node geom h small (~18) + stack { count: 9, gap: 26 }.
```
