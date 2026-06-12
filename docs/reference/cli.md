# Usage

iso-topology has two faces: a CLI (`isotopo`) and a Go library
(`github.com/MarkovWangRR/iso-topology`). Both expose the same
rendering pipeline.

## CLI

### `isotopo render` — produce SVG + embed assets

```
isotopo render [--layout dagre|elk] <input> <output-dir>
```

| Argument | Meaning |
|---|---|
| `--layout dagre` | default; natural-bend polyline edges |
| `--layout elk` | orthogonal right-angle edges with obstacle avoidance |
| `<input>` | path to `.yaml`, `.json`, or `.d2` (or `-` for stdin) |
| `<output-dir>` | directory to write `topology.{svg,html,…}` and `nodes/<id>.{svg,html,yaml}` |

The `--layout` flag is consulted only for `.d2` input (YAML/JSON
already encode placement). Output structure is documented in
[OUTPUTS.md](../reference/output-layout.md).

Examples:

```bash
# .d2 with dagre auto-layout
isotopo render scene.d2 ./out

# .d2 with ELK orthogonal routing
isotopo render --layout elk scene.d2 ./out-elk

# YAML composite
isotopo render scene.yaml ./out

# Stream a generated DSL through stdin
gen-dsl | isotopo render - ./out
```

### `isotopo capabilities` — discover supported features

```
isotopo capabilities
```

Emits a JSON document with five top-level keys:

| Key | Contents |
|---|---|
| `version` | semantic version of this build |
| `inputs` | accepted file extensions and what they mean |
| `layouts` | layout engines for `.d2` |
| `shapes` | every shape name and its aliases |
| `primitives` | composition primitives (group, stack, annotation, …) with one-line syntax |
| `style_keys` | every field inside `palette` / `stroke` / `text` / `effects` |

Designed for agents to read once at startup so they can generate
valid DSL without consulting source code. Example:

```bash
isotopo capabilities | jq '.shapes[].iso_name'
# "circle", "cloud", "composite", "cylinder", "group", …

isotopo capabilities | jq '.primitives[] | {name, syntax}'
```

### `isotopo validate` — structural check before render

```
isotopo validate <input>
```

Parses the input and runs structural validation: unknown shape names,
annotation anchors that don't resolve, connector references to
missing parts, unknown canvas grid modes, etc.

Emits JSON of issues:

```json
{
  "issues": [
    {
      "severity": "error",
      "path": "nodes.scene.parts[2].shape",
      "message": "unknown shape \"cilinder\"",
      "suggest": "cylinder"
    }
  ]
}
```

| Exit code | Meaning |
|---|---|
| 0 | clean — no issues |
| 2 | warnings only — will render but probably not as intended |
| 3 | errors — render will likely fail or produce broken output |

In an agent loop:

```bash
isotopo validate scene.yaml > /tmp/issues.json
if [ "$?" = "3" ]; then
    # Apply suggestions, re-emit DSL, re-validate
    ...
fi
isotopo render scene.yaml ./out
```

## `isotopo serve` — the Studio

The interactive page has a name and its own guide: **[isotopo Studio](../guides/studio.md)** — feature tour, design rationale, endpoints, and how it is tested. Quick reference below.

```bash
isotopo serve scene.yaml          # default port 8731
ISOTOPO_PORT=9000 isotopo serve scene.yaml
```

Serves the interactive viewer at `http://localhost:8731`:

- the YAML editor (line numbers, Tab inserts indent, drag the splitter
  to resize the pane) edits an **in-browser copy — the input file on
  disk is never written**; unsaved edits persist across a reload as a
  local draft, with a one-click `revert`
- hovering a node in the SVG highlights its source block; the filetab
  shows the absolute source path with a copy-path button
- scroll to zoom, drag to pan, double-click to reset
- edits re-render automatically (debounced) or with Cmd/Ctrl+Enter;
  validation errors show under the editor with fix suggestions, and
  the canvas keeps the last good render (with a badge) while the
  source is broken
- `↓ SVG` / `↓ PNG` export whatever the canvas currently shows —
  unsaved edits included — as `<name>.edited.svg` / `.png` (2x);
  `Download` saves the edited YAML copy
- `Browse nodes` opens the per-part gallery, served live from the
  current file content

