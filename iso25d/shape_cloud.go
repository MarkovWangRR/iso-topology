package iso25d

import (
	"fmt"
	"math"
	"strings"
)

func RenderIsoCloud(o IsoBoxOpts) string {
	w, d, h, m := o.Width, o.Depth, o.Height, o.Margin

	outline := sampleCloudOutline(w, d)
	n := len(outline)

	type vp struct{ topX, topY, botX, botY float64 }
	proj := make([]vp, n)
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for i, p := range outline {
		sx, sy := project(p[0], p[1], 0)
		proj[i] = vp{topX: sx, topY: sy - h, botX: sx, botY: sy}
		if sx < minX {
			minX = sx
		}
		if sx > maxX {
			maxX = sx
		}
		if sy-h < minY {
			minY = sy - h
		}
		if sy > maxY {
			maxY = sy
		}
	}
	tx, ty := -minX+m, -minY+m
	W := maxX - minX + 2*m
	H := maxY - minY + 2*m

	var sb strings.Builder
	sb.WriteString(svgHeader(W, H))
	openWrapper(&sb, W, H, o.Background, o.Opacity, o.StrokeDasharray, "")

	// Per-segment front-facing test, in SCREEN space: a vertical side wall is
	// visible iff it sits on the lower silhouette — its outward normal points
	// toward the camera (+screenY). We orient each edge's normal away from the
	// projected centroid, so the test holds for ANY outline winding (the old
	// dy>dx heuristic on local coords only worked for the previous fixed bump
	// layout, so the organic outline lost most of its wall).
	var cxs, cys float64
	for i := 0; i < n; i++ {
		cxs += proj[i].topX
		cys += proj[i].topY
	}
	cxs /= float64(n)
	cys /= float64(n)
	visible := make([]bool, n)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		ex := proj[j].topX - proj[i].topX
		ey := proj[j].topY - proj[i].topY
		nx, ny := ey, -ex // a perpendicular to the edge
		mx := (proj[i].topX+proj[j].topX)/2 - cxs
		my := (proj[i].topY+proj[j].topY)/2 - cys
		if mx*nx+my*ny < 0 { // flip so the normal points AWAY from centroid
			ny = -ny
		}
		visible[i] = ny > 0 // outward normal faces down → camera-facing wall
	}

	// Walk visible runs and draw one polygon per run (fill only, no stroke so
	// adjacent run boundaries don't double-stroke each other). The screen-space
	// test yields the single contiguous lower-silhouette arc, so no run-length
	// filtering is needed.
	type run struct{ idx []int }
	runs := []run{}
	for i := 0; i < n; i++ {
		if !visible[i] || visible[(i-1+n)%n] {
			continue
		}
		r := run{idx: []int{i}}
		j := i
		for visible[j] {
			j = (j + 1) % n
			r.idx = append(r.idx, j)
			if j == i {
				break
			}
		}
		runs = append(runs, r)
	}

	for _, r := range runs {
		var ptsSB strings.Builder
		for _, k := range r.idx {
			fmt.Fprintf(&ptsSB, "%.2f,%.2f ", proj[k].topX+tx, proj[k].topY+ty)
		}
		for k := len(r.idx) - 1; k >= 0; k-- {
			fmt.Fprintf(&ptsSB, "%.2f,%.2f ", proj[r.idx[k]].botX+tx, proj[r.idx[k]].botY+ty)
		}
		fmt.Fprintf(&sb,
			`<polygon data-face="side" points="%s" fill="%s" stroke="none"/>`,
			strings.TrimSpace(ptsSB.String()), escapeAttr(o.LeftFill),
		)
	}

	// Per-run silhouette outline: vertical-down at run start, along bottom
	// edge, vertical-up at run end. Single open path so no internal seams.
	if strings.TrimSpace(o.Stroke) != "" && o.StrokeWidth > 0 {
		for _, r := range runs {
			var pathSB strings.Builder
			first := r.idx[0]
			last := r.idx[len(r.idx)-1]
			fmt.Fprintf(&pathSB,
				"M %.2f %.2f L %.2f %.2f",
				proj[first].topX+tx, proj[first].topY+ty,
				proj[first].botX+tx, proj[first].botY+ty,
			)
			for k := 1; k < len(r.idx); k++ {
				fmt.Fprintf(&pathSB,
					" L %.2f %.2f",
					proj[r.idx[k]].botX+tx, proj[r.idx[k]].botY+ty,
				)
			}
			fmt.Fprintf(&pathSB,
				" L %.2f %.2f",
				proj[last].topX+tx, proj[last].topY+ty,
			)
			fmt.Fprintf(&sb,
				`<path data-face="outline" d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round" stroke-linecap="round"/>`,
				pathSB.String(), escapeAttr(o.Stroke), o.StrokeWidth,
			)
		}
	}

	// Top face: polyline path filled with TopFill.
	var topPath strings.Builder
	fmt.Fprintf(&topPath, "M %.2f %.2f", proj[0].topX+tx, proj[0].topY+ty)
	for i := 1; i < n; i++ {
		fmt.Fprintf(&topPath, " L %.2f %.2f", proj[i].topX+tx, proj[i].topY+ty)
	}
	topPath.WriteString(" Z")

	if strings.TrimSpace(o.Stroke) != "" && o.StrokeWidth > 0 {
		fmt.Fprintf(&sb,
			`<path data-face="top" d="%s" fill="%s" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"/>`,
			topPath.String(), escapeAttr(o.TopFill), escapeAttr(o.Stroke), o.StrokeWidth,
		)
	} else {
		fmt.Fprintf(&sb,
			`<path data-face="top" d="%s" fill="%s" stroke="none"/>`,
			topPath.String(), escapeAttr(o.TopFill),
		)
	}

	// Label on the top face (matrix transform tilts content onto the iso
	// plane). Cloud is an outline silhouette, not a flat slab — a pasted icon
	// reads as floating debris, so the cloud shape deliberately ignores o.Icon;
	// its identity IS the cloud form.
	writeTopLabelAndIcon(
		&sb,
		tx, ty-h, w, d,
		o.Label, "", o.IconScale,
		o.FontFamily, o.FontSize, o.FontWeight, o.FontColor,
	)

	closeWrapper(&sb)
	sb.WriteString(`</svg>`)
	return sb.String()
}

