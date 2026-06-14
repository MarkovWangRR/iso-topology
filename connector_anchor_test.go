package isotopo

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestConnectorDocksAcrossGroup guards the edge↔node detachment bug: when a
// connector runs from a node inside an AUTO-SIZED group to a standalone node,
// the connector layer must share the renderer's projection origin. A wide
// group substrate has no geom W/D, so a stale 140 default (or a zero-dim
// label part inflated to 140) used to skew partsScreenOrigin(), shifting the
// whole connector layer ~one cell off the parts. Here we render exactly that
// shape and assert the connector's endpoint lands INSIDE the target's
// silhouette.
func TestConnectorDocksAcrossGroup(t *testing.T) {
	// Mirror the rag-pipeline shape that exposed the bug: two auto-sized
	// group "planes" (each with a label → a zero-dim sub-part) and a
	// standalone target between them. The bug is in the SHARED projection
	// origin, so it's shape-agnostic — verify a spread of polygon-faced
	// shapes (the ones whose silhouette we can parse as points here) so a
	// regression is caught regardless of the target's geometry.
	for _, shape := range []string{"rectangle", "hexprism", "diamond", "octprism", "prism", "triprism"} {
		t.Run(shape, func(t *testing.T) { assertDocks(t, shape) })
	}
}

func assertDocks(t *testing.T, shape string) {
	src := `
nodes:
  scene:
    shape: composite
    layout: { mode: auto }
    parts:
      - id: back
        shape: group
        label: "Back Plane"
        layout: { mode: row, gap: 1.4 }
        parts:
          - { id: c1, shape: rectangle, geom: { w: 96, d: 96, h: 22 }, label: "C1" }
          - { id: c2, shape: rectangle, geom: { w: 96, d: 96, h: 22 }, label: "C2" }
          - { id: c3, shape: rectangle, geom: { w: 96, d: 96, h: 22 }, label: "C3" }
      - { id: tgt, shape: ` + shape + `, geom: { w: 96, d: 96, h: 30 }, label: "TGT" }
      - id: front
        shape: group
        label: "Front Plane"
        layout: { mode: row, gap: 1.4 }
        parts:
          - { id: d1, shape: rectangle, geom: { w: 96, d: 96, h: 22 }, label: "D1" }
          - { id: d2, shape: rectangle, geom: { w: 96, d: 96, h: 22 }, label: "D2" }
          - { id: d3, shape: rectangle, geom: { w: 96, d: 96, h: 22 }, label: "D3" }
    connectors:
      - { from: c1, to: tgt, arrow: triangle, routing: orthogonal }
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	svg := RenderDocument(doc)["scene"]

	// target node group + its translate
	g := regexp.MustCompile(`<g data-part="\d+" data-part-id="tgt" transform="translate\(([-\d.]+) ([-\d.]+)\)">`).FindStringSubmatch(svg)
	if g == nil {
		t.Fatal("tgt node group not found")
	}
	tx, ty := atof(g[1]), atof(g[2])
	// the tgt block (up to its closing) — gather its face polygon points
	gi := strings.Index(svg, g[0])
	block := svg[gi:]
	if e := strings.Index(block[1:], `<g data-part="`); e >= 0 {
		block = block[:e]
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, pm := range regexp.MustCompile(`points="([^"]+)"`).FindAllStringSubmatch(block, -1) {
		for _, pair := range strings.Fields(pm[1]) {
			xy := strings.Split(pair, ",")
			if len(xy) != 2 {
				continue
			}
			x, y := tx+atof(xy[0]), ty+atof(xy[1])
			minX, minY = math.Min(minX, x), math.Min(minY, y)
			maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
		}
	}
	if math.IsInf(minX, 1) {
		t.Fatal("no tgt silhouette points parsed")
	}

	// connector path's last point
	cp := regexp.MustCompile(`<path data-connector="0"[^>]* d="([^"]+)"`).FindStringSubmatch(svg)
	if cp == nil {
		t.Fatal("connector path not found")
	}
	nums := regexp.MustCompile(`[-\d.]+`).FindAllString(cp[1], -1)
	if len(nums) < 2 {
		t.Fatal("connector path has no points")
	}
	ex, ey := atof(nums[len(nums)-2]), atof(nums[len(nums)-1])

	// endpoint must sit within the target silhouette (allow a small margin
	// for stroke/arrow). The bug placed it ~one cell (≫margin) outside.
	const margin = 8.0
	if ex < minX-margin || ex > maxX+margin || ey < minY-margin || ey > maxY+margin {
		t.Errorf("connector endpoint (%.1f,%.1f) is outside the target silhouette [%.0f..%.0f]x[%.0f..%.0f] — edge detached from node",
			ex, ey, minX, maxX, minY, maxY)
	}
}

func atof(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
