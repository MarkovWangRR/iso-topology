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
	k := avg + gap*cell // ideal edge length (separates node bodies + gap)

	px := make([]float64, n)
	py := make([]float64, n)
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
