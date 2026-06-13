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

// ── Detail editor: extract a node/edge's scalar fields and write them back,
// with human-friendly labels, for the Studio right-click "edit details" UI.

type editField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Value    string   `json:"value"`
	Type     string   `json:"type"`              // "text" | "select"
	Options  []string `json:"options,omitempty"` // for select
	ReadOnly bool     `json:"readonly,omitempty"`
}

// fieldLabels maps raw YAML keys to a semantic, human-readable label so the
// detail modal reads as plain language rather than the bare DSL keys.
var fieldLabels = map[string]string{
	"id": "标识 ID", "label": "名称 Label", "icon": "图标 Icon",
	"shape": "形状 Shape", "preset": "预设 Preset", "from": "起点 From",
	"to": "终点 To", "arrow": "箭头 Arrow", "routing": "走线方式 Routing",
	"labelBg": "标签底色 Label BG", "labelColor": "标签文字色 Label Color",
	"labelFontSize": "标签字号 Label Size", "elbow": "拐弯方向 Elbow",
	"grid": "网格 Grid", "color": "颜色 Color", "dash": "虚线 Dash",
	"width": "线宽 Width", "group": "分组 Group", "kind": "类型 Kind",
}

func fieldLabel(k string) string {
	if v, ok := fieldLabels[k]; ok {
		return v
	}
	return k
}

// fieldEnums lists the allowed values for keys rendered as a dropdown.
var fieldEnums = map[string][]string{
	"arrow":   {"none", "triangle"},
	"routing": {"orthogonal", "straight", "bezier"},
	"elbow":   {"xFirst", "yFirst"},
}

// readOnlyKeys can be shown but not edited (changing them breaks references).
var readOnlyKeys = map[string]bool{"id": true}

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

// scalarKV reports whether a "key: value" pair carries an editable scalar
// (non-empty, not the start of a nested map/list), returning (key, value).
func scalarKV(s string) (string, string, bool) {
	kv := strings.SplitN(s, ":", 2)
	if len(kv) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(kv[0])
	val := strings.TrimSpace(kv[1])
	if c := strings.Index(val, " #"); c >= 0 { // strip trailing comment
		val = strings.TrimSpace(val[:c])
	}
	if key == "" || val == "" || strings.HasPrefix(val, "{") || strings.HasPrefix(val, "[") {
		return "", "", false
	}
	return key, val, true
}

func mkField(key, val string) editField {
	f := editField{Key: key, Label: fieldLabel(key), Value: unquoteYAML(val), Type: "text", ReadOnly: readOnlyKeys[key]}
	if opts, ok := fieldEnums[key]; ok {
		f.Type = "select"
		f.Options = opts
	}
	return f
}

// extractFields returns the scalar key/value pairs actually present in the
// node/edge block at startLine (flow or block form), in source order — what
// the detail modal renders. Nested maps/lists are skipped.
func extractFields(src string, startLine int) []editField {
	lines := strings.Split(src, "\n")
	if startLine < 0 || startLine >= len(lines) {
		return nil
	}
	var fields []editField
	seen := map[string]bool{}
	add := func(k, v string) {
		if seen[k] {
			return
		}
		seen[k] = true
		fields = append(fields, mkField(k, v))
	}
	line := lines[startLine]
	// Flow form: `- { id: c, label: "X", ... }`
	if i := strings.Index(line, "{"); i >= 0 && strings.Contains(line[i:], "}") {
		inner := line[i+1:]
		if j := strings.LastIndex(inner, "}"); j >= 0 {
			inner = inner[:j]
		}
		for _, part := range splitTopCommas(inner) {
			if k, v, ok := scalarKV(part); ok {
				add(k, v)
			}
		}
		return fields
	}
	// Block form: the start line may itself hold `- key: value`.
	itemIndent := indentOf(lines[startLine])
	if k, v, ok := scalarKV(strings.TrimPrefix(strings.TrimSpace(line), "- ")); ok {
		add(k, v)
	}
	childIndent := itemIndent + 2
	for i := startLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		ind := indentOf(lines[i])
		if ind <= itemIndent {
			break
		}
		if ind != childIndent || strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
			continue
		}
		if k, v, ok := scalarKV(strings.TrimSpace(lines[i])); ok {
			add(k, v)
		}
	}
	return fields
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
		line, ok := targetLine(src, r.URL.Query())
		if !ok {
			http.Error(w, "kind must be node|edge", 400)
			return
		}
		if line < 0 {
			http.Error(w, "target not found in source", 422)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"fields": extractFields(src, line)})
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
		// Re-find the target after each edit: a write can shift line numbers.
		for key, val := range changes {
			if readOnlyKeys[key] {
				continue
			}
			line, ok := targetLine(out, q)
			if !ok {
				http.Error(w, "kind must be node|edge", 400)
				return
			}
			if line < 0 {
				http.Error(w, "target not found in source", 422)
				return
			}
			out, _ = upsertScalar(out, line, key, val)
		}
		lang := q.Get("format")
		if lang == "" {
			lang = sourceLang
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
