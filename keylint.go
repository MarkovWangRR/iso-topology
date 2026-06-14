package isotopo

import (
	"bytes"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var unknownFieldRe = regexp.MustCompile(`line (\d+): field (\S+) not found in type isotopo\.(\w+)`)

// UnknownKeyIssues strict-decodes the YAML source and reports keys that match
// no struct field — the misspellings (style.palett, geom.with, effects.
// cornerRadious) that a lenient decode silently drops, the single largest
// class of invisible agent mistakes. Each is a warning (the document still
// renders, just missing that key's effect) carrying the source line and a
// nearest-field suggestion.
//
// Returns nil for JSON or anything that isn't a clean YAML type-mismatch
// (Parse already reports true syntax errors); this is a purely additive lint
// over otherwise-valid YAML, so it never blocks a render.
func UnknownKeyIssues(data []byte) []Issue {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	doc := &Document{}
	te, ok := dec.Decode(doc).(*yaml.TypeError)
	if !ok {
		return nil
	}
	fields := fieldsByType()
	var issues []Issue
	for _, e := range te.Errors {
		m := unknownFieldRe.FindStringSubmatch(e)
		if m == nil {
			continue // some other type error (e.g. wrong scalar kind) — skip
		}
		line, key, typ := m[1], m[2], m[3]
		iss := Issue{
			Severity: SeverityWarning,
			Path:     "$ (line " + line + ")",
			Message:  "unknown key \"" + key + "\" — not a field of " + typ + "; it is silently ignored",
		}
		if cand := fields[typ]; len(cand) > 0 {
			iss.Suggest = nearest(key, cand)
		}
		issues = append(issues, iss)
	}
	return issues
}

// fieldsByType walks the Document type graph once and returns, per struct type
// name, its YAML field names — the candidate set for unknown-key suggestions.
func fieldsByType() map[string][]string {
	out := map[string][]string{}
	seen := map[reflect.Type]bool{}
	var visit func(t reflect.Type)
	visit = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		var names []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("yaml"), ",")[0]
			if name != "" && name != "-" {
				names = append(names, name)
			}
			visit(f.Type)
		}
		out[t.Name()] = names
	}
	visit(reflect.TypeOf(Document{}))
	return out
}
