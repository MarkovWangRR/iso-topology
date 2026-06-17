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

	// Per-segment front-facing test.
	visible := make([]bool, n)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		dx := outline[j][0] - outline[i][0]
		dy := outline[j][1] - outline[i][1]
		visible[i] = dy > dx
	}

	// Walk visible runs and draw one polygon per run (fill only, no stroke
	// so adjacent run boundaries don't double-stroke each other).
	//
	// The (dy > dx) test catches the steep descent tail of each bump as
	// "visible", producing 2–5 segment "fin" runs under bumps 1–3. We drop
	// runs shorter than minRunPoints — the real bottom-arc run is 30+
	// segments so the threshold cleanly separates noise from signal.
	const minRunPoints = 7
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
		if len(r.idx) < minRunPoints {
			continue
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
	out := [][2]float64{}

	horizonY := 0.50 * d
	bottomY := 0.85 * d
	leftX := 0.04 * w
	rightX := 0.96 * w
	cornerR := 0.06 * d

	// Bump layout: [leftX, rightX, peakY] in local frame. Bump 1 small,
	// bump 2 big-and-tall, bump 3 small (matches reference cloud icon).
	bumps := [3][3]float64{
		{leftX, 0.26 * w, 0.32 * d},
		{0.26 * w, 0.74 * w, 0.06 * d},
		{0.74 * w, rightX, 0.24 * d},
	}

	nPerBump := 16
	for bi, b := range bumps {
		bLeft, bRight, bPeak := b[0], b[1], b[2]
		cx := (bLeft + bRight) / 2
		rx := (bRight - bLeft) / 2
		ry := horizonY - bPeak
		startI := 0
		if bi > 0 {
			startI = 1
		}
		for i := startI; i <= nPerBump; i++ {
			t := float64(i) / float64(nPerBump)
			angle := math.Pi - t*math.Pi
			x := cx + rx*math.Cos(angle)
			y := horizonY - ry*math.Sin(angle)
			out = append(out, [2]float64{x, y})
		}
	}

	// Right vertical edge of the trunk.
	nSide := 6
	for i := 1; i <= nSide; i++ {
		t := float64(i) / float64(nSide)
		y := horizonY + t*(bottomY-cornerR-horizonY)
		out = append(out, [2]float64{rightX, y})
	}

	// Right-bottom rounded corner (quarter circle).
	nCorner := 6
	cxR := rightX - cornerR
	cyR := bottomY - cornerR
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		angle := -t * math.Pi / 2
		x := cxR + cornerR*math.Cos(angle)
		y := cyR - cornerR*math.Sin(angle)
		out = append(out, [2]float64{x, y})
	}

	// Flat bottom from right-corner-end to left-corner-start.
	nBot := 16
	for i := 1; i <= nBot; i++ {
		t := float64(i) / float64(nBot)
		x := (rightX - cornerR) + t*((leftX+cornerR)-(rightX-cornerR))
		out = append(out, [2]float64{x, bottomY})
	}

	// Left-bottom rounded corner.
	cxL := leftX + cornerR
	cyL := bottomY - cornerR
	for i := 1; i <= nCorner; i++ {
		t := float64(i) / float64(nCorner)
		angle := -math.Pi/2 - t*math.Pi/2
		x := cxL + cornerR*math.Cos(angle)
		y := cyL - cornerR*math.Sin(angle)
		out = append(out, [2]float64{x, y})
	}

	// Left vertical edge.
	for i := 1; i <= nSide; i++ {
		t := float64(i) / float64(nSide)
		y := (bottomY - cornerR) + t*(horizonY-(bottomY-cornerR))
		out = append(out, [2]float64{leftX, y})
	}

	return out
}

// RenderIsoDiamondFlat draws a flat 2D rotated-square diamond with a soft
// drop shadow. We split the diamond into two halves (lit/shaded) so it still
// reads as a faceted gem rather than a 2D sticker.
