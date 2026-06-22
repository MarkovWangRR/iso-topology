package playbook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// ── role ontology (the universal vocabulary agents author against) ───────────

// LoadRoleOntology reads samples/playbook/_roles.yaml → {role: description}.
func LoadRoleOntology(root string) map[string]string {
	data, err := os.ReadFile(filepath.Join(root, "_roles.yaml"))
	if err != nil {
		return nil
	}
	var doc struct {
		Roles map[string]string `yaml:"roles"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return nil
	}
	return doc.Roles
}

// ── lint ─────────────────────────────────────────────────────────────────────

func isHexLike(s string) bool { return strings.HasPrefix(strings.TrimSpace(s), "#") }

// Lint validates one style bundle. Returns a list of human-readable issues
// (empty = clean).
func Lint(root, style string) []string {
	var issues []string
	m, err := LoadManual(root, style)
	if err != nil {
		return []string{"manual: " + err.Error()}
	}
	meta, err := LoadMeta(root, style)
	if err != nil {
		issues = append(issues, "meta: "+err.Error())
	}
	if m.Material.Space == "" {
		issues = append(issues, "material.space is empty (use hsl)")
	}
	for rn, r := range m.Roles {
		for label, ref := range map[string]string{"base": r.Base, "ink": r.Ink, "glow": r.Glow} {
			if ref == "" {
				continue
			}
			if _, ok := m.Tokens[ref]; !ok && !isHexLike(ref) {
				issues = append(issues, fmt.Sprintf("role %q %s references unknown token %q", rn, label, ref))
			}
		}
	}
	if onto := LoadRoleOntology(root); onto != nil {
		for rn := range m.Roles {
			if _, ok := onto[rn]; !ok {
				issues = append(issues, fmt.Sprintf("role %q is not in the _roles.yaml ontology", rn))
			}
		}
	}
	if meta != nil && meta.Trust != "blessed" && meta.Trust != "auto" {
		issues = append(issues, fmt.Sprintf("meta.trust %q must be blessed|auto", meta.Trust))
	}
	return issues
}

// ── index ────────────────────────────────────────────────────────────────────

// IndexEntry is one row of the agent-facing catalogue (INDEX.json). It carries
// exactly what an agent needs to act: which style, why, the role vocabulary it
// supports, a preview, and the ready apply command.
type IndexEntry struct {
	Style      string   `json:"style"`
	Title      string   `json:"title,omitempty"`
	Why        string   `json:"why,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Domain     string   `json:"domain,omitempty"`
	Mood       []string `json:"mood,omitempty"`
	Palette    []string `json:"palette,omitempty"`
	Trust      string   `json:"trust,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	Roles      []string `json:"roles"`
	Preview    string   `json:"preview,omitempty"`
	Apply      string   `json:"apply"`
}

func listStyles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "_") {
			if _, err := os.Stat(filepath.Join(root, e.Name(), "manual.yaml")); err == nil {
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out
}

func roleNames(roles map[string]Role) []string {
	out := make([]string, 0, len(roles))
	for k := range roles {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BuildIndex builds the catalogue from every <style>/{manual,meta}.yaml.
func BuildIndex(root string) ([]IndexEntry, error) {
	var out []IndexEntry
	for _, style := range listStyles(root) {
		m, err := LoadManual(root, style)
		if err != nil {
			continue
		}
		ent := IndexEntry{
			Style:   style,
			Roles:   roleNames(m.Roles),
			Preview: filepath.Join(root, style, "preview", "exemplar.svg"),
			Apply:   fmt.Sprintf("isotopo playbook apply %s <structure.yaml>", style),
		}
		if meta, err := LoadMeta(root, style); err == nil {
			ent.Title, ent.Tags, ent.Domain = meta.Title, meta.Tags, meta.Domain
			ent.Mood, ent.Palette, ent.Trust, ent.Confidence = meta.Mood, meta.Palette, meta.Trust, meta.Confidence
			ent.Why = strings.TrimSpace(strings.Join(meta.Mood, " / ") + " · " + meta.Domain)
		}
		out = append(out, ent)
	}
	return out, nil
}

// WriteIndex regenerates samples/playbook/INDEX.json (single source of truth).
func WriteIndex(root string) (int, error) {
	idx, err := BuildIndex(root)
	if err != nil {
		return 0, err
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return 0, err
	}
	return len(idx), os.WriteFile(filepath.Join(root, "INDEX.json"), append(b, '\n'), 0o644)
}

// ── search ───────────────────────────────────────────────────────────────────

func matchFacets(e IndexEntry, facets map[string]string) bool {
	for k, v := range facets {
		v = strings.ToLower(v)
		switch k {
		case "domain":
			if strings.ToLower(e.Domain) != v {
				return false
			}
		case "trust":
			if strings.ToLower(e.Trust) != v {
				return false
			}
		case "tag":
			if !containsFold(e.Tags, v) {
				return false
			}
		case "mood":
			if !containsFold(e.Mood, v) {
				return false
			}
		}
	}
	return true
}

func containsFold(ss []string, v string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func scoreEntry(e IndexEntry, terms []string) int {
	hay := strings.ToLower(strings.Join([]string{
		e.Style, e.Title, e.Domain, strings.Join(e.Tags, " "), strings.Join(e.Mood, " "),
	}, " "))
	s := 0
	for _, t := range terms {
		if t == "" {
			continue
		}
		if strings.Contains(strings.ToLower(e.Style), t) {
			s += 5
		}
		if containsFold(e.Tags, t) || containsFold(e.Mood, t) {
			s += 3
		}
		if strings.Contains(hay, t) {
			s += 1
		}
	}
	if e.Trust == "blessed" {
		s++ // tie-break toward vetted styles
	}
	return s
}

// Search ranks styles against a free-text query + structured facets.
func Search(root, query string, facets map[string]string) ([]IndexEntry, error) {
	idx, err := BuildIndex(root)
	if err != nil {
		return nil, err
	}
	terms := strings.Fields(strings.ToLower(query))
	type scored struct {
		e IndexEntry
		s int
	}
	var hits []scored
	for _, e := range idx {
		if !matchFacets(e, facets) {
			continue
		}
		hits = append(hits, scored{e, scoreEntry(e, terms)})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].s != hits[j].s {
			return hits[i].s > hits[j].s
		}
		return hits[i].e.Style < hits[j].e.Style
	})
	var out []IndexEntry
	for _, h := range hits {
		if h.s > 0 || query == "" {
			out = append(out, h.e)
		}
	}
	return out, nil
}
