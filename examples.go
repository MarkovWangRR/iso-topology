package isotopo

import (
	"embed"
	"path"
	"sort"
	"strings"
)

// Built-in gallery scenes, embedded so the Studio "Examples" menu is
// served LOCALLY — offline-capable and always matching this exact
// build (the old menu fetched a hardcoded, stale list from GitHub).
//
//go:embed samples/topology/*/input.yaml samples/topology/*/input.d2
var sampleFS embed.FS

type sampleEntry struct {
	lang    string // yaml | d2
	content []byte
}

var sampleStore = map[string]sampleEntry{}
var sampleOrder []string

func init() {
	entries, _ := sampleFS.ReadDir("samples/topology")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		for _, f := range []struct{ file, lang string }{{"input.yaml", "yaml"}, {"input.d2", "d2"}} {
			b, err := sampleFS.ReadFile(path.Join("samples/topology", name, f.file))
			if err == nil {
				sampleStore[name] = sampleEntry{lang: f.lang, content: b}
				sampleOrder = append(sampleOrder, name)
				break
			}
		}
	}
	sort.Strings(sampleOrder)
}

// SampleNames returns the sorted ids of the built-in gallery scenes.
func SampleNames() []string {
	out := make([]string, len(sampleOrder))
	copy(out, sampleOrder)
	return out
}

// Sample returns one built-in scene's source and language (yaml|d2).
func Sample(name string) (content []byte, lang string, ok bool) {
	// guard against path tricks — names are bare directory ids
	if strings.ContainsAny(name, "/.\\") {
		return nil, "", false
	}
	e, ok := sampleStore[name]
	return e.content, e.lang, ok
}
