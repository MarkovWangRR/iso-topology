package iso25d

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// M3 — per-face surface descriptions, mirroring the DSL's style.faces.
// Renderers look a face up by name, then "*", then fall back to the
// legacy palette fills.
type FaceSurface struct {
	Fill    *FaceFill
	Strokes []FaceStrokeLayer
}

type FaceFill struct {
	Kind    string // solid | linearGradient | radialGradient | pattern
	Color   string
	Stops   []FaceStop
	Angle   float64
	Cx, Cy  float64 // radial centre 0..1 (defaults 0.5)
	HasC    bool
	Pattern *FacePatternSpec
}

type FaceStop struct {
	Offset float64
	Color  string
}

type FacePatternSpec struct {
	Kind      string
	Color     string
	Spacing   float64
	Angle     float64
	Projected bool
}

type FaceStrokeLayer struct {
	Color   string
	Width   float64
	Dash    string
	Opacity float64
}

// surfaceFor resolves the face surface by exact name, then wildcard.
func surfaceFor(m map[string]*FaceSurface, name string) *FaceSurface {
	if m == nil {
		return nil
	}
	if s, ok := m[name]; ok {
		return s
	}
	return m["*"]
}

// isoTopMatrix is the patternTransform that pins a tile to the iso
// ground plane (the projection is affine, so the mapping is exact).
const isoTopMatrix = "matrix(0.8660 0.5000 -0.8660 0.5000 0 0)"

// sideMatrix returns the shear that pins a tile to a box's left/right
// wall (x along the world axis, y along world z).
func sideMatrix(name string) string {
	switch name {
	case "left":
		return "matrix(0.8660 0.5000 0 1 0 0)"
	case "right":
		return "matrix(0.8660 -0.5000 0 1 0 0)"
	}
	return ""
}

// emitFaceFill writes any defs the fill needs and returns the fill ref.
// Def ids are deterministic per (prefix, face) — callers namespace per
// part via the usual composite id prefixing.
func emitFaceFill(defs *strings.Builder, prefix, face string, f *FaceFill) string {
	switch f.Kind {
	case "", "solid":
		if f.Color != "" {
			return f.Color
		}
		return ""
	case "linearGradient", "radialGradient":
		stops := append([]FaceStop(nil), f.Stops...)
		sort.SliceStable(stops, func(i, j int) bool { return stops[i].Offset < stops[j].Offset })
		id := fmt.Sprintf("%sface-%s", prefix, face)
		if f.Kind == "linearGradient" {
			rad := f.Angle * math.Pi / 180
			x2, y2 := math.Cos(rad), math.Sin(rad)
			x1, y1 := 0.0, 0.0
			if x2 < 0 {
				x1, x2 = -x2, 0
			}
			if y2 < 0 {
				y1, y2 = -y2, 0
			}
			fmt.Fprintf(defs, `<linearGradient id="%s" x1="%.4f" y1="%.4f" x2="%.4f" y2="%.4f">`, id, x1, y1, x2, y2)
		} else {
			cx, cy := 0.5, 0.5
			if f.HasC {
				cx, cy = f.Cx, f.Cy
			}
			fmt.Fprintf(defs, `<radialGradient id="%s" cx="%.4f" cy="%.4f" r="0.75">`, id, cx, cy)
		}
		for _, s := range stops {
			fmt.Fprintf(defs, `<stop offset="%.4f" stop-color="%s"/>`, s.Offset, escapeAttr(s.Color))
		}
		if f.Kind == "linearGradient" {
			defs.WriteString(`</linearGradient>`)
		} else {
			defs.WriteString(`</radialGradient>`)
		}
		return "url(#" + id + ")"
	case "pattern":
		if f.Pattern == nil {
			return ""
		}
		id := fmt.Sprintf("%sfacepat-%s", prefix, face)
		var inner strings.Builder
		pid := emitPatternDef(&inner, id, f.Pattern.Kind, f.Pattern.Color, f.Pattern.Spacing, f.Pattern.Angle)
		if pid == "" {
			return ""
		}
		body := inner.String()
		if f.Pattern.Projected {
			tf := isoTopMatrix
			if m := sideMatrix(face); m != "" {
				tf = m
			}
			if strings.Contains(body, `patternTransform="`) {
				// compose with the tile's own rotation
				body = strings.Replace(body, `patternTransform="`, `patternTransform="`+tf+` `, 1)
			} else {
				body = strings.Replace(body, "<pattern ", `<pattern patternTransform="`+tf+`" `, 1)
			}
		}
		defs.WriteString(body)
		return "url(#" + pid + ")"
	}
	return ""
}

// writeFaceStrokeLayers re-traces a face polygon once per stroke layer
// (fill none), widest first so narrow highlight lines read on top.
func writeFaceStrokeLayers(sb *strings.Builder, name string, layers []FaceStrokeLayer, pts ...[2]float64) {
	for i, l := range layers {
		if l.Width <= 0 || l.Color == "" {
			continue
		}
		dashAttr := ""
		if l.Dash != "" {
			dashAttr = fmt.Sprintf(` stroke-dasharray="%s"`, escapeAttr(l.Dash))
		}
		opAttr := ""
		if l.Opacity > 0 && l.Opacity < 1 {
			opAttr = fmt.Sprintf(` stroke-opacity="%.3f"`, l.Opacity)
		}
		var d strings.Builder
		for j, p := range pts {
			if j == 0 {
				fmt.Fprintf(&d, "M %.2f %.2f", p[0], p[1])
			} else {
				fmt.Fprintf(&d, " L %.2f %.2f", p[0], p[1])
			}
		}
		d.WriteString(" Z")
		fmt.Fprintf(sb,
			`<path data-face="%s-stroke%d" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"%s%s/>`,
			name, i, d.String(), escapeAttr(l.Color), l.Width, dashAttr, opAttr,
		)
	}
}
