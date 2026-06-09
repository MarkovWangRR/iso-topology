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
	return doc, nil
}

// ParseJSON decodes a Document from JSON bytes.
func ParseJSON(data []byte) (*Document, error) {
	doc := &Document{}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("isotopo: json parse: %w", err)
	}
	return doc, nil
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
