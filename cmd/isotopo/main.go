// Command isotopo — DSL → iso topology renderer.
//
// Usage:
//
//	isotopo render [--layout dagre|elk] <input.yaml|input.d2|-> <output-dir>
//	isotopo capabilities                       # structured JSON of supported features
//
// The render subcommand output directory is laid out as two tiers:
//
//	<out>/
//	├── topology.svg              # the full scene
//	├── topology.html             # embed snippet + editable DSL source
//	├── topology.<yaml|d2>        # source copy (for re-rendering)
//	└── nodes/
//	    ├── _index.html           # gallery of all per-part SVGs
//	    ├── <id>.svg              # standalone iso element
//	    ├── <id>.html             # embed snippet + per-part DSL fragment
//	    └── <id>.yaml             # per-part DSL fragment
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	isotopo "github.com/MarkovWangRR/iso-topology"
	"gopkg.in/yaml.v3"
)

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// freezeLayoutText strips the layout that drives root-part positions so
// a frozen scene renders purely from explicit offsets: the scene-root
// `layout:` (inline or block, ANY mode) and every root part's `place:`
// (inline or block). Nested-group layouts and child place are left
// intact — only the root parts the user drags are detached from the
// engine. Identifies "root" by indentation: the shallowest `- ` item
// depth under the parts: list.
func freezeLayoutText(src string) string {
	lines := strings.Split(src, "\n")
	keyBlock := func(re *regexp.Regexp) {
		inlineRe := regexp.MustCompile(re.String() + `\s*\{`)
		out := make([]string, 0, len(lines))
		for i := 0; i < len(lines); i++ {
			if re.MatchString(lines[i]) {
				if inlineRe.MatchString(lines[i]) || strings.Contains(lines[i], "{") {
					continue // inline form: drop the one line
				}
				// block form: drop this line + its deeper-indented body
				ind := indentOf(lines[i])
				j := i + 1
				for j < len(lines) && (strings.TrimSpace(lines[j]) == "" || indentOf(lines[j]) > ind) {
					j++
				}
				i = j - 1
				continue
			}
			out = append(out, lines[i])
		}
		lines = out
	}
	// The scene's own keys (shape/layout/parts) share one indent: the
	// SHALLOWEST `parts:`. Strip only the layout: at that indent (the
	// scene root) — nested group layouts sit deeper and must survive.
	rootIndent := -1
	partsRe := regexp.MustCompile(`^( *)parts:\s*$`)
	for _, l := range lines {
		if m := partsRe.FindStringSubmatch(l); m != nil {
			if rootIndent < 0 || len(m[1]) < rootIndent {
				rootIndent = len(m[1])
			}
		}
	}
	if rootIndent >= 0 {
		keyBlock(regexp.MustCompile(`^ {` + itoa(rootIndent) + `}layout:`))
		// Root-part `place:` sits one list level under parts: — at the
		// root part's key indent (rootIndent+4 in block form). Strip place
		// only at root-part depth; nested children's place is deeper.
		keyBlock(regexp.MustCompile(`^ {` + itoa(rootIndent+4) + `}place:`))
	}
	// Flow-form root parts carry place inline on the `- { … }` line;
	// strip an inline place: { … } from those (root depth = rootIndent+2).
	if rootIndent >= 0 {
		inlinePlace := regexp.MustCompile(`,?\s*place:\s*\{[^}]*\}`)
		dash := regexp.MustCompile(`^ {` + itoa(rootIndent+2) + `}- \{`)
		for i, l := range lines {
			if dash.MatchString(l) {
				lines[i] = inlinePlace.ReplaceAllString(l, "")
			}
		}
	}
	return strings.Join(lines, "\n")
}

func itoa(n int) string { return strconv.Itoa(n) }

// upsertInlineKey replaces or inserts an inline `key: { wx: X, wy: Y }`
// line inside the YAML block that begins at startLine (an `id:` line or
// a connector `- ` item). Block ends at the next line whose indent is
// ≤ the start line's indent. Preserves all other formatting/comments —
// the Studio drag must not reflow the user's YAML. Returns (newSrc, ok).
func upsertInlineKey(src string, startLine int, key string, wx, wy float64) (string, bool) {
	lines := strings.Split(src, "\n")
	if startLine < 0 || startLine >= len(lines) {
		return src, false
	}
	itemIndent := indentOf(lines[startLine])
	childIndent := itemIndent + 2
	blockEnd := len(lines)
	for i := startLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= itemIndent {
			blockEnd = i
			break
		}
	}
	// Flow-map form on the start line itself (e.g. `- { id: c, … }`):
	// upsert the key INSIDE the braces, since the part is one line.
	if strings.Contains(lines[startLine], "{") && strings.Contains(lines[startLine], "}") {
		l := lines[startLine]
		// drop an existing top-level offset/bend (value has no nested braces)
		l = regexp.MustCompile(`,?\s*`+regexp.QuoteMeta(key)+`:\s*\{[^}]*\}`).ReplaceAllString(l, "")
		val := fmt.Sprintf("%s: { wx: %.0f, wy: %.0f }, ", key, wx, wy)
		if i := strings.Index(l, "{"); i >= 0 {
			l = l[:i+1] + " " + val + strings.TrimLeft(l[i+1:], " ")
		}
		// Normalize commas the removal/insert can leave behind ("{ ,",
		// ", ,", ", }") — a stray double comma is invalid YAML and made
		// re-dragging a frozen flow-form node silently fail to render.
		l = regexp.MustCompile(`\{\s*,`).ReplaceAllString(l, "{ ")
		l = regexp.MustCompile(`,\s*,`).ReplaceAllString(l, ", ")
		l = regexp.MustCompile(`,\s*\}`).ReplaceAllString(l, " }")
		lines[startLine] = l
		return strings.Join(lines, "\n"), true
	}
	newLine := fmt.Sprintf("%s%s: { wx: %.0f, wy: %.0f }", strings.Repeat(" ", childIndent), key, wx, wy)
	keyRe := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `:`)
	for i := startLine + 1; i < blockEnd; i++ {
		if keyRe.MatchString(lines[i]) {
			// drop a block-form value's deeper-indented sub-lines too
			j := i + 1
			for j < blockEnd && strings.TrimSpace(lines[j]) != "" && indentOf(lines[j]) > indentOf(lines[i]) {
				j++
			}
			out := append([]string{}, lines[:i]...)
			out = append(out, newLine)
			out = append(out, lines[j:]...)
			return strings.Join(out, "\n"), true
		}
	}
	out := append([]string{}, lines[:startLine+1]...)
	out = append(out, newLine)
	out = append(out, lines[startLine+1:]...)
	return strings.Join(out, "\n"), true
}

