package isotopo

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseYAML decodes a Document from YAML bytes.
func ParseYAML(data []byte) (*Document, error) {
	doc := &Document{}
	if err := yaml.Unmarshal([]byte(normalizeFlowColons(string(data))), doc); err != nil {
		return nil, fmt.Errorf("isotopo: yaml parse: %w", err)
	}
	normalizeDoc(doc)
	return doc, nil
}

// normalizeFlowColons makes the renderer read a TIGHT flow mapping the way a
// user means it: `{wx:30,wy:30}` is, per the YAML spec, the single null-valued
// key "wx:30,wy:30" — so yaml.v3 silently dropped tight offsets to zero while
// the editor's text surgery read 30, the two diverging. Inside flow mappings
// (and never inside quotes) it inserts the missing space after a key colon, so
// both readers agree. Guards: skip `://` (URLs like iso://…), and `:` already
// followed by space or a flow delimiter.
func normalizeFlowColons(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	depth := 0
	var inSQ, inDQ, esc bool
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		b.WriteRune(c)
		if esc { // previous char was a backslash inside a double-quoted string
			esc = false
			continue
		}
		switch {
		case inSQ:
			if c == '\'' {
				inSQ = false
			}
		case inDQ:
			if c == '\\' {
				esc = true // skip the escaped char so \" doesn't close the string
			} else if c == '"' {
				inDQ = false
			}
		case c == '\'':
			inSQ = true
		case c == '"':
			inDQ = true
		case c == '{':
			depth++
		case c == '}':
			if depth > 0 {
				depth--
			}
		case c == ':' && depth > 0 && i+1 < len(rs):
			if nx := rs[i+1]; nx != ' ' && nx != '\t' && nx != '\n' && nx != '/' && nx != ',' && nx != '}' && nx != ':' {
				b.WriteRune(' ')
			}
		}
	}
	return b.String()
}

// ParseJSON decodes a Document from JSON bytes.
func ParseJSON(data []byte) (*Document, error) {
	doc := &Document{}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("isotopo: json parse: %w", err)
	}
	normalizeDoc(doc)
	return doc, nil
}

// normalizeDoc strips the nil pointers the YAML/JSON decoders leave behind for
// empty values — `scene:` with no body yields a nil *Node, a blank `-` list
// item a nil *CompositePart/*Connector/*Annotation. None are representable in a
// rendered scene, and every walk over the tree (Validate, render, edit ops)
// would nil-deref on them. Stripping once here collapses a whole class of panics
// into a clean "empty" instead of guarding dozens of call sites.
func normalizeDoc(doc *Document) {
	if doc == nil {
		return
	}
	for id, n := range doc.Nodes {
		if n == nil {
			delete(doc.Nodes, id)
			continue
		}
		n.Parts = cleanParts(n.Parts)
		n.Connectors = cleanConnectors(n.Connectors)
		n.Annotations = cleanAnnotations(n.Annotations)
	}
	doc.Annotations = cleanAnnotations(doc.Annotations)
}

func cleanParts(parts []*CompositePart) []*CompositePart {
	if parts == nil {
		return nil
	}
	out := parts[:0]
	for _, p := range parts {
		if p == nil {
			continue
		}
		p.Parts = cleanParts(p.Parts)
		out = append(out, p)
	}
	return out
}

func cleanConnectors(cs []*Connector) []*Connector {
	if cs == nil {
		return nil
	}
	out := cs[:0]
	for _, c := range cs {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}

func cleanAnnotations(as []*Annotation) []*Annotation {
	if as == nil {
		return nil
	}
	out := as[:0]
	for _, a := range as {
		if a != nil {
			out = append(out, a)
		}
	}
	return out
}

// Parse picks YAML or JSON by sniffing the leading non-whitespace byte:
// '{' or '[' → JSON, anything else → YAML.
func Parse(data []byte) (*Document, error) {
	trimmed := strings.TrimLeft(string(data), " \t\r\n")
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return ParseJSON(data)
	}
	return ParseYAML(data)
}
