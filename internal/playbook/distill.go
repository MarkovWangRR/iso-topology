package playbook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MarkovWangRR/iso-topology/internal/llm"
	yaml "gopkg.in/yaml.v3"
)

// distill.go — Flow B: reverse-distil a design manual from a source image.
// extract(vision) → synthesize(deterministic) → [render→judge→refine]×iters.
// The renderer is the forward model; the vision judge is the loss; we search
// manual params whose rendered exemplar best matches the source design language.

// Extracted is the design vocabulary read from a source landing page.
type Extracted struct {
	Accent     string   `json:"accent"`
	Surface    string   `json:"surface"`
	SurfaceAlt string   `json:"surfaceAlt"`
	Ink        string   `json:"ink"`
	Muted      string   `json:"muted"`
	Line       string   `json:"line"`
	Background string   `json:"background"`
	Radius          float64  `json:"radius"`
	Mood            []string `json:"mood"`
	Domain          string   `json:"domain"`
	Icon            string   `json:"icon"`
	Energy          string   `json:"energy"`
	Corners         string   `json:"corners"`         // sharp | rounded | pill
	Depth           string   `json:"depth"`           // flat | subtle | deep
	EdgeStyle       string   `json:"edgeStyle"`       // solid | dashed | dotted | flow
	Glossiness      string   `json:"glossiness"`      // matte | satin | glossy
	AccentSecondary string   `json:"accentSecondary"` // a second brand/gradient colour
}

const extractPrompt = `You are a design-system analyst. Study this company's landing page and extract its visual design LANGUAGE — not just colours, but the *feel*. Be precise about the ACCENT (the exact colour of primary buttons/links/highlights, NOT a generic blue) and capture a SECONDARY accent if gradients are used. Return ONLY a JSON object, no prose, no markdown fences:
{"accent":"#hex exact primary brand colour","accentSecondary":"#hex second gradient/brand colour, or repeat accent","surface":"#hex card background","surfaceAlt":"#hex secondary surface","ink":"#hex primary text","muted":"#hex secondary text","line":"#hex border/divider","background":"#hex page background","radius": <card corner radius px 0-28>,"mood":["3-4 adjectives: clean, dark, playful, technical, bold, elegant, neon, retro..."],"domain":"data-platform|ai|cloud|devtools|fintech|generic","icon":"brand or mono","energy":"calm | balanced | vivid","corners":"sharp | rounded | pill","depth":"flat | subtle | deep (how much shadow/3D the UI uses)","edgeStyle":"solid | dashed | dotted | flow (line/connector style the brand feels like)","glossiness":"matte | satin | glossy (how much gloss/gradient sheen on surfaces)"}`

// Extract reads design tokens from an image via the vision model.
func Extract(c *llm.Client, imgPath string) (*Extracted, error) {
	out, err := c.Vision(extractPrompt, imgPath)
	if err != nil {
		return nil, err
	}
	js := llm.ExtractJSON(out)
	if js == "" {
		return nil, fmt.Errorf("extract: no JSON in reply: %.140s", out)
	}
	var ex Extracted
	if err := json.Unmarshal([]byte(js), &ex); err != nil {
		return nil, fmt.Errorf("extract: bad JSON: %w\n%s", err, js)
	}
	return &ex, nil
}

