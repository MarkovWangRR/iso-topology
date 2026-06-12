package iso25d

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestBoxProviderParity proves the new geometry layer describes the
// SAME polygons the legacy RenderIsoBox emits — point-for-point, for a
// spread of dimensions. This is the M1 gate: providers may not drift
// from the pixels users already have.
func TestBoxProviderParity(t *testing.T) {
	var prov BoxShapeProvider
	for _, dims := range [][3]float64{
		{140, 140, 80}, {200, 120, 40}, {64, 64, 200}, {118, 118, 12},
	} {
		w, d, h := dims[0], dims[1], dims[2]
		o := DefaultIsoBox()
		o.Width, o.Depth, o.Height = w, d, h
		o.Label, o.Icon = "", ""
		svg := RenderIsoBox(o)
		m := o.Margin

		got := facePolysFromSVG(t, svg)
		for _, f := range prov.Faces(w, d, h, nil) {
			want := make([][2]float64, len(f.Points))
			for i, p := range f.Points {
				want[i] = [2]float64{p[0] + m, p[1] + m}
			}
			emitted, ok := got[f.Name]
			if !ok {
				t.Fatalf("dims %v: legacy output has no data-face=%q polygon", dims, f.Name)
			}
			if !samePointSeq(want, emitted, 0.011) {
				t.Errorf("dims %v face %s: provider %v vs legacy %v", dims, f.Name, want, emitted)
			}
		}
	}
}

// TestBoxFaceInvariants runs the geomtest battery over the provider.
func TestBoxFaceInvariants(t *testing.T) {
	var prov BoxShapeProvider
	w, d, h := 160.0, 120.0, 64.0
	faces := prov.Faces(w, d, h, nil)
	if len(faces) != 3 {
		t.Fatalf("want 3 faces, got %d", len(faces))
	}
	seenZ := -1
	for _, f := range faces {
		if len(f.Points) < 3 {
			t.Fatalf("face %s: degenerate polygon", f.Name)
		}
		for _, p := range f.Points {
			if math.IsNaN(p[0]) || math.IsNaN(p[1]) || math.IsInf(p[0], 0) || math.IsInf(p[1], 0) {
				t.Fatalf("face %s: non-finite point", f.Name)
			}
		}
		if f.ZOrder < seenZ {
			t.Errorf("faces not in ascending ZOrder")
		}
		seenZ = f.ZOrder
		// side walls: every edge on an iso axis or vertical
		if f.Name == "left" || f.Name == "right" {
			for i := range f.Points {
				a, b := f.Points[i], f.Points[(i+1)%len(f.Points)]
				if !isoSlopeOK(b[0]-a[0], b[1]-a[1]) {
					t.Errorf("face %s edge %d: off-axis slope (%v→%v)", f.Name, i, a, b)
				}
			}
		}
		// visibility ⇔ outward normal faces the camera (1,1,~) for sides,
		// or points up for tops
		facing := f.Normal[0]+f.Normal[1] > 0 || f.Normal[2] > 0
		if f.Visible != facing {
			t.Errorf("face %s: Visible=%v but normal %v", f.Name, f.Visible, f.Normal)
		}
	}

	// silhouette must contain every face point
	sil := prov.Silhouette(w, d, h, nil)
	if len(sil) != 6 {
		t.Fatalf("box silhouette should have 6 corners, got %d", len(sil))
	}
	for _, f := range faces {
		for _, p := range f.Points {
			if !pointInConvexPoly(p, sil, 0.01) {
				t.Errorf("face %s point %v outside silhouette", f.Name, p)
			}
		}
	}

	// content rect within the top face's local bounds
	cr := prov.ContentRectFor(w, d, h, nil)
	if cr.X < 0 || cr.Y < 0 || cr.X+cr.W > w+0.01 || cr.Y+cr.H > d+0.01 {
		t.Errorf("content rect %+v exceeds top face %gx%g", cr, w, d)
	}

	if fw, fd := prov.Footprint(w, d, h); fw != w || fd != d {
		t.Errorf("box footprint should be identity, got %g %g", fw, fd)
	}
}

func TestRegistryLookup(t *testing.T) {
	for _, name := range []string{"rectangle", "box", "square"} {
		if LookupShape(name) == nil {
			t.Errorf("%s not registered", name)
		}
	}
	names := RegisteredShapeNames()
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("names not sorted: %v", names)
		}
	}
}

// --- helpers ---

var faceTagRe = regexp.MustCompile(`<polygon data-face="(top|left|right)" points="([^"]+)"`)

func facePolysFromSVG(t *testing.T, svg string) map[string][][2]float64 {
	t.Helper()
	out := map[string][][2]float64{}
	for _, m := range faceTagRe.FindAllStringSubmatch(svg, -1) {
		var pts [][2]float64
		fields := strings.FieldsFunc(m[2], func(r rune) bool { return r == ' ' || r == ',' })
		for i := 0; i+1 < len(fields); i += 2 {
			x, err1 := strconv.ParseFloat(fields[i], 64)
			y, err2 := strconv.ParseFloat(fields[i+1], 64)
			if err1 != nil || err2 != nil {
				t.Fatalf("bad points attr %q", m[2])
			}
			pts = append(pts, [2]float64{x, y})
		}
		out[m[1]] = pts
	}
	return out
}

// samePointSeq requires the SAME order, not just the same set — the
// live renderer consumes provider points verbatim, so sequence is part
// of the byte-parity contract.
func samePointSeq(a, b [][2]float64, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i][0]-b[i][0]) > tol || math.Abs(a[i][1]-b[i][1]) > tol {
			return false
		}
	}
	return true
}

func pointInConvexPoly(pt [2]float64, poly [][2]float64, tol float64) bool {
	n := len(poly)
	sign := 0.0
	for i := 0; i < n; i++ {
		a, b := poly[i], poly[(i+1)%n]
		cross := (b[0]-a[0])*(pt[1]-a[1]) - (b[1]-a[1])*(pt[0]-a[0])
		if math.Abs(cross) <= tol {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if sign*cross < 0 {
			return false
		}
	}
	return true
}
