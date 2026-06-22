# ascii-dot

**References merged**: bw-ascii line-art

## Style semantics
Data feel / retro terminal / print halftone. Sits between line-art and solid: faces are
filled with a **dot or hatch texture**, mostly monochrome, conveying "sampling, raster,
information density".

## Node dimensions
- **Typical structure**: flat~cubic boxes, stacked slabs; often with thin annotation leaders.
- **Fill**: light solid or half-fill -- **the face texture is the point**.
- **Border**: thin dark line width 1, monochrome (near-black / dark grey), solid.
- **Corner radius**: sharp or minimal (0~2).
- **Faces**: light grey / white base + `pattern dots` (or hatch) as texture; dot colour darker than the base.
- **Light**: none (keep the flat print feel).
- **Form**: flat to cubic.
- **Texture**: `effects.pattern{kind:dots, spacing 6~9}` is the signature; dense diagonals = hatch.

## Style prompt
> Monochrome dot-matrix / ASCII isometric: light-grey or white base, faces tiled with a
> dot-grid texture (dots darker than the base), thin dark stroke width 1, sharp corners, no
> shadow no glow; reads like a schematic/sampling diagram, high information density, retro
> terminal feel.

## Canonical DSL template
```yaml
canvas: { background: "#F5F5F7", grid: "", padding: 70 }
theme:
  presets:
    ascii:
      palette: { top: "#FBFBFD", left: "#E9E9EE", right: "#DFDFE6" }
      stroke: { color: "#23262E", width: 1 }
      effects: { cornerRadius: 2, pattern: { kind: dots, color: "#5B6070", spacing: 7 } }
    asciiHatch:                       # diagonal-line variant
      palette: { top: "#FBFBFD" }
      stroke: { color: "#23262E", width: 1 }
      effects: { pattern: { kind: hatch, color: "#5B6070", spacing: 6, angle: 45 } }
```
