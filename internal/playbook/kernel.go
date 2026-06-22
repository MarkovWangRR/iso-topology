package playbook

// kernel.go — the shading kernel. Turns a single base colour into the three lit
// iso faces of a box under the material's directional light + ambient-occlusion
// falloff. This is the "rounded-box + left/right gradient simulating light"
// technique made explicit and reusable.

// Faces holds the derived per-face fills: a solid top + a vertical gradient on
// each side wall (lighter at the lit top edge → darker at the shaded base).
type Faces struct {
	Top                                   string
	LeftFrom, LeftTo, RightFrom, RightTo  string
}

// DeriveFaces computes the faces of a box from base colour + material. top is
// lit from above; the side facing the light is brighter, the other shadowed;
// each side carries an AO gradient. Default light leaves the RIGHT wall brighter
// (matching isotopo's built-in iso shading); light=topLeft flips it.
func DeriveFaces(base string, m Material) Faces {
	top := shade(base, m.TopDL)
	lit := shade(base, m.LitSideDL)    // face toward the light
	dark := shade(base, m.ShadeSideDL) // face in shadow

	leftBase, rightBase := dark, lit // default: right-lit
	if m.Light == "topLeft" {
		leftBase, rightBase = lit, dark
	}
	ao := m.AoDL / 2
	return Faces{
		Top:       top,
		LeftFrom:  shade(leftBase, ao),
		LeftTo:    shade(leftBase, -ao),
		RightFrom: shade(rightBase, ao),
		RightTo:   shade(rightBase, -ao),
	}
}
