package isotopo

import (
	"strings"

	"github.com/MarkovWangRR/iso-topology/iso25d"
)

// Flatten lowers a Node + Theme into the flat iso25d.ConvertOpts surface
// the renderer expects. Theme is applied first, then per-shape-type
// defaults, then per-node Style merged on top (3-layer ResolveStyle).
func Flatten(n *Node, theme *Theme) (string, iso25d.ConvertOpts) {
	merged := ResolveStyle(theme, n.Shape, n.Style)
	o := iso25d.ConvertOpts{}

	if n.Geom != nil {
		o.Width, o.Depth, o.Height = n.Geom.W, n.Geom.D, n.Geom.H
		o.Sides = n.Geom.Sides
	}

	if merged != nil {
		if p := merged.Palette; p != nil {
			o.TopFill, o.LeftFill, o.RightFill = p.Top, p.Left, p.Right
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
		}
	}

	o.Label = n.Label
	if strings.Contains(n.Label, "\n") {
		o.LabelLines = strings.Split(n.Label, "\n")
	}
	o.Icon = n.Icon

	if c := n.Content; c != nil {
		o.Header = c.Header
		o.Rows = append([]string(nil), c.Rows...)
		o.RowColors = append([]string(nil), c.RowColors...)
	}

	return n.Shape, o
}
