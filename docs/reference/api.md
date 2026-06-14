# API reference

The single entry point for every programmable surface of iso-topology. Pick the
surface you need — each row links to its authoritative contract; nothing is
documented in two places.

| Surface | What it is | Contract |
|---|---|---|
| **DSL** (input format) | the YAML / JSON / d2 you author or an agent generates | [CAPABILITIES.md](../agent/CAPABILITIES.md) (generated) · [dsl.schema.json](../agent/schema/dsl.schema.json) (generated) · [YAML](dsl-yaml.md) · [d2](dsl-d2.md) · [Style/Theme](dsl-theme.md) |
| **Go library** | import the engine — parse, render, validate, **and edit** DSL | **this page (below)** · generated godoc on [pkg.go.dev](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology) |
| **HTTP API** | the Studio server's `/api/*` endpoints | [studio.md → Endpoints](../guides/studio.md#endpoints-for-tooling) |
| **CLI** | the `isotopo` command | [cli.md](cli.md) |
| **MCP** | tool server for Claude / Cursor / any MCP client | [MCP.md](../agent/MCP.md) |

> The **canonical, always-current** Go API is the godoc on
> [pkg.go.dev/github.com/MarkovWangRR/iso-topology](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology)
> — it is generated from the source and never drifts. The sections below are the
> hand-written tour.

---

## Go library

Module path: `github.com/MarkovWangRR/iso-topology`

```bash
go get github.com/MarkovWangRR/iso-topology@latest
```

Three things you can do with the library:

