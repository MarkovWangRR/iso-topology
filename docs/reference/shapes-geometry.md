# Shape Geometry Reference

This document describes every geometric shape available in isotopo, grouped by family.

## Shape Families

| Family | Shapes | Base Geometry |
|--------|--------|---------------|
| Box | `rectangle`, `square` | Rectangular prism |
| Prism | `prism`, `diamond`, `triprism`, `hexprism`, `octprism` | Regular n-gon extruded vertically |
| Tapered Prism | `cone`, `pyramid`, `frustum` | Converging extrusion (apex or truncated top) |
| Revolution | `dome`, `torus`, `capsule` | Rotation of a 2D profile |
| Wedge | `wedge` | Ramp — one edge lifted |
| Custom Path | `custom_path` | User-supplied SVG outline extruded |
| Special | `cylinder`, `circle`, `cloud`, `person`, `iso_text`, `composite`, `group`, `boundary` | Dedicated renderers |

---

## Box Family

### rectangle / square

A standard isometric box with three visible faces: top, left, right.

```yaml
shape: rectangle
geom: { w: 160, d: 120, h: 40 }
style:
  palette: { top: "#E0F2FE", left: "#0369A1", right: "#0EA5E9" }
```

**Face names:** `top`, `left`, `right`

---

## Prism Family

Regular n-gon base extruded vertically.

### diamond

4-sided prism (rotated 22.5° so both walls are visible).

```yaml
shape: diamond
geom: { w: 140, d: 140, h: 50 }
```

### triprism

3-sided triangular prism — alert / one-way fan-out semantics.

```yaml
shape: triprism
geom: { w: 140, d: 140, h: 50 }
```

### hexprism

6-sided hexagonal prism — API gateway / middleware semantics.

```yaml
shape: hexprism
geom: { w: 150, d: 150, h: 44 }
```

### octprism

8-sided octagonal prism — firewall / stop-sign semantics.

```yaml
shape: octprism
geom: { w: 150, d: 150, h: 44 }
```

### prism (arbitrary n-gon)

Regular polygon with `geom.sides` sides.

```yaml
shape: prism
geom: { w: 140, d: 140, h: 50, sides: 5 }
```

**Face names:** `top`, `side0` … `sideN-1`

---

## Tapered Prism Family

Base polygon at z=0 converging to a smaller (or zero) top polygon at z=h.

### cone

8-gon base converging to a point apex. Use for alert / convergence / pipeline-end semantics.

```yaml
shape: cone
geom: { w: 140, d: 140, h: 80 }
style:
  palette: { top: "#FEE2E2", left: "#DC2626", right: "#EF4444" }
```

### pyramid

4-point rectangular base converging to a point apex. Monument / hierarchy semantics.

```yaml
shape: pyramid
geom: { w: 140, d: 140, h: 80 }
style:
  palette: { top: "#FEF3C7", left: "#B45309", right: "#D97706" }
```

### frustum

Truncated cone — circular base with a smaller circular top (0.5× scale). Container / tier semantics.

```yaml
shape: frustum
geom: { w: 140, d: 140, h: 60 }
style:
  palette: { top: "#D1FAE5", left: "#065F46", right: "#059669" }
```

**Face names:** `top`, `side0` … `side7`

---

## Revolution Family

Shapes formed by rotating a 2D profile. Currently rendered as box approximations.

### dome

Half-sphere canopy. Canopy / coverage semantics.

```yaml
shape: dome
geom: { w: 140, d: 140, h: 70 }
```

### capsule

Cylinder with hemispherical end-caps. Pill / pod / container semantics.

```yaml
shape: capsule
geom: { w: 120, d: 120, h: 100 }
```

### torus

Donut ring shape.

```yaml
shape: torus
geom: { w: 140, d: 140, h: 40 }
```

**Face names:** `top`, `left`, `right` (approximation)

---

## Wedge

A rectangular ramp — bottom face is flat at z=0, back edge is lifted to z=h, front edge stays at z=0. Creates a slope/ramp shape.

```yaml
shape: wedge
geom: { w: 160, d: 120, h: 50 }
label: "Ramp"
style:
  palette: { top: "#FEF3C7", left: "#D97706", right: "#F59E0B" }
  stroke: { color: "#92400E", width: 1.4 }
```

**Face names:** `slope` (the sloped top face), `right` (right triangle), `back` (not visible), `left` (not visible)

**Semantic guide:** Use wedge for ramps, transitions, gradients of complexity, or load-balancing fan-outs. The slope face is the content-anchor face (label/icon appear here).

---

## Custom Path

Extrudes an arbitrary SVG outline path (M/L/Z commands only) into a 3D shape. The path is normalized to fit the `geom.w × geom.d` footprint, then extruded to height `geom.h`.

```yaml
shape: custom_path
geom: { w: 160, d: 120, h: 40 }
label: "Custom"
style:
  palette: { top: "#DBEAFE", left: "#1D4ED8", right: "#3B82F6" }
  stroke: { color: "#1E3A8A", width: 1.4 }
```

**Supported path commands:**
- `M x,y` — move to (starts outline)
- `L x,y` — line to (adds edge)
- `Z` — close path

Curves (`C`, `Q`, `A`) are not yet supported — use line segments to approximate curves.

**Face names:** `top`, `side0` … `sideN-1` (one per edge of the outline polygon)

**Semantic guide:** Use custom_path when no built-in shape matches your diagram's semantics. Pass the outline as a normalized path (coordinates in any range — they are scaled to fit the geom). Falls back to a rectangle when the path is empty or has fewer than 3 points.

---

## Face Names Summary

| Shape family | Face names |
|---|---|
| box (rectangle/square) | `top`, `left`, `right` |
| prism family | `top`, `side0` … `sideN-1` |
| tapered prism (cone/pyramid/frustum) | `top`, `side0` … `side7` |
| revolution (dome/torus/capsule) | `top`, `left`, `right` |
| wedge | `slope`, `right`, `back`, `left` |
| custom_path | `top`, `side0` … `sideN-1` |
| cylinder | `top`, `left`, `right` |

Face names are used in `style.faces` to apply per-face fills, gradients, patterns, and stroke layers.

```yaml
style:
  faces:
    slope:
      fill: { kind: linearGradient, stops: [{offset: 0, color: "#FEF3C7"}, {offset: 1, color: "#D97706"}] }
    right:
      fill: { kind: solid, color: "#F59E0B" }
```
