// Package yamledit performs comment-preserving text surgery on the topology
// YAML/d2 source. Studio edits must never reflow the user's document, so this
// operates on the raw text (line/regex surgery) rather than a yaml.Node round-
// trip (which reformats indentation, flow spacing and comment positions). Each
// exported op takes the source string and returns the edited string, touching
// only the lines it must.
package yamledit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// FreezeLayoutText strips the layout that drives root-part positions so
// a frozen scene renders purely from explicit offsets: the scene-root
// `layout:` (inline or block, ANY mode) and every root part's `place:`
// (inline or block). Nested-group layouts and child place are left
// intact — only the root parts the user drags are detached from the
// engine. Identifies "root" by indentation: the shallowest `- ` item
// depth under the parts: list.
func FreezeLayoutText(src string) string {
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

// UpsertInlineKey replaces or inserts an inline `key: { wx: X, wy: Y }`
// line inside the YAML block that begins at startLine (an `id:` line or
// a connector `- ` item). Block ends at the next line whose indent is
// ≤ the start line's indent. Preserves all other formatting/comments —
// the Studio drag must not reflow the user's YAML. Returns (newSrc, ok).
//
// wz is written only when non-zero (so 2D scenes stay byte-identical and a
// node already carrying a wz keeps it through a 2D drag).
func offsetVal(key string, wx, wy, wz float64) string {
	if wz != 0 {
		return fmt.Sprintf("%s: { wx: %.0f, wy: %.0f, wz: %.0f }", key, wx, wy, wz)
	}
	return fmt.Sprintf("%s: { wx: %.0f, wy: %.0f }", key, wx, wy)
}

func UpsertInlineKey(src string, startLine int, key string, wx, wy, wz float64) (string, bool) {
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
		val := offsetVal(key, wx, wy, wz) + ", "
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
	newLine := strings.Repeat(" ", childIndent) + offsetVal(key, wx, wy, wz)
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

// UpsertInlineList replaces or inserts an inline `key: [ { wx, wy }, … ]`
// on the connector block at startLine, and removes any existing `bend`
// (waypoints supersede a single-corner bend). An empty pts list removes the
// key entirely (reverts to the auto route). Values are always written inline
// (one line), so replacement just swaps single child lines — mirroring
// UpsertInlineKey's comment-preserving, flow/block-aware surgery.
func UpsertInlineList(src string, startLine int, key string, pts [][2]float64) (string, bool) {
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

// ReadPath walks a dotted path (e.g. "style.palette.top") through a parsed
// YAML map and returns the scalar at the leaf as a string ("" if absent).
func ReadPath(m map[string]interface{}, path string) string {
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

// FindNodeMap locates a node/part subtree by id anywhere in the parsed doc:
// a top-level node keyed by name, or any nested part whose `id` matches.
func FindNodeMap(root map[string]interface{}, id string) map[string]interface{} {
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

// FindConnectors returns the first `connectors` list in the parsed doc
// (matching FindConnectorLine's "first connectors block" convention).
func FindConnectors(root map[string]interface{}) []interface{} {
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

// SetField writes a dotted path into the node/edge block at startLine,
// preserving comments. Flow-form items edit inside their braces; block-form
// items recurse into inline child maps or deeper blocks, creating inline maps
// for missing intermediates. Empty value removes a scalar leaf.
func SetField(src string, startLine int, path []string, value string) (string, bool) {
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
			return SetField(strings.Join(lines, "\n"), k, path[1:], value) // recurse into sub-block
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
// removes the key. Mirrors UpsertInlineKey's surgery for scalar values.
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

// FindPartIDLine returns the line index of `id: <id>` (with optional
// `- ` prefix and quotes), or -1.
func FindPartIDLine(src, id string) int {
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

// FindConnectorLine returns the line index of the ci-th `- ` item under
// the first `connectors:` key, or -1.
func FindConnectorLine(src string, ci int) int {
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

// FindCanvasLine returns the line index of the top-level `canvas:` key
// (flow `canvas: { … }` or block form), or -1.
func FindCanvasLine(src string) int {
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

// ConnectorItemRange returns [start,end) for the ci-th connector item.
func ConnectorItemRange(src string, ci int) (int, int, bool) {
	start := FindConnectorLine(src, ci)
	if start < 0 {
		return 0, 0, false
	}
	return start, itemEnd(strings.Split(src, "\n"), start), true
}

// findListItemLine returns the line of the idx-th `- ` item under the first
// top-level `<blockKey>:` block, or -1.
func findListItemLine(src, blockKey string, idx int) int {
	lines := strings.Split(src, "\n")
	bl, bind := -1, 0
	keyRe := regexp.MustCompile(`^(\s*)` + regexp.QuoteMeta(blockKey) + `:\s*$`)
	for i, l := range lines {
		if m := keyRe.FindStringSubmatch(l); m != nil {
			bl, bind = i, len(m[1])
			break
		}
	}
	if bl < 0 {
		return -1
	}
	itemRe := regexp.MustCompile(`^\s*-\s`)
	n := 0
	for i := bl + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= bind {
			break
		}
		if itemRe.MatchString(lines[i]) && indentOf(lines[i]) > bind {
			if n == idx {
				return i
			}
			n++
		}
	}
	return -1
}

func DeleteLineRange(src string, start, end int) string {
	lines := strings.Split(src, "\n")
	if start < 0 || start >= len(lines) || end < start {
		return src
	}
	return strings.Join(append(append([]string{}, lines[:start]...), lines[end:]...), "\n")
}

// DeletePart removes a node and any connectors that reference it (so the
// scene stays valid — no dangling from/to). Connectors are removed
// bottom-up so earlier indices don't shift.
func DeletePart(src, id string) (string, bool) {
	s, e, ok := findPartItemRange(src, id)
	if !ok {
		return src, false
	}
	// Deleting a container (group/boundary/composite) also removes its nested
	// parts — collect EVERY id inside the removed subtree so we clean up the
	// connectors and place: refs that point at any of them, not just the root.
	removed := map[string]bool{id: true}
	idRe := regexp.MustCompile(`id:\s*"?([A-Za-z0-9_-]+)`)
	for _, l := range strings.Split(src, "\n")[s:e] {
		if m := idRe.FindStringSubmatch(l); m != nil {
			removed[m[1]] = true
		}
	}
	// A connector references a removed id by bare id ("core") or an anchor-
	// qualified endpoint ("core.front"); both must go.
	refsRemoved := func(v interface{}) bool {
		s, _ := v.(string)
		if removed[s] {
			return true
		}
		if i := strings.IndexByte(s, '.'); i > 0 {
			return removed[s[:i]]
		}
		return false
	}
	conns := FindConnectors(mustParse(src))
	var refs []int
	for i, c := range conns {
		if m, ok := c.(map[string]interface{}); ok {
			if refsRemoved(m["from"]) || refsRemoved(m["to"]) {
				refs = append(refs, i)
			}
		}
	}
	out := src
	for i := len(refs) - 1; i >= 0; i-- {
		if cs, ce, ok := ConnectorItemRange(out, refs[i]); ok {
			out = DeleteLineRange(out, cs, ce)
		}
	}
	// Annotations anchored to any removed part would dangle too.
	if anns, ok := mustParse(src)["annotations"].([]interface{}); ok {
		var aref []int
		for i, a := range anns {
			if m, ok := a.(map[string]interface{}); ok {
				if s, _ := m["anchor"].(string); removed[s] {
					aref = append(aref, i)
				}
			}
		}
		for i := len(aref) - 1; i >= 0; i-- {
			if as := findListItemLine(out, "annotations", aref[i]); as >= 0 {
				out = DeleteLineRange(out, as, itemEnd(strings.Split(out, "\n"), as))
			}
		}
	}
	// Other parts positioned relative to any removed part fall back to layout.
	for rid := range removed {
		out = stripPlaceReferencing(out, rid)
	}
	s, e, ok = findPartItemRange(out, id)
	if !ok {
		return src, false
	}
	return DeleteLineRange(out, s, e), true
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

// AddPart appends a default rectangle node to the scene's (shallowest)
// parts: list. In an auto-layout scene it joins the layout; otherwise it
// lands near the origin and the user drags it.
func AddPart(src string) (string, bool) {
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

// stripInlineOffset removes an inline `offset: { ... }` (and a dangling comma)
// from a part's flow/line text, so a reparented node is positioned by its new
// parent instead of a stale offset in the old frame.
func stripInlineOffset(s string) string {
	s = regexp.MustCompile(`offset:\s*\{[^}]*\}\s*,?\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`,\s*offset:\s*\{[^}]*\}`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "{ , ", "{ ")
	s = strings.ReplaceAll(s, "{,", "{")
	return s
}

// MovePart relocates a part's item block into another parent's parts: list,
// re-indenting it and dropping its now-stale offset so the new parent places
// it. targetParentID == "" moves the part to the SCENE-ROOT parts: block. It is
// a no-op (false) when the part isn't found, the target group has no parts:
// block to find/seed, or the move is degenerate (into itself).
func MovePart(src, id, targetParentID string) (string, bool) {
	if id == targetParentID {
		return src, false
	}
	lines := strings.Split(src, "\n")
	s, e, ok := findPartItemRange(src, id)
	if !ok {
		return src, false
	}
	item := stripInlineOffset(strings.Join(lines[s:e], "\n"))
	srcIndent := indentOf(lines[s])

	// Remove the item from its current location first.
	without := append(append([]string{}, lines[:s]...), lines[e:]...)

	// Find the destination parts: block and the indent its items sit at.
	partsLine, partsIndent := -1, 0
	if targetParentID == "" {
		// scene root = the shallowest parts: key
		re := regexp.MustCompile(`^( *)parts:\s*$`)
		for i, l := range without {
			if m := re.FindStringSubmatch(l); m != nil {
				if partsLine < 0 || len(m[1]) < partsIndent {
					partsLine, partsIndent = i, len(m[1])
				}
			}
		}
	} else {
		// the target group's OWN nested parts: (first parts: after its id line,
		// before the next line at ≤ the group's indent)
		gLine := FindPartIDLine(strings.Join(without, "\n"), targetParentID)
		if gLine < 0 {
			return src, false
		}
		gIndent := indentOf(without[gLine])
		reParts := regexp.MustCompile(`^( *)parts:\s*$`)
		for i := gLine + 1; i < len(without); i++ {
			if strings.TrimSpace(without[i]) == "" {
				continue
			}
			if indentOf(without[i]) <= gIndent && !strings.HasPrefix(strings.TrimSpace(without[i]), "-") {
				break // left the group block
			}
			if m := reParts.FindStringSubmatch(without[i]); m != nil && len(m[1]) > gIndent {
				partsLine, partsIndent = i, len(m[1])
				break
			}
		}
		if partsLine < 0 {
			// seed a parts: block right after the group's id line
			partsIndent = gIndent + 2
			header := strings.Repeat(" ", partsIndent) + "parts:"
			without = append(without[:gLine+1], append([]string{header}, without[gLine+1:]...)...)
			partsLine = gLine + 1
		}
	}
	if partsLine < 0 {
		return src, false
	}

	// Insert as the FIRST child, right after the parts: header — robust against
	// the block's deeper nested items (scanning for "the last sibling" would be
	// pulled into a grandchild's indent). Arrangement within parts: is order-
	// independent; the new parent's layout places it.
	itemIndent := partsIndent + 2
	end := partsLine + 1

	// Re-indent the extracted item lines by the indent delta.
	delta := itemIndent - srcIndent
	reindented := make([]string, 0)
	for _, l := range strings.Split(item, "\n") {
		if strings.TrimSpace(l) == "" {
			reindented = append(reindented, l)
			continue
		}
		if delta >= 0 {
			reindented = append(reindented, strings.Repeat(" ", delta)+l)
		} else {
			trim := -delta
			if trim > len(l)-len(strings.TrimLeft(l, " ")) {
				trim = len(l) - len(strings.TrimLeft(l, " "))
			}
			reindented = append(reindented, l[trim:])
		}
	}

	out := append(append(append([]string{}, without[:end]...), reindented...), without[end:]...)
	return strings.Join(out, "\n"), true
}

// FreezeGroupLayoutText drops the `layout:` from a single named group's block
// (inline or block form) so its children render from explicit offsets once one
// is dragged. Other groups and the scene root are untouched.
func FreezeGroupLayoutText(src, groupID string) string {
	lines := strings.Split(src, "\n")
	gLine := FindPartIDLine(src, groupID)
	if gLine < 0 {
		return src
	}
	gIndent := indentOf(lines[gLine])
	end := len(lines)
	for i := gLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= gIndent {
			end = i
			break
		}
	}
	layoutRe := regexp.MustCompile(`^ *layout:`)
	out := append([]string{}, lines[:gLine+1]...)
	for i := gLine + 1; i < end; i++ {
		if layoutRe.MatchString(lines[i]) {
			if strings.Contains(lines[i], "{") {
				continue // inline → drop the one line
			}
			ki := indentOf(lines[i]) // block → drop line + its deeper body
			j := i + 1
			for j < end && (strings.TrimSpace(lines[j]) == "" || indentOf(lines[j]) > ki) {
				j++
			}
			i = j - 1
			continue
		}
		out = append(out, lines[i])
	}
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// DuplicatePart clones a node's block under a fresh id, placed at (ox,oy).
func DuplicatePart(src, id string, ox, oy float64) (string, bool) {
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
	out, _ = UpsertInlineKey(out, FindPartIDLine(out, newID), "offset", ox, oy, 0)
	return out, true
}

// AddConnector appends a new connector (from → to, orthogonal routing) to the
// first connectors: block found in the scene node. If no connectors: key exists
// yet, one is inserted just before the first `parts:` key at the same indent.
//
// fromAnchor and toAnchor are optional anchor names (e.g. "right", "left").
// When non-empty they are appended to the id with a dot: "partID.anchorName".
// Empty strings produce the bare id, preserving the original behaviour.
func AddConnector(src, from, to, fromAnchor, toAnchor string) (string, bool) {
	if fromAnchor != "" {
		from = from + "." + fromAnchor
	}
	if toAnchor != "" {
		to = to + "." + toAnchor
	}
	lines := strings.Split(src, "\n")

	// Find the first `connectors:` block and its indent.
	connLine, connIndent := -1, 0
	reConn := regexp.MustCompile(`^( *)connectors:\s*$`)
	for i, l := range lines {
		if m := reConn.FindStringSubmatch(l); m != nil {
			connLine, connIndent = i, len(m[1])
			break
		}
	}

	itemLine := -1 // line AFTER which we insert the new item
	var itemIndent string

	if connLine >= 0 {
		// Walk to end of connectors block to find the insertion point.
		itemIndent = strings.Repeat(" ", connIndent+2)
		end := len(lines)
		for i := connLine + 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "" {
				continue
			}
			if indentOf(lines[i]) <= connIndent {
				end = i
				break
			}
			end = i + 1
		}
		itemLine = end - 1
	} else {
		// No connectors: key yet — find the shallowest `parts:` key and insert
		// a connectors: block just before it (same indent level).
		reParts := regexp.MustCompile(`^( *)parts:\s*$`)
		partsLine, partsIndent := -1, 1<<30
		for i, l := range lines {
			if m := reParts.FindStringSubmatch(l); m != nil {
				if len(m[1]) < partsIndent {
					partsLine, partsIndent = i, len(m[1])
				}
			}
		}
		if partsLine < 0 {
			return src, false
		}
		connIndent = partsIndent
		itemIndent = strings.Repeat(" ", connIndent+2)
		header := strings.Repeat(" ", connIndent) + "connectors:"
		newLines := append(append([]string{}, lines[:partsLine]...),
			header,
		)
		newLines = append(newLines, lines[partsLine:]...)
		lines = newLines
		itemLine = partsLine // insert after the new connectors: header
	}

	nl := itemIndent + fmt.Sprintf(`- { from: %s, to: %s, routing: orthogonal }`, from, to)
	out := append(append(append([]string{}, lines[:itemLine+1]...), nl), lines[itemLine+1:]...)
	return strings.Join(out, "\n"), true
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
