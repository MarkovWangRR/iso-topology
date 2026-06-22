# frosted-glass

**References merged**: borderless frosted glass, transparent frosted

## Style semantics
Premium, light, translucent. Nodes float like **semi-transparent frosted-glass blocks**,
"breathing" via a soft glow. Minimal, lots of whitespace, no hard edges -- common on
high-end SaaS / AI landing pages.

## Node dimensions
- **Typical structure**: scattered **translucent cubes**, staggered in height (lifted via wz), not dense.
- **Fill**: translucent (`effects.opacity` 0.55~0.66).
- **Border**: **none / very faint** (no border is the signature; if needed, a slightly brighter face colour, very thin).
- **Corner radius**: mid (8~12).
- **Faces**: cool pale gradient (top brightest, sides slightly deeper), low saturation (ice blue / misty white / pale purple).
- **Light**: **soft `backglow`** (radius 40~56, opacity 0.3~0.4) = the breathing; optional very faint shadow; optional `blur` for a hazier "frosted" look.
- **Form**: mostly cubic, varied sizes.
- **Texture**: none (rely on transparency + glow, not dots/grain).

## Style prompt
> Frosted-glass isometric: translucent cubes (opacity ~0.6), cool pale gradient faces,
> **no stroke**, mid radius, soft backglow for a breathing feel; pale cool background with
> lots of whitespace; cubes float at staggered heights; optional light blur for haze.

## Canonical DSL template
```yaml
canvas: { background: "#EEF3FB", grid: "", padding: 80 }
theme:
  presets:
    glass:
      palette:
        topGradient:   { from: "#DCE9FF", to: "#BFD6FA", dir: down }
        leftGradient:  { from: "#9FC0F2", to: "#7FA8EC", dir: down }
        rightGradient: { from: "#B4CFF6", to: "#93B8F0", dir: down }
      effects: { cornerRadius: 8, opacity: 0.6, backglow: { color: "#9CC2FF", radius: 46, opacity: 0.35 } }
# No stroke is essential. Deeper variant glassDeep: opacity 0.66 + deeper blue gradient.
# Scatter: give each node offset { wx, wy, wz }, staggering wz for layered depth.
```
