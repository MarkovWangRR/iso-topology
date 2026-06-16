package isotopo

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// planeval_iso.go is P0: score the ISO ENGINE'S REAL connector routes, not the
// plan view's simplified ones. The engine already computes each orthogonal
// route in WORLD space and emits it in the SVG as data-route
// ("sx,sy,wx,wy ..."), in the SAME world frame as the plan footprints — so we
// render iso, parse those world polylines, and run the identical metric kernel.
// This is the honest scorecard for what users actually get, and the basis for
// later letting the scorecard DRIVE the engine router.

var reIsoRoute = regexp.MustCompile(`data-connector="(\d+)" data-from="[^"]*" data-to="[^"]*" data-route="([^"]*)"`)

// isoRealRoutes renders the scene isometrically and parses each connector's
// real WORLD route from its data-route attribute, keyed by connector index.
// Straight-routed connectors emit no data-route and are simply absent.
func isoRealRoutes(n *Node, theme *Theme, canvas *Canvas) map[int][][2]float64 {
	cv := &Canvas{}
	if canvas != nil {
		c := *canvas
		cv = &c
	}
	cv.Projection = "iso" // force iso even if the document asked for top
	svg := RenderWithCanvas(n, theme, cv, nil)

	out := map[int][][2]float64{}
	for _, m := range reIsoRoute.FindAllStringSubmatch(svg, -1) {
		ci, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		var pts [][2]float64
		for _, corner := range strings.Fields(m[2]) {
			f := strings.Split(corner, ",")
			if len(f) != 4 {
				continue
			}
			wx, e1 := strconv.ParseFloat(f[2], 64)
			wy, e2 := strconv.ParseFloat(f[3], 64)
			if e1 == nil && e2 == nil {
				pts = append(pts, [2]float64{wx, wy})
			}
		}
		if len(pts) >= 2 {
			out[ci] = pts
		}
	}
	return out
}

// EvaluateIsoText parses a document and scores the iso engine's real routing.
func EvaluateIsoText(format string, src []byte) (*PlanReport, error) {
	doc, err := LoadInput(context.Background(), format, src, LayoutDagre)
	if err != nil {
		return nil, err
	}
	scene := doc.Scene()
	if scene == nil {
		return nil, fmt.Errorf("document has no scene to evaluate")
	}
	return EvaluateIso(scene, doc.Theme, doc.Canvas), nil
}

// EvaluateIso scores the scene using the engine's real connector routes where
// available (orthogonal edges), falling back to the plan route for straight
// edges that emit none. Node footprints are the same applyLayout output.
func EvaluateIso(n *Node, theme *Theme, canvas *Canvas) *PlanReport {
	rects, _, edges := buildPlanModel(n, theme, canvas)
	real := isoRealRoutes(n, theme, canvas)
	for i := range edges {
		if r, ok := real[edges[i].ci]; ok {
			edges[i].pts = r
		}
	}
	var leaves []planRect
	for _, r := range rects {
		if !r.container {
			leaves = append(leaves, r)
		}
	}
	return evalGeom(leaves, edges)
}
