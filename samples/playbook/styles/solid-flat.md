# solid-flat

**References merged**: structuralism, solid-colour tech

## Style semantics
Clean, assertive, modern flat isometric illustration. Solid two-tone faces, clear geometry --
professional yet not cold. The most general, most reliable "solid family" baseline.

## Node dimensions
- **Typical structure**: solid cubes/prisms; often one object per small "dotted tray" arranged in a loose grid (see references).
- **Fill**: solid.
- **Border**: a thin **same-hue-darker** line (width 1~1.2) -- this is the key to the 3D read: the border = the face's shadow tone, solid, never dashed.
- **Corner radius**: small (2~6), leaning sharp.
- **Faces**: **two-tone solid** -- top face bright, left/right faces same-hue darker (iso lighting), no gradient. Typical indigo: top `#8C7DF2` / left `#332A93` / right `#5747CE`, stroke `#241C6E`.
- **Light**: none or very faint shadow; no glow (keep the flat illustration clean).
- **Form**: cubic to mid-tall.
- **Texture**: none; trays may use dots.

## Style prompt
> Solid flat isometric: two-tone solid faces (top bright, left/right same-hue darker), a
> same-hue-darker thin solid stroke (width 1.1), small corners (2~4), no gradient no glow;
> clean geometry; white background; optionally seat each object on a small dotted tray.

## Canonical DSL template
```yaml
canvas: { background: "#FFFFFF", grid: "", padding: 70 }
theme:
  presets:
    solid:                            # primary (bright)
      palette: { top: "#8C7DF2", left: "#332A93", right: "#5747CE" }
      stroke: { color: "#241C6E", width: 1.1 }
      effects: { cornerRadius: 3 }
    solidDark:                        # contrast (dark)
      palette: { top: "#5B4FD6", left: "#241C6E", right: "#3B2FA6" }
      stroke: { color: "#1B154F", width: 1.1 }
    tile:                             # dotted tray (optional)
      palette: { top: "#F1EEFF" }
      stroke: { color: "#D8D2F7", width: 1 }
      effects: { cornerRadius: 2, pattern: { kind: dots, color: "#B7AEEE", spacing: 9 } }
# Rebrand by swapping the three faces to bright/dark/mid shades of any single hue.
```
