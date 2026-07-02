package isotopo

import (
	"fmt"
	"math"
	"sort"
)

// Composition scoring — the POSITIVE half of scene quality. The readability
// score counts defects (occlusion, overlap, crossings: things that are wrong);
// this file measures what makes a scene look composed: balanced visual weight,
// shared alignment tracks, even spacing rhythm, a sane aspect ratio, a
// dominant focal element, and color discipline. Emitted by `evaluate` as a
// report-only block so humans and agents get machine-readable aesthetics
// feedback with located, actionable findings — it does not (yet) participate
// in the repair loop or the readability cost.
//
// Calibration contract (enforced by TestCompositionCalibration): the
// hand-tuned gallery scenes must score in the top band and a deliberately
// sloppy arrangement must score below them — a metric that cannot separate
// known-good from known-bad does not ship.

// CompositionFinding is one located, actionable observation.
type CompositionFinding struct {
	Metric  string `json:"metric"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// CompositionReport is the per-scene aesthetic breakdown. Every sub-score is
// in [0,1] (1 = ideal); Score is their weighted mean. HeroDominance is nil
// when the scene declares no role:hero part — absence of a hero is a style
// choice, not a defect.
type CompositionReport struct {
	Balance       float64  `json:"balance"`
	Alignment     float64  `json:"alignment"`
	Rhythm        float64  `json:"rhythm"`
	Aspect        float64  `json:"aspect"`
	HeroDominance *float64 `json:"hero_dominance,omitempty"`
	AccentHues    int      `json:"accent_hues"`
	Score         float64  `json:"score"`

	Findings []CompositionFinding `json:"findings,omitempty"`
}

// compRect is a part's solved, absolute plan-space footprint.
type compRect struct {
	ord       int // identity for pairwise comparisons (ids may be empty/duplicated)
	id        string
	path      string
	x, y      float64
	w, d      float64
	container bool
	role      string
	depth     int
}

func (r compRect) cx() float64   { return r.x + r.w/2 }
func (r compRect) cy() float64   { return r.y + r.d/2 }
func (r compRect) area() float64 { return r.w * r.d }

// EvaluateComposition scores the document's scene. Safe on a raw document:
// layout is solved on a clone, matching what the renderer paints.
func EvaluateComposition(doc *Document) *CompositionReport {
	rep := &CompositionReport{Balance: 1, Alignment: 1, Rhythm: 1, Aspect: 1, Score: 1}
	if doc == nil {
		return rep
	}
	scene := doc.Scene()
	if scene == nil {
		return rep
	}
	clone := cloneSceneForEval(scene)
	applyLayout(clone, doc.Canvas)

	var all []compRect
	var walk func(parts []*CompositePart, bx, by float64, prefix string, depth int)
	walk = func(parts []*CompositePart, bx, by float64, prefix string, depth int) {
		for i, p := range parts {
			if p == nil {
				continue
			}
			x, y := bx, by
			if p.Offset != nil {
				x += p.Offset.WX
				y += p.Offset.WY
			}
			path := fmt.Sprintf("%s.parts[%d]", prefix, i)
			if p.ID != "" {
				path = fmt.Sprintf("%s.parts[%s]", prefix, p.ID)
			}
			w, d := partFootprint(p)
			all = append(all, compRect{
				id: p.ID, path: path, x: x, y: y, w: w, d: d,
				container: isContainerShape(p.Shape), role: p.Role, depth: depth,
			})
			walk(p.Parts, x, y, path, depth+1)
		}
	}
	walk(clone.Parts, 0, 0, "nodes.scene", 0)
	if len(all) == 0 {
		return rep
	}

	var top, leaves []compRect
	for _, r := range all {
		if r.depth == 0 {
			top = append(top, r)
		}
		if !r.container {
			leaves = append(leaves, r)
		}
	}

	rep.Balance = compBalance(top, &rep.Findings)
	rep.Alignment = compAlignment(leaves, &rep.Findings)
	rep.Rhythm = compRhythm(top, &rep.Findings)
	rep.Aspect = compAspect(all, &rep.Findings)
	rep.HeroDominance = compHero(all, leaves, &rep.Findings)
	rep.AccentHues = compAccents(doc, clone, &rep.Findings)

	// Weighted composite; hero renormalises out when absent.
	colorScore := 1.0
	switch {
	case rep.AccentHues == 2:
		colorScore = 0.85
	case rep.AccentHues == 3:
		colorScore = 0.6
	case rep.AccentHues > 3:
		colorScore = 0.35
	}
	num := 0.22*rep.Balance + 0.25*rep.Alignment + 0.18*rep.Rhythm + 0.10*rep.Aspect + 0.15*colorScore
	den := 0.90
	if rep.HeroDominance != nil {
		num += 0.10 * *rep.HeroDominance
		den += 0.10
	}
	rep.Score = num / den
	return rep
}

// compBalance: area-weighted centroid of the top-level footprints vs the
// scene's bounding-box centre. A composed scene distributes visual weight;
// a lopsided one reads as tipping over.
func compBalance(top []compRect, findings *[]CompositionFinding) float64 {
	if len(top) < 2 {
		return 1
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	var wSum, cxSum, cySum float64
	for _, r := range top {
		minX = math.Min(minX, r.x)
		minY = math.Min(minY, r.y)
		maxX = math.Max(maxX, r.x+r.w)
		maxY = math.Max(maxY, r.y+r.d)
		w := r.area()
		wSum += w
		cxSum += r.cx() * w
		cySum += r.cy() * w
	}
	if wSum <= 0 || maxX <= minX || maxY <= minY {
		return 1
	}
	bw, bh := maxX-minX, maxY-minY
	dx := (cxSum/wSum - (minX + bw/2)) / (bw / 2)
	dy := (cySum/wSum - (minY + bh/2)) / (bh / 2)
	imb := math.Min(1, math.Max(math.Abs(dx), math.Abs(dy)))
	if imb > 0.3 {
		axis, dir := "x", "right"
		if math.Abs(dy) > math.Abs(dx) {
			axis, dir = "y", "front"
			if dy < 0 {
				dir = "back"
			}
		} else if dx < 0 {
			dir = "left"
		}
		*findings = append(*findings, CompositionFinding{
			Metric:  "balance",
			Path:    "nodes.scene",
			Message: fmt.Sprintf("visual weight leans %s (%.0f%% off-centre on the %s axis) — move a tray toward the opposite side or rebalance sizes", dir, imb*100, axis),
		})
	}
	return 1 - imb
}

// compAlignment: the fraction of leaf parts that share at least one
// alignment track (an x or y edge/centre, within tolerance) with some other
// leaf. One shared axis is what rows, columns, and grids produce — solver
// output aligns naturally; freehand scatter does not. (Requiring BOTH axes
// was wrong: a perfectly composed single row shares only its y-track — the
// calibration sweep scored solver-laid scenes 0.0 and caught the mistake.)
func compAlignment(leaves []compRect, findings *[]CompositionFinding) float64 {
	if len(leaves) < 3 {
		return 1
	}
	const tol = 6.0
	near := func(a, b float64) bool { return math.Abs(a-b) <= tol }
	alignedCount := 0
	var loose []compRect
	for i, r := range leaves {
		ax, ay := false, false
		for j, o := range leaves {
			if i == j {
				continue
			}
			if near(r.x, o.x) || near(r.cx(), o.cx()) || near(r.x+r.w, o.x+o.w) {
				ax = true
			}
			if near(r.y, o.y) || near(r.cy(), o.cy()) || near(r.y+r.d, o.y+o.d) {
				ay = true
			}
			if ax || ay {
				break
			}
		}
		if ax || ay {
			alignedCount++
		} else {
			loose = append(loose, r)
		}
	}
	score := float64(alignedCount) / float64(len(leaves))
	for i, r := range loose {
		if i >= 2 { // cap the noise; the score already says how widespread it is
			break
		}
		*findings = append(*findings, CompositionFinding{
			Metric:  "alignment",
			Path:    r.path,
			Message: "shares no alignment track (edge or centre) with any other node — nudge it onto a neighbour's row or column",
		})
	}
	return score
}

// compRhythm: coefficient of variation of each top-level part's
// nearest-neighbour clearance. Even breathing room reads as designed;
// erratic gaps read as scattered.
func compRhythm(top []compRect, findings *[]CompositionFinding) float64 {
	if len(top) < 3 {
		return 1
	}
	gap := func(a, b compRect) float64 {
		dx := math.Max(0, math.Max(b.x-(a.x+a.w), a.x-(b.x+b.w)))
		dy := math.Max(0, math.Max(b.y-(a.y+a.d), a.y-(b.y+b.d)))
		return math.Hypot(dx, dy)
	}
	var gaps []float64
	for i, r := range top {
		best := math.Inf(1)
		for j, o := range top {
			if i != j {
				best = math.Min(best, gap(r, o))
			}
		}
		if !math.IsInf(best, 1) {
			gaps = append(gaps, best)
		}
	}
	var sum float64
	for _, g := range gaps {
		sum += g
	}
	mean := sum / float64(len(gaps))
	if mean <= 1 {
		return 1 // everything touching/nested — rhythm not meaningful
	}
	var vsum float64
	for _, g := range gaps {
		vsum += (g - mean) * (g - mean)
	}
	cv := math.Sqrt(vsum/float64(len(gaps))) / mean
	score := 1 / (1 + cv)
	if cv > 0.75 {
		*findings = append(*findings, CompositionFinding{
			Metric:  "rhythm",
			Path:    "nodes.scene",
			Message: fmt.Sprintf("tray spacing is erratic (gap variation %.0f%% of the mean) — equalise the gaps between neighbouring trays", cv*100),
		})
	}
	return score
}

// compAspect: the scene bounding box against the 16:10-ish band the renderer
// already pads toward. Too tall or too wide wastes screen when embedded.
func compAspect(all []compRect, findings *[]CompositionFinding) float64 {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range all {
		minX = math.Min(minX, r.x)
		minY = math.Min(minY, r.y)
		maxX = math.Max(maxX, r.x+r.w)
		maxY = math.Max(maxY, r.y+r.d)
	}
	w, h := maxX-minX, maxY-minY
	if w <= 0 || h <= 0 {
		return 1
	}
	ratio := w / h
	const lo, hi = 1.1, 2.4 // wide band: iso projection stretches x on screen
	score := 1.0
	if ratio < lo {
		score = ratio / lo
	} else if ratio > hi {
		score = hi / ratio
	}
	if score < 0.7 {
		shape := "tall and narrow — spread trays horizontally"
		if ratio > hi {
			shape = "wide and flat — stack trays into a second row"
		}
		*findings = append(*findings, CompositionFinding{
			Metric:  "aspect",
			Path:    "nodes.scene",
			Message: fmt.Sprintf("scene plan is %.1f:1 (%s)", ratio, shape),
		})
	}
	return score
}

// compHero: when a role:hero part exists, it should read as the focal
// element — larger than the median leaf and near the scene's centre.
func compHero(all, leaves []compRect, findings *[]CompositionFinding) *float64 {
	var hero *compRect
	for i := range all {
		if all[i].role == "hero" {
			hero = &all[i]
			break
		}
	}
	if hero == nil {
		return nil
	}
	var areas []float64
	for _, r := range leaves {
		if r.role != "hero" {
			areas = append(areas, r.area())
		}
	}
	if len(areas) == 0 {
		one := 1.0
		return &one
	}
	sort.Float64s(areas)
	median := areas[len(areas)/2]
	dominance := hero.area() / math.Max(median, 1)

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range all {
		minX = math.Min(minX, r.x)
		minY = math.Min(minY, r.y)
		maxX = math.Max(maxX, r.x+r.w)
		maxY = math.Max(maxY, r.y+r.d)
	}
	diag := math.Hypot(maxX-minX, maxY-minY) / 2
	centrality := 1.0
	if diag > 0 {
		dist := math.Hypot(hero.cx()-(minX+maxX)/2, hero.cy()-(minY+maxY)/2)
		centrality = math.Max(0, 1-dist/diag)
	}
	score := 0.5*math.Min(dominance, 2)/2 + 0.5*centrality
	if dominance < 1.2 {
		*findings = append(*findings, CompositionFinding{
			Metric:  "hero",
			Path:    hero.path,
			Message: fmt.Sprintf("the hero is only %.1fx the median node area — enlarge it (or its roleGeom) so the focal element dominates", dominance),
		})
	}
	if centrality < 0.45 {
		*findings = append(*findings, CompositionFinding{
			Metric:  "hero",
			Path:    hero.path,
			Message: "the hero sits at the scene's edge — move it toward the centre so the eye lands on it first",
		})
	}
	return &score
}

// compAccents counts distinct ACCENT hues across the resolved top-face fills
// (saturated, mid-lightness colors; neutrals are free). The gallery norm is a
// single accent; three or more competing hues read as noise.
func compAccents(doc *Document, scene *Node, findings *[]CompositionFinding) int {
	var hues []float64
	var walk func(parts []*CompositePart)
	walk = func(parts []*CompositePart) {
		for _, p := range parts {
			if p == nil {
				continue
			}
			eff := ResolveStyleWithRole(doc.Theme, p.Shape, p.Role, p.Preset, p.Style)
			for _, c := range topFillColors(eff) {
				if h, s, l, ok := hexToHSL(c); ok && s >= 0.28 && l >= 0.18 && l <= 0.85 {
					hues = append(hues, h)
				}
			}
			walk(p.Parts)
		}
	}
	walk(scene.Parts)
	if len(hues) == 0 {
		return 0
	}
	sort.Float64s(hues)
	const gap = 35.0 // degrees between distinct accent families
	clusters := 1
	for i := 1; i < len(hues); i++ {
		if hues[i]-hues[i-1] > gap {
			clusters++
		}
	}
	// Circular wrap: first and last may be the same family through 360°.
	if clusters > 1 && (360-hues[len(hues)-1])+hues[0] <= gap {
		clusters--
	}
	if clusters > 2 {
		*findings = append(*findings, CompositionFinding{
			Metric:  "color",
			Path:    "theme",
			Message: fmt.Sprintf("%d competing accent hues — the gallery norm is ONE accent family; move secondary colors toward neutrals", clusters),
		})
	}
	return clusters
}

// hexToHSL converts #RRGGBB(AA) to hue [0,360), saturation and lightness [0,1].
func hexToHSL(hex string) (h, s, l float64, ok bool) {
	r8, g8, b8, ok := parseHex(hex)
	if !ok {
		return 0, 0, 0, false
	}
	r, g, b := float64(r8)/255, float64(g8)/255, float64(b8)/255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2
	if max == min {
		return 0, 0, l, true
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = math.Mod((g-b)/d+6, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l, true
}