1. **Render** a DSL document to SVG — [Quick example](#quick-example-parse--render)
2. **Edit** a DSL document by applying direct-manipulation ops (drag, set field,
   add/delete/duplicate) while preserving comments — [Editing for embedders](#direct-manipulation-editing-for-embedders)
3. **Introspect** the DSL contract — `CapabilityReport()`

### Quick example (parse → render)

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
    // One-shot: validate + render any DSL text (yaml | json | d2) to one SVG.
    svg, issues, _ := isotopo.RenderSource("yaml", yaml)
    for _, i := range issues {
        fmt.Println(i.Severity, i.Path, i.Message)
    }
    fmt.Print(svg)
}
```

`RenderSource` is the convenience entry. For finer control (intercept a stage,
render per-part, pick the layout engine) use the pipeline functions in
[Public API surface](#public-api-surface).

### Direct-manipulation editing (for embedders)

This is the contract a cloud service or a WASM client uses to turn a canvas
gesture into a new DSL document — the same logic that powers Studio's
drag/edit, exposed as **pure, stateless functions**:

- The transform is a pure function of `(format, src, op)` — **no server state**.
  (Studio works by POSTing the whole DSL each time; the text is the only source
  of truth.)
- The rewrite **preserves the user's comments and formatting** — it is line/regex
  text surgery, not a reformatting YAML round-trip.

```go
src := []byte(`# my diagram
nodes:
  scene:
    shape: composite
    layout: {mode: row, gap: 1}
    parts:
      - {id: api, shape: rectangle, geom: {w: 120, d: 80, h: 30}, label: API}
      - {id: db,  shape: cylinder,  geom: {w: 100, d: 100, h: 36}, label: DB}
`)

// Drag "api" by (+50, 0) world units. ApplyOp returns the rewritten DSL,
// a fresh SVG and validation issues; ApplyOpText returns only the new text.
op := isotopo.EditOp{Kind: "move", Target: "node", ID: "api", DWX: 50, DWY: 0}
newSrc, svg, issues, err := isotopo.ApplyOp("yaml", src, op)
_ = svg; _ = issues; _ = err
// newSrc still starts with "# my diagram" — comments survive.
```

**`EditOp` kinds** (the field set read depends on `Kind`):

| Kind | Fields used | Effect |
|---|---|---|
| `move` | `Target` (`node`\|`edge`), `ID`/`CI`, `DWX`,`DWY`, `Snap`, `Waypoints` | nudge a node offset, or move an edge (waypoints / bend). First move on an auto-layout scene freezes it to explicit coordinates. |
| `set-field` | `Target` (`node`\|`edge`\|`canvas`), `ID`/`CI`, `Fields` (dotted path → value) | write any DSL field, e.g. `{"style.palette.top": "#FFF"}` |
| `add` | — | append a default rectangle to the scene |
| `delete` | `Target`, `ID`/`CI` | remove a node (and its connectors) or an edge |
| `duplicate` | `Target: node`, `ID` | clone a node with a fresh id, offset down-right |

```go
// Apply only the text transform (e.g. you render separately, or with ELK):
newSrc, err := isotopo.ApplyOpText("yaml", src, op)
```

For the lowest level — the individual comment-preserving text operations — import
the [`yamledit` sub-package](#sub-package-yamledit).

### Public API surface

Pipeline (each step is a separate function so you can intercept any stage):

```go
// 1. Parse input
isotopo.Parse(data []byte) (*Document, error)        // YAML or JSON auto-sniffed
isotopo.ParseYAML(data []byte) (*Document, error)
isotopo.ParseJSON(data []byte) (*Document, error)
isotopo.LoadInput(ctx, format string, data []byte, engine LayoutEngine) (*Document, error) // yaml|json|d2

// 2. For .d2 input: compile then translate
isotopo.CompileD2(ctx, src string, engine LayoutEngine) (*d2target.Diagram, error)
isotopo.Translate(diag *d2target.Diagram, opts *RenderOpts) *Document

// 3. Validate (optional)
isotopo.Validate(doc *Document) []Issue
isotopo.UnknownKeyIssues(data []byte) []Issue        // misspelled/unknown DSL keys

// 4. Render
isotopo.Render(n *Node, theme *Theme) string                          // single node, no canvas
isotopo.RenderWithCanvas(n, theme, canvas, anns) string                // scene with backdrop + callouts
isotopo.RenderDocument(doc *Document) map[string]string                // every node → SVG
isotopo.RenderParts(doc *Document) map[string]string                   // atomic per-part SVGs
```

One-shot render + the stateless edit contract (the embedding surface):

```go
isotopo.RenderSource(format string, src []byte) (svg string, issues []Issue, err error)
isotopo.ApplyOp(format string, src []byte, op EditOp) (newSrc []byte, svg string, issues []Issue, err error)
isotopo.ApplyOpText(format string, src []byte, op EditOp) (newSrc []byte, err error)
```

Auxiliary:

```go
// Document model
doc.Scene() *Node                       // resolves the topology-level composite
isotopo.PartIDs(doc) []string           // every renderable part id
isotopo.PartFragments(doc) map[string]*Document  // each part as a standalone doc
isotopo.MarshalFragmentYAML(d) ([]byte, error)

// Resolve what the renderer actually uses (cascade + layout) for a part —
// the basis of Studio's "effective value" hints.
isotopo.ResolvePartStyle(doc *Document, id string) *Style
isotopo.ResolvePartGeom(doc *Document, id string) (w, d, h float64, ok bool)
isotopo.ResolvePartOffset(doc *Document, id string) (wx, wy, wz float64, ok bool)

// Theme cascade
isotopo.ResolveStyle(theme, shape, preset string, override *Style) *Style
isotopo.MergeStyle(base, over *Style) *Style

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
| `EditOp` | one direct-manipulation edit for `ApplyOp`/`ApplyOpText` (see table above) |
| `Stack` | replicate a part N times vertically |
| `Connector` | directed line between parts |
| `Annotation` | screen-space callout with a leader line |
| `Canvas` | document-level backdrop (`background`, `grid`, `gridColor`, `gridStep`) |
| `Theme` | document-level default Style with optional per-shape overrides |
| `Style` | `{palette, stroke, text, effects}` — see [Style/Theme](dsl-theme.md) |
| `Issue` | a validation finding: `{Severity, Path, Message, Suggest}` |
| `LayoutEngine` | `LayoutDagre` (default) or `LayoutELK` |

### Sub-package: `yamledit`

`github.com/MarkovWangRR/iso-topology/yamledit` — comment- and
format-preserving text surgery on DSL source. `ApplyOp` is built on it; import it
directly only if you need a single operation (e.g. just `SetField`). Every
function takes the source string and returns the edited string, touching only
the lines it must:

```go
import "github.com/MarkovWangRR/iso-topology/yamledit"

out, ok := yamledit.SetField(src, line, []string{"style","palette","top"}, "#FFF")
```

Notable functions: `SetField`, `UpsertInlineKey`, `UpsertInlineList`,
`FreezeLayoutText`, `AddPart`, `DeletePart`, `DuplicatePart`,
`FindPartIDLine`, `FindConnectorLine`, `FindCanvasLine`, `ReadPath`. Full list
on [pkg.go.dev](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology/yamledit).

### Sub-package: `iso25d`

The low-level iso renderer, if you want geometry without the DSL:

```go
import "github.com/MarkovWangRR/iso-topology/iso25d"

svg := iso25d.Convert2DTo25D("cylinder", iso25d.ConvertOpts{
    Width: 100, Depth: 100, Height: 40,
    TopFill: "#FFF", LeftFill: "#888", RightFill: "#BBB",
})
```

### Browser / WASM

The engine compiles for `GOOS=js GOARCH=wasm`, so `RenderSource` and
`ApplyOp`/`ApplyOpText` can run **client-side** (zero-latency edit + preview). The
only filesystem touch — inlining a local-file icon — is build-tagged off in the
js build and degrades to passing the reference through, so a wasm build never
fails on a local icon. Inline local icons as `data:` URIs before handing the DSL
to the module if you need them in the browser.
