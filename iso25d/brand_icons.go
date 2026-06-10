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
		svg := fmt.Sprintf(
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">`+
				`<rect x="2" y="2" width="20" height="20" rx="5" fill="#FFFFFF" stroke="%s" stroke-width="1.6"/>`+
				`<text x="12" y="16.5" text-anchor="middle" font-size="11" font-weight="800" fill="%s" `+
				`font-family="-apple-system, Inter, system-ui, sans-serif">%s</text>`+
				`</svg>`,
			b.color, b.color, b.glyph,
		)
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))

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
		// color also drives fill="currentColor" overrides inside glyphs.
		svg := fmt.Sprintf(
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" `+
				`fill="none" stroke="%s" color="%s" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">%s</svg>`,
			color, color, inner,
		)
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
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
