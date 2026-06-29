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
// cloneSceneForEval deep-copies the parts (whose offsets/Layout applyLayout
// mutates) and the node's own Layout, sharing the connector slice (untouched by
// solving), so evaluating a scene never alters the caller's document.
func cloneSceneForEval(n *Node) *Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Parts = cloneParts(n.Parts)
	if n.Layout != nil {
		l := *n.Layout
		c.Layout = &l
	}
	return &c
}

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
	// Evaluation must NOT mutate the caller's scene. buildPlanModel and
	// isoRealRoutes both run applyLayout (which clears Layout/Place and writes
	// Offsets) in place, so score a deep clone — otherwise callers like
	// Readability silently solve-and-clear the doc, breaking any later pass
	// (e.g. RepairScene) that needs the original Layout declarations.
	n = cloneSceneForEval(n)
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
	rep := evalGeom(leaves, edges)
	// Overlap reflects the VISUAL truth: count collisions between distinct
	// top-level parts' footprints — group SLABS included, not just leaves — so a
	// tray colliding with another tray (which leaf-only counting misses) is in R,
	// matching validate and the eye. Parent/child pairs share an owner and don't
	// count.
	rep.NodeOverlaps = countTopLevelCollisions(n.Parts, rects)
	return rep
}

// countTopLevelCollisions returns the number of distinct top-level-part pairs
// whose footprints collide (Z-aware), excluding parent/child. Footprint = the
// part's own rect (a group's slab, or a leaf's box).
func countTopLevelCollisions(topParts []*CompositePart, rects []planRect) int {
	descTop := map[string]string{}
	var mark func(top string, p *CompositePart)
	mark = func(top string, p *CompositePart) {
		if p.ID != "" {
			descTop[p.ID] = top
		}
		for _, c := range p.Parts {
			if c != nil {
				mark(top, c)
			}
		}
	}
	topIDs := map[string]bool{}
	for _, p := range topParts {
		if p != nil {
			mark(p.ID, p)
			if p.ID != "" {
				topIDs[p.ID] = true
			}
		}
	}
	var boxes []planRect
	for _, r := range rects {
		if !r.container || topIDs[r.id] {
			boxes = append(boxes, r)
		}
	}
	seen := map[[2]string]bool{}
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			ta, tb := descTop[boxes[i].id], descTop[boxes[j].id]
			if ta == "" || tb == "" || ta == tb {
				continue
			}
			if !rectsOverlap(boxes[i], boxes[j]) {
				continue
			}
			key := [2]string{ta, tb}
			if ta > tb {
				key = [2]string{tb, ta}
			}
			seen[key] = true
		}
	}
	return len(seen)
}