// Synthesize maps extracted tokens into a full Manual deterministically, reusing
// the proven lustre material constants. (Only the perceptual extraction is LLM;
// the schema assembly is mechanical and reliable.)
func Synthesize(name string, ex *Extracted) *Manual {
	accent := orHex(ex.Accent, "#3B82F6")
	bg := orHex(ex.Background, "#F4F6F8")
	dark := isDark(bg)

	var tok map[string]string
	var icon, shadowColor string
	if dark {
		// dark theme: cards must sit ABOVE the bg in lightness, ink/icons go light.
		card := orHex(ex.Surface, "")
		if card == "" || !isDark(card) || lightnessOf(card)-lightnessOf(bg) < 0.06 {
			card = shade(bg, 0.12) // lift the card off the page so it reads
		}
		if l := lightnessOf(card); l < 0.18 { // floor so dark cards stay legible
			card = shade(card, 0.18-l)
		}
		tok = map[string]string{
			"surface":    card,
			"surfaceAlt": shade(card, 0.05),
			"accent":     accent,
			"accentInk":  pickInk(accent),
			"ink":        "#EAF0F7",
			"muted":      "#9AA6B6",
			"line":       shade(bg, 0.18),
		}
		icon = "white"
		shadowColor = "#00000066"
	} else {
		tok = map[string]string{
			"surface":    orHex(ex.Surface, "#FFFFFF"),
			"surfaceAlt": orHex(ex.SurfaceAlt, "#F5F7FA"),
			"accent":     accent,
			"accentInk":  pickInk(accent),
			"ink":        orHex(ex.Ink, "#1A1D21"),
			"muted":      orHex(ex.Muted, "#6B7280"),
			"line":       orHex(ex.Line, "#E3E6EA"),
		}
		icon = "brand"
		if strings.HasPrefix(strings.ToLower(ex.Icon), "mono") {
			icon = "mono-ink"
		}
		shadowColor = "#8A95A83A"
	}
	// ── style AXES → distinct geometry/material/edges (this is what breaks the
	// "every style looks the same" monotony — feel varies, not just hue) ──
	// Axes derived primarily from MOOD (the vision model returns rich, reliable
	// adjectives) with a wide spread, so styles differ in FEEL not just hue.
	corners, depth, gloss, edgeStyle, energy := deriveAxes(ex)
	accent2 := orHex(ex.AccentSecondary, shade(accent, 0.18))

	// corners → radius
	radius := ex.Radius
	switch corners {
	case "sharp":
		radius = 5
	case "pill":
		radius = 26
	case "rounded":
		if radius <= 0 {
			radius = 16 // keep the site's own extracted radius when it has one
		}
	}
	if radius <= 0 {
		radius = 14
	}
	if radius > 28 {
		radius = 28
	}
	// depth → side-face contrast + AO + shadow (wide spread)
	litDL, shadeDL, ao, shDy, shBlur := -0.05, -0.13, 0.05, 16.0, 22.0
	switch depth {
	case "flat":
		litDL, shadeDL, ao, shDy, shBlur = -0.03, -0.07, 0.03, 7, 11
	case "deep":
		litDL, shadeDL, ao, shDy, shBlur = -0.08, -0.20, 0.08, 28, 40
	}
	// glossiness → top-face sheen
	sheen := 0.05
	switch gloss {
	case "matte":
		sheen = 0.0
	case "glossy":
		sheen = 0.14
	}
	if energy == "vivid" {
		sheen += 0.04
		ao += 0.02
	}
	// edgeStyle → connector dash/width
	dash, ew := "1 6", 2.0
	switch edgeStyle {
	case "solid":
		dash, ew = "", 1.6
	case "dashed":
		dash, ew = "7 5", 1.8
	case "flow":
		dash, ew = "1 9", 2.4
	}

	return &Manual{
		Name:     name,
		Tokens:   withSecondary(tok, accent2),
		Material: Material{Space: "hsl", Light: "topRight", TopDL: 0, LitSideDL: litDL, ShadeSideDL: shadeDL, AoDL: ao, FaceSplitOnRounded: true, TopSheen: sheen, Energy: energy},
		Geometry: Geometry{Radius: radius, Shadow: Shadow{Dy: shDy, Blur: shBlur, Color: shadowColor}},
		Roles:    rolesFor(energy),
		Edge:     Edge{Color: accent, Width: ew, Dash: dash, GradientTo: "accent2"},
		Canvas:   CanvasSpec{Background: bg, Grid: "iso", GridColor: tok["line"]},
		Icon:     IconSpec{Treatment: icon},
	}
}

func withSecondary(tok map[string]string, accent2 string) map[string]string {
	tok["accent2"] = accent2
	return tok
}

