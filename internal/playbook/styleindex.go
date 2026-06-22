package playbook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// styleindex.go — the node-style retrieval layer behind `isotopo playbook style …`.
// Turns a fuzzy request into a ranked family + paste-ready DSL, so a coding agent
// makes a TOOL CALL instead of reading markdown and improvising.

type StyleFamily struct {
	Slug      string   `json:"slug"`
	Triggers  []string `json:"triggers"`
	Mood      []string `json:"mood"`
	UseCases  []string `json:"use_cases"`
	Signature string   `json:"signature"`
	DslFile   string   `json:"dsl_file"`
}

type StyleIndex struct {
	Fallback string        `json:"fallback"`
	Families []StyleFamily `json:"families"`
}

func LoadStyleIndex(root string) (*StyleIndex, error) {
	b, err := os.ReadFile(filepath.Join(root, "styles", "index.json"))
	if err != nil {
		return nil, err
	}
	var idx StyleIndex
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

type StyleHit struct {
	Slug      string   `json:"slug"`
	Score     int      `json:"score"`
	Why       []string `json:"why"`
	Signature string   `json:"signature"`
	DslFile   string   `json:"dsl_file"`
}

func styleTokenize(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	return f
}

// SuggestStyle ranks families against a free-text request. Returns ranked hits +
// the fallback family (use it when hits is empty).
func SuggestStyle(root, query string) ([]StyleHit, string, error) {
	idx, err := LoadStyleIndex(root)
	if err != nil {
		return nil, "", err
	}
	ql := strings.ToLower(query)
	terms := styleTokenize(query)
	var hits []StyleHit
	for _, f := range idx.Families {
		score := 0
		seen := map[string]bool{}
		why := []string{}
		note := func(w int, tag string) {
			score += w
			if tag != "" && !seen[tag] {
				seen[tag] = true
				why = append(why, tag)
			}
		}
		// whole multi-word trigger phrase appears in the request → strong signal
		for _, tg := range f.Triggers {
			if len(tg) >= 3 && strings.Contains(ql, strings.ToLower(tg)) {
				note(4, tg)
			}
		}
		bag := map[string]int{}
		addBag := func(words []string, w int) {
			for _, x := range words {
				for _, t := range styleTokenize(x) {
					if bag[t] < w {
						bag[t] = w
					}
				}
			}
		}
		addBag(f.Triggers, 3)
		addBag(f.Mood, 2)
		addBag(f.UseCases, 2)
		addBag([]string{f.Slug}, 3)
		for _, t := range terms {
			if w, ok := bag[t]; ok {
				note(w, t)
			}
		}
		if score > 0 {
			hits = append(hits, StyleHit{f.Slug, score, why, f.Signature, f.DslFile})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Slug < hits[j].Slug
	})
	return hits, idx.Fallback, nil
}

// ShowStyle returns the full family doc (semantics + prompt + node dimensions +
// canonical DSL template) so the agent can paste the preset directly.
func ShowStyle(root, slug string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "styles", slug+".md"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
