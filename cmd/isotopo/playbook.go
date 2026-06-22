package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	isotopo "github.com/MarkovWangRR/iso-topology"
	"github.com/MarkovWangRR/iso-topology/internal/playbook"
	yaml "gopkg.in/yaml.v3"
)

// playbookCmd dispatches `isotopo playbook <sub> …` — the design-asset flywheel.
func playbookCmd(args []string) {
	root := "samples/playbook"
	if r := os.Getenv("PLAYBOOK_ROOT"); r != "" {
		root = r
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook <lint|apply|render|index|search|list|distill> …")
		os.Exit(2)
	}
	switch args[0] {
	case "lint":
		pbLint(root, args[1:])
	case "apply":
		pbApply(root, args[1:])
	case "render":
		pbRender(root, args[1:])
	case "index":
		pbIndex(root)
	case "search":
		pbSearch(root, args[1:])
	case "list":
		pbList(root)
	case "distill":
		pbDistill(root, args[1:])
	case "resynth":
		pbResynth(root, args[1:])
	case "critique":
		pbCritique(args[1:])
	case "style":
		pbStyle(root, args[1:])
	default:
		fmt.Fprintln(os.Stderr, "unknown playbook subcommand:", args[0])
		os.Exit(2)
	}
}

func pbLint(root string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook lint <style>")
		os.Exit(2)
	}
	issues := playbook.Lint(root, args[0])
	if len(issues) == 0 {
		fmt.Printf("ok: %s — 0 issues\n", args[0])
		return
	}
	for _, i := range issues {
		fmt.Fprintln(os.Stderr, "  "+i)
	}
	os.Exit(1)
}

// applyStyle reads a style + a structure file (default the shared exemplar) and
// returns the compiled isotopo YAML.
func applyStyle(root, style, structurePath string) ([]byte, error) {
	m, err := playbook.LoadManual(root, style)
	if err != nil {
		return nil, err
	}
	if structurePath == "" {
		structurePath = filepath.Join(root, "_exemplar.yaml")
	}
	structure, err := os.ReadFile(structurePath)
	if err != nil {
		return nil, err
	}
	return playbook.Apply(structure, m)
}

func pbApply(root string, args []string) {
	style, structure, out := "", "", ""
	rest := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			out = args[i+1]
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	if len(rest) >= 1 {
		style = rest[0]
	}
	if len(rest) >= 2 {
		structure = rest[1]
	}
	if style == "" {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook apply <style> [structure.yaml] [-o out.yaml]")
		os.Exit(2)
	}
	yml, err := applyStyle(root, style, structure)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply:", err)
		os.Exit(1)
	}
	if out != "" {
		if err := os.WriteFile(out, yml, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("wrote", out)
		return
	}
	os.Stdout.Write(yml)
}

func renderYAML(yml []byte) (string, error) {
	doc, err := loadDocument("yaml", yml)
	if err != nil {
		return "", err
	}
	for _, iss := range isotopo.Validate(doc) {
		if iss.Severity == isotopo.SeverityError {
			return "", fmt.Errorf("validate: %s", iss.Message)
		}
	}
	return renderTopologySVG(doc), nil
}

func pbRender(root string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook render <style> [structure.yaml]")
		os.Exit(2)
	}
	style := args[0]
	structure := ""
	if len(args) >= 2 {
		structure = args[1]
	}
	yml, err := applyStyle(root, style, structure)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply:", err)
		os.Exit(1)
	}
	svg, err := renderYAML(yml)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
	dir := filepath.Join(root, style, "preview")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "exemplar.svg"), []byte(svg), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// swatch sheet — one box per universal role
	if sw, err := renderSwatches(root, style); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "swatches.svg"), []byte(sw), 0o644)
	}
	fmt.Printf("ok: %s → %s/exemplar.svg (+ swatches.svg)\n", style, dir)
}

