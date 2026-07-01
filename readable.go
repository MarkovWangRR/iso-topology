package isotopo

// ApplyReadableProfile turns a scene into a legibility-first "documentation"
// render (issue #11). It is the one-switch bundle behind `isotopo render
// --readable`: instead of asking authors to combine several knobs, it layers
// readability defaults over whatever theme the scene already uses.
//
// It is OPT-IN and conservative — it only fills gaps the author left blank, so a
// scene that has already made a choice (explicit label orientation, colors,
// padding) keeps it:
//
//   - Labels default to screen orientation (flat, upright) with a canvas-aware
//     contrast chip, so text is legible on any background and fill — the single
//     biggest readability win over sheared on-face text.
//   - A canvas padding floor gives the scene breathing room.
//
// Contrast auto-repair (#8) still runs in the normal render path on top of this.
func ApplyReadableProfile(doc *Document) {
	if doc == nil {
		return
	}
	if doc.Canvas == nil {
		doc.Canvas = &Canvas{}
	}
	// Padding floor — generous breathing room for a documentation diagram.
	const readablePadding = 48
	if doc.Canvas.Padding < readablePadding {
		doc.Canvas.Padding = readablePadding
	}

	// Pick a label chip that contrasts with the canvas background, so screen
	// labels are legible whether the canvas is light or dark.
	bg := doc.Canvas.Background
	if bg == "" {
		bg = "#F8FAFC"
	}
	textColor, chipBg, chipBorder := "#14181F", "#FFFFFFCC", "#00000022" // light canvas
	if lum, ok := lumOf(bg); ok && lum < 0.5 {
		textColor, chipBg, chipBorder = "#F5F7FA", "#0B0D12CC", "#FFFFFF22" // dark canvas
	}

	for _, n := range doc.Nodes {
		if n == nil || n.Shape != "composite" {
			continue
		}
		applyReadableToParts(n.Parts, textColor, chipBg, chipBorder)
	}
}

func applyReadableToParts(parts []*CompositePart, textColor, chipBg, chipBorder string) {
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Label != "" {
			if p.Style == nil {
				p.Style = &Style{}
			}
			if p.Style.Text == nil {
				p.Style.Text = &Text{}
			}
			t := p.Style.Text
			// Only fill gaps: respect an author who already chose an orientation.
			if t.Orient == "" {
				t.Orient = "screen"
				if t.Color == "" {
					t.Color = textColor
				}
				if t.BoxBg == "" {
					t.BoxBg = chipBg
				}
				if t.BoxBorder == "" {
					t.BoxBorder = chipBorder
				}
			}
		}
		// Minimum footprint (Option A): bump a small LEAF node up to a legible
		// floor so its label/logo isn't cramped. Containers auto-size to their
		// children, so they're left alone; nodes without an explicit geom already
		// use the 140-default. Only enlarges — never shrinks an author's node.
		if p.Geom != nil && len(p.Parts) == 0 && !isContainerShape(p.Shape) {
			if p.Geom.W > 0 && p.Geom.W < readableMinNodeW {
				p.Geom.W = readableMinNodeW
			}
			if p.Geom.D > 0 && p.Geom.D < readableMinNodeD {
				p.Geom.D = readableMinNodeD
			}
		}
		applyReadableToParts(p.Parts, textColor, chipBg, chipBorder)
	}
}

// readableMinNodeW/D are the minimum leaf-node footprint the readable profile
// enforces, so tiny tiles (e.g. 66x60 logo chips) grow enough for a legible
// label/logo instead of shrinking their content to fit.
const (
	readableMinNodeW = 96
	readableMinNodeD = 80
)
