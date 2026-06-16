package iso25d

import (
	"math"
	"strings"
	"testing"
)

func TestRevolveProviderInvariants(t *testing.T) {
	var prov RevolveShapeProvider
	w, d, h := 160.0, 160.0, 100.0

	for _, name := range []string{"dome", "torus", "capsule"} {
		params := map[string]any{"name": name}

		faces := prov.Faces(w, d, h, params)
		if len(faces) == 0 {
			t.Errorf("%s: no faces returned", name)
		}

		// No NaN.
		for _, f := range faces {
			for _, p := range f.Points {
				if math.IsNaN(p[0]) || math.IsNaN(p[1]) {
					t.Fatalf("%s face %s: NaN coordinate", name, f.Name)
				}
			}
		}

		// Silhouette non-degenerate.
		sil := prov.Silhouette(w, d, h, params)
		if len(sil) < 4 {
			t.Errorf("%s: degenerate silhouette (len=%d)", name, len(sil))
		}

		// ZOrder monotone.
		for i := 1; i < len(faces); i++ {
			if faces[i].ZOrder < faces[i-1].ZOrder {
				t.Errorf("%s: ZOrder not monotone at index %d", name, i)
			}
		}
	}
}

func TestRevolveRenderSmoke(t *testing.T) {
	for _, name := range []string{"dome", "torus", "capsule"} {
		o := ConvertOpts{
			Width: 160, Depth: 160, Height: 100,
			TopFill: "#7FB3FF", LeftFill: "#3A6FBA", RightFill: "#5589D6",
			Stroke: "#1D3A66", StrokeWidth: 1.5,
			Label: name, Margin: 24,
		}
		svg := Convert2DTo25D(name, o)
		if !strings.HasPrefix(svg, "<svg") {
			t.Errorf("%s: output not SVG", name)
		}
		if strings.Count(svg, "</svg>") != 1 {
			t.Errorf("%s: malformed SVG", name)
		}
		if !strings.Contains(svg, "<polygon") && !strings.Contains(svg, "<path") {
			t.Errorf("%s: no geometry in output", name)
		}
	}
}

func TestRevolveWithEffects(t *testing.T) {
	o := ConvertOpts{
		Width: 160, Depth: 160, Height: 100,
		TopFill: "#6EE7B7", LeftFill: "#059669", RightFill: "#047857",
		Stroke: "#064E3B", StrokeWidth: 1.5,
		Label: "dome", Margin: 24,
		BackglowColor: "#6EE7B7", BackglowRadius: 28, BackglowOpacity: 0.5,
		OutlineColor: "#A7F3D0", OutlineWidth: 2,
	}
	svg := Convert2DTo25D("dome", o)
	if !strings.Contains(svg, "feGaussianBlur") {
		t.Errorf("dome with backglow: missing feGaussianBlur")
	}
	if strings.Count(svg, "</svg>") != 1 {
		t.Errorf("dome with effects: malformed SVG")
	}
}