// renderSwatches builds a tiny structure (one card per role) and applies the
// style — a visual contact-sheet of how each role looks.
func renderSwatches(root, style string) (string, error) {
	onto := playbook.LoadRoleOntology(root)
	roles := make([]string, 0, len(onto))
	for r := range onto {
		roles = append(roles, r)
	}
	// stable order
	order := []string{"hero", "surface", "source", "sink", "store", "gateway", "group", "accent"}
	parts := []map[string]any{}
	for _, r := range order {
		if _, ok := onto[r]; !ok {
			continue
		}
		parts = append(parts, map[string]any{
			"id": "sw_" + r, "role": r, "shape": "rectangle",
			"geom":  map[string]any{"w": 120, "d": 90, "h": 26},
			"label": r,
		})
	}
	doc := map[string]any{
		"nodes": map[string]any{
			"scene": map[string]any{
				"shape":  "composite",
				"layout": map[string]any{"mode": "grid", "cols": 4, "gap": 1},
				"parts":  parts,
			},
		},
	}
	structure, _ := yaml.Marshal(doc)
	m, err := playbook.LoadManual(root, style)
	if err != nil {
		return "", err
	}
	yml, err := playbook.Apply(structure, m)
	if err != nil {
		return "", err
	}
	return renderYAML(yml)
}

func pbIndex(root string) {
	n, err := playbook.WriteIndex(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "index:", err)
		os.Exit(1)
	}
	fmt.Printf("ok: wrote %s/INDEX.json (%d styles)\n", root, n)
}

func pbSearch(root string, args []string) {
	facets := map[string]string{}
	var terms []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--facet" && i+1 < len(args) {
			kv := strings.SplitN(args[i+1], "=", 2)
			if len(kv) == 2 {
				facets[kv[0]] = kv[1]
			}
			i++
		} else {
			terms = append(terms, args[i])
		}
	}
	hits, err := playbook.Search(root, strings.Join(terms, " "), facets)
	if err != nil {
		fmt.Fprintln(os.Stderr, "search:", err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(hits, "", "  ")
	fmt.Println(string(b))
}

func pbList(root string) {
	idx, _ := playbook.BuildIndex(root)
	for _, e := range idx {
		fmt.Printf("%-14s [%s] %s — %s\n", e.Style, e.Trust, e.Title, e.Why)
	}
}

// pbStyle — the node-style retrieval/emission tool:
//
//	isotopo playbook style suggest "<fuzzy request>"   → ranked families (JSON)
//	isotopo playbook style show <family>               → full family doc + DSL template
func pbStyle(root string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook style <suggest \"<query>\"|show <family>>")
		os.Exit(2)
	}
	switch args[0] {
	case "suggest":
		hits, fallback, err := playbook.SuggestStyle(root, strings.Join(args[1:], " "))
		if err != nil {
			fmt.Fprintln(os.Stderr, "style suggest:", err)
			os.Exit(1)
		}
		out := map[string]any{"query": strings.Join(args[1:], " "), "fallback": fallback, "matches": hits}
		if len(hits) > 0 {
			out["pick"] = hits[0].Slug
			out["next"] = fmt.Sprintf("isotopo playbook style show %s", hits[0].Slug)
		} else {
			out["pick"] = fallback
			out["next"] = fmt.Sprintf("isotopo playbook style show %s", fallback)
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: isotopo playbook style show <family>")
			os.Exit(2)
		}
		doc, err := playbook.ShowStyle(root, args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "style show:", err)
			os.Exit(1)
		}
		fmt.Print(doc)
		fmt.Println("\n---\nApply across the whole topology: see samples/playbook/APPLY_TO_TOPOLOGY.md")
	default:
		fmt.Fprintln(os.Stderr, "unknown: playbook style", args[0])
		os.Exit(2)
	}
}

// pbResynth re-runs Synthesize on cached extracted tokens (no LLM) and re-renders
// previews — instant iteration on the synthesis/kernel across rounds.
func pbResynth(root string, args []string) {
	styles := args
	if len(styles) == 0 {
		styles = playbook.CachedStyles(root)
	}
	for _, s := range styles {
		if _, err := playbook.ResynthFromCache(root, s); err != nil {
			fmt.Fprintf(os.Stderr, "  resynth %s: %v\n", s, err)
			continue
		}
		if yml, e := applyStyle(root, s, ""); e == nil {
			if svg, e2 := renderYAML(yml); e2 == nil {
				dir := filepath.Join(root, s, "preview")
				_ = os.MkdirAll(dir, 0o755)
				_ = os.WriteFile(filepath.Join(dir, "exemplar.svg"), []byte(svg), 0o644)
			}
		}
		fmt.Printf("  resynth %s ✓\n", s)
	}
	_, _ = playbook.WriteIndex(root)
	fmt.Println("ok: resynth done")
}
