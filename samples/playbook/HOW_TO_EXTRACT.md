# How to extract a "node visual template" from a reference

Goal: ignore the picture's topology/composition; look only at what a **single node**
looks like, and decompose it into dimensions expressible in the isotopo DSL. For any
reference image, read off the checklist below to reconstruct one node style.

## Extraction checklist (7 dimensions -> DSL knobs)

| # | Dimension | What to observe | DSL knob |
|---|---|---|---|
| 1 | **Fill vs line** | Are faces filled, or only an outline with hollow faces? | `effects.wireframe: true` (pure line) / else use `palette` |
| 2 | **Face treatment** | Are top/left/right faces solid, gradient, or translucent/frosted? | `palette.{top,left,right}` solid / `*Gradient{from,to,dir}` / `style.faces[f].fill{kind:linearGradient|radialGradient,stops}` |
| 3 | **Texture** | Any dot/hatch/grain/grid on the faces? | `effects.pattern{kind:dots\|hatch\|grid}` / `effects.grain{intensity,scale}` / `faces[f].fill.pattern` |
| 4 | **Border** | Is there a stroke? Weight? Dashed? Colour relative to the face: "same-hue darker / neutral grey / brighter neon / none"? | `stroke{color,width,dash}` (same-hue darker = solid 3D; brighter = neon; none = glass/card) |
| 5 | **Corner radius** | Sharp (0), small (2-8), or large (16-24)? | `effects.cornerRadius` (with `faceSplit:true` so rounded boxes light each side independently) |
| 6 | **Light & shadow** | Drop shadow (soft/hard)? Glow / "breathing" halo? | `effects.dropShadow{dx,dy,blur,color}` / `effects.backglow{color,radius,opacity}` / `effects.outline` |
| 7 | **Form / height ratio** | Height vs footprint: flat slab (h<<w), cubic (h~w), or tall (h>w)? | the ratio of `geom.{w,d,h}` |

Plus **context** (not the node itself but tightly coupled): canvas background lightness/colour
and whether there is an iso/dots/hatch grid (`canvas.{background,grid,gridColor}`) -- dark+neon,
light+frosted, and white+line-art are strongly bound pairings.

## Suggested reading order
1. First decide **fill vs line** (dim 1) -- "line-art family" or "solid family".
2. Then **border colour relative to face** (dim 4) -- the single most decisive stroke for
   telling solid / neon / frosted / card apart.
3. Then read **face treatment + texture + light** (2/3/6) -- pins the exact family.
4. Finally note **corner radius + form** (5/7) and the **context background**.

## One-line decision tree
- Hollow faces, only lines -> **wireframe-lineart**; if lines look grainy/wobbly -> **handdrawn-sketch**; if faces carry dots -> **ascii-dot**.
- Solid faces, border = same-hue darker, sharp/small radius -> **solid-flat**.
- Faces are gradients, faint/no border -> **gradient-face**.
- Faces translucent, almost no border, soft glow -> **frosted-glass**.
- Dark background, border brighter than face, strong glow -> **neon-glow**.
- Large radius, soft shadow, light/neutral background -> **soft-rounded-card**.

> Read these 7 dimensions, then apply the matching DSL template in `styles/<style>.md`
> and just swap the colours.
