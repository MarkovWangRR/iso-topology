// M0 acceptance infrastructure (flexible-geometry plan): render-output
// invariants that hold across the geometry-system migration because
// they assert on the PRODUCT (SVG), not the implementation.
//
//  1. Determinism — rendering the same document twice yields identical
//     bytes. Kills map-iteration-order and randomness regressions
//     before they turn golden tests into flakes.
//  2. SVG integrity — every emitted coordinate is finite, every
//     polygon point lies inside the declared viewBox (small slack for
//     stroke overhang), and every url(#id) reference resolves to a
//     defined id.
package isotopo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func forEachSample(t *testing.T, fn func(t *testing.T, name, svg string)) {
	t.Helper()
	for _, category := range []string{"node", "topology"} {
		root := filepath.Join("samples", category)
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := category + "/" + e.Name()
			t.Run(name, func(t *testing.T) {
				dir := filepath.Join(root, e.Name())
				data, ext, err := readInput(dir)
				if err != nil {
					t.Fatal(err)
				}
				doc, err := loadGoldenDoc(ext, data)
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				fn(t, name, renderTopology(doc))
			})
		}
	}
}

// TestRenderDeterministic renders every sample twice from a fresh parse
// and requires byte-identical output. Catches unordered map walks and
// accidental randomness — the #1 way a SurfaceMap-style refactor turns
// the golden suite into a coin flip.
func TestRenderDeterministic(t *testing.T) {
	for _, category := range []string{"node", "topology"} {
		root := filepath.Join("samples", category)
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := category + "/" + e.Name()
			t.Run(name, func(t *testing.T) {
				dir := filepath.Join(root, e.Name())
				data, ext, err := readInput(dir)
				if err != nil {
					t.Fatal(err)
				}
				render := func() string {
					doc, err := loadGoldenDoc(ext, data)
					if err != nil {
						t.Fatalf("load: %v", err)
					}
					return renderTopology(doc)
				}
				a, b := render(), render()
				if a != b {
					t.Errorf("non-deterministic output: two renders differ (%d vs %d bytes)", len(a), len(b))
				}
			})
		}
	}
}

var (
	viewBoxRe = regexp.MustCompile(`viewBox="([-\d.]+) ([-\d.]+) ([\d.]+) ([\d.]+)"`)
	pointsRe  = regexp.MustCompile(`points="([^"]+)"`)
	numRe     = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?|NaN|Inf|-Inf`)
	urlRefRe  = regexp.MustCompile(`url\(#([^)]+)\)`)
	hrefRefRe = regexp.MustCompile(`href="#([^"]+)"`)
	idDefRe   = regexp.MustCompile(`\bid="([^"]+)"`)
)

// TestSVGIntegrity asserts structural sanity of every sample render.
func TestSVGIntegrity(t *testing.T) {
	forEachSample(t, func(t *testing.T, name, svg string) {
		if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
			t.Fatalf("non-finite coordinate emitted")
		}

		m := viewBoxRe.FindStringSubmatch(svg)
		if m == nil {
			t.Fatalf("no viewBox on root svg")
		}
		vx, _ := strconv.ParseFloat(m[1], 64)
		vy, _ := strconv.ParseFloat(m[2], 64)
		vw, _ := strconv.ParseFloat(m[3], 64)
		vh, _ := strconv.ParseFloat(m[4], 64)

		// Polygon points must land inside the viewBox. Slack covers
		// stroke width and the arrowhead overhang at clipped endpoints.
		// Points inside <pattern>/<defs> live in pattern-local space, so
		// only the document AFTER </defs>… is checked; nested transforms
		// inside part groups keep polygon coords in composite space.
		const slack = 6.0
		body := svg
		// strip every defs block — gradients/patterns use local coords
		for {
			i := strings.Index(body, "<defs>")
			if i < 0 {
				break
			}
			j := strings.Index(body[i:], "</defs>")
			if j < 0 {
				break
			}
			body = body[:i] + body[i+j+len("</defs>"):]
		}
		// strip transformed content groups whose local frame is not the
		// composite frame (iso-text matrices, top-content transforms)
		body = regexp.MustCompile(`transform="matrix[^"]*"[^>]*>`).ReplaceAllString(body, ">")
		for _, pm := range pointsRe.FindAllStringSubmatch(body, -1) {
			for _, nm := range numRe.FindAllString(pm[1], -1) {
				v, err := strconv.ParseFloat(nm, 64)
				if err != nil {
					t.Fatalf("unparseable coordinate %q", nm)
				}
				_ = v
			}
			fields := strings.FieldsFunc(pm[1], func(r rune) bool { return r == ' ' || r == ',' })
			for i := 0; i+1 < len(fields); i += 2 {
				x, err1 := strconv.ParseFloat(fields[i], 64)
				y, err2 := strconv.ParseFloat(fields[i+1], 64)
				if err1 != nil || err2 != nil {
					continue
				}
				if x < vx-slack || x > vx+vw+slack || y < vy-slack || y > vy+vh+slack {
					t.Errorf("polygon point (%.1f, %.1f) outside viewBox (%.1f %.1f %.1f %.1f)",
						x, y, vx, vy, vw, vh)
					return
				}
			}
		}

		// Every url(#id) / href="#id" reference must resolve.
		defined := map[string]bool{}
		for _, dm := range idDefRe.FindAllStringSubmatch(svg, -1) {
			defined[dm[1]] = true
		}
		check := func(kind, id string) {
			if !defined[id] {
				t.Errorf("%s references undefined id %q", kind, id)
			}
		}
		for _, rm := range urlRefRe.FindAllStringSubmatch(svg, -1) {
			check("url()", rm[1])
		}
		for _, rm := range hrefRefRe.FindAllStringSubmatch(svg, -1) {
			check("href", rm[1])
		}
	})
}

// TestOuterDimsIntegral guards the v3.0 contract: root width/height are
// integers so 1:1 raster capture never grows scrollbars.
func TestOuterDimsIntegral(t *testing.T) {
	re := regexp.MustCompile(`<svg[^>]*\bwidth="(\d+)" height="(\d+)"`)
	forEachSample(t, func(t *testing.T, name, svg string) {
		if re.FindStringSubmatch(svg) == nil {
			head := svg
			if len(head) > 160 {
				head = head[:160]
			}
			t.Errorf("root svg width/height not integral: %s", head)
		}
	})
}

// silence unused-import lint if fmt ends up unused in future edits
var _ = fmt.Sprintf