// upsertInlineList replaces or inserts an inline `key: [ { wx, wy }, … ]`
// on the connector block at startLine, and removes any existing `bend`
// (waypoints supersede a single-corner bend). An empty pts list removes the
// key entirely (reverts to the auto route). Values are always written inline
// (one line), so replacement just swaps single child lines — mirroring
// upsertInlineKey's comment-preserving, flow/block-aware surgery.
func upsertInlineList(src string, startLine int, key string, pts [][2]float64) (string, bool) {
	lines := strings.Split(src, "\n")
	if startLine < 0 || startLine >= len(lines) {
		return src, false
	}
	var vb strings.Builder
	vb.WriteString("[")
	for i, p := range pts {
		if i > 0 {
			vb.WriteString(", ")
		}
		fmt.Fprintf(&vb, "{ wx: %.0f, wy: %.0f }", p[0], p[1])
	}
	vb.WriteString("]")
	listVal := vb.String()

	itemIndent := indentOf(lines[startLine])
	childIndent := itemIndent + 2

	// Flow-map form on the start line itself (e.g. `- { from: a, to: b, … }`):
	// edit inside the braces.
	if strings.Contains(lines[startLine], "{") && strings.Contains(lines[startLine], "}") {
		l := lines[startLine]
		l = regexp.MustCompile(`,?\s*bend:\s*\{[^}]*\}`).ReplaceAllString(l, "")
		l = regexp.MustCompile(`,?\s*`+regexp.QuoteMeta(key)+`:\s*\[[^\]]*\]`).ReplaceAllString(l, "")
		if len(pts) > 0 {
			val := fmt.Sprintf("%s: %s, ", key, listVal)
			if i := strings.Index(l, "{"); i >= 0 {
				l = l[:i+1] + " " + val + strings.TrimLeft(l[i+1:], " ")
			}
		}
		l = regexp.MustCompile(`\{\s*,`).ReplaceAllString(l, "{ ")
		l = regexp.MustCompile(`,\s*,`).ReplaceAllString(l, ", ")
		l = regexp.MustCompile(`,\s*\}`).ReplaceAllString(l, " }")
		lines[startLine] = l
		return strings.Join(lines, "\n"), true
	}

	// Block form: drop existing single-line `bend:`/`waypoints:` children,
	// then insert the new inline list (if any) as the first child line.
	dropRe := regexp.MustCompile(`^\s*(?:bend|` + regexp.QuoteMeta(key) + `):`)
	kept := make([]string, 0, len(lines))
	for i, l := range lines {
		if i > startLine && indentOf(l) > itemIndent && dropRe.MatchString(l) {
			continue
		}
		kept = append(kept, l)
	}
	lines = kept
	if len(pts) > 0 {
		newLine := strings.Repeat(" ", childIndent) + key + ": " + listVal
		out := append([]string{}, lines[:startLine+1]...)
		out = append(out, newLine)
		out = append(out, lines[startLine+1:]...)
		lines = out
	}
	return strings.Join(lines, "\n"), true
}

// ── Detail editor (Studio right-click → Edit details) ────────────────────
//
// The form is SCHEMA-DRIVEN, not "show whatever scalar keys exist": each DSL
// key that matters for visual tuning is declared here ONCE with an English
// label, a one-line description, and an input type. detailSchema is the single
// source of truth — designing the DSL means deciding each key's semantics up
// front. Values are read from the element's current YAML (any depth, dotted
// path) and changes are written back in place (comment-preserving).

type schemaField struct {
	Path    string   `json:"key"`   // dotted YAML path; also the form key
	Label   string   `json:"label"` // English, human-readable
	Desc    string   `json:"desc"`  // one-line semantic description
	Type    string   `json:"type"`  // text | number | color | icon | choice
	Options []string `json:"options,omitempty"`
	Group   string   `json:"group,omitempty"`  // section header in the modal
	Inline  bool     `json:"inline,omitempty"` // compact, laid out side-by-side
	Value   string   `json:"value"`            // current value, filled per request
}

// nodeSchema / edgeSchema declare the editable, visually-impactful fields.
// shapeOptions are the real, accepted shape tokens (see `isotopo capabilities`).
var shapeOptions = []string{"rectangle", "cylinder", "circle", "cloud", "person",
	"hexprism", "prism", "diamond", "triprism", "octprism", "group", "boundary", "text"}

// shapeClass buckets a shape by how it takes colour, so the detail form only
// offers controls that actually affect that shape:
//
//	faces   — top/left/right palette (box family, prisms, cylinder, group)
//	outline — dashed border only (boundary)
//	text    — label colour/size (iso_text)
//	fill    — a single surface colour (circle, cloud, person)
func shapeClass(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "boundary":
		return "outline"
	case "text", "iso_text", "title":
		return "text"
	case "circle", "oval", "cloud", "person", "c4-person", "c4_person", "sphere":
		return "fill"
	default: // rectangle/box/square, prism family, hexprism, cylinder, group, …
		return "faces"
	}
}

// nodeSchema is shape-aware: the colour section depends on what the shape can
// actually render, so users never tune a knob that does nothing.
func nodeSchema(shape string) []schemaField {
	f := []schemaField{
		{Group: "Content", Path: "label", Label: "Label", Desc: "Text rendered on the node", Type: "text"},
		{Group: "Content", Path: "shape", Label: "Shape", Desc: "Geometric form of the node", Type: "choice", Options: shapeOptions},
		{Group: "Content", Path: "icon", Label: "Icon", Desc: "iso://… ref, image URL, or pick a local file", Type: "icon"},
		{Group: "Content", Path: "preset", Label: "Style preset", Desc: "Named style from theme.presets", Type: "text"},
		{Group: "Size — world units", Path: "geom.w", Label: "Width", Type: "number", Inline: true},
		{Group: "Size — world units", Path: "geom.d", Label: "Depth", Type: "number", Inline: true},
		{Group: "Size — world units", Path: "geom.h", Label: "Height", Type: "number", Inline: true},
	}
	switch shapeClass(shape) {
	case "outline":
		f = append(f,
			schemaField{Group: "Outline", Path: "style.stroke.color", Label: "Border color", Desc: "Outline color (CSS color)", Type: "color"},
			schemaField{Group: "Outline", Path: "style.stroke.width", Label: "Border width", Type: "number"},
			schemaField{Group: "Outline", Path: "style.stroke.dash", Label: "Border style", Desc: "Solid, dashed, or dotted",
				Type: "choice", Options: []string{"", "6 4", "1 5"}})
	case "text":
		f = append(f,
			schemaField{Group: "Text", Path: "style.text.color", Label: "Text color", Desc: "Label color (CSS color)", Type: "color"},
			schemaField{Group: "Text", Path: "style.text.size", Label: "Font size", Type: "number"})
	case "fill":
		f = append(f, schemaField{Group: "Color", Path: "style.palette.top", Label: "Fill color", Desc: "Surface fill (CSS color)", Type: "color"})
	default: // faces
		f = append(f,
			schemaField{Group: "Face colors", Path: "style.palette.top", Label: "Top", Type: "color", Inline: true},
			schemaField{Group: "Face colors", Path: "style.palette.left", Label: "Left", Type: "color", Inline: true},
			schemaField{Group: "Face colors", Path: "style.palette.right", Label: "Right", Type: "color", Inline: true})
		if strings.EqualFold(strings.TrimSpace(shape), "group") {
			f = append(f, schemaField{Group: "Face colors", Path: "style.stroke.color", Label: "Border", Type: "color", Inline: true})
		}
	}
	return f
}

