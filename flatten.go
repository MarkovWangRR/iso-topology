package isotopo

import (
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// inlineLocalIcon turns a local image-file path into a self-contained data URI.
// It has two build-tagged implementations: the native build reads the file from
// disk (icons.go); the js/wasm build can't touch a filesystem, so it passes the
// reference through unchanged (icons_js.go). iso://, data: and http(s) values
// pass through in both — they never touch the filesystem.

// Flatten lowers a Node + Theme into the flat iso25d.ConvertOpts surface
// the renderer expects. Theme is applied first, then per-shape-type
// defaults, then per-node Style merged on top (3-layer ResolveStyle).
func Flatten(n *Node, theme *Theme) (string, iso25d.ConvertOpts) {
	merged := ResolveStyle(theme, n.Shape, n.Preset, n.Style)
	o := iso25d.ConvertOpts{}

	if n.Geom != nil {
		o.Width, o.Depth, o.Height = n.Geom.W, n.Geom.D, n.Geom.H
		o.Sides = n.Geom.Sides
		o.TopScale = n.Geom.TopScale
		o.Params = n.Geom.Params
	}

	if merged != nil {
		if p := merged.Palette; p != nil {
			o.TopFill, o.LeftFill, o.RightFill = p.Top, p.Left, p.Right
			// v2.1 — per-face gradient overrides solid fill (mirrors
			// iso25d.IsoBoxOpts behavior).
			// A gradient with only `to` set uses the face's solid fill as its
			// start, so the editor can offer one "base colour + gradient to"
			// model per face instead of separate solid/gradient blocks.
			if g := p.TopGradient; g != nil && (g.From != "" || g.To != "") {
				from := g.From
				if from == "" {
					from = p.Top
				}
				o.TopGradient = &iso25d.FaceGradient{From: from, To: g.To, Dir: g.Dir}
			}
			if g := p.LeftGradient; g != nil && (g.From != "" || g.To != "") {
				from := g.From
				if from == "" {
					from = p.Left
				}
				o.LeftGradient = &iso25d.FaceGradient{From: from, To: g.To, Dir: g.Dir}
			}
			if g := p.RightGradient; g != nil && (g.From != "" || g.To != "") {
				from := g.From
				if from == "" {
					from = p.Right
				}
				o.RightGradient = &iso25d.FaceGradient{From: from, To: g.To, Dir: g.Dir}
			}
		}
		if merged.Faces != nil {
			fsm := map[string]*iso25d.FaceSurface{}
			for name, face := range merged.Faces {
				if face == nil {
					continue
				}
				out := &iso25d.FaceSurface{}
				if ff := face.Fill; ff != nil {
					nf := &iso25d.FaceFill{Kind: ff.Kind, Color: ff.Color, Angle: ff.Angle, Projected: ff.Projected}
					for _, st := range ff.Stops {
						nf.Stops = append(nf.Stops, iso25d.FaceStop{Offset: st.Offset, Color: st.Color})
					}
					if ff.Cx != nil && ff.Cy != nil {
						nf.Cx, nf.Cy, nf.HasC = *ff.Cx, *ff.Cy, true
					}
					if ff.Pattern != nil {
						nf.Pattern = &iso25d.FacePatternSpec{
							Kind: ff.Pattern.Kind, Color: ff.Pattern.Color,
							Spacing: ff.Pattern.Spacing, Angle: ff.Pattern.Angle,
							Projected: ff.Pattern.Projected,
						}
					}
					out.Fill = nf
				}
				for _, sl := range face.Strokes {
					out.Strokes = append(out.Strokes, iso25d.FaceStrokeLayer{
						Color: sl.Color, Width: sl.Width, Dash: sl.Dash, Opacity: sl.Opacity,
					})
				}
				fsm[name] = out
			}
			o.FaceSurfaces = fsm
		}
		if s := merged.Stroke; s != nil {
			o.Stroke = s.Color
			if s.Width != nil {
				o.StrokeWidth = *s.Width
			}
			o.StrokeDasharray = s.Dash
		}
		if t := merged.Text; t != nil {
			o.FontFamily = t.Family
			if t.Size != nil {
				o.FontSize = *t.Size
			}
			o.FontWeight = t.Weight
			o.FontColor = t.Color
		}
		if e := merged.Effects; e != nil {
			// v3.9 — when the ordered effects list is present, synthesize
			// the scalar effect fields from it so all renderers benefit
			// without per-renderer changes. The list takes precedence over
			// individual scalar fields.
			for _, item := range e.List {
				if item == nil {
					continue
				}
				switch item.Kind {
				case "backglow":
					if item.Color != "" {
						o.BackglowColor = item.Color
					}
					if item.Radius > 0 {
						o.BackglowRadius = item.Radius
					}
					if item.Opacity > 0 {
						o.BackglowOpacity = item.Opacity
					}
				case "blur":
					if item.StdDev > 0 {
						o.Blur = item.StdDev
					}
					// 'blur' field is an alias for stdDev.
					if item.Blur > 0 {
						o.Blur = item.Blur
					}
				case "grain":
					if item.Intensity > 0 {
						o.GrainIntensity = item.Intensity
					}
					if item.Scale > 0 {
						o.GrainScale = item.Scale
					}
				case "outline":
					if item.Color != "" {
						o.OutlineColor = item.Color
					}
					if item.Width > 0 {
						o.OutlineWidth = item.Width
					}
					if item.Dash != "" {
						o.OutlineDash = item.Dash
					}
					if item.Opacity > 0 {
						o.OutlineOpacity = item.Opacity
					}
				case "dropShadow":
					if item.Color != "" {
						o.ShadowColor = item.Color
					}
					o.ShadowDx = item.Dx
					o.ShadowDy = item.Dy
					if item.Blur > 0 {
						o.ShadowBlur = item.Blur
					}
				}
			}
			if e.Opacity != nil {
				o.Opacity = *e.Opacity
			}
			if e.Margin != nil {
				o.Margin = *e.Margin
			}
			if e.CornerRadius != nil {
				o.CornerRadius = *e.CornerRadius
			}
			// v2.1 — wire DropShadow / Backglow / Pattern through to the
			// renderer. These are all 0/empty-defaulted in ConvertOpts so
			// existing fixtures don't drift unless the DSL opts in.
			if ds := e.DropShadow; ds != nil {
				o.ShadowDx, o.ShadowDy, o.ShadowBlur, o.ShadowColor = ds.Dx, ds.Dy, ds.Blur, ds.Color
			}
			if e.Blur != nil && *e.Blur > 0 {
				o.Blur = *e.Blur
			}
			if ol := e.Outline; ol != nil {
				o.OutlineColor, o.OutlineWidth = ol.Color, ol.Width
				o.OutlineDash, o.OutlineOpacity = ol.Dash, ol.Opacity
			}
			if bg := e.Backglow; bg != nil {
				o.BackglowColor, o.BackglowRadius, o.BackglowOpacity = bg.Color, bg.Radius, bg.Opacity
			}
			if pt := e.Pattern; pt != nil {
				o.PatternKind, o.PatternColor, o.PatternSpacing, o.PatternAngle = pt.Kind, pt.Color, pt.Spacing, pt.Angle
			}
			// v2.6 — wireframe line-art + film-grain texture.
			if e.Wireframe != nil {
				o.Wireframe = *e.Wireframe
			}
			if e.FaceSplit != nil {
				o.FaceSplit = *e.FaceSplit
			}
			if g := e.Grain; g != nil {
				o.GrainIntensity, o.GrainScale = g.Intensity, g.Scale
			}
		}
	}

	o.Label = n.Label
	if strings.Contains(n.Label, "\n") {
		o.LabelLines = strings.Split(n.Label, "\n")
	}
	// v2.1 — "iso://brand/<name>" Icon URIs get expanded into a data-URI
	// badge; a local image-file path is inlined to a data URI; any other
	// value (data:, http(s)) passes through untouched.
	o.Icon = inlineLocalIcon(iso25d.ResolveBrandIcon(n.Icon))

	if c := n.Content; c != nil {
		o.Header = c.Header
		o.Rows = append([]string(nil), c.Rows...)
		o.RowColors = append([]string(nil), c.RowColors...)
	}

	return n.Shape, o
}
