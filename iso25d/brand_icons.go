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

// ResolveBrandIcon turns an "iso://brand/<name>" URI into a data-URI SVG
// of a minimalist badge. Anything else (https://, data:, "", …) is
// returned unchanged, so callers can pass any Icon value through this
// function unconditionally.
func ResolveBrandIcon(uri string) string {
	const prefix = "iso://brand/"
	if !strings.HasPrefix(uri, prefix) {
		return uri
	}
	name := strings.ToLower(strings.TrimPrefix(uri, prefix))
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
}
