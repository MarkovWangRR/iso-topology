// Brand-icon resolver: turn an "iso://brand/<name>" URI into a
// self-contained data-URI SVG so authors can write
//
//	icon: "iso://brand/spark"
//
// in the DSL without having to host an external image. The output is a
// minimalist stand-in (rounded badge + monogram + brand-coloured stroke),
// NOT a reproduction of the official logo — the goal is to communicate
// "this node represents Spark" at a glance without any copyright concern.
package iso25d

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

// brandBadge is the spec for one generated badge.
type brandBadge struct {
	glyph string // 1–3 chars rendered in the centre
	color string // accent — used for both stroke and glyph
}

// brandBadges keeps the table tiny and editable. Add an entry → it's
// immediately addressable via iso://brand/<key> from any DSL Icon field.
var brandBadges = map[string]brandBadge{
	"spark":      {"S", "#E25A2C"},
	"hadoop":     {"H", "#65B832"},
	"mysql":      {"M", "#00618A"},
	"postgresql": {"P", "#336791"},
	"postgres":   {"P", "#336791"},
	"iceberg":    {"I", "#2486C0"},
	"hive":       {"Hv", "#D4B106"},
	"pulsar":     {"P", "#D63341"},
	"kafka":      {"K", "#231F20"},
	"redis":      {"R", "#DC382D"},
	"mongo":      {"M", "#4FAA41"},
	"kubernetes": {"K8", "#326CE5"},
	"docker":     {"D", "#2496ED"},
	"github":     {"G", "#181717"},
	"aws":        {"AW", "#FF9900"},
	"gcp":        {"GC", "#4285F4"},
	"azure":      {"Az", "#0078D4"},
	"starrocks":  {"⚡", "#F59E0B"},
	"vite":       {"V", "#646CFF"},
	"rolldown":   {"R", "#C2410C"},
	"oxc":        {"OX", "#6B7280"},
}