Endpoints, for tooling: `GET /` viewer · `GET /topology.svg` fresh
render of the file on disk · `GET /nodes/_index.html` and
`/nodes/<id>.svg|.yaml|.html` gallery · `POST /api/render` renders the
posted source, returns `{"svg", "issues"}` · `GET /api/ping` health.

The same interactive page is written to `out/topology.html` by
`isotopo render`; opened as a static file it keeps everything except
live re-render (the toolbar tells you to start `isotopo serve`).

## Go library

Module path: `github.com/MarkovWangRR/iso-topology`

### Quick example

```go
package main

import (
    "fmt"
    isotopo "github.com/MarkovWangRR/iso-topology"
)

func main() {
    yaml := []byte(`
canvas: {background: "#FAFBFC", grid: iso}
nodes:
  scene:
    shape: composite
    parts:
      - id: api
        shape: rectangle
        geom: {w: 120, d: 80, h: 30}
        label: API
      - id: db
        shape: cylinder
        offset: {wx: 200, wy: 0}
        geom: {w: 100, d: 100, h: 36}
        label: DB
    connectors:
      - {from: api, to: db, arrow: triangle}
`)
    doc, err := isotopo.Parse(yaml)
    if err != nil { panic(err) }

    svg := isotopo.RenderWithCanvas(
        doc.Scene(),
        doc.Theme,
        doc.Canvas,
        doc.Annotations,
    )
    fmt.Print(svg)
}
```

### Public API surface

Pipeline (each step is a separate function so you can intercept any
stage):

```go
// 1. Parse input
isotopo.Parse(data []byte) (*Document, error)        // YAML or JSON auto-sniffed
isotopo.ParseYAML(data []byte) (*Document, error)
isotopo.ParseJSON(data []byte) (*Document, error)

// 2. For .d2 input: compile then translate
isotopo.CompileD2(ctx, src string, engine LayoutEngine) (*d2target.Diagram, error)
isotopo.Translate(diag *d2target.Diagram, opts *RenderOpts) *Document

// 3. Validate (optional)
isotopo.Validate(doc *Document) []Issue

// 4. Render
isotopo.Render(n *Node, theme *Theme) string                          // single node, no canvas
isotopo.RenderWithCanvas(n, theme, canvas, anns) string                // scene with backdrop + callouts
isotopo.RenderDocument(doc *Document) map[string]string                // every node → SVG
isotopo.RenderParts(doc *Document) map[string]string                   // atomic per-part SVGs
```

Auxiliary:

```go
// Document model
doc.Scene() *Node                       // resolves the topology-level composite
isotopo.PartIDs(doc) []string           // every renderable part id
isotopo.PartFragments(doc) map[string]*Document  // each part as a standalone doc
isotopo.MarshalFragmentYAML(d) ([]byte, error)

// Theme cascade
isotopo.ResolveStyle(theme, shape, override) *Style
isotopo.MergeStyle(base, over) *Style

// Iso lowering
isotopo.Flatten(n *Node, theme *Theme) (shape string, opts iso25d.ConvertOpts)

// Introspection
isotopo.CapabilityReport() Capabilities
```

### Types worth knowing

| Type | What it is |
|---|---|
| `Document` | top-level parsed DSL (Canvas + Theme + Nodes + Annotations) |
| `Node` | one element. `Shape == "composite"` makes it a scene; nested `Parts` allowed |
| `CompositePart` | one sub-element inside a composite |
| `Stack` | replicate a part N times vertically |
| `Connector` | directed line between parts |
| `Annotation` | screen-space callout with a leader line |
| `Canvas` | document-level backdrop (`background`, `grid`, `gridColor`, `gridStep`) |
| `Theme` | document-level default Style with optional per-shape overrides |
| `Style` | `{palette, stroke, text, effects}` — see [DSL_THEME.md](../reference/dsl-theme.md) |
| `LayoutEngine` | `LayoutDagre` or `LayoutELK` |

### Iso geometry sub-package

If you need to call the low-level iso renderer directly (without going
through the DSL), import:

```go
import "github.com/MarkovWangRR/iso-topology/iso25d"

svg := iso25d.Convert2DTo25D("cylinder", iso25d.ConvertOpts{
    Width: 100, Depth: 100, Height: 40,
    TopFill: "#FFF", LeftFill: "#888", RightFill: "#BBB",
})
```

`iso25d` is documented inline in `iso25d/*.go`. For most users the
top-level package is enough.