// ResynthFromCache re-runs Synthesize on a style's cached extracted.json (no
// LLM call) and rewrites manual.yaml — lets us iterate on synthesis/kernel for
// free across rounds. Returns an error if no cache exists.
func ResynthFromCache(root, style string) (*Manual, error) {
	b, err := os.ReadFile(filepath.Join(root, style, "distill", "extracted.json"))
	if err != nil {
		return nil, err
	}
	var ex Extracted
	if err := json.Unmarshal(b, &ex); err != nil {
		return nil, err
	}
	m := Synthesize(style, &ex)
	mb, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	return m, os.WriteFile(filepath.Join(root, style, "manual.yaml"), mb, 0o644)
}

// CachedStyles lists styles that have a cached extracted.json (re-synthesizable).
func CachedStyles(root string) []string {
	var out []string
	for _, s := range listStyles(root) {
		if _, err := os.Stat(filepath.Join(root, s, "distill", "extracted.json")); err == nil {
			out = append(out, s)
		}
	}
	return out
}

// deriveAxes resolves the five style axes. It trusts the model's explicit axis
// field when given, else infers from the mood adjectives (which the model returns
// reliably) with a deliberately WIDE spread so brands diverge in feel.
func deriveAxes(ex *Extracted) (corners, depth, gloss, edge, energy string) {
	m := strings.ToLower(strings.Join(ex.Mood, " ") + " " + ex.Domain)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(m, w) {
				return true
			}
		}
		return false
	}
	pick := func(explicit string, valid ...string) string {
		e := strings.ToLower(strings.TrimSpace(explicit))
		for _, v := range valid {
			if e == v {
				return e
			}
		}
		return ""
	}

	// measurable facts (reliable) — break ties the mood adjectives can't
	sat := satOf(orHex(ex.Accent, "#3B82F6"))
	dark := isDark(orHex(ex.Background, "#FFFFFF"))

	energy = pick(ex.Energy, "calm", "vivid")
	if energy == "" {
		switch {
		case has("neon", "vibrant", "playful", "bold", "colorful", "colourful", "retro", "fun"), sat > 0.78 && dark:
			energy = "vivid"
		case has("minimal", "elegant", "refined", "monochrome", "mono", "understated", "subtle"), sat < 0.32:
			energy = "calm"
		default:
			energy = "balanced"
		}
	}
	corners = pick(ex.Corners, "sharp", "pill", "rounded")
	if corners == "" {
		switch {
		case has("playful", "friendly", "soft", "fun", "rounded", "approachable", "organic"):
			corners = "pill"
		case has("technical", "precise", "brutalist", "engineered", "mono", "serious", "enterprise"):
			corners = "sharp"
		default:
			corners = "rounded"
		}
	}
	depth = pick(ex.Depth, "flat", "deep")
	if depth == "" {
		switch {
		case has("3d", "depth", "immersive", "layered", "dimensional"), dark && sat > 0.6:
			depth = "deep"
		case has("flat", "minimal", "clean", "crisp", "elegant", "mono"):
			depth = "flat"
		default:
			depth = "subtle"
		}
	}
	gloss = pick(ex.Glossiness, "matte", "glossy")
	if gloss == "" {
		switch {
		case has("glossy", "gradient", "shiny", "glow"), energy == "vivid" && dark:
			gloss = "glossy"
		case has("matte", "minimal", "flat", "mono", "paper"), energy == "calm":
			gloss = "matte"
		default:
			gloss = "satin"
		}
	}
	edge = pick(ex.EdgeStyle, "solid", "dashed", "dotted", "flow")
	if edge == "" {
		switch {
		case has("technical", "data", "engineered", "pipeline", "precise"):
			edge = "dotted"
		case has("minimal", "elegant", "clean", "mono"):
			edge = "solid"
		case energy == "vivid":
			edge = "flow"
		default:
			edge = "dashed"
		}
	}
	return
}

