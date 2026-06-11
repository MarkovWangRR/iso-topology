// import-icons vendors a curated subset of Simple Icons
// (https://simpleicons.org, CC0-1.0) into iso25d/icons/si/ so the
// renderer can embed real brand logos offline.
//
//   - reads tools/import-icons/allowlist.txt (one slug per line)
//   - downloads each icon from jsDelivr at a PINNED package version
//     (deterministic re-runs; bump the version here deliberately)
//   - normalizes to our minimal form: 24×24 viewBox, single <path>
//     filled with currentColor (recolored at resolve time)
//   - writes iso25d/icons/si/<slug>.svg, _meta.json (slug → title,
//     extracted from the upstream <title>), and the CC0 NOTICE
//
// Missing or renamed slugs are reported at the end, not fatal —
// Simple Icons occasionally removes marks at trademark holders'
// request, and the allowlist should be pruned when that happens.
//
// Usage: go run ./tools/import-icons
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Pinned upstream version — bump deliberately and re-run.
var versions = []string{"15", "14", "13"}

const outDir = "iso25d/icons/si"

var (
	rePath  = regexp.MustCompile(`<path[^>]*\bd="([^"]+)"`)
	reTitle = regexp.MustCompile(`<title>([^<]+)</title>`)
)

func main() {
	raw, err := os.ReadFile("tools/import-icons/allowlist.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "allowlist:", err)
		os.Exit(1)
	}
	var slugs []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		slugs = append(slugs, line)
	}
	sort.Strings(slugs)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	titles := map[string]string{}
	var missing []string
	ok := 0
	for _, slug := range slugs {
		svg, err := fetch(client, slug)
		if err != nil {
			missing = append(missing, slug+" ("+err.Error()+")")
			continue
		}
		d := rePath.FindStringSubmatch(svg)
		if d == nil {
			missing = append(missing, slug+" (no path)")
			continue
		}
		title := slug
		if t := reTitle.FindStringSubmatch(svg); t != nil {
			title = t[1]
		}
		norm := fmt.Sprintf(
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24"><path d="%s" fill="currentColor"/></svg>`,
			d[1],
		)
		if err := os.WriteFile(filepath.Join(outDir, slug+".svg"), []byte(norm+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		titles[slug] = title
		ok++
	}

	meta, _ := json.MarshalIndent(titles, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "_meta.json"), append(meta, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	notice := `Icons in this directory are from Simple Icons
(https://simpleicons.org), released under CC0-1.0 (public domain):
https://github.com/simple-icons/simple-icons/blob/master/LICENSE.md

Brand names and logos may be trademarks of their respective owners.
Their inclusion here is for identification in architecture diagrams
and implies no endorsement by or affiliation with the trademark
holders. Vendored at a pinned upstream version by
tools/import-icons; do not edit these files by hand.
`
	if err := os.WriteFile(filepath.Join(outDir, "NOTICE.txt"), []byte(notice), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("imported %d icons into %s\n", ok, outDir)
	if len(missing) > 0 {
		fmt.Printf("missing (%d) — prune or fix the allowlist:\n", len(missing))
		for _, m := range missing {
			fmt.Println("  -", m)
		}
	}
}

func fetch(client *http.Client, slug string) (string, error) {
	var lastErr error
	for _, v := range versions {
		url := fmt.Sprintf("https://cdn.jsdelivr.net/npm/simple-icons@%s/icons/%s.svg", v, slug)
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == 200 && strings.Contains(string(body), "<svg") {
			return string(body), nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return "", lastErr
}