func canvasSchema() []schemaField {
	return []schemaField{
		{Group: "Background", Path: "background", Label: "Fill color", Desc: "Canvas fill behind the diagram (CSS color)", Type: "color"},
		{Group: "Background", Path: "grid", Label: "Grid pattern", Desc: "Background texture", Type: "choice",
			Options: []string{"none", "iso", "dots", "hatch", "solid"}},
		{Group: "Background", Path: "gridColor", Label: "Grid color", Desc: "Grid/texture line color (CSS color)", Type: "color"},
		{Group: "Background", Path: "gridStep", Label: "Grid step", Desc: "Grid cell size in world units (blank = default)", Type: "number"},
		{Group: "Layout", Path: "padding", Label: "Padding", Desc: "Outer breathing margin around the scene, px", Type: "number"},
	}
}

func edgeSchema() []schemaField {
	return []schemaField{
		{Group: "Connection", Path: "from", Label: "From", Desc: "Source anchor — node id or node.face", Type: "text"},
		{Group: "Connection", Path: "to", Label: "To", Desc: "Target anchor — node id or node.face", Type: "text"},
		{Group: "Style", Path: "label", Label: "Label", Desc: "Text rendered mid-route", Type: "text"},
		{Group: "Style", Path: "arrow", Label: "Arrowhead", Desc: "Marker drawn at the target end", Type: "choice",
			Options: []string{"none", "triangle"}},
		{Group: "Style", Path: "routing", Label: "Routing", Desc: "Path style between endpoints", Type: "choice",
			Options: []string{"orthogonal", "straight", "bezier"}},
		{Group: "Style", Path: "stroke.color", Label: "Line color", Desc: "Stroke color (CSS color)", Type: "color"},
		{Group: "Style", Path: "stroke.width", Label: "Line width", Desc: "Stroke width", Type: "number"},
		{Group: "Style", Path: "stroke.dash", Label: "Line style", Desc: "Solid, dashed, or dotted",
			Type: "choice", Options: []string{"", "6 4", "1 5"}},
	}
}

// splitTopCommas splits a YAML flow-map body on top-level commas, respecting
// nested {}/[] and double-quoted strings.
func splitTopCommas(s string) []string {
	var out []string
	depth, inStr := 0, false
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '"':
			inStr = !inStr
		case '{', '[':
			if !inStr {
				depth++
			}
		case '}', ']':
			if !inStr {
				depth--
			}
		case ',':
			if !inStr && depth == 0 {
				out = append(out, b.String())
				b.Reset()
				continue
			}
		}
		b.WriteRune(r)
	}
	if strings.TrimSpace(b.String()) != "" {
		out = append(out, b.String())
	}
	return out
}

func unquoteYAML(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if uq, err := strconv.Unquote(v); err == nil {
			return uq
		}
		return v[1 : len(v)-1]
	}
	return v
}

// stringifyYAML renders a parsed YAML scalar as the string the form shows.
func stringifyYAML(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// readPath walks a dotted path (e.g. "style.palette.top") through a parsed
// YAML map and returns the scalar at the leaf as a string ("" if absent).
func readPath(m map[string]interface{}, path string) string {
	var cur interface{} = m
	for _, seg := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur, ok = mm[seg]
		if !ok {
			return ""
		}
	}
	if _, isMap := cur.(map[string]interface{}); isMap {
		return ""
	}
	return stringifyYAML(cur)
}

// findNodeMap locates a node/part subtree by id anywhere in the parsed doc:
// a top-level node keyed by name, or any nested part whose `id` matches.
func findNodeMap(root map[string]interface{}, id string) map[string]interface{} {
	if nodes, ok := root["nodes"].(map[string]interface{}); ok {
		if n, ok := nodes[id].(map[string]interface{}); ok {
			return n
		}
	}
	var walk func(v interface{}) map[string]interface{}
	walk = func(v interface{}) map[string]interface{} {
		switch t := v.(type) {
		case map[string]interface{}:
			if s, _ := t["id"].(string); s == id {
				return t
			}
			for _, vv := range t {
				if r := walk(vv); r != nil {
					return r
				}
			}
		case []interface{}:
			for _, vv := range t {
				if r := walk(vv); r != nil {
					return r
				}
			}
		}
		return nil
	}
	return walk(root)
}

// findConnectors returns the first `connectors` list in the parsed doc
// (matching findConnectorLine's "first connectors block" convention).
func findConnectors(root map[string]interface{}) []interface{} {
	if nodes, ok := root["nodes"].(map[string]interface{}); ok {
		if scene, ok := nodes["scene"].(map[string]interface{}); ok {
			if c, ok := scene["connectors"].([]interface{}); ok {
				return c
			}
		}
	}
	var walk func(v interface{}) []interface{}
	walk = func(v interface{}) []interface{} {
		switch t := v.(type) {
		case map[string]interface{}:
			if c, ok := t["connectors"].([]interface{}); ok {
				return c
			}
			for _, vv := range t {
				if r := walk(vv); r != nil {
					return r
				}
			}
		case []interface{}:
			for _, vv := range t {
				if r := walk(vv); r != nil {
					return r
				}
			}
		}
		return nil
	}
	return walk(root)
}

// schemaWithValues returns the kind's schema with each field's Value read
// from the element's current YAML.
func schemaWithValues(src, kind, id string, ci int) ([]schemaField, bool) {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		return nil, false
	}
	var subtree map[string]interface{}
	var fields []schemaField
	switch kind {
	case "node":
		subtree = findNodeMap(root, id)
		shape, _ := subtree["shape"].(string) // nil subtree → "" → face schema; guarded below
		fields = nodeSchema(shape)
	case "edge":
		conns := findConnectors(root)
		if ci >= 0 && ci < len(conns) {
			subtree, _ = conns[ci].(map[string]interface{})
		}
		fields = edgeSchema()
	case "canvas":
		// canvas may be absent — still show the schema so the user can add a
		// background/grid from scratch (the editor creates the block on save).
		subtree, _ = root["canvas"].(map[string]interface{})
		if subtree == nil {
			subtree = map[string]interface{}{}
		}
		fields = canvasSchema()
	default:
		return nil, false
	}
	if subtree == nil {
		return nil, false
	}
	for i := range fields {
		fields[i].Value = readPath(subtree, fields[i].Path)
	}
	return fields, true
}

// buildChain renders a nested inline map "{ k1: { k2: … : v } }" for a
// dotted path that doesn't yet exist in the source.
func buildChain(keys []string, v string) string {
	if len(keys) == 1 {
		return "{ " + keys[0] + ": " + yamlScalar(v) + " }"
	}
	return "{ " + keys[0] + ": " + buildChain(keys[1:], v) + " }"
}

