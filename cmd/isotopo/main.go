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
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	isotopo "github.com/MarkovWangRR/iso-topology"
	"github.com/MarkovWangRR/iso-topology/internal/yamledit"
	"gopkg.in/yaml.v3"
)

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
// shapeOptions is DERIVED from the capability report (the single source of
// truth for what the renderer accepts), so adding a shape to the engine makes
// it appear in the Studio picker automatically — no hand-maintained list to
// drift. Only `composite` (the scene container, never a node choice) is
// dropped, and iso_text is shown under its friendlier `text` alias.
func shapeOptions() []string {
	var out []string
	for _, s := range isotopo.CapabilityReport().Shapes {
		switch s.IsoName {
		case "composite":
			// scene container only — not a per-node shape
		case "iso_text":
			out = append(out, "text")
		default:
			out = append(out, s.IsoName)
		}
	}
	return out
}

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
		{Group: "Content", Path: "shape", Label: "Shape", Desc: "Geometric form of the node", Type: "choice", Options: shapeOptions()},
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
	// Effects — visual polish previously only reachable by hand-editing YAML.
	// Applies to the solid shapes (faces + fill); text/outline shapes have no
	// volume to glow or shadow. cornerRadius rounds box edges only.
	switch shapeClass(shape) {
	case "faces", "fill":
		f = append(f,
			schemaField{Group: "Effects", Path: "style.effects.opacity", Label: "Opacity", Desc: "0–1, whole-part transparency", Type: "number", Inline: true},
			schemaField{Group: "Effects", Path: "style.effects.blur", Label: "Blur", Desc: "Gaussian blur in px — fog/ghost nodes", Type: "number", Inline: true},
			schemaField{Group: "Effects", Path: "style.effects.backglow.color", Label: "Glow color", Desc: "Soft halo behind the part", Type: "color", Inline: true},
			schemaField{Group: "Effects", Path: "style.effects.backglow.radius", Label: "Glow radius", Type: "number", Inline: true},
			schemaField{Group: "Effects", Path: "style.effects.dropShadow.color", Label: "Shadow color", Desc: "Soft drop shadow under the silhouette", Type: "color", Inline: true},
			schemaField{Group: "Effects", Path: "style.effects.dropShadow.blur", Label: "Shadow blur", Type: "number", Inline: true},
		)
		if shapeClass(shape) == "faces" {
			f = append(f, schemaField{Group: "Effects", Path: "style.effects.cornerRadius", Label: "Corner radius", Desc: "Rounds the box's vertical edges", Type: "number", Inline: true})
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
		subtree = yamledit.FindNodeMap(root, id)
		shape, _ := subtree["shape"].(string) // nil subtree → "" → face schema; guarded below
		fields = nodeSchema(shape)
	case "edge":
		conns := yamledit.FindConnectors(root)
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
		fields[i].Value = yamledit.ReadPath(subtree, fields[i].Path)
	}
	return fields, true
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
	if sourceLang != "d2" {
		issues = append(issues, isotopo.UnknownKeyIssues(data)...)
	}
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
		if lang != "d2" {
			issues = append(issues, isotopo.UnknownKeyIssues(data)...)
		}
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
		// snap>0 rounds the dragged node's final offset to a grid step.
		snapStep := atof(q.Get("snap"))
		snap := func(v float64) float64 {
			if snapStep > 0 {
				return math.Round(v/snapStep) * snapStep
			}
			return v
		}
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
				out = yamledit.FreezeLayoutText(src)
				ids := make([]string, 0, len(offs))
				for id := range offs {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				for _, id := range ids {
					o := offs[id]
					wx, wy, wz := o[0], o[1], o[2]
					if id == q.Get("id") {
						wx = snap(wx + dwx)
						wy = snap(wy + dwy)
					}
					out, ok = yamledit.UpsertInlineKey(out, yamledit.FindPartIDLine(out, id), "offset", wx, wy, wz)
				}
			} else {
				cx, cy, cz, found := isotopo.ResolvePartOffset(doc, q.Get("id"))
				if !found {
					http.Error(w, "part not found", 422)
					return
				}
				out, ok = yamledit.UpsertInlineKey(src, yamledit.FindPartIDLine(src, q.Get("id")), "offset", snap(cx+dwx), snap(cy+dwy), cz)
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
				out, ok = yamledit.UpsertInlineList(src, yamledit.FindConnectorLine(src, ci), "waypoints", raw)
			} else {
				bx, by := isotopo.ConnectorBend(doc, ci)
				out, ok = yamledit.UpsertInlineKey(src, yamledit.FindConnectorLine(src, ci), "bend", bx+dwx, by+dwy, 0)
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
			return yamledit.FindPartIDLine(src, q.Get("id")), true
		case "edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			return yamledit.FindConnectorLine(src, ci), true
		case "canvas":
			return yamledit.FindCanvasLine(src), true
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
		if q.Get("kind") == "canvas" && yamledit.FindCanvasLine(out) < 0 {
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
			out, _ = yamledit.SetField(out, line, strings.Split(key, "."), val)
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
			out, ok = yamledit.AddPart(src)
		case "delete:node":
			out, ok = yamledit.DeletePart(src, q.Get("id"))
		case "delete:edge":
			ci, _ := strconv.Atoi(q.Get("ci"))
			if s, e, found := yamledit.ConnectorItemRange(src, ci); found {
				out, ok = yamledit.DeleteLineRange(src, s, e), true
			}
		case "duplicate:node":
			ox, oy := 40.0, 40.0
			if doc, derr := loadDocument(lang, body); derr == nil {
				if cx, cy, _, found := isotopo.ResolvePartOffset(doc, q.Get("id")); found {
					ox, oy = cx+40, cy+40
				}
			}
			out, ok = yamledit.DuplicatePart(src, q.Get("id"), ox, oy)
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
