package isotopo

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// inlineLocalIcon turns a local image-file path into a self-contained data URI
// so the icon renders in the browser and exports standalone. iso://, data:,
// and http(s) values pass through untouched; an unreadable path or unknown
// extension is left as-is (the href simply won't resolve).
func inlineLocalIcon(icon string) string {
	if icon == "" ||
		strings.HasPrefix(icon, "data:") ||
		strings.HasPrefix(icon, "http://") ||
		strings.HasPrefix(icon, "https://") ||
		strings.HasPrefix(icon, "iso://") {
		return icon
	}
	p := strings.TrimPrefix(icon, "file://")
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	var mime string
	switch strings.ToLower(filepath.Ext(p)) {
	case ".svg":
		mime = "image/svg+xml"
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	default:
		return icon
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return icon
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// Flatten lowers a Node + Theme into the flat iso25d.ConvertOpts surface
// the renderer expects. Theme is applied first, then per-shape-type
// defaults, then per-node Style merged on top (3-layer ResolveStyle).
func Flatten(n *Node, theme *Theme) (string, iso25d.ConvertOpts) {
	merged := ResolveStyle(theme, n.Shape, n.Preset, n.Style)
	o := iso25d.ConvertOpts{}

	if n.Geom != nil {
		o.Width, o.Depth, o.Height = n.Geom.W, n.Geom.D, n.Geom.H
		o.Sides = n.Geom.Sides
	}

	if merged != nil {
		if p := merged.Palette; p != nil {
			o.TopFill, o.LeftFill, o.RightFill = p.Top, p.Left, p.Right
			// v2.1 — per-face gradient overrides solid fill (mirrors
			// iso25d.IsoBoxOpts behavior).
			if g := p.TopGradient; g != nil {
				o.TopGradient = &iso25d.FaceGradient{From: g.From, To: g.To, Dir: g.Dir}
			}
			if g := p.LeftGradient; g != nil {
				o.LeftGradient = &iso25d.FaceGradient{From: g.From, To: g.To, Dir: g.Dir}
			}
			if g := p.RightGradient; g != nil {
				o.RightGradient = &iso25d.FaceGradient{From: g.From, To: g.To, Dir: g.Dir}
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