// setInInlineMap sets a (possibly nested) path inside a YAML flow map string
// "{ … }", creating intermediate maps as needed; empty value removes the leaf.
func setInInlineMap(s string, path []string, value string) string {
	open := strings.Index(s, "{")
	close := strings.LastIndex(s, "}")
	if open < 0 || close <= open {
		return s
	}
	pre, post := s[:open], s[close+1:]
	inner := s[open+1 : close]
	entries := splitTopCommas(inner)
	idx := -1
	for i, e := range entries {
		k := strings.TrimSpace(strings.SplitN(e, ":", 2)[0])
		if k == path[0] {
			idx = i
			break
		}
	}
	if idx >= 0 {
		if len(path) == 1 {
			if value == "" {
				entries = append(entries[:idx], entries[idx+1:]...)
			} else {
				entries[idx] = path[0] + ": " + yamlScalar(value)
			}
		} else {
			cur := strings.TrimSpace(strings.SplitN(entries[idx], ":", 2)[1])
			if strings.HasPrefix(cur, "{") {
				entries[idx] = path[0] + ": " + setInInlineMap(cur, path[1:], value)
			} else {
				entries[idx] = path[0] + ": " + buildChain(path[1:], value)
			}
		}
	} else if value != "" {
		if len(path) == 1 {
			entries = append(entries, path[0]+": "+yamlScalar(value))
		} else {
			entries = append(entries, path[0]+": "+buildChain(path[1:], value))
		}
	}
	// trim blanks
	out := entries[:0]
	for _, e := range entries {
		if strings.TrimSpace(e) != "" {
			out = append(out, strings.TrimSpace(e))
		}
	}
	if len(out) == 0 {
		return pre + "{}" + post
	}
	return pre + "{ " + strings.Join(out, ", ") + " }" + post
}

// setField writes a dotted path into the node/edge block at startLine,
// preserving comments. Flow-form items edit inside their braces; block-form
// items recurse into inline child maps or deeper blocks, creating inline maps
// for missing intermediates. Empty value removes a scalar leaf.
func setField(src string, startLine int, path []string, value string) (string, bool) {
	lines := strings.Split(src, "\n")
	if startLine < 0 || startLine >= len(lines) {
		return src, false
	}
	line := lines[startLine]
	// Flow-form item: the whole element lives in `{ … }` on this line.
	if open := strings.Index(line, "{"); open >= 0 {
		if close := strings.LastIndex(line, "}"); close > open {
			lines[startLine] = line[:open] + setInInlineMap(line[open:close+1], path, value) + line[close+1:]
			return strings.Join(lines, "\n"), true
		}
	}
	if len(path) == 1 {
		return upsertScalar(src, startLine, path[0], value)
	}
	// Block-form item: descend into path[0].
	itemIndent := indentOf(line)
	childIndent := itemIndent + 2
	blockEnd := len(lines)
	for k := startLine + 1; k < len(lines); k++ {
		if strings.TrimSpace(lines[k]) == "" {
			continue
		}
		if indentOf(lines[k]) <= itemIndent {
			blockEnd = k
			break
		}
	}
	keyRe := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(path[0]) + `:`)
	for k := startLine + 1; k < blockEnd; k++ {
		if indentOf(lines[k]) != childIndent || !keyRe.MatchString(lines[k]) {
			continue
		}
		rest := strings.TrimSpace(lines[k][strings.Index(lines[k], ":")+1:])
		switch {
		case strings.HasPrefix(rest, "{"):
			open := strings.Index(lines[k], "{")
			close := strings.LastIndex(lines[k], "}")
			lines[k] = lines[k][:open] + setInInlineMap(lines[k][open:close+1], path[1:], value) + lines[k][close+1:]
			return strings.Join(lines, "\n"), true
		case rest == "" || strings.HasPrefix(rest, "&"):
			return setField(strings.Join(lines, "\n"), k, path[1:], value) // recurse into sub-block
		default:
			pre := lines[k][:strings.Index(lines[k], ":")+1]
			lines[k] = pre + " " + buildChain(path[1:], value)
			return strings.Join(lines, "\n"), true
		}
	}
	if value == "" {
		return strings.Join(lines, "\n"), true
	}
	newLine := strings.Repeat(" ", childIndent) + path[0] + ": " + buildChain(path[1:], value)
	out := append([]string{}, lines[:startLine+1]...)
	out = append(out, newLine)
	out = append(out, lines[startLine+1:]...)
	return strings.Join(out, "\n"), true
}

// yamlScalar renders a value for write-back, quoting when it contains
// characters that aren't safe bare (spaces, ':', flow punctuation, …).
func yamlScalar(v string) string {
	if v == "" {
		return `""`
	}
	if regexp.MustCompile(`^[A-Za-z0-9_.\-/]+$`).MatchString(v) {
		return v
	}
	return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
}

// upsertScalar sets `key: value` inside the node/edge block at startLine
// (flow or block form), preserving comments/formatting. An empty value
// removes the key. Mirrors upsertInlineKey's surgery for scalar values.
func upsertScalar(src string, startLine int, key, value string) (string, bool) {
	lines := strings.Split(src, "\n")
	if startLine < 0 || startLine >= len(lines) {
		return src, false
	}
	itemIndent := indentOf(lines[startLine])
	childIndent := itemIndent + 2
	remove := value == ""

	if strings.Contains(lines[startLine], "{") && strings.Contains(lines[startLine], "}") {
		l := lines[startLine]
		l = regexp.MustCompile(`,?\s*`+regexp.QuoteMeta(key)+`:\s*("(?:[^"\\]|\\.)*"|[^,}]*)`).ReplaceAllString(l, "")
		if !remove {
			val := fmt.Sprintf("%s: %s, ", key, yamlScalar(value))
			if i := strings.Index(l, "{"); i >= 0 {
				l = l[:i+1] + " " + val + strings.TrimLeft(l[i+1:], " ")
			}
		}
		l = regexp.MustCompile(`\{\s*,`).ReplaceAllString(l, "{ ")
		l = regexp.MustCompile(`,\s*,`).ReplaceAllString(l, ", ")
		l = regexp.MustCompile(`,\s*\}`).ReplaceAllString(l, " }")
		lines[startLine] = l
		return strings.Join(lines, "\n"), true
	}

	// Block form. The anchor key on the start line (`- key: value`) is edited
	// in place; never removed (it holds the item together).
	if m := regexp.MustCompile(`^(\s*-\s*)` + regexp.QuoteMeta(key) + `:`).FindStringSubmatch(lines[startLine]); m != nil {
		if !remove {
			lines[startLine] = m[1] + key + ": " + yamlScalar(value)
		}
		return strings.Join(lines, "\n"), true
	}
	blockEnd := len(lines)
	for i := startLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= itemIndent {
			blockEnd = i
			break
		}
	}
	keyRe := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `:`)
	newLine := strings.Repeat(" ", childIndent) + key + ": " + yamlScalar(value)
	for i := startLine + 1; i < blockEnd; i++ {
		if indentOf(lines[i]) == childIndent && keyRe.MatchString(lines[i]) {
			out := append([]string{}, lines[:i]...)
			if !remove {
				out = append(out, newLine)
			}
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n"), true
		}
	}
	if remove {
		return strings.Join(lines, "\n"), true
	}
	out := append([]string{}, lines[:startLine+1]...)
	out = append(out, newLine)
	out = append(out, lines[startLine+1:]...)
	return strings.Join(out, "\n"), true
}

