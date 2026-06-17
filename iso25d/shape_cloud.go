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
			strings.TrimSpace(ptsSB.String()), o.LeftFill,
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
			topPath.String(), o.TopFill, escapeAttr(o.Stroke), o.StrokeWidth,
		)
	} else {
		fmt.Fprintf(&sb,
			`<path data-face="top" d="%s" fill="%s" stroke="none"/>`,
			topPath.String(), o.TopFill,
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

// sampleCloudOutline returns a CW (in screen y-down) polyline around the
// cloud silhouette, modelled on a classic icon-style cloud: small left
// bump + tall wide middle bump + small right bump, joined to a substantial
// rectangular trunk with rounded bottom corners. The 3-bump layout gives
// recognisable variation; the trunk gives the silhouette body so the
// cloud reads as "fluffy" rather than "pinched arc".
func sampleCloudOutline(w, d float64) [][2]float64 {
	// Organic cloud: the silhouette is the OUTER envelope of several overlapping
	// lobes of VARYING radius, sampled radially from the centroid. Irregular
	// lobe sizes/positions give natural billows; nothing is a straight or
	// parallel edge (the old 3-uniform-bump-on-a-rectangular-trunk look read as
	// mechanical). Lobes are authored in a unit frame, y-down (top = small y).
	type lobe struct{ cx, cy, r float64 }
	lobes := []lobe{
		{0.17, 0.55, 0.16}, // left shoulder
		{0.33, 0.42, 0.21}, // upper-left billow
		{0.52, 0.35, 0.25}, // crown (tallest, off-centre → asymmetric)
		{0.71, 0.44, 0.20}, // upper-right billow
		{0.86, 0.55, 0.15}, // right shoulder
		{0.40, 0.63, 0.21}, // lower-left belly
		{0.63, 0.62, 0.21}, // lower-right belly
	}
	cx, cy := 0.52, 0.53

	// The cloud is authored upright (billows up); the previous extrusion read
	// the long axis the wrong way, so transpose the unit frame 90° before
	// scaling — local x↔y — to set it upright in the iso projection.
	pt := func(ux, uy float64) [2]float64 { return [2]float64{uy * w, ux * d} }

	const N = 112
	out := make([][2]float64, 0, N)
	for i := 0; i < N; i++ {
		ang := 2 * math.Pi * float64(i) / float64(N)
		dx, dy := math.Cos(ang), math.Sin(ang)
		best := 0.0
		for _, lo := range lobes {
			// farthest hit of ray centroid + t·dir with this lobe's circle
			ox, oy := cx-lo.cx, cy-lo.cy
			b := ox*dx + oy*dy
			c := ox*ox + oy*oy - lo.r*lo.r
			disc := b*b - c
			if disc < 0 {
				continue
			}
			if t := -b + math.Sqrt(disc); t > best {
				best = t
			}
		}
		if best <= 0 {
			continue
		}
		out = append(out, pt(cx+best*dx, cy+best*dy))
	}
	return out
}

// RenderIsoDiamondFlat draws a flat 2D rotated-square diamond with a soft
// drop shadow. We split the diamond into two halves (lit/shaded) so it still
// reads as a faceted gem rather than a 2D sticker.
