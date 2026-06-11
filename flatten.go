package isotopo

import (
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

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
			if bg := e.Backglow; bg != nil {
				o.BackglowColor, o.BackglowRadius, o.BackglowOpacity = bg.Color, bg.Radius, bg.Opacity
			}
			if pt := e.Pattern; pt != nil {
				o.PatternKind, o.PatternColor, o.PatternSpacing, o.PatternAngle = pt.Kind, pt.Color, pt.Spacing, pt.Angle
			}
		}
	}

	o.Label = n.Label
	if strings.Contains(n.Label, "\n") {
		o.LabelLines = strings.Split(n.Label, "\n")
	}
	// v2.1 — "iso://brand/<name>" Icon URIs get expanded into a
	// data-URI badge; any other value passes through untouched.
	o.Icon = iso25d.ResolveBrandIcon(n.Icon)

	if c := n.Content; c != nil {
		o.Header = c.Header
		o.Rows = append([]string(nil), c.Rows...)
		o.RowColors = append([]string(nil), c.RowColors...)
	}

	return n.Shape, o
}