// findPartIDLine returns the line index of `id: <id>` (with optional
// `- ` prefix and quotes), or -1.
func findPartIDLine(src, id string) int {
	// Match `id: <id>` in both block form (`- id: c` / `id: c`) and flow
	// form (`- { id: c, … }`). The id is bounded by a comma, brace,
	// whitespace, or line end so "c" never matches "client".
	re := regexp.MustCompile(`(?:^|[-{,]\s*)id:\s*"?` + regexp.QuoteMeta(id) + `"?\s*(?:,|}|$)`)
	for i, l := range strings.Split(src, "\n") {
		if re.MatchString(l) {
			return i
		}
	}
	return -1
}

// findConnectorLine returns the line index of the ci-th `- ` item under
// the first `connectors:` key, or -1.
func findConnectorLine(src string, ci int) int {
	lines := strings.Split(src, "\n")
	connLine, connIndent := -1, 0
	for i, l := range lines {
		if regexp.MustCompile(`^\s*connectors:\s*$`).MatchString(l) {
			connLine, connIndent = i, indentOf(l)
			break
		}
	}
	if connLine < 0 {
		return -1
	}
	itemRe := regexp.MustCompile(`^\s*-\s`)
	n := 0
	for i := connLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= connIndent {
			break // left the connectors block
		}
		if itemRe.MatchString(lines[i]) && indentOf(lines[i]) > connIndent {
			if n == ci {
				return i
			}
			n++
		}
	}
	return -1
}

// findCanvasLine returns the line index of the top-level `canvas:` key
// (flow `canvas: { … }` or block form), or -1.
func findCanvasLine(src string) int {
	re := regexp.MustCompile(`^\s*canvas\s*:`)
	for i, l := range strings.Split(src, "\n") {
		if re.MatchString(l) {
			return i
		}
	}
	return -1
}

// itemEnd returns the line index just past the YAML list item that starts at
// `start` — the next non-blank line indented ≤ the item, or EOF.
func itemEnd(lines []string, start int) int {
	ind := indentOf(lines[start])
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= ind {
			return i
		}
	}
	return len(lines)
}

// findPartItemRange locates the `- ` list item that declares part `id`,
// returning [start,end) line indices (covers flow `- { id: x … }` and block
// `- id: x` / `- shape: …\n  id: x` forms, at any nesting depth).
func findPartItemRange(src, id string) (int, int, bool) {
	lines := strings.Split(src, "\n")
	reDash := regexp.MustCompile(`(?:^|[-{,]\s*)id:\s*"?` + regexp.QuoteMeta(id) + `"?\s*(?:,|}|$)`)
	reIndented := regexp.MustCompile(`^\s*id:\s*"?` + regexp.QuoteMeta(id) + `"?\s*$`)
	idLine := -1
	for i, l := range lines {
		if reDash.MatchString(l) || reIndented.MatchString(l) {
			idLine = i
			break
		}
	}
	if idLine < 0 {
		return 0, 0, false
	}
	start := idLine
	for start >= 0 && !strings.HasPrefix(strings.TrimSpace(lines[start]), "- ") {
		start--
	}
	if start < 0 {
		return 0, 0, false
	}
	return start, itemEnd(lines, start), true
}

// connectorItemRange returns [start,end) for the ci-th connector item.
func connectorItemRange(src string, ci int) (int, int, bool) {
	start := findConnectorLine(src, ci)
	if start < 0 {
		return 0, 0, false
	}
	return start, itemEnd(strings.Split(src, "\n"), start), true
}

func deleteLineRange(src string, start, end int) string {
	lines := strings.Split(src, "\n")
	if start < 0 || start >= len(lines) || end < start {
		return src
	}
	return strings.Join(append(append([]string{}, lines[:start]...), lines[end:]...), "\n")
}

// deletePart removes a node and any connectors that reference it (so the
// scene stays valid — no dangling from/to). Connectors are removed
// bottom-up so earlier indices don't shift.
func deletePart(src, id string) (string, bool) {
	conns := findConnectors(mustParse(src))
	// A connector references the node by bare id ("core") OR an anchor-
	// qualified endpoint ("core.front"); both must go or the delete leaves a
	// dangling reference that fails validation.
	refsID := func(v interface{}) bool {
		s, _ := v.(string)
		return s == id || strings.HasPrefix(s, id+".")
	}
	var refs []int
	for i, c := range conns {
		if m, ok := c.(map[string]interface{}); ok {
			if refsID(m["from"]) || refsID(m["to"]) {
				refs = append(refs, i)
			}
		}
	}
	out := src
	for i := len(refs) - 1; i >= 0; i-- {
		if s, e, ok := connectorItemRange(out, refs[i]); ok {
			out = deleteLineRange(out, s, e)
		}
	}
	// Other parts may be positioned relative to this one (place: { rightOf: id,
	// … }); drop that place: block so they fall back to the layout instead of
	// dangling on a missing anchor.
	out = stripPlaceReferencing(out, id)
	s, e, ok := findPartItemRange(out, id)
	if !ok {
		return src, false
	}
	return deleteLineRange(out, s, e), true
}

// stripPlaceReferencing removes the place: block from any part whose place
// constraint points at `id` (rightOf/leftOf/inFrontOf/behind/above).
func stripPlaceReferencing(src, id string) string {
	refs := map[string]bool{}
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case map[string]interface{}:
			if pl, ok := t["place"].(map[string]interface{}); ok {
				for _, k := range []string{"rightOf", "leftOf", "inFrontOf", "behind", "above"} {
					if s, _ := pl[k].(string); s == id {
						if pid, _ := t["id"].(string); pid != "" {
							refs[pid] = true
						}
					}
				}
			}
			for _, vv := range t {
				walk(vv)
			}
		case []interface{}:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(mustParse(src))
	out := src
	for pid := range refs {
		if s, e, ok := findPartItemRange(out, pid); ok {
			out = removeKeyInRange(out, s, e, "place")
		}
	}
	return out
}