// glyphIcons is the generic (non-brand) icon family — hand-authored
// minimalist stroke icons in the feather/lucide visual idiom, but
// original paths (no third-party assets, no licensing concerns).
// Addressable as:
//
//	iso://glyph/<name>           ink on transparent (for light tops)
//	iso://glyph/<name>/light     white (for dark tops)
//	iso://glyph/<name>/<RRGGBB>  any color
//
// Each value is the inner markup of a 24×24 viewBox; stroke color is
// injected at the <svg> root (elements override locally when filled).
var glyphIcons = map[string]string{
	"cloud":    `<path d="M7 18h10a4 4 0 0 0 .8-7.9A6 6 0 0 0 6.2 9.5 4.5 4.5 0 0 0 7 18z"/>`,
	"database": `<ellipse cx="12" cy="6" rx="7" ry="2.6"/><path d="M5 6v12c0 1.5 3.1 2.6 7 2.6s7-1.1 7-2.6V6"/><path d="M5 12c0 1.5 3.1 2.6 7 2.6s7-1.1 7-2.6"/>`,
	"bolt":     `<path d="M13 2 5 14h6l-1 8 8-12h-6z" fill="currentColor" stroke="none"/>`,
	"chart":    `<path d="M5 20v-8M12 20V6m7 14v-9" stroke-width="2.4"/><path d="M3.5 20h17"/>`,
	"globe":    `<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><ellipse cx="12" cy="12" rx="4.2" ry="9"/>`,
	"shield":   `<path d="M12 2l8 3v6c0 5-3.4 8.6-8 11-4.6-2.4-8-6-8-11V5z"/>`,
	"lock":     `<rect x="5.5" y="11" width="13" height="9" rx="2"/><path d="M8.5 11V8a3.5 3.5 0 0 1 7 0v3"/>`,
	"gear":     `<path d="M12 2.8l7.4 4.3v9.8L12 21.2l-7.4-4.3V7.1z"/><circle cx="12" cy="12" r="3.2"/>`,
	"cpu":      `<rect x="7" y="7" width="10" height="10" rx="1.5"/><rect x="10.4" y="10.4" width="3.2" height="3.2"/><path d="M9.5 7V4M14.5 7V4M9.5 20v-3M14.5 20v-3M7 9.5H4M7 14.5H4M20 9.5h-3M20 14.5h-3"/>`,
	"code":     `<path d="m9 8-4 4 4 4M15 8l4 4-4 4"/>`,
	"layers":   `<path d="M12 3l9 5-9 5-9-5z"/><path d="M3 13.5l9 5 9-5"/>`,
	"rocket":   `<path d="M12 2.5c2.8 2.2 4 5.5 4 8.8l2.8 3.7h-3.6L12 18.2 8.8 15H5.2L8 11.3c0-3.3 1.2-6.6 4-8.8z"/><circle cx="12" cy="9" r="1.6"/>`,
	"user":     `<circle cx="12" cy="8" r="3.6"/><path d="M5 20a7 7 0 0 1 14 0"/>`,
	"mobile":   `<rect x="7" y="2.5" width="10" height="19" rx="2.4"/><path d="M11 18.2h2"/>`,
	"browser":  `<rect x="3" y="4.5" width="18" height="15" rx="2"/><path d="M3 9h18M6 6.8h.01M8.5 6.8h.01"/>`,
	"search":   `<circle cx="10.8" cy="10.8" r="6"/><path d="m15.3 15.3 5.2 5.2"/>`,
	"bell":     `<path d="M18 9.5a6 6 0 1 0-12 0c0 6-2.5 7-2.5 7h17s-2.5-1-2.5-7"/><path d="M10.3 20a2 2 0 0 0 3.4 0"/>`,
	"queue":    `<path d="M4 7h16M4 12h16M4 17h10"/>`,

	// ── AI / big-data domain (v2.3) ──────────────────────────────────
	"model":     `<circle cx="5" cy="12" r="1.9"/><circle cx="12" cy="5.5" r="1.9"/><circle cx="12" cy="18.5" r="1.9"/><circle cx="19" cy="12" r="1.9"/><path d="m6.5 10.8 4-4.2M6.5 13.2l4 4.2M13.5 6.6l4 4.2M13.5 17.4l4-4.2"/>`,
	"sparkles":  `<path d="M12 4.5 13.7 9l4.5 1.7-4.5 1.7L12 16.9l-1.7-4.5L5.8 10.7 10.3 9z" fill="currentColor" stroke="none"/><path d="M18.7 15.6l.9 2.3 2.3.9-2.3.9-.9 2.3-.9-2.3-2.3-.9 2.3-.9z" fill="currentColor" stroke="none"/>`,
	"gpu":       `<rect x="4.5" y="7" width="15" height="10" rx="1.5"/><path d="M8 17v2.5M12 17v2.5M16 17v2.5M4.5 10H2.5"/><circle cx="10" cy="12" r="2.2"/><path d="M15 9.8v4.4"/>`,
	"agent":     `<rect x="6" y="8" width="12" height="9.5" rx="2.4"/><path d="M12 8V5.2"/><circle cx="12" cy="3.8" r="1.3"/><path d="M9.5 11.8v1.4M14.5 11.8v1.4M9.8 15h4.4"/>`,
	"chat":      `<path d="M4 7a2.6 2.6 0 0 1 2.6-2.6h10.8A2.6 2.6 0 0 1 20 7v6a2.6 2.6 0 0 1-2.6 2.6H10l-4.4 3.9v-3.9h.99A2.6 2.6 0 0 1 4 13z"/><path d="M8.4 10h7.2"/>`,
	"vector":    `<path d="M5 4.5v15h15"/><path d="m8.5 16 8.5-8.5M13.5 7.5H17V11"/>`,
	"stream":    `<path d="M3 7.5c3 0 3 2.4 6 2.4s3-2.4 6-2.4 3 2.4 6 2.4M3 14.1c3 0 3 2.4 6 2.4s3-2.4 6-2.4 3 2.4 6 2.4"/>`,
	"warehouse": `<path d="M3.5 19.5V9.4L12 4.5l8.5 4.9v10.1"/><path d="M7.5 19.5v-6.2h9v6.2M7.5 16.4h9"/>`,
	"etl":       `<path d="M4.5 4.5h15l-5.6 7.4v6.6l-3.8-2.4v-4.2z"/>`,
	"table":     `<rect x="4" y="5" width="16" height="14" rx="1.6"/><path d="M4 10h16M10.5 5v14"/>`,
	"graph":     `<circle cx="6" cy="6" r="2"/><circle cx="18" cy="8" r="2"/><circle cx="8" cy="18" r="2"/><circle cx="17" cy="16.5" r="2"/><path d="m8 6.4 8 1.3M6.5 8l1.2 8M9.9 17.6l5.2-.7M17.6 10l-.4 4.5"/>`,
	"gauge":     `<path d="M4.5 15.5a7.5 7.5 0 1 1 15 0"/><path d="M12 15.5 16 11"/><path d="M4 19.5h16"/>`,
	"terminal":  `<rect x="3.5" y="4.5" width="17" height="15" rx="2"/><path d="m7 9.5 3 2.8-3 2.8M12.8 15.1h4.2"/>`,
	"doc":       `<path d="M6.5 3.5h7.5l3.5 3.5v13.5h-11z"/><path d="M14 3.5V7h3.5M9.5 12h5M9.5 15.4h5"/>`,
	"key":       `<circle cx="8" cy="15.5" r="3.6"/><path d="m10.7 12.8 8.3-8.3M15.1 8.4l2.4 2.4M18 5.5l2 2"/>`,
	"training":  `<path d="M4 5c.4 7.6 4.3 12.7 16 13.9"/><circle cx="8.6" cy="10.5" r="1.5"/><circle cx="14.8" cy="16.1" r="1.5"/>`,
	"lake":      `<path d="M12 3.8c3.2 3.7 5.3 6.4 5.3 9a5.3 5.3 0 0 1-10.6 0c0-2.6 2.1-5.3 5.3-9z"/>`,
}

