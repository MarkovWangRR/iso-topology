package isotopo

// ResolveStyle applies the canonical three-layer merge for a node:
//
//	theme.Style           (document-wide defaults)
//	theme.Shapes[shape]   (per-shape-type defaults — optional)
//	nodeStyle             (per-node overrides — optional)
func ResolveStyle(theme *Theme, shape string, nodeStyle *Style) *Style {
	var base *Style
	var perShape *Style
	if theme != nil {
		base = &theme.Style
		if theme.Shapes != nil {
			perShape = theme.Shapes[shape]
		}
	}
	merged := MergeStyle(base, perShape)
	merged = MergeStyle(merged, nodeStyle)
	return merged
}

// MergeStyle returns a new Style whose fields come from `over` when present,
// otherwise from `base`.
func MergeStyle(base, over *Style) *Style {
	if base == nil && over == nil {
		return nil
	}
	if base == nil {
		base = &Style{}
	}
	if over == nil {
		over = &Style{}
	}
	return &Style{
		Palette: mergePalette(base.Palette, over.Palette),
		Stroke:  mergeStroke(base.Stroke, over.Stroke),
		Text:    mergeText(base.Text, over.Text),
		Effects: mergeEffects(base.Effects, over.Effects),
	}
}

func mergePalette(b, o *Palette) *Palette {
	if b == nil && o == nil {
		return nil
	}
	if b == nil {
		b = &Palette{}
	}
	if o == nil {
		o = &Palette{}
	}
	return &Palette{
		Top:   firstNonEmpty(o.Top, b.Top),
		Left:  firstNonEmpty(o.Left, b.Left),
		Right: firstNonEmpty(o.Right, b.Right),
	}
}

func mergeStroke(b, o *Stroke) *Stroke {
	if b == nil && o == nil {
		return nil
	}
	if b == nil {
		b = &Stroke{}
	}
	if o == nil {
		o = &Stroke{}
	}
	return &Stroke{
		Color: firstNonEmpty(o.Color, b.Color),
		Width: firstNonNilFloat(o.Width, b.Width),
		Dash:  firstNonEmpty(o.Dash, b.Dash),
	}
}

func mergeText(b, o *Text) *Text {
	if b == nil && o == nil {
		return nil
	}
	if b == nil {
		b = &Text{}
	}
	if o == nil {
		o = &Text{}
	}
	return &Text{
		Family:    firstNonEmpty(o.Family, b.Family),
		Size:      firstNonNilFloat(o.Size, b.Size),
		Weight:    firstNonEmpty(o.Weight, b.Weight),
		Color:     firstNonEmpty(o.Color, b.Color),
		Orient:    firstNonEmpty(o.Orient, b.Orient),
		BoxBg:     firstNonEmpty(o.BoxBg, b.BoxBg),
		BoxBorder: firstNonEmpty(o.BoxBorder, b.BoxBorder),
	}
}

func mergeEffects(b, o *Effects) *Effects {
	if b == nil && o == nil {
		return nil
	}
	if b == nil {
		b = &Effects{}
	}
	if o == nil {
		o = &Effects{}
	}
	return &Effects{
		Opacity:      firstNonNilFloat(o.Opacity, b.Opacity),
		Margin:       firstNonNilFloat(o.Margin, b.Margin),
		CornerRadius: firstNonNilFloat(o.CornerRadius, b.CornerRadius),
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonNilFloat(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}