// removeKeyInRange deletes `key:` (inline flow value or block child) from the
// list item spanning [start,end).
func removeKeyInRange(src string, start, end int, key string) string {
	lines := strings.Split(src, "\n")
	if start < 0 || start >= len(lines) {
		return src
	}
	if strings.Contains(lines[start], "{") && strings.Contains(lines[start], "}") {
		l := regexp.MustCompile(`,?\s*`+regexp.QuoteMeta(key)+`:\s*\{[^}]*\}`).ReplaceAllString(lines[start], "")
		l = regexp.MustCompile(`\{\s*,`).ReplaceAllString(l, "{ ")
		l = regexp.MustCompile(`,\s*,`).ReplaceAllString(l, ", ")
		l = regexp.MustCompile(`,\s*\}`).ReplaceAllString(l, " }")
		lines[start] = l
		return strings.Join(lines, "\n")
	}
	re := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `:`)
	for i := start + 1; i < end && i < len(lines); i++ {
		if re.MatchString(lines[i]) {
			ind := indentOf(lines[i])
			j := i + 1
			for j < len(lines) && (strings.TrimSpace(lines[j]) == "" || indentOf(lines[j]) > ind) {
				j++
			}
			return strings.Join(append(append([]string{}, lines[:i]...), lines[j:]...), "\n")
		}
	}
	return src
}

// addPart appends a default rectangle node to the scene's (shallowest)
// parts: list. In an auto-layout scene it joins the layout; otherwise it
// lands near the origin and the user drags it.
func addPart(src string) (string, bool) {
	lines := strings.Split(src, "\n")
	partsLine, partsIndent := -1, 0
	re := regexp.MustCompile(`^( *)parts:\s*$`)
	for i, l := range lines {
		if m := re.FindStringSubmatch(l); m != nil {
			if partsLine < 0 || len(m[1]) < partsIndent {
				partsLine, partsIndent = i, len(m[1])
			}
		}
	}
	if partsLine < 0 {
		return src, false
	}
	itemIndent := partsIndent + 2
	end := len(lines)
	for i := partsLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= partsIndent {
			end = i
			break
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "- ") {
			itemIndent = indentOf(lines[i])
		}
		end = i + 1
	}
	newID := uniquePartID(src, "node")
	nl := strings.Repeat(" ", itemIndent) +
		fmt.Sprintf(`- { id: %s, shape: rectangle, geom: { w: 80, d: 80, h: 30 }, label: "New" }`, newID)
	out := append(append(append([]string{}, lines[:end]...), nl), lines[end:]...)
	return strings.Join(out, "\n"), true
}

// duplicatePart clones a node's block under a fresh id, placed at (ox,oy).
func duplicatePart(src, id string, ox, oy float64) (string, bool) {
	lines := strings.Split(src, "\n")
	s, e, ok := findPartItemRange(src, id)
	if !ok {
		return src, false
	}
	newID := uniquePartID(src, id+"_copy")
	clone := append([]string{}, lines[s:e]...)
	cloneStr := strings.Join(clone, "\n")
	// rename only the item's own id (first id: occurrence in the block)
	cloneStr = regexp.MustCompile(`id:\s*"?`+regexp.QuoteMeta(id)+`"?`).
		ReplaceAllString(cloneStr, "id: "+newID)
	out := strings.Join(append(append(append([]string{}, lines[:e]...), strings.Split(cloneStr, "\n")...), lines[e:]...), "\n")
	out, _ = upsertInlineKey(out, findPartIDLine(out, newID), "offset", ox, oy)
	return out, true
}

func uniquePartID(src, base string) string {
	id := base
	for n := 2; ; n++ {
		if _, _, ok := findPartItemRange(src, id); !ok {
			return id
		}
		id = fmt.Sprintf("%s%d", base, n)
	}
}

func mustParse(src string) map[string]interface{} {
	var m map[string]interface{}
	_ = yaml.Unmarshal([]byte(src), &m)
	return m
}

// CLI flag state — set by parseFlags. Layout is consulted only for .d2
// inputs (YAML/JSON have no layout step).
var (
	flagLayout = "dagre"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "render":
		args, err := parseFlags(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			usage()
			os.Exit(2)
		}
		if len(args) != 2 {
			usage()
			os.Exit(2)
		}
		if err := renderFile(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "capabilities":
		if err := emitCapabilities(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "serve":
		args, err := parseFlags(os.Args[2:])
		if err != nil || len(args) != 1 {
			usage()
			os.Exit(2)
		}
		if err := serveFile(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "validate":
		if len(os.Args) != 3 {
			usage()
			os.Exit(2)
		}
		code, err := validateFile(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		os.Exit(code)
	default:
		usage()
		os.Exit(2)
	}
}

// validateFile parses the input and runs Validate, emitting structured
// JSON of any issues. Exit code: 0 = clean, 2 = warnings only, 3 = any
// errors. Designed for agent CI loops.
func validateFile(in string) (int, error) {
	var data []byte
	var err error
	if in == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(in)
	}
	if err != nil {
		return 1, err
	}
	sourceLang, _ := classifyInput(in)
	doc, err := loadDocument(sourceLang, data)
	if err != nil {
		// Parse failure is a single hard error — emit as one Issue so
		// agents have a uniform JSON contract.
		out := map[string]any{
			"issues": []any{
				map[string]any{
					"severity": "error",
					"path":     "$",
					"message":  fmt.Sprintf("parse failed: %s", err),
				},
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 3, nil
	}
	issues := isotopo.Validate(doc)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"issues": issues})
	for _, i := range issues {
		if i.Severity == isotopo.SeverityError {
			return 3, nil
		}
	}
	if len(issues) > 0 {
		return 2, nil
	}
	return 0, nil
}

// emitCapabilities writes the structured capability report as
// pretty-printed JSON. Agents call this at startup to learn what
// shapes, primitives, layouts, and style keys are available without
// reading source.
func emitCapabilities(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(isotopo.CapabilityReport())
}

// parseFlags strips known flags from argv and returns the positional
// remainder. Kept hand-rolled (no `flag` package) because the CLI has
// exactly one optional flag and we want it to appear in any position.
func parseFlags(argv []string) ([]string, error) {
	positional := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--layout":
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("--layout requires a value")
			}
			flagLayout = argv[i+1]
			i++
		case strings.HasPrefix(a, "--layout="):
			flagLayout = strings.TrimPrefix(a, "--layout=")
		default:
			positional = append(positional, a)
		}
	}
	switch flagLayout {
	case "dagre", "elk":
	default:
		return nil, fmt.Errorf("invalid --layout %q (want dagre|elk)", flagLayout)
	}
	return positional, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  isotopo render [--layout dagre|elk] <input.yaml|input.d2|-> <output-dir>
  isotopo capabilities

input formats:
  .yaml / .json  manual iso composite (precise placement)
  .d2            d2 graph source (auto-layout)

flags:
  --layout dagre  natural-bend polyline edges (default)
  --layout elk    orthogonal right-angle edges with obstacle avoidance