// ResolveBrandIcon turns "iso://brand/<name>" and "iso://glyph/<name>"
// URIs into data-URI SVGs. Anything else (https://, data:, "", …) is
// returned unchanged, so callers can pass any Icon value through this
// function unconditionally.
func ResolveBrandIcon(uri string) string {
	const (
		brandPrefix = "iso://brand/"
		glyphPrefix = "iso://glyph/"
	)
	switch {
	case strings.HasPrefix(uri, brandPrefix):
		name := strings.ToLower(strings.TrimPrefix(uri, brandPrefix))
		b, ok := brandBadges[name]
		if !ok {
			return uri
		}
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(brandSVG(b)))

	case strings.HasPrefix(uri, glyphPrefix):
		rest := strings.TrimPrefix(uri, glyphPrefix)
		name, variant, _ := strings.Cut(rest, "/")
		inner, ok := glyphIcons[strings.ToLower(name)]
		if !ok {
			return uri
		}
		color := "#1F2937" // ink — for light top faces
		switch {
		case variant == "" :
		case strings.EqualFold(variant, "light"):
			color = "#FAFAFA"
		case isHexColor(variant):
			color = "#" + variant
		}
		_ = inner
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(glyphSVG(name, color)))
	}
	return uri
}

func isHexColor(s string) bool {
	if len(s) != 6 && len(s) != 3 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// glyphDescs is the human/agent-facing one-liner for every glyph —
// the generated icon index (docs/agent/ICONS.md) is built from it,
// and a unit test enforces that no glyph ships without one.
var glyphDescs = map[string]string{
	"cloud":     "cloud — external system, SaaS, hosted service",
	"database":  "database cylinder — storage, persistence",
	"bolt":      "lightning bolt (filled) — functions, speed, compute",
	"chart":     "bar chart — analytics, metrics, BI",
	"globe":     "globe with meridians — CDN, edge, world-facing",
	"shield":    "shield — security, WAF, guardrails",
	"lock":      "padlock — auth, secrets, encryption",
	"gear":      "hex nut with bore — settings, infrastructure, ops",
	"cpu":       "chip with pins — processor, hardware, runtime",
	"code":      "angle brackets — code, SDK, developer surface",
	"layers":    "two stacked rhombi — cache layers, tiers, stack",
	"rocket":    "rocket with porthole — launch, deploy, startup",
	"user":      "head and shoulders — human actor, account",
	"mobile":    "phone outline — mobile client",
	"browser":   "window with toolbar dots — web client, frontend",
	"search":    "magnifier — search, retrieval, lookup",
	"bell":      "bell with clapper — notifications, alerts",
	"queue":     "three horizontal bars — queue, log, buffer",
	"model":     "four connected nodes — neural net, ML model",
	"sparkles":  "four-point stars (filled) — GenAI, magic, LLM",
	"gpu":       "wide chip with pins and core — GPU, accelerator",
	"agent":     "robot face with antenna — AI agent, bot",
	"chat":      "speech bubble with line — chat, LLM gateway, conversation",
	"vector":    "axes with rising arrow — embeddings, vector store",
	"stream":    "double wave — streaming, realtime data",
	"warehouse": "gabled building — data warehouse",
	"etl":       "funnel — ETL, filtering, transformation",
	"table":     "grid with header — dataset, table, spreadsheet",
	"graph":     "four linked nodes — knowledge graph, network",
	"gauge":     "dial with needle — observability, dashboards, SLO",
	"terminal":  "window with prompt — CLI, shell, developer tool",
	"doc":       "page with folded corner — documents, files",
	"key":       "key on ring — API keys, credentials, access",
	"training":  "descending curve with two points — model training, gradient descent",
	"lake":      "water drop — data lake",
}

// IconInfo is one entry of the built-in icon catalog.
type IconInfo struct {
	Kind        string // "glyph" | "brand"
	Name        string
	URI         string // what authors write in the DSL
	Description string
	SVG         string // standalone renderable SVG markup
}

// IconCatalog returns every built-in icon, sorted glyphs-then-brands
// by name. The generated docs/agent/ICONS.md and the per-icon SVG
// files under docs/assets/icons/ are derived from this — the single
// source of truth stays in this file.
func IconCatalog() []IconInfo {
	var out []IconInfo
	names := make([]string, 0, len(glyphIcons))
	for n := range glyphIcons {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, IconInfo{
			Kind: "glyph", Name: n,
			URI:         "iso://glyph/" + n,
			Description: glyphDescs[n],
			SVG:         glyphSVG(n, "#1F2937"),
		})
	}
	names = names[:0]
	for n := range brandBadges {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		b := brandBadges[n]
		out = append(out, IconInfo{
			Kind: "brand", Name: n,
			URI:         "iso://brand/" + n,
			Description: fmt.Sprintf("letter badge %q in brand color %s", b.glyph, b.color),
			SVG:         brandSVG(b),
		})
	}
	return out
}

// glyphSVG builds the standalone SVG for one glyph at the given stroke
// color (which also drives fill="currentColor" overrides inside the
// markup).
func glyphSVG(name, color string) string {
	inner := glyphIcons[strings.ToLower(name)]
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" `+
			`fill="none" stroke="%s" color="%s" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">%s</svg>`,
		color, color, inner,
	)
}

// brandSVG builds the standalone SVG for one letter badge.
func brandSVG(b brandBadge) string {
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">`+
			`<rect x="2" y="2" width="20" height="20" rx="5" fill="#FFFFFF" stroke="%s" stroke-width="1.6"/>`+
			`<text x="12" y="16.5" text-anchor="middle" font-size="11" font-weight="800" fill="%s" `+
			`font-family="-apple-system, Inter, system-ui, sans-serif">%s</text>`+
			`</svg>`,
		b.color, b.color, b.glyph,
	)
}