// rolesFor varies role treatments by energy so calm brands read minimal/flat and
// vivid brands read glowing — same roles, different intensity.
func rolesFor(energy string) map[string]Role {
	hero := Role{Base: "accent", Ink: "accentInk", Glow: "accent", Elevation: "lg", Sheen: true, Outline: true, Radius: 22}
	accent := Role{Base: "accent", Ink: "accentInk", Sheen: true}
	store := Role{Base: "surfaceAlt", Ink: "ink", Sheen: true}
	switch energy {
	case "calm":
		hero = Role{Base: "accent", Ink: "accentInk", Elevation: "md", Outline: true} // no glow, flatter
		accent = Role{Base: "accent", Ink: "accentInk"}
		store = Role{Base: "surfaceAlt", Ink: "ink"}
	case "vivid":
		accent.Ring = true
		hero.Glow = "accent"
	default: // balanced
		accent.Ring = true
	}
	return map[string]Role{
		"hero":    hero,
		"surface": {Base: "surface", Ink: "ink"},
		"source":  {Base: "surface", Ink: "ink"},
		"sink":    {Base: "surface", Ink: "ink"},
		"store":   store,
		"gateway": {Base: "surface", Ink: "ink"},
		"group":   {Base: "surface", Ink: "muted"},
		"accent":  accent,
	}
}

// Verdict is the judge's scored comparison + concrete corrections.
type Verdict struct {
	Score    float64 `json:"score"`
	Accent   string  `json:"accent"`
	Radius   float64 `json:"radius"`
	Critique string  `json:"critique"`
}

const judgePrompt = `Image 1 is a reference brand's landing page. Image 2 is an isometric architecture diagram we generated, trying to match that brand's DESIGN LANGUAGE (colour palette, accent colour, card style, corner radius, overall mood) — ignore the actual content/topology, judge only the visual language. Return ONLY JSON, no prose:
{"score": <0-100 how well image 2 captures image 1's design language>,"accent":"#hex — the corrected primary brand colour if image 2's accent is off, otherwise repeat image 1's accent","radius": <corrected card corner radius px if off, else current>,"critique":"one short sentence: the single most important thing to fix"}`

// Judge compares the rendered preview against the source via the vision model.
func Judge(c *llm.Client, previewPNG, sourceImg string) (*Verdict, error) {
	out, err := c.Vision(judgePrompt, sourceImg, previewPNG)
	if err != nil {
		return nil, err
	}
	js := llm.ExtractJSON(out)
	if js == "" {
		return nil, fmt.Errorf("judge: no JSON: %.140s", out)
	}
	var v Verdict
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		return nil, fmt.Errorf("judge: bad JSON: %w\n%s", err, js)
	}
	return &v, nil
}

// Refine applies the judge's concrete corrections to the manual.
func Refine(m *Manual, v *Verdict) {
	if isHexLike(v.Accent) {
		m.Tokens["accent"] = v.Accent
		m.Tokens["accentInk"] = pickInk(v.Accent)
		m.Edge.Color = v.Accent
	}
	if v.Radius > 0 && v.Radius <= 28 {
		m.Geometry.Radius = v.Radius
	}
}

// ── orchestration ────────────────────────────────────────────────────────────

type DistillOpts struct {
	Root, Style, SourceImg string
	Iters                  int
	Target                 float64
}

type IterLog struct {
	Iter     int     `json:"iter"`
	Score    float64 `json:"score"`
	Accent   string  `json:"accent"`
	Radius   float64 `json:"radius"`
	Critique string  `json:"critique"`
}

type DistillResult struct {
	Manual     *Manual
	Extracted  *Extracted
	FinalScore float64
	Iterations []IterLog
}