subcommands:
  render         render an input file to <output-dir>
  capabilities   emit structured JSON of supported shapes, primitives,
                 layouts, and style keys — intended for agents to read
                 before generating DSL
  validate <in>  parse + structural validate the input file. Emits JSON
                 of issues with paths and "did you mean" suggestions.
                 exit: 0 = clean, 2 = warnings only, 3 = errors
  serve <in>     local live-preview server (default :8731, override with
                 ISOTOPO_PORT): the interactive topology.html with hover
                 source-mapping, zoom/pan, edit-to-re-render against an
                 in-browser COPY (the input file is never written),
                 SVG/PNG export of the edited canvas, and the per-node
                 gallery at /nodes/`)
}

func renderFile(in, outDir string) error {
	var data []byte
	var err error
	if in == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(in)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	nodesDir := filepath.Join(outDir, "nodes")
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		return err
	}

	sourceLang, sourceExt := classifyInput(in)
	doc, err := loadDocument(sourceLang, data)
	if err != nil {
		return err
	}

	topologySVG := renderTopologySVG(doc)
	if err := writeFile(filepath.Join(outDir, "topology.svg"), []byte(topologySVG)); err != nil {
		return err
	}

	sourceFilename := "topology" + sourceExt
	if err := writeFile(filepath.Join(outDir, sourceFilename), data); err != nil {
		return err
	}

	absCopy, err := filepath.Abs(filepath.Join(outDir, sourceFilename))
	if err != nil {
		absCopy = sourceFilename
	}
	topologyHTML := isotopo.TopologyHTML(topologySVG, string(data), sourceLang, absCopy)
	if err := writeFile(filepath.Join(outDir, "topology.html"), []byte(topologyHTML)); err != nil {
		return err
	}

	parts := isotopo.RenderParts(doc)
	frags := isotopo.PartFragments(doc)
	ids := isotopo.PartIDs(doc)
	for _, id := range ids {
		svg := parts[id]
		if svg == "" {
			continue
		}
		if err := writeFile(filepath.Join(nodesDir, id+".svg"), []byte(svg)); err != nil {
			return err
		}
		var fragYAML []byte
		if f := frags[id]; f != nil {
			fragYAML, err = isotopo.MarshalFragmentYAML(f)
			if err != nil {
				return err
			}
			if err := writeFile(filepath.Join(nodesDir, id+".yaml"), fragYAML); err != nil {
				return err
			}
		}
		nodeHTML := isotopo.NodeHTML(id, svg, string(fragYAML))
		if err := writeFile(filepath.Join(nodesDir, id+".html"), []byte(nodeHTML)); err != nil {
			return err
		}
	}

	indexHTML := isotopo.NodesIndexHTML(ids)
	if err := writeFile(filepath.Join(nodesDir, "_index.html"), []byte(indexHTML)); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "rendered %d node(s) into %s\n", len(ids), outDir)
	return nil
}

// renderTopologySVG returns the topology-level scene SVG. Resolution
// is delegated to doc.Scene() so the CLI stays in sync with the
// library's scene rules.
func renderTopologySVG(doc *isotopo.Document) string {
	if scene := doc.Scene(); scene != nil {
		return isotopo.RenderWithCanvas(scene, doc.Theme, doc.Canvas, doc.Annotations)
	}
	return ""
}

// loadDocument routes the input through the right parser based on file
// extension. YAML/JSON go through the composite parser directly; .d2 is
// compiled + auto-laid-out by d2lib first, then translated to a
// composite document. The layout engine comes from the --layout CLI
// flag (default dagre).
func loadDocument(lang string, data []byte) (*isotopo.Document, error) {
	engine := isotopo.LayoutDagre
	if flagLayout == "elk" {
		engine = isotopo.LayoutELK
	}
	return isotopo.LoadInput(context.Background(), lang, data, engine)
}

// classifyInput maps an input path to (sourceLang, fileExtension). The
// language drives loadDocument; the extension drives the
// `topology.<ext>` source-copy filename so users can re-render later.
func classifyInput(path string) (lang, ext string) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".d2":
		return "d2", ".d2"
	case ".json":
		return "json", ".json"
	default:
		return "yaml", ".yaml"
	}
}

func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// serveFile starts a local preview server for one input file: GET /
// renders the CURRENT file content into the interactive topology.html,
// and POST /api/render renders whatever source the page's editor holds
// — a copy living in the browser; the original file is never written.
func serveFile(in string) error {
	if _, err := os.Stat(in); err != nil {
		return err
	}
	sourceLang, _ := classifyInput(in)
	absIn, err := filepath.Abs(in)
	if err != nil {
		absIn = in
	}

	port := os.Getenv("ISOTOPO_PORT")
	if port == "" {
		port = "8731"
	}

	render := func(lang string, data []byte) (string, []isotopo.Issue, error) {
		doc, err := loadDocument(lang, data)
		if err != nil {
			return "", []isotopo.Issue{{Severity: isotopo.SeverityError, Path: "$", Message: err.Error()}}, nil
		}
		issues := isotopo.Validate(doc)
		for _, i := range issues {
			if i.Severity == isotopo.SeverityError {
				return "", issues, nil
			}
		}
		svg := renderTopologySVG(doc)
		if svg == "" {
			// BUG3 (cross-test suite): a doc with zero nodes parses and
			// validates clean, then renders nothing — the page showed the
			// stale badge with an EMPTY issues panel. Say why instead.
			issues = append(issues, isotopo.Issue{
				Severity: isotopo.SeverityError, Path: "$",
				Message: "document renders no scene — it has no nodes (or the scene resolves empty)",
			})
		}
		return svg, issues, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/render", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		lang := r.URL.Query().Get("format")
		if lang == "" {
			lang = sourceLang
		}
		svg, issues, err := render(lang, body)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"svg": svg, "issues": issues})
	})
	// v4.4 — drag-to-edit: the editor copy + a move op come in, the
	// server text-edits the YAML (preserving comments/formatting), then
	// renders. kind=node writes an absolute offset on the part; kind=edge
	// writes a bend delta on the ci-th connector. Returns {yaml, svg,
	// issues} so Studio updates editor AND canvas in one round-trip.
	mux.HandleFunc("POST /api/move", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		q := r.URL.Query()
		atof := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
		// dwx, dwy are a WORLD-space drag DELTA; the server resolves the
		// target's current position and adds the delta to get an absolute
		// value, so dragging a pure-auto node (no coords yet) works.
		dwx, dwy := atof(q.Get("dwx")), atof(q.Get("dwy"))
		src := string(body)
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
		}
		doc, derr := loadDocument(lang, body)
		if derr != nil {
			http.Error(w, derr.Error(), 422)
			return
		}
		var out string
		var ok bool
		switch q.Get("kind") {
		case "node":
			// drawio model: the FIRST manual move freezes the whole scene
			// into explicit coordinates and drops auto-layout, so the
			// engine never re-decides positions again — no unexpected
			// jumps. Every later move just nudges that one node.
			if isotopo.SceneNeedsFreeze(doc) {
				offs := isotopo.ResolveAllOffsets(doc)
				out = freezeLayoutText(src)
				ids := make([]string, 0, len(offs))
				for id := range offs {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				for _, id := range ids {
					o := offs[id]
					wx, wy := o[0], o[1]
					if id == q.Get("id") {
						wx += dwx
						wy += dwy
					}
					out, ok = upsertInlineKey(out, findPartIDLine(out, id), "offset", wx, wy)
				}
			} else {
				cx, cy, found := isotopo.ResolvePartOffset(doc, q.Get("id"))
				if !found {
					http.Error(w, "part not found", 422)
					return
				}
				out, ok = upsertInlineKey(src, findPartIDLine(src, q.Get("id")), "offset", cx+dwx, cy+dwy)
			}
		case "edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			// v4.6 — Studio computes the new interior waypoint list in world
			// coords (drawio-style per-segment edit) and posts it as wp; the
			// server just serialises it. Falls back to the legacy single-corner
			// bend when wp is absent (non-orthogonal routes / older clients).
			if wp := q.Get("wp"); wp != "" {
				var raw [][2]float64
				if err := json.Unmarshal([]byte(wp), &raw); err != nil {
					http.Error(w, "bad wp: "+err.Error(), 400)
					return
				}
				out, ok = upsertInlineList(src, findConnectorLine(src, ci), "waypoints", raw)
			} else {
				bx, by := isotopo.ConnectorBend(doc, ci)
				out, ok = upsertInlineKey(src, findConnectorLine(src, ci), "bend", bx+dwx, by+dwy)
			}
		default:
			http.Error(w, "kind must be node|edge", 400)
			return
		}
		if !ok {
			http.Error(w, "target not found in source", 422)
			return
		}
		svg, issues, _ := render(lang, []byte(out))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"yaml": out, "svg": svg, "issues": issues})
	})
	// v4.7 — detail editor. /api/fields returns the scalar fields actually
	// present in a node/edge's YAML, each with a semantic label, for the
	// right-click "edit details" modal. /api/edit writes the user's changes
	// back (comment-preserving) and re-renders.
	targetLine := func(src string, q url.Values) (int, bool) {
		switch q.Get("kind") {
		case "node":
			return findPartIDLine(src, q.Get("id")), true
		case "edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			return findConnectorLine(src, ci), true
		case "canvas":
			return findCanvasLine(src), true
		}
		return -1, false
	}
	mux.HandleFunc("POST /api/fields", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		src := string(body)
		q := r.URL.Query()
		ci, _ := strconv.Atoi(q.Get("ci"))
		fields, ok := schemaWithValues(src, q.Get("kind"), q.Get("id"), ci)
		if !ok {
			http.Error(w, "target not found in source", 422)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"fields": fields})
	})
	mux.HandleFunc("POST /api/edit", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		q := r.URL.Query()
		var changes map[string]string
		if err := json.Unmarshal([]byte(q.Get("f")), &changes); err != nil {
			http.Error(w, "bad f: "+err.Error(), 400)
			return
		}
		out := string(body)
		// Editing the canvas when no `canvas:` block exists yet: create an
		// empty one at the top so the writes below have somewhere to land.
		if q.Get("kind") == "canvas" && findCanvasLine(out) < 0 {
			out = "canvas: {}\n" + out
		}
		// Re-find the target after each edit: a write can shift line numbers.
		for key, val := range changes {
			line, ok := targetLine(out, q)
			if !ok {
				http.Error(w, "kind must be node|edge|canvas", 400)
				return
			}
			if line < 0 {
				http.Error(w, "target not found in source", 422)
				return
			}
			out, _ = setField(out, line, strings.Split(key, "."), val)
		}
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
		}
		svg, issues, _ := render(lang, []byte(out))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"yaml": out, "svg": svg, "issues": issues})
	})
	// v4.8 — structural ops: delete a node (+its connectors) or edge, or
	// duplicate a node. Comment-preserving text surgery on the posted source.
	mux.HandleFunc("POST /api/op", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		q := r.URL.Query()
		src := string(body)
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
		}
		var out string
		ok := false
		switch q.Get("op") + ":" + q.Get("kind") {
		case "add:node":
			out, ok = addPart(src)
		case "delete:node":
			out, ok = deletePart(src, q.Get("id"))
		case "delete:edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			if s, e, found := connectorItemRange(src, ci); found {
				out, ok = deleteLineRange(src, s, e), true
			}
		case "duplicate:node":
			ox, oy := 40.0, 40.0
			if doc, derr := loadDocument(lang, body); derr == nil {
				if cx, cy, found := isotopo.ResolvePartOffset(doc, q.Get("id")); found {
					ox, oy = cx+40, cy+40
				}
			}
			out, ok = duplicatePart(src, q.Get("id"), ox, oy)
		default:
			http.Error(w, "op must be add|delete|duplicate", 400)
			return
		}
		if !ok {
			http.Error(w, "target not found in source", 422)
			return
		}
		svg, issues, _ := render(lang, []byte(out))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"yaml": out, "svg": svg, "issues": issues})
	})
	// The per-part gallery the footer links to. The render command writes
	// these as files; serve answers them on the fly from the CURRENT file
	// content so the link works in live mode too.
	mux.HandleFunc("GET /nodes/", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(in)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		doc, err := loadDocument(sourceLang, data)
		if err != nil {
			http.Error(w, err.Error(), 422)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/nodes/")
		if name == "" || name == "_index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(isotopo.NodesIndexHTML(isotopo.PartIDs(doc))))
			return
		}
		ext := filepath.Ext(name)
		id := strings.TrimSuffix(name, ext)
		switch ext {
		case ".svg":
			svg := isotopo.RenderParts(doc)[id]
			if svg == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write([]byte(svg))
		case ".yaml":
			f := isotopo.PartFragments(doc)[id]
			if f == nil {
				http.NotFound(w, r)
				return
			}
			y, err := isotopo.MarshalFragmentYAML(f)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.Write(y)
		case ".html":
			svg := isotopo.RenderParts(doc)[id]
			if svg == "" {
				http.NotFound(w, r)
				return
			}
			var fragYAML []byte
			if f := isotopo.PartFragments(doc)[id]; f != nil {
				fragYAML, _ = isotopo.MarshalFragmentYAML(f)
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(isotopo.NodeHTML(id, svg, string(fragYAML))))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /topology.svg", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(in)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		svg, _, _ := render(sourceLang, data)
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(svg))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(in)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		svg, _, _ := render(sourceLang, data)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Never let the browser serve a stale Studio page — the template
		// changes between builds and a cached copy would run old JS (the
		// source of the "still teleports after I fixed it" reports).
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Write([]byte(isotopo.TopologyHTML(svg, string(data), sourceLang, absIn)))
	})

	fmt.Printf("isotopo Studio · %s\nopen:  http://localhost:%s\nedits in the browser are a copy — %s is never written\n", in, port, in)
	return http.ListenAndServe("localhost:"+port, mux)
}
