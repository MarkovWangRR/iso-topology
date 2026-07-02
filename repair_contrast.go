package isotopo

import "math"

// repairContrast is the issue #8 auto-repair pass: for any labelled part whose
// top-face fill vs. label-text contrast is below the 3.0 threshold the validator
// enforces, retint the LABEL (never the author's fill) to whichever of white or
// near-black maximizes contrast against the worst top-face stop. For any single
// solid fill the better of the two always clears ~4.58:1, so this lifts a
// sub-threshold label to legible without touching the palette the author chose.
//
// It is a strict no-op on a scene that already passes (byte-identical output),
// so it is safe to run unconditionally inside RepairScene. Returns whether any
// label was retinted.
func repairContrast(doc *Document) bool {
	if doc == nil {
		return false
	}
	changed := false
	for _, n := range doc.Nodes {
		if n == nil || n.Shape != "composite" {
			continue
		}
		var walk func(parts []*CompositePart)
		walk = func(parts []*CompositePart) {
			for _, p := range parts {
				if p == nil {
					continue
				}
				if p.Label != "" {
					if retintLabelForContrast(doc.Theme, p) {
						changed = true
					}
				}
				walk(p.Parts)
			}
		}
		walk(n.Parts)
	}
	return changed
}

// retintLabelForContrast mutates p.Style.Text.Color when the label is below the
// 3.0 contrast threshold against its worst top-face fill stop. Returns true if it
// changed the color.
func retintLabelForContrast(theme *Theme, p *CompositePart) bool {
	eff := ResolveStyleWithRole(theme, p.Shape, p.Role, p.Preset, p.Style)
	txt := textColor(eff)
	txtLum, ok := lumOf(txt)
	if !ok {
		return false
	}
	// Worst (lowest-contrast) top-face stop against the current text color.
	worst := math.Inf(1)
	worstLum := 0.0
	found := false
	for _, f := range topFillColors(eff) {
		fl, ok := lumOf(f)
		if !ok {
			continue
		}
		found = true
		if cr := contrastRatio(fl, txtLum); cr < worst {
			worst, worstLum = cr, fl
		}
	}
	if !found || worst >= 3.0 {
		return false
	}
	// Pick whichever extreme text color contrasts best with the worst stop.
	const white, black = "#FFFFFF", "#0B0B0B"
	whiteLum, _ := lumOf(white)
	blackLum, _ := lumOf(black)
	pick := white
	if contrastRatio(worstLum, blackLum) > contrastRatio(worstLum, whiteLum) {
		pick = black
	}
	if pick == txt {
		return false // already the best extreme; can't improve via text color
	}
	if p.Style == nil {
		p.Style = &Style{}
	}
	if p.Style.Text == nil {
		p.Style.Text = &Text{}
	}
	p.Style.Text.Color = pick
	return true
}