// RunDistill executes the full loop. render compiles+renders the exemplar to SVG;
// rasterize turns that SVG into a PNG the judge can see (both injected by the CLI
// layer, which owns the renderer and headless Chrome).
func RunDistill(c *llm.Client, o DistillOpts, render func([]byte) (string, error), rasterize func(svg, outPNG string) error) (*DistillResult, error) {
	if !c.Available() {
		return nil, fmt.Errorf("no LLM API key (set OPENAI_API_KEY)")
	}
	ex, err := Extract(c, o.SourceImg)
	if err != nil {
		return nil, err
	}
	m := Synthesize(o.Style, ex)
	res := &DistillResult{Manual: m, Extracted: ex}

	exemplar, err := os.ReadFile(filepath.Join(o.Root, "_exemplar.yaml"))
	if err != nil {
		return res, err
	}
	tmpPNG := filepath.Join(os.TempDir(), "pb_distill_preview.png")
	for i := 0; i <= o.Iters; i++ {
		yml, err := Apply(exemplar, m)
		if err != nil {
			return res, err
		}
		svg, err := render(yml)
		if err != nil {
			return res, err
		}
		if err := rasterize(svg, tmpPNG); err != nil {
			return res, err
		}
		v, err := Judge(c, tmpPNG, o.SourceImg)
		if err != nil {
			return res, err
		}
		res.Iterations = append(res.Iterations, IterLog{i, v.Score, v.Accent, v.Radius, v.Critique})
		res.FinalScore = v.Score
		if v.Score >= o.Target || i == o.Iters {
			break
		}
		Refine(m, v)
	}
	return res, nil
}

// SaveDistilled writes the style bundle (manual/meta/preview/distill) and is the
// gate: auto-distilled styles are marked trust=auto with confidence=score/100.
func SaveDistilled(root string, res *DistillResult, sourceImg, previewSVG string) error {
	dir := filepath.Join(root, res.Manual.Name)
	_ = os.MkdirAll(filepath.Join(dir, "distill"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "preview"), 0o755)

	mb, err := yaml.Marshal(res.Manual)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manual.yaml"), mb, 0o644); err != nil {
		return err
	}

	ex := res.Extracted
	meta := Meta{
		Name: res.Manual.Name, Title: titleCase(res.Manual.Name) + " — distilled",
		Tags: ex.Mood, Domain: orStr(ex.Domain, "generic"), Mood: ex.Mood,
		Palette: []string{res.Manual.Tokens["accent"], res.Manual.Tokens["surface"], res.Manual.Tokens["ink"]},
		Provenance: []Provenance{{
			Source:   filepath.Base(sourceImg),
			License:  "style-reference — derived parameters only (colours/radii/shadow); no assets or logos copied",
			Captured: "auto-distilled",
		}},
		Trust: "auto", Confidence: res.FinalScore / 100, Version: 1,
	}
	metab, _ := yaml.Marshal(meta)
	if err := os.WriteFile(filepath.Join(dir, "meta.yaml"), metab, 0o644); err != nil {
		return err
	}
	if previewSVG != "" {
		_ = os.WriteFile(filepath.Join(dir, "preview", "exemplar.svg"), []byte(previewSVG), 0o644)
	}

	// distillation log + report
	var jl strings.Builder
	for _, it := range res.Iterations {
		b, _ := json.Marshal(it)
		jl.Write(b)
		jl.WriteByte('\n')
	}
	_ = os.WriteFile(filepath.Join(dir, "distill", "iterations.jsonl"), []byte(jl.String()), 0o644)
	if exb, e := json.MarshalIndent(res.Extracted, "", "  "); e == nil {
		_ = os.WriteFile(filepath.Join(dir, "distill", "extracted.json"), exb, 0o644) // cache for fast re-synthesis
	}

	var rep strings.Builder
	fmt.Fprintf(&rep, "# Distillation report — %s\n\nSource: `%s`\nFinal score: **%.0f/100** (trust=auto)\n\n| iter | score | accent | radius | critique |\n|---|---|---|---|---|\n",
		res.Manual.Name, filepath.Base(sourceImg), res.FinalScore)
	for _, it := range res.Iterations {
		fmt.Fprintf(&rep, "| %d | %.0f | %s | %.0f | %s |\n", it.Iter, it.Score, it.Accent, it.Radius, it.Critique)
	}
	return os.WriteFile(filepath.Join(dir, "distill", "report.md"), []byte(rep.String()), 0o644)
}

// ── small helpers ────────────────────────────────────────────────────────────

func orHex(v, def string) string {
	if isHexLike(v) {
		return strings.ToUpper(v)
	}
	return def
}
func orStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
func pickInk(accent string) string {
	if isDark(accent) {
		return "#FFFFFF"
	}
	return "#202124"
}
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
