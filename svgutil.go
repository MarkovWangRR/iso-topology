// SVG string-manipulation utilities used by the injectors. These
// don't care about iso semantics — they're tiny parse/edit helpers
// that work on the SVG envelope as text.
package isotopo

import (
	"fmt"
	"strings"
)

func growViewBox(svg string, needW, needH float64) string {
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	end := strings.Index(svg[start:], ">")
	if end < 0 {
		return svg
	}
	end += start
	header := svg[start : end+1]
	rest := svg[end+1:]

	out := header
	// Update viewBox if present and too small.
	if vb := extractAttr(out, "viewBox"); vb != "" {
		var x, y, w, h float64
		if _, err := fmt.Sscanf(vb, "%f %f %f %f", &x, &y, &w, &h); err == nil {
			grew := false
			if needW > x+w {
				w = needW - x
				grew = true
			}
			if needH > y+h {
				h = needH - y
				grew = true
			}
			if grew {
				newVB := fmt.Sprintf("%.2f %.2f %.2f %.2f", x, y, w, h)
				out = replaceAttr(out, "viewBox", newVB)
				out = replaceAttr(out, "width", fmt.Sprintf("%.2f", w))
				out = replaceAttr(out, "height", fmt.Sprintf("%.2f", h))
			}
		}
	}
	return svg[:start] + out + rest
}

func extractAttr(tag, name string) string {
	key := name + `="`
	i := strings.Index(tag, key)
	if i < 0 {
		return ""
	}
	i += len(key)
	j := strings.Index(tag[i:], `"`)
	if j < 0 {
		return ""
	}
	return tag[i : i+j]
}

func replaceAttr(tag, name, val string) string {
	key := name + `="`
	i := strings.Index(tag, key)
	if i < 0 {
		return tag
	}
	i += len(key)
	j := strings.Index(tag[i:], `"`)
	if j < 0 {
		return tag
	}
	return tag[:i] + val + tag[i+j:]
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// injectCompositeConnectors computes the iso-projected screen coords of
// each part's named anchor, then splices <line> elements for each
// connector before the closing </svg>. Mirrors the (tx, ty) shift that

type minSvgRect struct {
	minX, minY, maxX, maxY float64
}

// growViewBoxAround expands the SVG's viewBox in all four directions to
// contain the requested rect (in SVG user-space coords). It also
// updates any data-layer="canvas-bg" rect so the backdrop continues to
// fill the full viewBox after expansion. Idempotent — no-op if the
// existing viewBox already contains the rect.
func growViewBoxAround(svg string, need minSvgRect) string {
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	tagEnd := strings.Index(svg[start:], ">")
	if tagEnd < 0 {
		return svg
	}
	tagEnd += start
	header := svg[start : tagEnd+1]
	vb := extractAttr(header, "viewBox")
	if vb == "" {
		return svg
	}
	var x, y, w, h float64
	if _, err := fmt.Sscanf(vb, "%f %f %f %f", &x, &y, &w, &h); err != nil {
		return svg
	}
	x2, y2 := x+w, y+h
	if need.minX < x {
		x = need.minX
	}
	if need.minY < y {
		y = need.minY
	}
	if need.maxX > x2 {
		x2 = need.maxX
	}
	if need.maxY > y2 {
		y2 = need.maxY
	}
	w, h = x2-x, y2-y
	newVB := fmt.Sprintf("%.2f %.2f %.2f %.2f", x, y, w, h)
	header = replaceAttr(header, "viewBox", newVB)
	header = replaceAttr(header, "width", fmt.Sprintf("%.2f", w))
	header = replaceAttr(header, "height", fmt.Sprintf("%.2f", h))
	body := svg[tagEnd+1:]
	// Resize the canvas-bg rect to track the new viewBox.
	body = resizeCanvasBg(body, x, y, w, h)
	return svg[:start] + header + body
}

// resizeCanvasBg finds the data-layer="canvas-bg" rect and rewrites its
// x/y/width/height so it always covers the full viewBox after growth.
func resizeCanvasBg(body string, x, y, w, h float64) string {
	marker := `<rect data-layer="canvas-bg"`
	i := strings.Index(body, marker)
	if i < 0 {
		return body
	}
	end := strings.Index(body[i:], "/>")
	if end < 0 {
		return body
	}
	end += i
	rewritten := fmt.Sprintf(
		`<rect data-layer="canvas-bg" x="%.2f" y="%.2f" width="%.2f" height="%.2f"`,
		x, y, w, h,
	)
	// Preserve the original fill attr by appending whatever was after
	// width=... up to "/>". Simplest robust path: extract fill and append.
	if fill := extractAttr(body[i:end+2], "fill"); fill != "" {
		rewritten += fmt.Sprintf(` fill="%s"`, fill)
	}
	rewritten += `/>`
	return body[:i] + rewritten + body[end+2:]
}

// RenderDocument renders every node in a Document and returns a map of
// node-id → SVG string. The document's Theme is applied to every node.
// v2 — the document's top-level "scene" composite picks up Canvas
