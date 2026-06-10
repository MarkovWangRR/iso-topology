// Input loading — the one place that maps (format, bytes) onto the
// Document model, shared by the CLI, the MCP server, and library
// callers so the dialect-dispatch logic can't fork between them.
package isotopo

import (
	"context"
	"fmt"
)

// LoadInput parses diagram source of the given format ("yaml", "json",
// or "d2") into a Document. YAML and JSON parse directly; d2 compiles
// through the given layout engine (LayoutDagre when engine is empty)
// and translates the laid-out diagram.
func LoadInput(ctx context.Context, format string, data []byte, engine LayoutEngine) (*Document, error) {
	switch format {
	case "d2":
		if engine == "" {
			engine = LayoutDagre
		}
		diagram, err := CompileD2(ctx, string(data), engine)
		if err != nil {
			return nil, err
		}
		return Translate(diagram, DefaultRenderOpts()), nil
	case "yaml", "json", "":
		return Parse(data)
	default:
		return nil, fmt.Errorf("isotopo: unknown input format %q (yaml | json | d2)", format)
	}
}
