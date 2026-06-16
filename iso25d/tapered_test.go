package iso25d

import (
	"math"
	"strings"
	"testing"
)

// TestTaperedFaceInvariants checks geometric correctness of TaperedPrismShapeProvider.
func TestTaperedFaceInvariants(t *testing.T) {
	var prov TaperedPrismShapeProvider
	w, d, h := 140.0, 140.0, 100.0

	cases := []struct {
		name     string
		sides    int
		topScale float64
		wantTop  bool // top face expected?
	}{
		{"cone", 32, 0.0, false},
		{"pyramid", 4, 0.0, false},
		{"frustum-half", 32, 0.5, true},
		{"frustum-small", 6, 0.25, true},
	}

	for _, tc := range cases {
		params := map[string]any{"sides": tc.sides, "topScale": tc.topScale}
		faces := prov.Faces(w, d, h, params)

		// Basic count: sides (+ top face when topScale > 0)
		wantFaces := tc.sides
		if tc.wantTop {
			wantFaces++
		}
		if len(faces) != wantFaces {
			t.Errorf("%s: want %d faces, got %d", tc.name, wantFaces, len(faces))
		}

		// No NaN coordinates.
		for _, f := range faces {
			for _, p := range f.Points {
				if math.IsNaN(p[0]) || math.IsNaN(p[1]) {
					t.Fatalf("%s face %s: NaN coordinate", tc.name, f.Name)
				}
			}
		}

		// Silhouette is non-degenerate.
		sil := prov.Silhouette(w, d, h, params)
		if len(sil) < 3 {
			t.Errorf("%s: degenerate silhouette", tc.name)
		}

		// Visible side faces must have camera-facing normals.
		for _, f := range faces {
			if f.Name == "top" {
				if !f.Visible {
					t.Errorf("%s: top face must be visible", tc.name)
				}
				if f.Normal[2] != 1 {
					t.Errorf("%s: top face normal must be (0,0,1)", tc.name)
				}
				continue
			}
			if f.Visible && f.Normal[0]+f.Normal[1] <= 0 {
				t.Errorf("%s face %s: visible but normal points away from camera", tc.name, f.Name)
			}
		}

		// Top face exists iff topScale > 0.
		hasTop := false
		for _, f := range faces {
			if f.Name == "top" {
				hasTop = true
			}
		}
		if hasTop != tc.wantTop {
			t.Errorf("%s: wantTop=%v but hasTop=%v", tc.name, tc.wantTop, hasTop)
		}

		// Faces must be sorted by ZOrder (painter order).
		for i := 1; i < len(faces); i++ {
			if faces[i].ZOrder < faces[i-1].ZOrder {
				t.Errorf("%s: ZOrder not monotone at index %d", tc.name, i)
			}
		}
	}
}

// TestTaperedRenderSmoke renders each alias via Convert2DTo25D and checks
// the output is well-formed SVG containing at least one polygon.
func TestTaperedRenderSmoke(t *testing.T) {
	shapes := []struct {
		name     string
		topScale float64
		wantTop  bool
	}{
		{"cone", 0, false},
		{"pyramid", 0, false},
		{"frustum", 0, true},
	}

	for _, tc := range shapes {
		o := ConvertOpts{
			Width: 140, Depth: 140, Height: 80,
			TopFill: "#7FB3FF", LeftFill: "#3A6FBA", RightFill: "#5589D6",
			Stroke: "#1D3A66", StrokeWidth: 1.5,
			Label: tc.name, Margin: 24,
			TopScale: tc.topScale,
		}
		svg := Convert2DTo25D(tc.name, o)

		if !strings.HasPrefix(svg, "<svg") {
			t.Errorf("%s: output is not SVG", tc.name)
		}
		if strings.Count(svg, "</svg>") != 1 {
			t.Errorf("%s: malformed SVG", tc.name)
		}
		if !strings.Contains(svg, "<polygon") && !strings.Contains(svg, "<path") {
			t.Errorf("%s: no polygon in output", tc.name)
		}
		if tc.wantTop && !strings.Contains(svg, `data-face="top"`) {
			t.Errorf("%s: missing top face", tc.name)
		}
	}
}

// TestTaperedRenderWithEffects exercises the full effect stack on frustum.
func TestTaperedRenderWithEffects(t *testing.T) {
	o := ConvertOpts{
		Width: 160, Depth: 160, Height: 100,
		TopFill: "#A78BFA", LeftFill: "#7C3AED", RightFill: "#6D28D9",
		Stroke: "#4C1D95", StrokeWidth: 1.5,
		Label: "storage", Margin: 24,
		TopScale: 0.5,
		BackglowColor: "#A78BFA", BackglowRadius: 32, BackglowOpacity: 0.5,
		GrainIntensity: 0.25, GrainScale: 1.0,
		OutlineColor: "#C4B5FD", OutlineWidth: 2, OutlineOpacity: 0.8,
	}
	svg := Convert2DTo25D("frustum", o)
	if !strings.Contains(svg, "feGaussianBlur") {
		t.Errorf("frustum with backglow: missing feGaussianBlur in defs")
	}
	if strings.Count(svg, "</svg>") != 1 {
		t.Errorf("frustum with effects: malformed SVG")
	}
}

// TestTaperedCustomSides verifies that geom.sides and geom.topScale are
// respected when the user passes them via ConvertOpts.
func TestTaperedCustomSides(t *testing.T) {
	// 6-sided pyramid (hexagonal pyramid).
	o := ConvertOpts{
		Width: 140, Depth: 140, Height: 80,
		TopFill: "#F59E0B", LeftFill: "#D97706", RightFill: "#B45309",
		Stroke: "#92400E", StrokeWidth: 1.5,
		Label: "hex-pyramid", Margin: 24,
		Sides: 6, TopScale: 0, // explicit 6-sided apex
	}
	svg := Convert2DTo25D("cone", o)
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("hex-pyramid: not SVG")
	}

	// Partial frustum with 8 sides.
	o2 := ConvertOpts{
		Width: 140, Depth: 140, Height: 80,
		TopFill: "#10B981", LeftFill: "#059669", RightFill: "#047857",
		Stroke: "#064E3B", StrokeWidth: 1.5,
		Label: "oct-frustum", Margin: 24,
		Sides: 8, TopScale: 0.3,
	}
	svg2 := Convert2DTo25D("frustum", o2)
	if !strings.Contains(svg2, `data-face="top"`) {
		t.Errorf("oct-frustum: missing top face")
	}
}
