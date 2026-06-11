// Screen-space text placement — the layout contract for everything
// that lives on the canvas plane (screen labels, annotation boxes):
//
//  1. after projection, text must neither cross NOR touch any part's
//     silhouette (a clearance margin is enforced);
//  2. text prefers the position nearest the OUTSIDE of the picture —
//     candidates are tried periphery-first, so copy gravitates to the
//     scene's edge instead of threading between cubes.
//
// placeTextBox is a pure function with its own unit suite.
package isotopo

import "math"

// screenRect is an axis-aligned box in composite screen coords.
type screenRect struct{ x0, y0, x1, y1 float64 }

func (r screenRect) intersects(o screenRect) bool {
	return r.x0 < o.x1 && o.x0 < r.x1 && r.y0 < o.y1 && o.y0 < r.y1
}

func (r screenRect) inflate(m float64) screenRect {
	return screenRect{r.x0 - m, r.y0 - m, r.x1 + m, r.y1 + m}
}

func (r screenRect) cx() float64 { return (r.x0 + r.x1) / 2 }
func (r screenRect) cy() float64 { return (r.y0 + r.y1) / 2 }

// textClearance is the no-touch margin between text and any other
// element, in screen units.
const textClearance = 6.0

// partScreenRects projects every part's world bbox into composite
// screen coords (same tx/ty as every injector) and returns one
// axis-aligned obstacle rect per part.
func partScreenRects(infos []partInfo) []screenRect {
	tx, ty := partsScreenOrigin(infos)
	out := make([]screenRect, 0, len(infos))
	for _, p := range infos {
		r := screenRect{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
		corners := [8][3]float64{
			{p.offWX, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY, p.offWZ},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY + p.d, p.offWZ},
			{p.offWX, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY, p.offWZ + p.h},
			{p.offWX + p.w, p.offWY + p.d, p.offWZ + p.h},
			{p.offWX, p.offWY + p.d, p.offWZ + p.h},
		}
		for _, c := range corners {
			sx, sy := projectIso(c[0], c[1], c[2])
			r.x0 = math.Min(r.x0, sx+tx)
			r.y0 = math.Min(r.y0, sy+ty)
			r.x1 = math.Max(r.x1, sx+tx)
			r.y1 = math.Max(r.y1, sy+ty)
		}
		out = append(out, r)
	}
	return out
}

func sceneCenter(rects []screenRect) (float64, float64) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range rects {
		minX, minY = math.Min(minX, r.x0), math.Min(minY, r.y0)
		maxX, maxY = math.Max(maxX, r.x1), math.Max(maxY, r.y1)
	}
	return (minX + maxX) / 2, (minY + maxY) / 2
}

func collides(c screenRect, obstacles []screenRect) bool {
	for _, o := range obstacles {
		if c.intersects(o.inflate(textClearance)) {
			return true
		}
	}
	return false
}

// placeTextBox positions a boxW×boxH text box for an element whose
// silhouette is anchor. The preferred position (the legacy spot) wins
// when it is already collision-free — scenes that were clean render
// unchanged. Otherwise candidates around the anchor are tried in
// periphery-first order (directions pointing away from the scene
// centre first) at increasing clearances; the first collision-free
// candidate wins, the preferred spot is the final fallback.
func placeTextBox(boxW, boxH float64, anchor screenRect, prefX, prefY float64, sceneCx, sceneCy float64, obstacles []screenRect) (float64, float64) {
	pref := screenRect{prefX, prefY, prefX + boxW, prefY + boxH}
	if !collides(pref, obstacles) {
		return prefX, prefY
	}

	outX, outY := anchor.cx()-sceneCx, anchor.cy()-sceneCy
	if n := math.Hypot(outX, outY); n > 0.001 {
		outX, outY = outX/n, outY/n
	} else {
		outX, outY = 0, 1 // dead centre: fall outward = downward
	}

	type dir struct{ dx, dy float64 }
	dirs := []dir{{0, 1}, {1, 0}, {-1, 0}, {0, -1}} // below, right, left, above
	// Stable periphery-first sort: higher dot(dir, outward) first.
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			si := dirs[i].dx*outX + dirs[i].dy*outY
			sj := dirs[j].dx*outX + dirs[j].dy*outY
			if sj > si+1e-9 {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	for _, g := range []float64{10, 34, 62, 96, 136} {
		for _, d := range dirs {
			var x, y float64
			switch {
			case d.dy > 0: // below
				x, y = anchor.cx()-boxW/2, anchor.y1+g
			case d.dy < 0: // above
				x, y = anchor.cx()-boxW/2, anchor.y0-g-boxH
			case d.dx > 0: // right
				x, y = anchor.x1+g, anchor.cy()-boxH/2
			default: // left
				x, y = anchor.x0-g-boxW, anchor.cy()-boxH/2
			}
			c := screenRect{x, y, x + boxW, y + boxH}
			if !collides(c, obstacles) {
				return x, y
			}
		}
	}
	return prefX, prefY
}
