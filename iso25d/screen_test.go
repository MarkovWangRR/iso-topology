package iso25d

import (
	"strings"
	"testing"
)

func TestScreenRenderSmoke(t *testing.T) {
	for _, name := range []string{"screen", "browser-panel"} {
		svg := Convert2DTo25D(name, ConvertOpts{
			Width: 100, Depth: 14, Height: 160,
			TopFill: "#0F172A", LeftFill: "#020617", RightFill: "#1E3A8A",
			Stroke: "#334155", StrokeWidth: 1.5,
			Label: "App",
		})
		if !strings.HasPrefix(svg, "<svg") {
			t.Errorf("%s: expected SVG output", name)
		}
	}
}
