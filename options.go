package isotopo

// BackgroundMode is the canvas backdrop style.
//
//	"transparent" — no background
//	"solid"       — flat colour rect
//	"grid"        — iso-tilted line grid (diamond pattern), classic "ground plane"
//	"dots"        — iso-tilted dot grid
//	"hatch"       — diagonal hatched fill
type BackgroundMode string

const (
	BgTransparent BackgroundMode = "transparent"
	BgSolid       BackgroundMode = "solid"
	BgGrid        BackgroundMode = "grid"
	BgDots        BackgroundMode = "dots"
	BgHatch       BackgroundMode = "hatch"
)

// RenderOpts configures the iso rendering pipeline. Zero values fall back
// to the defaults from DefaultRenderOpts().
type RenderOpts struct {
	// Background
	Background  BackgroundMode
	BgColor     string  // for solid (and bleed colour behind patterns)
	BgGridColor string  // pattern stroke / dot colour
	BgGridStep  float64 // pattern tile size in world units
	Padding     float64 // viewBox margin around the diagram

	// Iso geometry
	IsoHeight float64 // default extrusion height for nodes
	NodeR     float64 // rounded-corner radius applied to box-family nodes

	// Per-element style defaults (applied unless d2 attrs override)
	NodeFill   string
	NodeStroke string
	NodeStrokeW float64
	EdgeStroke string
	EdgeStrokeW float64
	EdgeArrow  string
	FontFamily string
	FontColor  string
	FontSize   float64
}

func DefaultRenderOpts() *RenderOpts {
	return &RenderOpts{
		Background:  BgGrid,
		BgColor:     "#FAFBFC",
		BgGridColor: "#E2E6EE",
		BgGridStep:  40,
		Padding:     64,
		IsoHeight:   30,
		NodeR:       8,
		NodeFill:    "#FFFFFF",
		NodeStroke:  "#1F2433",
		NodeStrokeW: 1.4,
		EdgeStroke:  "#1F2433",
		EdgeStrokeW: 1.6,
		EdgeArrow:   "triangle",
		FontFamily:  "Inter, 'Helvetica Neue', Arial, sans-serif",
		FontColor:   "#0F1A33",
		FontSize:    13,
	}
}
