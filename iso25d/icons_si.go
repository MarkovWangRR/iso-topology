// Simple Icons pack — real brand logos vendored as normalized SVG
// files under icons/si/ (CC0-1.0, see icons/si/NOTICE.txt) and
// embedded at compile time, so the single-binary / no-network
// promise holds. Addressable as:
//
//	iso://si/<slug>            ink, for light top faces
//	iso://si/<slug>/light      white, for dark top faces
//	iso://si/<slug>/<RRGGBB>   any color (e.g. the brand color)
//
// The vendored set is curated by tools/import-icons/allowlist.txt;
// run `go run ./tools/import-icons` after editing it.
package iso25d

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed icons/si/*.svg icons/si/_meta.json
var siFS embed.FS

var (
	siIcons  map[string]string // slug → svg markup (fill="currentColor")
	siTitles map[string]string // slug → upstream display title
)

func init() {
	siIcons = map[string]string{}
	siTitles = map[string]string{}
	entries, err := siFS.ReadDir("icons/si")
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".svg") {
			continue
		}
		data, err := siFS.ReadFile("icons/si/" + name)
		if err != nil {
			continue
		}
		siIcons[strings.TrimSuffix(name, ".svg")] = strings.TrimSpace(string(data))
	}
	if meta, err := siFS.ReadFile("icons/si/_meta.json"); err == nil {
		_ = json.Unmarshal(meta, &siTitles)
	}
}

// siSVG renders one vendored logo in the given color.
func siSVG(slug, color string) string {
	return strings.ReplaceAll(siIcons[slug], "currentColor", color)
}

func siTitle(slug string) string {
	if t := siTitles[slug]; t != "" {
		return t
	}
	return slug
}
