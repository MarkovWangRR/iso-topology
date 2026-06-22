# soft-rounded-card

**References merged**: rounded-layers (the `lustre` preset is the mature example of this family)

## Style semantics
Gentle, product UI, layered. Large radius + soft shadow make nodes feel like touchable
cards / acrylic blocks; approachable, contemporary, good for SaaS platform diagrams and a
"layered stack" narrative.

## Node dimensions
- **Typical structure**: rounded boxes; often **stacked vertically into layers** (stack) or laid out as a card grid.
- **Fill**: solid or a **very light gradient** (subtle top sheen).
- **Border**: **light**, width 1, a neutral line slightly darker than the face (or a touch same-hue-darker, weaker than solid-flat).
- **Corner radius**: **large (16~24)** -- the family signature; always pair with `faceSplit:true` so a rounded box lights each side independently.
- **Faces**: light / neutral / brand colour; the top face may use `topGradient` for a subtle lift.
- **Light**: **soft `dropShadow`** (dy 12~16, blur 20~24, low opacity) for a floating feel; generally no strong glow.
- **Form**: flat to mid (cards lean thin); many layers when stacked.
- **Texture**: none.

## Style prompt
> Soft rounded-card isometric: large radius (18~22) + faceSplit, a soft dropShadow for float,
> a light stroke (width 1); faces light or with a subtle gradient (gentle top sheen); can stack
> into clear layers; neutral/light background; clean, product feel, no glow.

## Canonical DSL template
```yaml
canvas: { background: "#F2F3F5", grid: iso, gridColor: "#E7E9ED", padding: 70 }
theme:
  presets:
    card:
      palette:
        topGradient:   { from: "#FFFFFF", to: "#F1F4FB", dir: down }
        leftGradient:  { from: "#E2E6EC", to: "#D2D7E0", dir: down }
        rightGradient: { from: "#EAEDF3", to: "#DBE0E8", dir: down }
      stroke: { color: "#D7DCE4", width: 1 }
      effects: { cornerRadius: 20, faceSplit: true, dropShadow: { dx: 0, dy: 14, blur: 22, color: "#8A95A833" } }
    cardHero:                          # brand accent card (same family)
      palette: { top: "#FFD66B", left: "#C98A12", right: "#E6A923" }
      stroke: { color: "#A9740E", width: 1 }
      effects: { cornerRadius: 22, faceSplit: true, dropShadow: { dx: 0, dy: 16, blur: 26, color: "#8A6A1A33" }, backglow: { color: "#FFD66B", radius: 50, opacity: 0.35 } }
# Dark-card variant: background #10172A, faces to a deep-blue gradient, dropShadow color #0006.
# Layered stack: node stack { count: 5, gap: 40 }.
```
