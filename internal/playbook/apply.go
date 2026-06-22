package playbook

import (
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Apply compiles a structural diagram (isotopo-shaped YAML whose parts carry a
// `role:` and no styling) onto a manual, emitting plain isotopo YAML — only the
// existing palette/effects/stroke/text fields, no role/token/material. Output is
// deterministic (yaml.v3 sorts map keys), so it can be golden-tested.
func Apply(structure []byte, m *Manual) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(structure, &doc); err != nil {
		return nil, err
	}
	applyCanvas(doc, m)
	if nodes, ok := doc["nodes"].(map[string]any); ok {
		for _, n := range nodes {
			scene, ok := n.(map[string]any)
			if !ok {
				continue
			}
			if parts, ok := scene["parts"].([]any); ok {
				styleParts(parts, m)
			}
			if conns, ok := scene["connectors"].([]any); ok {
				styleConnectors(conns, m)
			}
		}
	}
	return yaml.Marshal(doc)
}

func applyCanvas(doc map[string]any, m *Manual) {
	cv, ok := doc["canvas"].(map[string]any)
	if !ok {
		cv = map[string]any{}
		doc["canvas"] = cv
	}
	setIfEmpty(cv, "background", m.Canvas.Background)
	setIfEmpty(cv, "grid", m.Canvas.Grid)
	setIfEmpty(cv, "gridColor", m.Canvas.GridColor)
	if _, has := cv["gridStep"]; !has {
		cv["gridStep"] = 40
	}
}

func styleParts(parts []any, m *Manual) {
	for _, p := range parts {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if role, ok := part["role"].(string); ok {
			stylePart(part, role, m)
			delete(part, "role")
		}
		if nested, ok := part["parts"].([]any); ok {
			styleParts(nested, m)
		}
	}
}

func stylePart(part map[string]any, roleName string, m *Manual) {
	role, ok := m.Roles[roleName]
	if !ok {
		role = m.Roles["surface"]
	}
	base := m.token(role.Base)
	radius := m.Geometry.Radius
	var style map[string]any

	if roleName == "group" {
		if _, has := part["shape"]; !has {
			part["shape"] = "group" // role: group implies the group shape
		}
		style = map[string]any{
			"palette": map[string]any{"top": base},
			"stroke":  map[string]any{"color": mix(m.token("line"), m.token("accent"), 0.35), "width": 1.1, "dash": "4 3"},
			"text":    map[string]any{"color": m.token(orDefault(role.Ink, "muted"))},
			"effects": map[string]any{"cornerRadius": radius, "opacity": 0.9}, // translucent tray
		}
	} else {
		f := DeriveFaces(base, m.Material)
		r := radius
		if role.Radius > 0 {
			r = role.Radius
		}
		eff := map[string]any{
			"cornerRadius": r,
			"dropShadow":   shadowMap(m.Geometry.Shadow, role.Elevation),
		}
		if m.Material.FaceSplitOnRounded && r > 0 {
			eff["faceSplit"] = true
		}
		// backglow halo — explicit glow token or a ring flag (accent/hero pop)
		if role.Glow != "" || role.Ring {
			gc := m.token(orDefault(role.Glow, "accent"))
			rad, op := 52.0, 0.42
			if role.Elevation == "lg" {
				rad, op = 66, 0.5
			}
			eff["backglow"] = map[string]any{"color": gc, "radius": rad, "opacity": op}
		}
		// silhouette outline accent (emphasis)
		if role.Outline {
			eff["outline"] = map[string]any{"color": m.token("accent"), "width": 2.0, "opacity": 0.9}
		}
		if role.Opacity > 0 && role.Opacity < 1 {
			eff["opacity"] = role.Opacity
		}
		// top face: a sheen gradient (livelier than flat); sheen roles get a
		// stronger accent-lit top.
		pal := map[string]any{
			"leftGradient":  map[string]any{"from": f.LeftFrom, "to": f.LeftTo, "dir": "down"},
			"rightGradient": map[string]any{"from": f.RightFrom, "to": f.RightTo, "dir": "down"},
		}
		sheen := m.Material.TopSheen
		if role.Sheen && sheen < 0.08 {
			sheen = 0.12
		}
		if sheen > 0 {
			// faint brand tint in the sheen so the accent permeates every face
			from := mix(shade(f.Top, sheen), m.token("accent"), 0.06)
			pal["topGradient"] = map[string]any{"from": from, "to": shade(f.Top, -sheen*0.45), "dir": "down"}
		} else {
			pal["top"] = f.Top
		}
		style = map[string]any{
			"palette": pal,
			"stroke":  map[string]any{"color": borderColor(m, base), "width": 1},
			"text":    map[string]any{"color": m.token(orDefault(role.Ink, "ink"))},
			"effects": eff,
		}
	}
	mergeStyle(part, style)

	if icon, ok := part["icon"].(string); ok {
		part["icon"] = tintIcon(icon, m, role)
	}
	if role.Shape != "" {
		if _, has := part["shape"]; !has {
			part["shape"] = role.Shape
		}
	}
}

func styleConnectors(conns []any, m *Manual) {
	for _, c := range conns {
		conn, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if _, has := conn["stroke"]; !has {
			color := m.token(m.Edge.Color)
			st := map[string]any{"color": color, "width": m.Edge.Width}
			if m.Edge.Dash != "" {
				st["dash"] = m.Edge.Dash
			}
			// edge gradient — explicit gradientTo, else auto-brighten the tail
			to := shade(color, 0.18)
			if m.Edge.GradientTo != "" {
				to = m.token(m.Edge.GradientTo)
			}
			st["gradient"] = map[string]any{"from": color, "to": to}
			conn["stroke"] = st
		}
		if _, has := conn["routing"]; !has {
			conn["routing"] = "orthogonal"
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func borderColor(m *Manual, base string) string {
	line := m.token("line")
	if line == "" {
		line = shade(base, -0.08)
	}
	if acc, ok := m.Tokens["accent"]; ok {
		return mix(line, acc, 0.28) // thread the brand colour through every edge
	}
	return line
}

func shadowMap(s Shadow, elevation string) map[string]any {
	mul := 1.0
	switch elevation {
	case "sm":
		mul = 0.6
	case "lg":
		mul = 1.5
	}
	return map[string]any{"dx": s.Dx, "dy": s.Dy * mul, "blur": s.Blur * mul, "color": s.Color}
}

// mergeStyle fills only the style keys the author hasn't set — explicit
// per-node style is the escape hatch and always wins.
func mergeStyle(part map[string]any, skin map[string]any) {
	existing, ok := part["style"].(map[string]any)
	if !ok {
		part["style"] = skin
		return
	}
	for k, v := range skin {
		if _, has := existing[k]; !has {
			existing[k] = v
		}
	}
}

func tintIcon(icon string, m *Manual, role Role) string {
	if !strings.HasPrefix(icon, "iso://") {
		return icon // data:/http/local file — leave alone
	}
	base := stripIconVariant(icon)
	switch m.Icon.Treatment {
	case "white":
		return base + "/light"
	case "mono-ink":
		return base + "/" + hexNoHash(m.token(orDefault(role.Ink, "ink")))
	case "mono-accent":
		return base + "/" + hexNoHash(m.token("accent"))
	default: // brand — keep as authored
		return icon
	}
}

func stripIconVariant(icon string) string {
	rest := strings.TrimPrefix(icon, "iso://")
	parts := strings.Split(rest, "/")
	if len(parts) >= 2 {
		return "iso://" + parts[0] + "/" + parts[1]
	}
	return icon
}

func hexNoHash(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) > 6 {
		s = s[:6] // drop any alpha for icon tints
	}
	return s
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func setIfEmpty(mp map[string]any, key, val string) {
	if val == "" {
		return
	}
	if _, has := mp[key]; !has {
		mp[key] = val
	}
}

// sortedKeys is a determinism helper for any callers that need stable iteration.
func sortedKeys(mp map[string]any) []string {
	ks := make([]string, 0, len(mp))
	for k := range mp {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
