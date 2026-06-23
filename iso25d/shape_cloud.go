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

	// 顶面文字与其他节点保持一致：用等轴投影矩阵倾斜放置。
	// Cloud 不渲染 icon，cloud 轮廓本身即是身份标识。
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

// sampleCloudOutline returns the outline of the classic 2-bump cloud silhouette
// (Apple / iOS / macOS style): a large main bump on the left, a smaller secondary
// bump on the right, and a rounded-rectangle body below.
//
// The shape is the union boundary of 4 circles analytically traced:
//   bMain  – large left bump
//   bSec   – smaller right bump
//   bCL    – rounded bottom-left corner of the body
//   bCR    – rounded bottom-right corner of the body
//
// Unit frame: ux=0..1 left→right, uy=0..1 top→bottom (small uy = high).
// Returned in local iso coords (ux↔uy transposed, scaled to w and d).
// Winding is CW in screen space (increasing angle in uy-down convention).
func sampleCloudOutline(w, d float64) [][2]float64 {
	type circ struct{ cx, cy, r float64 }

	bMain := circ{0.36, 0.40, 0.29} // large main bump (left-center)
	bSec  := circ{0.69, 0.54, 0.20} // smaller secondary bump (right)
	bCL   := circ{0.15, 0.74, 0.15} // body — bottom-left rounded corner
	bCR   := circ{0.85, 0.74, 0.15} // body — bottom-right rounded corner

	// ── helpers ──────────────────────────────────────────────────────────────

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

	// Angle of p on circle c, normalised to [0, 2π).
	angOf := func(c circ, p [2]float64) float64 {
		a := math.Atan2(p[1]-c.cy, p[0]-c.cx)
		if a < 0 {
			a += 2 * math.Pi
		}
		return a
	}

	// sampleArcCW samples n equally-spaced points on circle c from angle a1 to
	// a2, going CW in screen space (= increasing angle in uy-down convention).
	// If a2 ≤ a1 the arc wraps past 2π.
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

	// ── intersections ────────────────────────────────────────────────────────

	// bMain ∩ bCL: junction on bMain's lower-left / bCL's upper-right.
	// We want the LOWER intersection (larger uy) — that is the physical seam
	// where the left side of the body transitions into the main bump.
	mc0, mc1, okMC := intersect2(bMain, bCL)
	if !okMC {
		return sampleEllipseOutline(w, d)
	}
	jMainCL := mc0 // lower (larger uy) = the body→bump seam
	if mc1[1] > mc0[1] {
		jMainCL = mc1
	}

	// bMain ∩ bSec: the valley between the two bumps.
	// We want the UPPER intersection (smaller uy) — the visible notch.
	ms0, ms1, okMS := intersect2(bMain, bSec)
	if !okMS {
		return sampleEllipseOutline(w, d)
	}
	jMainSec := ms0 // upper (smaller uy) = the inter-bump notch
	if ms1[1] < ms0[1] {
		jMainSec = ms1
	}

	// bSec ∩ bCR: junction where the secondary bump meets the right body corner.
	// We want the UPPER intersection (smaller uy).
	sc0, sc1, okSC := intersect2(bSec, bCR)
	if !okSC {
		return sampleEllipseOutline(w, d)
	}
	jSecCR := sc0 // upper (smaller uy) = bump→corner seam
	if sc1[1] < sc0[1] {
		jSecCR = sc1
	}

	// ── arc sequence (5 segments) ─────────────────────────────────────────────
	//
	//  1. bMain outer arc  : jMainCL  → jMainSec (over bMain's crown, ~244°)
	//  2. bSec  outer arc  : jMainSec → jSecCR   (over bSec's top,   ~119°)
	//  3. bCR   corner arc : jSecCR   → bCR ⊥    (right+bottom,      ~167°)
	//  4. flat base        : bCR ⊥   → bCL ⊥
	//  5. bCL   corner arc : bCL ⊥   → jMainCL  (bottom+left+upper,  ~247°)

	const K = 24
	var raw [][2]float64

	// 1. Main bump — from body-left seam, over the crown, to the valley notch.
	raw = append(raw, sampleArcCW(bMain,
		angOf(bMain, jMainCL),  // lower-left of bMain (~105°)
		angOf(bMain, jMainSec), // upper-right of bMain (~349°)
		K)...)

	// 2. Secondary bump — from the valley notch, over the top, to the body-right seam.
	raw = append(raw, sampleArcCW(bSec,
		angOf(bSec, jMainSec), // upper-left of bSec  (~257°)
		angOf(bSec, jSecCR),   // lower-right of bSec (~16°)  → wraps past 2π
		K)...)

	// 3. Right corner — from the body-right seam, around the right+bottom of bCR.
	raw = append(raw, sampleArcCW(bCR,
		angOf(bCR, jSecCR), // upper-right of bCR (~283°)
		math.Pi/2,          // bottom of bCR (90°)  → wraps past 2π
		K)...)

	// 4. Flat base connecting the two bottom corners.
	baseY := bCL.cy + bCL.r // = 0.89
	raw = append(raw, [2]float64{bCR.cx, baseY}) // right end of base
	raw = append(raw, [2]float64{bCL.cx, baseY}) // left end of base

	// 5. Left corner — from bCL's bottom, up the left side, to the body-left seam.
	raw = append(raw, sampleArcCW(bCL,
		math.Pi/2,          // bottom of bCL (90°)
		angOf(bCL, jMainCL), // upper-right of bCL (~338°)
		K)...)

	// ── rotate CW 90° then transpose into iso local coords ───────────────────
	// Rotation in unit frame (around centre 0.5,0.5):  (ux,uy) → (uy, 1-ux)
	// vyScale compresses the depth so the cloud reads as a flat slab.
	const vyScale = 0.62
	out := make([][2]float64, len(raw))
	for i, p := range raw {
		rux := p[1]       // new ux = old uy
		ruy := 1 - p[0]  // new uy = 1 - old ux
		out[i] = [2]float64{ruy * vyScale * w, rux * d}
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
