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
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("isotopo: yaml parse: %w", err)
	}
	normalizeDoc(doc)
	return doc, nil
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
