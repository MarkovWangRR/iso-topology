package iso25d

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestArrayRenderSmoke(t *testing.T) {
	for _, name := range []string{"array1d", "array2d", "array3d"} {
		svg := Convert2DTo25D(name, ConvertOpts{
			Width: 200, Depth: 200, Height: 100,
			TopFill: "#DBEAFE", LeftFill: "#1E40AF", RightFill: "#2563EB",
			Stroke: "#1E3A8A", StrokeWidth: 0.8,
		})
		if !strings.HasPrefix(svg, "<svg") {
			t.Errorf("%s: expected SVG output", name)
		}
		if !strings.Contains(svg, "polygon") {
			t.Errorf("%s: expected polygons in SVG", name)
		}
	}
}

// TestArrayPaintOrderBackToFront locks the iso painter order for a cube grid:
// a cell occludes another only when its i+j+k is larger, so cells must be drawn
// with non-decreasing i+j+k. The old k-outermost / j-descending loop violated
// this — a back-top cell could paint over a front-bottom cell, making the cube
// look like its cells poke through each other.
func TestArrayPaintOrderBackToFront(t *testing.T) {
	svg := Convert2DTo25D("array3d", ConvertOpts{
		Width: 180, Depth: 180, Height: 180,
		TopFill: "#E0A85A", LeftFill: "#7A4E20", RightFill: "#A06A30",
		Params: map[string]any{"countX": 3, "countY": 3, "countZ": 3},
	})
	re := regexp.MustCompile(`cell-(\d+)-(\d+)-(\d+)-`)
	matches := re.FindAllStringSubmatch(svg, -1)
	if len(matches) == 0 {
		t.Fatal("no cell faces found in array3d output")
	}
	prev := -1
	for _, m := range matches {
		i, _ := strconv.Atoi(m[1])
		j, _ := strconv.Atoi(m[2])
		k, _ := strconv.Atoi(m[3])
		if s := i + j + k; s < prev {
			t.Fatalf("paint order went front-to-back: i+j+k dropped %d→%d (a back cell paints over a front cell)", prev, s)
		} else {
			prev = s
		}
	}
}

func TestArrayCellCount(t *testing.T) {
	svg := Convert2DTo25D("array2d", ConvertOpts{
		Width: 200, Depth: 200, Height: 60,
		TopFill: "#DBEAFE", LeftFill: "#1E40AF", RightFill: "#2563EB",
		Params: map[string]any{"countX": 3, "countY": 3},
	})
	count := strings.Count(svg, "<polygon")
	if count < 27 {
		t.Errorf("array2d 3x3: expected >=27 polygons, got %d", count)
	}
}
