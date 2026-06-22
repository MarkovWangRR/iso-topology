// Package playbook is the design-asset flywheel: a registry of design "manuals"
// (one per visual style) that the engine compiles onto a structural diagram, so
// agents author STRUCTURE + ROLES and the style is applied deterministically.
//
// Nothing here touches the isotopo DSL/renderer: Apply emits plain isotopo YAML
// (palette/effects/stroke/text only). Roles/tokens/material live only at this
// layer. See docs/design/playbook-flywheel-plan.md.
package playbook

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

// Manual is the executable design handbook for one style.
type Manual struct {
	Name     string            `yaml:"name" json:"name"`
	Extends  string            `yaml:"extends,omitempty" json:"extends,omitempty"`
	Tokens   map[string]string `yaml:"tokens" json:"tokens"`
	Material Material          `yaml:"material" json:"material"`
	Geometry Geometry          `yaml:"geometry" json:"geometry"`
	Roles    map[string]Role   `yaml:"roles" json:"roles"`
	Edge     Edge              `yaml:"edge" json:"edge"`
	Canvas   CanvasSpec        `yaml:"canvas" json:"canvas"`
	Icon     IconSpec          `yaml:"icon" json:"icon"`
}

// Material — the lighting kernel parameters (the "tuned taste").
type Material struct {
	Space              string  `yaml:"space" json:"space"`   // hsl (oklch reserved)
	Light              string  `yaml:"light" json:"light"`   // topLeft | topRight | top
	TopDL              float64 `yaml:"topDL" json:"topDL"`   // lightness deltas (−1..1)
	LitSideDL          float64 `yaml:"litSideDL" json:"litSideDL"`
	ShadeSideDL        float64 `yaml:"shadeSideDL" json:"shadeSideDL"`
	AoDL               float64 `yaml:"aoDL" json:"aoDL"` // per-face vertical falloff
	FaceSplitOnRounded bool    `yaml:"faceSplitOnRounded" json:"faceSplitOnRounded"`
	TopSheen           float64 `yaml:"topSheen,omitempty" json:"topSheen,omitempty"` // top-face gradient sheen (0=flat top)
	Energy             string  `yaml:"energy,omitempty" json:"energy,omitempty"`     // calm | balanced | vivid
}

type Geometry struct {
	Radius float64 `yaml:"radius" json:"radius"`
	Shadow Shadow  `yaml:"shadow" json:"shadow"`
}

type Shadow struct {
	Dx    float64 `yaml:"dx" json:"dx"`
	Dy    float64 `yaml:"dy" json:"dy"`
	Blur  float64 `yaml:"blur" json:"blur"`
	Color string  `yaml:"color" json:"color"`
}

// Role maps a semantic role to a look. base/ink/glow reference token names
// (or raw hex). elevation: sm|md|lg scales the shadow.
type Role struct {
	Base      string  `yaml:"base" json:"base"`
	Ink       string  `yaml:"ink,omitempty" json:"ink,omitempty"`
	Glow      string  `yaml:"glow,omitempty" json:"glow,omitempty"`
	Elevation string  `yaml:"elevation,omitempty" json:"elevation,omitempty"`
	Shape     string  `yaml:"shape,omitempty" json:"shape,omitempty"`
	Radius    float64 `yaml:"radius,omitempty" json:"radius,omitempty"`   // per-role corner radius override
	Sheen     bool    `yaml:"sheen,omitempty" json:"sheen,omitempty"`     // strong top-face gradient sheen
	Ring      bool    `yaml:"ring,omitempty" json:"ring,omitempty"`       // backglow halo even without glow token
	Outline   bool    `yaml:"outline,omitempty" json:"outline,omitempty"` // silhouette accent stroke
	Opacity   float64 `yaml:"opacity,omitempty" json:"opacity,omitempty"` // 0..1 (ghost effect)
}

type Edge struct {
	Color      string  `yaml:"color" json:"color"`
	Width      float64 `yaml:"width" json:"width"`
	Dash       string  `yaml:"dash,omitempty" json:"dash,omitempty"`
	GradientTo string  `yaml:"gradientTo,omitempty" json:"gradientTo,omitempty"` // token → edge linear gradient
}

type CanvasSpec struct {
	Background string `yaml:"background" json:"background"`
	Grid       string `yaml:"grid,omitempty" json:"grid,omitempty"`
	GridColor  string `yaml:"gridColor,omitempty" json:"gridColor,omitempty"`
}

type IconSpec struct {
	Treatment string `yaml:"treatment" json:"treatment"` // brand|mono-ink|mono-accent|white
}

// Meta is the index/retrieval metadata for a style.
type Meta struct {
	Name       string       `yaml:"name" json:"name"`
	Title      string       `yaml:"title" json:"title"`
	Tags       []string     `yaml:"tags" json:"tags"`
	Domain     string       `yaml:"domain" json:"domain"`
	Mood       []string     `yaml:"mood" json:"mood"`
	Palette    []string     `yaml:"palette" json:"palette"`
	Provenance []Provenance `yaml:"provenance" json:"provenance"`
	Trust      string       `yaml:"trust" json:"trust"`           // blessed | auto
	Confidence float64      `yaml:"confidence" json:"confidence"` // 0..1
	Version    int          `yaml:"version" json:"version"`
}

type Provenance struct {
	Source   string `yaml:"source" json:"source"`
	Asset    string `yaml:"asset,omitempty" json:"asset,omitempty"`
	Captured string `yaml:"captured,omitempty" json:"captured,omitempty"`
	License  string `yaml:"license" json:"license"`
}

// LoadManual reads and resolves a style's manual.yaml (following `extends`).
func LoadManual(root, style string) (*Manual, error) {
	m, err := readManual(filepath.Join(root, style, "manual.yaml"))
	if err != nil {
		return nil, err
	}
	if m.Extends != "" {
		base, err := LoadManual(root, m.Extends)
		if err != nil {
			return nil, fmt.Errorf("extends %q: %w", m.Extends, err)
		}
		m = mergeManual(base, m)
	}
	return m, nil
}

func readManual(path string) (*Manual, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manual
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// mergeManual overlays child onto base (child wins per field; token/role maps merge).
func mergeManual(base, child *Manual) *Manual {
	out := *base
	out.Name, out.Extends = child.Name, ""
	if out.Tokens == nil {
		out.Tokens = map[string]string{}
	}
	merged := map[string]string{}
	for k, v := range base.Tokens {
		merged[k] = v
	}
	for k, v := range child.Tokens {
		merged[k] = v
	}
	out.Tokens = merged
	if len(child.Roles) > 0 {
		rs := map[string]Role{}
		for k, v := range base.Roles {
			rs[k] = v
		}
		for k, v := range child.Roles {
			rs[k] = v
		}
		out.Roles = rs
	}
	if child.Material.Space != "" {
		out.Material = child.Material
	}
	if child.Geometry.Radius != 0 {
		out.Geometry = child.Geometry
	}
	if child.Edge.Color != "" {
		out.Edge = child.Edge
	}
	if child.Canvas.Background != "" {
		out.Canvas = child.Canvas
	}
	if child.Icon.Treatment != "" {
		out.Icon = child.Icon
	}
	return &out
}

func LoadMeta(root, style string) (*Meta, error) {
	data, err := os.ReadFile(filepath.Join(root, style, "meta.yaml"))
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// token resolves a token name to a hex value; a raw hex passes through.
func (m *Manual) token(name string) string {
	if name == "" {
		return ""
	}
	if v, ok := m.Tokens[name]; ok {
		return v
	}
	return name // allow inline hex
}
