package isotopo

import "math"

// graphIsCyclic reports whether the connector graph over the top-level parts
// contains a directed cycle. Cyclic / dense graphs (hub-and-spoke with
// back-edges, meshes) are exactly where longest-path layering degrades — it
// crams nodes into columns so edges tunnel through them — so they are routed to
// the force-directed placer instead. The result is order-independent, hence
// deterministic.
func graphIsCyclic(parts []*CompositePart, conns []*Connector) bool {
	idx := map[string]bool{}
	for _, p := range parts {
		if p != nil && p.ID != "" {
			idx[p.ID] = true
		}
	}
	adj := map[string][]string{}
	for _, c := range conns {
		if c == nil {
			continue
		}
		u, v := connectorTarget(c.From), connectorTarget(c.To)
		if !idx[u] || !idx[v] || u == v {
			continue
		}
		adj[u] = append(adj[u], v)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var dfs func(string) bool
	dfs = func(u string) bool {
		color[u] = gray
		for _, v := range adj[u] {
			if color[v] == gray {
				return true
			}
			if color[v] == white && dfs(v) {
				return true
			}
		}
		color[u] = black
		return false
	}
	for id := range idx {
		if color[id] == white && dfs(id) {
			return true
		}
	}
	return false
}

// feedbackEdges returns the indices of connectors that form a feedback arc
// set — the minority of edges that run against the graph's dominant flow. A
// graph whose feedback arcs are few is a flow with feedback (retry loops,
// cache writebacks), not a mesh: reversing just those edges for RANKING lets
// the layered Sugiyama pass keep the left-to-right narrative instead of
// forfeiting the whole graph to force layout, which is what a wholesale
// cyclic check did.
//
// The set is computed with the Eades–Lin–Smyth GreedyFAS vertex ordering (the
// standard cycle-breaking pre-pass for layered layout): repeatedly peel sinks
// to the right and sources to the left, otherwise move the vertex with the
// largest outdegree−indegree left; edges pointing right-to-left in the final
// order are the feedback arcs. Unlike raw DFS back edges, this blames the
// edges that genuinely oppose the flow — DFS blames whichever edge its
// traversal order happens to close a cycle on (reaching a node THROUGH the
// feedback arc makes DFS blame a flow edge instead). Ties break by declared
// part order, so the result is deterministic.
func feedbackEdges(parts []*CompositePart, conns []*Connector) []int {
	idx := map[string]bool{}
	order := []string{}
	for _, p := range parts {
		if p != nil && p.ID != "" && !idx[p.ID] {
			idx[p.ID] = true
			order = append(order, p.ID)
		}
	}
	type edge struct {
		u, v string
		ci   int
	}
	var edges []edge
	for ci, c := range conns {
		if c == nil {
			continue
		}
		u, v := connectorTarget(c.From), connectorTarget(c.To)
		if !idx[u] || !idx[v] || u == v {
			continue
		}
		edges = append(edges, edge{u, v, ci})
	}
	if len(edges) == 0 {
		return nil
	}

	// Live degree bookkeeping over the shrinking vertex set.
	outdeg := map[string]int{}
	indeg := map[string]int{}
	for _, e := range edges {
		outdeg[e.u]++
		indeg[e.v]++
	}
	alive := map[string]bool{}
	for _, id := range order {
		alive[id] = true
	}
	remove := func(id string) {
		alive[id] = false
		for _, e := range edges {
			if e.u == id && alive[e.v] {
				indeg[e.v]--
			}
			if e.v == id && alive[e.u] {
				outdeg[e.u]--
			}
		}
	}

	var left, right []string // right is built back-to-front
	remaining := len(order)
	for remaining > 0 {
		progress := true
		for progress {
			progress = false
			for _, id := range order { // declared order ⇒ deterministic
				if alive[id] && outdeg[id] == 0 { // sink → right
					right = append(right, id)
					remove(id)
					remaining--
					progress = true
				}
			}
			for _, id := range order {
				if alive[id] && indeg[id] == 0 && outdeg[id] > 0 { // source → left
					left = append(left, id)
					remove(id)
					remaining--
					progress = true
				}
			}
		}
		if remaining == 0 {
			break
		}
		best, bestDelta := "", 0
		for _, id := range order { // first max wins ⇒ deterministic
			if !alive[id] {
				continue
			}
			if d := outdeg[id] - indeg[id]; best == "" || d > bestDelta {
				best, bestDelta = id, d
			}
		}
		left = append(left, best)
		remove(best)
		remaining--
	}

	pos := map[string]int{}
	for i, id := range left {
		pos[id] = i
	}
	for i := range right { // reverse the sink pile onto the tail
		pos[right[len(right)-1-i]] = len(left) + i
	}

	var back []int
	for _, e := range edges {
		if pos[e.u] > pos[e.v] { // points right-to-left → feedback arc
			back = append(back, e.ci)
		}
	}
	return back
}

// reversedForRanking returns a connector list with the given edges' endpoints
// swapped — used ONLY to feed the ranking pass; the real connectors still
// render in their authored direction (drawn right-to-left as feedback).
func reversedForRanking(conns []*Connector, back []int) []*Connector {
	rev := map[int]bool{}
	for _, ci := range back {
		rev[ci] = true
	}
	out := make([]*Connector, len(conns))
	for i, c := range conns {
		if c == nil || !rev[i] {
			out[i] = c
			continue
		}
		flipped := *c
		flipped.From, flipped.To = c.To, c.From
		out[i] = &flipped
	}
	return out
}

// snapshotOffsets / restoreOffsets bracket a trial layout so the dispatcher
// can attempt the layered arrangement and roll it back if it tunnels.
func snapshotOffsets(parts []*CompositePart) []*WorldPoint {
	out := make([]*WorldPoint, len(parts))
	for i, p := range parts {
		if p != nil && p.Offset != nil {
			cp := *p.Offset
			out[i] = &cp
		}
	}
	return out
}

func restoreOffsets(parts []*CompositePart, saved []*WorldPoint) {
	for i, p := range parts {
		if p == nil || i >= len(saved) {
			continue
		}
		if saved[i] == nil {
			p.Offset = nil
			continue
		}
		cp := *saved[i]
		p.Offset = &cp
	}
}

// hasReciprocalBackEdge reports whether any DFS back edge is the reverse of a
// forward edge (a 2-cycle, A<->B). Reciprocal pairs are bidirectional traffic
// (request/response, hub-and-spoke), not pipeline feedback — a graph carrying
// them reads best as the force placer's radial spread, so the layered trial is
// skipped entirely and the bench hub keeps its compact ring.
func hasReciprocalBackEdge(conns []*Connector, back []int) bool {
	bk := map[int]bool{}
	for _, ci := range back {
		bk[ci] = true
	}
	fwd := map[[2]string]bool{}
	for i, c := range conns {
		if c == nil || bk[i] {
			continue
		}
		fwd[[2]string{connectorTarget(c.From), connectorTarget(c.To)}] = true
	}
	for _, ci := range back {
		c := conns[ci]
		if c == nil {
			continue
		}
		if fwd[[2]string{connectorTarget(c.To), connectorTarget(c.From)}] {
			return true
		}
	}
	return false
}

// trialTunnels judges a trial layered arrangement at the CURRENT solved
// offsets. Flow edges are judged by straight center-line line-of-sight (after
// median alignment they are straight in the render too); back edges — which
// the router draws as an L — clear if the straight line OR either elbow
// candidate does, mirroring the router's actual two-candidate capability. Any
// edge with no clear route means the graph is too dense for ranks and the
// dispatcher rolls back to the force placer. (A single-file pipeline's
// feedback edge has no clear elbow either — those scenes honestly render
// better as a force ring until the router learns detours.)
func trialTunnels(parts []*CompositePart, conns []*Connector, back []int) int {
	type box struct {
		id     string
		r      planRect
		cx, cy float64
		ok     bool
	}
	byID := map[string]box{}
	var boxes []box
	for _, p := range parts {
		if p == nil || p.ID == "" {
			continue
		}
		w, d := partFootprint(p)
		x, y := 0.0, 0.0
		if p.Offset != nil {
			x, y = p.Offset.WX, p.Offset.WY
		}
		b := box{id: p.ID, r: planRect{x: x, y: y, w: w, d: d}, cx: x + w/2, cy: y + d/2, ok: true}
		byID[p.ID] = b
		boxes = append(boxes, b)
	}
	hits := func(route [][2]float64, aID, bID string) bool {
		for _, o := range boxes {
			if o.id == aID || o.id == bID {
				continue
			}
			if routeHitsRect(route, o.r) {
				return true
			}
		}
		return false
	}
	bk := map[int]bool{}
	for _, ci := range back {
		bk[ci] = true
	}
	n := 0
	for i, c := range conns {
		if c == nil {
			continue
		}
		a, b := byID[connectorTarget(c.From)], byID[connectorTarget(c.To)]
		if !a.ok || !b.ok || a.id == b.id {
			continue
		}
		straight := [][2]float64{{a.cx, a.cy}, {b.cx, b.cy}}
		clear := !hits(straight, a.id, b.id)
		if !clear && bk[i] {
			// L-shaped candidates, corner at (ax,by) then (bx,ay).
			for _, corner := range [][2]float64{{a.cx, b.cy}, {b.cx, a.cy}} {
				if !hits([][2]float64{{a.cx, a.cy}, corner, {b.cx, b.cy}}, a.id, b.id) {
					clear = true
					break
				}
			}
		}
		if !clear {
			n++
		}
	}
	return n
}

// arrangeForce positions top-level parts with a deterministic Fruchterman-
// Reingold force-directed layout: connected nodes attract, all nodes repel, so
// the graph spreads to a near-uniform-distance arrangement with room for edges
// to run between adjacent nodes instead of tunnelling a third. Used for cyclic /
// mesh graphs where longest-path ranking degrades. Init is a fixed circle and
// iterations are fixed, so output is deterministic; final positions snap to the
// cell grid to stay in register with the iso lattice.
func arrangeForce(parts []*CompositePart, conns []*Connector, lay *Layout, cell float64) {
	idx := map[string]int{}
	order := []string{}
	for _, p := range parts {
		if p == nil || p.ID == "" {
			continue
		}
		if _, dup := idx[p.ID]; !dup {
			idx[p.ID] = len(order)
			order = append(order, p.ID)
		}
	}
	n := len(order)
	if n < 2 {
		return
	}
	fw := make([]float64, n)
	fd := make([]float64, n)
	avg := 0.0
	for i, id := range order {
		w, d := partFootprint(parts[idx[id]])
		fw[i], fd[i] = w, d
		avg += (w + d) / 2
	}
	avg /= float64(n)

	type edge struct{ a, b int }
	var edges []edge
	seen := map[[2]int]bool{}
	for _, c := range conns {
		if c == nil {
			continue
		}
		ai, ok1 := idx[connectorTarget(c.From)]
		bi, ok2 := idx[connectorTarget(c.To)]
		if !ok1 || !ok2 || ai == bi {
			continue
		}
		key := [2]int{ai, bi}
		if ai > bi {
			key = [2]int{bi, ai}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, edge{ai, bi})
	}

	gap := 1.4
	if lay != nil && lay.Gap != nil && *lay.Gap >= 0 {
		gap = *lay.Gap
	}
	// runFD runs Fruchterman-Reingold at a given ideal edge length k. Fixed
	// circle init + fixed iterations ⇒ deterministic.
	runFD := func(k float64) (px, py []float64) {
		px = make([]float64, n)
		py = make([]float64, n)
		radius := k * float64(n) / (2 * math.Pi)
		for i := range order { // deterministic circle init
			a := 2 * math.Pi * float64(i) / float64(n)
			px[i], py[i] = radius*math.Cos(a), radius*math.Sin(a)
		}
		const iters = 240
		temp := radius / 2
		cool := temp / float64(iters+1)
		dx := make([]float64, n)
		dy := make([]float64, n)
		for it := 0; it < iters; it++ {
			for i := range dx {
				dx[i], dy[i] = 0, 0
			}
			for i := 0; i < n; i++ { // repulsion (all pairs)
				for j := i + 1; j < n; j++ {
					ex, ey := px[i]-px[j], py[i]-py[j]
					d := math.Hypot(ex, ey)
					if d < 0.01 {
						d, ex = 0.01, 0.01
					}
					f := k * k / d
					ux, uy := ex/d, ey/d
					dx[i] += ux * f
					dy[i] += uy * f
					dx[j] -= ux * f
					dy[j] -= uy * f
				}
			}
			for _, e := range edges { // attraction (edges)
				ex, ey := px[e.a]-px[e.b], py[e.a]-py[e.b]
				d := math.Hypot(ex, ey)
				if d < 0.01 {
					d = 0.01
				}
				f := d * d / k
				ux, uy := ex/d, ey/d
				dx[e.a] -= ux * f
				dy[e.a] -= uy * f
				dx[e.b] += ux * f
				dy[e.b] += uy * f
			}
			for i := 0; i < n; i++ { // apply, capped by cooling temperature
				d := math.Hypot(dx[i], dy[i])
				if d < 0.01 {
					continue
				}
				lim := math.Min(d, temp)
				px[i] += dx[i] / d * lim
				py[i] += dy[i] / d * lim
			}
			temp -= cool
		}
		return px, py
	}

	// hasTunnel reports whether any edge's straight center-line passes through a
	// non-endpoint node's footprint — the defect we spread to remove.
	hasTunnel := func(px, py []float64) bool {
		for _, e := range edges {
			seg := [][2]float64{{px[e.a], py[e.a]}, {px[e.b], py[e.b]}}
			for c := 0; c < n; c++ {
				if c == e.a || c == e.b {
					continue
				}
				r := planRect{x: px[c] - fw[c]/2, y: py[c] - fd[c]/2, w: fw[c], d: fd[c]}
				if routeHitsRect(seg, r) {
					return true
				}
			}
		}
		return false
	}

	// Adaptive spread: start tight — best for sparse graphs like hub-and-spoke,
	// which keep a compact ring — then escalate the ideal distance ONLY while
	// edges still tunnel a node, so a dense mesh spreads out just enough to clear
	// them without over-sprawling. Bounded ⇒ deterministic and terminating.
	base := avg + gap*cell
	px, py := runFD(base)
	for mult := 1.4; mult <= 3.0 && hasTunnel(px, py); mult += 0.4 {
		px, py = runFD(base * mult)
	}

	minx, miny := math.Inf(1), math.Inf(1)
	for i := range order {
		minx = math.Min(minx, px[i]-fw[i]/2)
		miny = math.Min(miny, py[i]-fd[i]/2)
	}
	snap := func(v float64) float64 { return math.Round(v/cell) * cell }
	for i, id := range order {
		p := parts[idx[id]]
		if p.Offset == nil {
			p.Offset = &WorldPoint{}
		}
		p.Offset.WX = snap(px[i] - fw[i]/2 - minx)
		p.Offset.WY = snap(py[i] - fd[i]/2 - miny)
	}
}
