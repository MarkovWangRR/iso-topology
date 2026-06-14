package isotopo

// ResolveStyle applies the canonical four-layer merge for a node:
//
//	theme.Style            (document-wide defaults)
//	theme.Shapes[shape]    (per-shape-type defaults — optional)
//	theme.Presets[preset]  (named design-system preset — optional, v2.5)
//	nodeStyle              (per-node overrides — optional)
func ResolveStyle(theme *Theme, shape, preset string, nodeStyle *Style) *Style {
	var base *Style
	var perShape *Style
	var fromPreset *Style
	if theme != nil {
		base = &theme.Style
		if theme.Shapes != nil {
			perShape = theme.Shapes[shape]
		}
		if preset != "" && theme.Presets != nil {
			fromPreset = theme.Presets[preset]
		}
	}
	merged := MergeStyle(base, perShape)
	merged = MergeStyle(merged, fromPreset)
	merged = MergeStyle(merged, nodeStyle)
	return merged
}

// ResolvePartStyle returns the fully-resolved style (theme → shape default →
// preset → node overrides) for the part with the given id — i.e. exactly what
// the renderer paints — or nil if no such part exists. Studio uses it to show
// the EFFECTIVE colour of a face whose colour is inherited from a preset/theme
// or expressed as a gradient, where the raw node YAML carries nothing.
func ResolvePartStyle(doc *Document, id string) *Style {
	if doc == nil {
		return nil
	}
	var found *CompositePart
	var walk func(ps []*CompositePart)
	walk = func(ps []*CompositePart) {
		for _, p := range ps {
			if p == nil || found != nil {
				continue
			}
			if p.ID == id {
				found = p
				return
			}
			walk(p.Parts)
		}
	}
	for _, n := range doc.Nodes {
		if n == nil {
			continue
		}
		walk(n.Parts)
		if found != nil {
			break
		}
	}
	if found != nil {
		return ResolveStyle(doc.Theme, found.Shape, found.Preset, found.Style)
	}
	// id may name a top-level (non-composite) node, keyed in the map.
	if n, ok := doc.Nodes[id]; ok && n != nil {
		return ResolveStyle(doc.Theme, n.Shape, n.Preset, n.Style)
	}
	return nil
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
		Faces:   mergeFaces(base.Faces, over.Faces),
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
		// v2.1 — per-face gradients flow through the same precedence as
		// solid fills: the more-specific layer wins as a whole struct.
		TopGradient:   firstNonNilGradient(o.TopGradient, b.TopGradient),
		LeftGradient:  firstNonNilGradient(o.LeftGradient, b.LeftGradient),
		RightGradient: firstNonNilGradient(o.RightGradient, b.RightGradient),
	}
}

func firstNonNilGradient(a, b *FaceGradient) *FaceGradient {
	if a != nil {
		return a
	}
	return b
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
	out := &Effects{
		Opacity:      firstNonNilFloat(o.Opacity, b.Opacity),
		Margin:       firstNonNilFloat(o.Margin, b.Margin),
		CornerRadius: firstNonNilFloat(o.CornerRadius, b.CornerRadius),
		Blur:         firstNonNilFloat(o.Blur, b.Blur),
	}
	// v2.1 — DropShadow / Backglow / Pattern follow the same all-or-nothing
	// override semantics as the rest of Style: a non-nil override wins
	// the whole struct.
	if o.DropShadow != nil {
		out.DropShadow = o.DropShadow
	} else {
		out.DropShadow = b.DropShadow
	}
	if o.Backglow != nil {
		out.Backglow = o.Backglow
	} else {
		out.Backglow = b.Backglow
	}
	if o.Pattern != nil {
		out.Pattern = o.Pattern
	} else {
		out.Pattern = b.Pattern
	}
	if o.Wireframe != nil {
		out.Wireframe = o.Wireframe
	} else {
		out.Wireframe = b.Wireframe
	}
	if o.Grain != nil {
		out.Grain = o.Grain
	} else {
		out.Grain = b.Grain
	}
	if o.Outline != nil {
		out.Outline = o.Outline
	} else {
		out.Outline = b.Outline
	}
	return out
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

// mergeFaces — per-key override: a part's face entry wins wholesale
// over the preset's entry for the same face name.
func mergeFaces(b, o map[string]*FaceStyle) map[string]*FaceStyle {
	if b == nil && o == nil {
		return nil
	}
	out := map[string]*FaceStyle{}
	for k, v := range b {
		out[k] = v
	}
	for k, v := range o {
		out[k] = v
	}
	return out
}
