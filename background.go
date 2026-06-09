package isotopo

import (
	"fmt"
	"strings"
)

// EmitBackgroundDefs is the exported version of emitBackgroundDefs.
// Callers outside this package (e.g. the `topodsl wrap` subcommand)
// use it to splice a TopoDSL-style canvas background into externally
// produced SVGs (NodeDSL composites, hand-authored diagrams, etc.).
//
// It writes <defs> entries for the chosen mode and returns the fill
// reference (an SVG paint expression — colour or "url(#...)") to use
// on the underlying full-viewBox rect.
func EmitBackgroundDefs(defs *strings.Builder, opts *RenderOpts) (fillRef string) {
	return emitBackgroundDefs(defs, opts)
}

// emitBackgroundDefs writes <defs> entries for the chosen mode and returns
// the fill reference (an SVG paint expression — colour or "url(#...)") to
// use on the underlying full-viewBox rect.
//
// The grid / dots / hatch patterns are sized in world units so they line
// up with the iso projection's natural rhombus tiling.
func emitBackgroundDefs(defs *strings.Builder, opts *RenderOpts) (fillRef string) {
	switch opts.Background {

	case BgTransparent, "":
		return ""

	case BgSolid:
		return opts.BgColor

	case BgGrid:
		// True 2.5D iso ground grid. A flat square grid on z=0 projected
		// to screen becomes a diamond lattice where each diamond has
		// width 2·S·cos30 (≈1.732·S) and height 2·S·sin30 (=S), so the
		// lines run at ±30° from horizontal (= ±tan30° slope ≈ ±0.577).
		// The tile holds one diamond outline; adjacent tiles meet
		// vertex-to-vertex for a continuous iso ground plane.
		step := opts.BgGridStep
		if step <= 0 {
			step = 36
		}
		tileW := step * 2 * cos30
		tileH := step * 2 * sin30 // = step
		fmt.Fprintf(defs,
			`<pattern id="bg-grid" patternUnits="userSpaceOnUse" width="%.2f" height="%.2f">`+
				`<rect width="%.2f" height="%.2f" fill="%s"/>`+
				`<path d="M %.2f,0 L %.2f,%.2f L %.2f,%.2f L 0,%.2f Z" fill="none" stroke="%s" stroke-width="1"/>`+
				`</pattern>`,
			tileW, tileH,
			tileW, tileH, escAttr(opts.BgColor),
			tileW/2, tileW, tileH/2, tileW/2, tileH, tileH/2,
			escAttr(opts.BgGridColor),
		)
		return "url(#bg-grid)"

	case BgDots:
		// Dots at every iso grid vertex (the 4 diamond corners of each tile).
		step := opts.BgGridStep
		if step <= 0 {
			step = 32
		}
		tileW := step * 2 * cos30
		tileH := step * 2 * sin30
		r := 1.6
		fmt.Fprintf(defs,
			`<pattern id="bg-dots" patternUnits="userSpaceOnUse" width="%.2f" height="%.2f">`+
				`<rect width="%.2f" height="%.2f" fill="%s"/>`+
				`<circle cx="%.2f" cy="0" r="%.2f" fill="%s"/>`+
				`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
				`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
				`<circle cx="0" cy="%.2f" r="%.2f" fill="%s"/>`+
				`</pattern>`,
			tileW, tileH,
			tileW, tileH, escAttr(opts.BgColor),
			tileW/2, r, escAttr(opts.BgGridColor),
			tileW, tileH/2, r, escAttr(opts.BgGridColor),
			tileW/2, tileH, r, escAttr(opts.BgGridColor),
			tileH/2, r, escAttr(opts.BgGridColor),
		)
		return "url(#bg-dots)"

	case BgHatch:
		// Single iso-axis stripes at +30° from horizontal. Lines drawn
		// directly with slope tan30° — no patternTransform needed.
		step := opts.BgGridStep
		if step <= 0 {
			step = 16
		}
		// Tile carries one line at +30° going corner-to-corner. The
		// adjacent tile continues the line seamlessly.
		tileW := step * 2 * cos30
		tileH := step * 2 * sin30
		fmt.Fprintf(defs,
			`<pattern id="bg-hatch" patternUnits="userSpaceOnUse" width="%.2f" height="%.2f">`+
				`<rect width="%.2f" height="%.2f" fill="%s"/>`+
				`<line x1="0" y1="0" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1"/>`+
				`</pattern>`,
			tileW, tileH,
			tileW, tileH, escAttr(opts.BgColor),
			tileW, tileH, escAttr(opts.BgGridColor),
		)
		return "url(#bg-hatch)"
	}
	return ""
}

func escAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func escXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

const (
	cos30 = 0.8660254037844387
	sin30 = 0.5
)