// sampleCloudOutline returns the outline of a classic 3-bump cloud silhouette,
// analytically constructed as the union boundary of three overlapping circles
// (left small, center tall/dominant, right medium) plus a flat base.
//
// The unit frame is ux=0..1 left→right, uy=0..1 top→bottom. The outline is
// returned in local iso coords via the pt() transpose and is CW in screen space.
func sampleCloudOutline(w, d float64) [][2]float64 {
	// Three circles — chosen so each pair of adjacent circles overlaps, the
	// tops of the side bumps protrude above the center circle, and the left and
	// right circles do NOT directly overlap (gives the classic 3-bump silhouette).
	type circ struct{ cx, cy, r float64 }
	bL := circ{0.19, 0.60, 0.22} // left bump  (small)
	bC := circ{0.50, 0.40, 0.29} // center bump (crown / tallest)
	bR := circ{0.81, 0.54, 0.24} // right bump  (medium)

	// ── helpers ──────────────────────────────────────────────────────────────

	// Circle-circle intersection: returns the two intersection points and true,
	// or false when the circles are disjoint or one contains the other.
	intersect2 := func(a, b circ) ([2]float64, [2]float64, bool) {
		dx, dy := b.cx-a.cx, b.cy-a.cy
		dd := math.Sqrt(dx*dx + dy*dy)
		if dd < 1e-9 || dd > a.r+b.r+1e-9 || dd < math.Abs(a.r-b.r)-1e-9 {
			return [2]float64{}, [2]float64{}, false
		}
		aa := (a.r*a.r - b.r*b.r + dd*dd) / (2 * dd)
		hh := math.Sqrt(math.Max(0, a.r*a.r-aa*aa))
		px, py := a.cx+aa*dx/dd, a.cy+aa*dy/dd
		return [2]float64{px + hh*dy/dd, py - hh*dx/dd},
			[2]float64{px - hh*dy/dd, py + hh*dx/dd}, true
	}

	// Angle of point p on circle c, returned in [0, 2π).
	angOf := func(c circ, p [2]float64) float64 {
		a := math.Atan2(p[1]-c.cy, p[0]-c.cx)
		if a < 0 {
			a += 2 * math.Pi
		}
		return a
	}

	// sampleArcCW samples n points along the arc of circle c from angle a1 to
	// a2 going CW in screen space (= increasing angle in [0,2π) uy-down coords).
	// If a2 <= a1 the arc wraps through 2π.
	sampleArcCW := func(c circ, a1, a2 float64, n int) [][2]float64 {
		for a2 <= a1 {
			a2 += 2 * math.Pi
		}
		pts := make([][2]float64, n)
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n-1)
			a := a1 + t*(a2-a1)
			pts[i] = [2]float64{c.cx + c.r*math.Cos(a), c.cy + c.r*math.Sin(a)}
		}
		return pts
	}

	// ── find intersections ────────────────────────────────────────────────────

	lc0, lc1, okLC := intersect2(bL, bC)
	cr0, cr1, okCR := intersect2(bC, bR)
	if !okLC || !okCR {
		// Degenerate geometry: fall back to simple ellipse outline.
		return sampleEllipseOutline(w, d)
	}

	// Upper intersection = smaller uy (higher on screen).
	upLC, loLC := lc0, lc1
	if lc1[1] < lc0[1] {
		upLC, loLC = lc1, lc0
	}
	upCR, loCR := cr0, cr1
	if cr1[1] < cr0[1] {
		upCR, loCR = cr1, cr0
	}

	// ── arc angles ───────────────────────────────────────────────────────────

	// bL outer arc: from loLC going CW (increasing angle) through bL's top
	// (3π/2) to upLC. This traces the left, top, and upper-right of the left bump.
	aL1 := angOf(bL, loLC) // lower intersection on bL  (~30°)
	aL2 := angOf(bL, upLC) // upper intersection on bL  (~275°)
	// Both endpoints should be in [0°..90°) and [270°..360°) respectively when
	// bL is to the left of bC. Make sure the arc passes through 3π/2 (bL's top).
	// CW means increasing; if aL2 > aL1 already the arc is < 180°: wrong direction.
	// Force aL2 to be after 3π/2 by normalising.
	topL := 3 * math.Pi / 2
	if aL2 < topL { // upLC is below bL's top in angle — should not happen with our circles
		aL2 += 2 * math.Pi
	}

	// bC top arc: from upLC going CW through bC's top (3π/2) to upCR.
	aC1 := angOf(bC, upLC) // ~183° (left side of bC, just above mid)
	aC2 := angOf(bC, upCR) // ~341° (upper-right of bC)
	topC := 3 * math.Pi / 2
	if aC2 < topC {
		aC2 += 2 * math.Pi
	}

	// bR outer arc: from upCR going CW through bR's top (3π/2) to loCR.
	aR1 := angOf(bR, upCR) // upper intersection on bR (~178°, upper-left of bR)
	aR2 := angOf(bR, loCR) // lower intersection on bR (~88°, lower-left of bR)
	topR := 3 * math.Pi / 2
	// Arc must pass through 3π/2 (top of bR). If aR2 > aR1 the short path goes
	// the wrong way; normalise so we travel the long CW route through the top.
	if aR2 > aR1 {
		aR2 += 2 * math.Pi // wrap: arc continues past 2π
	}
	if aR2 < topR+aR1-aR1 { // ensure midpoint 3π/2 is within [aR1, aR2]
		_ = topR // guard used above; this branch is informational
	}

	// ── assemble outline ─────────────────────────────────────────────────────

	const K = 24 // sample density per arc
	var raw [][2]float64

	// 1. Left bump outer arc (loLC → upLC going over bL's top)
	raw = append(raw, sampleArcCW(bL, aL1, aL2, K)...)

	// 2. Center bump top arc (upLC → upCR going over bC's crown)
	raw = append(raw, sampleArcCW(bC, aC1, aC2, K)...)

	// 3. Right bump outer arc (upCR → loCR going over bR's top)
	raw = append(raw, sampleArcCW(bR, aR1, aR2, K)...)

	// 4. Flat base: straight line from loCR back to loLC.
	// To keep the base horizontal, clamp both to the same uy level.
	baseUY := math.Max(loLC[1], loCR[1])
	raw = append(raw, [2]float64{loCR[0], baseUY})
	raw = append(raw, [2]float64{loLC[0], baseUY})

	// ── transpose into iso local coords ──────────────────────────────────────
	// The cloud is authored with bumps rising in the uy direction; transposing
	// ux↔uy (and scaling) sets it upright in the isometric projection where w
	// is the wide axis and d the depth axis.
	out := make([][2]float64, len(raw))
	for i, p := range raw {
		out[i] = [2]float64{p[1] * w, p[0] * d}
	}
	return out
}

// sampleEllipseOutline is the fallback when the cloud circles don't intersect.
func sampleEllipseOutline(w, d float64) [][2]float64 {
	const N = 72
	out := make([][2]float64, N)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(N)
		out[i] = [2]float64{(0.5 + 0.48*math.Cos(a)) * w, (0.5 + 0.48*math.Sin(a)) * d}
	}
	return out
}

// RenderIsoDiamondFlat draws a flat 2D rotated-square diamond with a soft
// drop shadow. We split the diamond into two halves (lit/shaded) so it still
// reads as a faceted gem rather than a 2D sticker.
