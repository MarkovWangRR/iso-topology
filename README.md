# iso-topology

**Text in. Isometric SVG out. Architecture diagrams your agent can generate, validate, and diff.**

![iso-topology hero — an AI platform core ringed by eight capability tiles](docs/assets/ai-platform.png)

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/MarkovWangRR/iso-topology.svg)](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology)

A renderer that turns textual graph DSL into self-contained 2.5D
isometric SVG. The DSL is small enough for an LLM to generate from a
prompt, structured enough to validate before render, and diffable
enough to commit alongside your code.

Agent-first. Humans welcome.

## Quickstart

```bash
# install
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@v0.2.1

# render a 3-node topology
echo 'user -> api -> db' > scene.d2
isotopo render scene.d2 ./out

# inspect: SVG + editable DSL side-by-side
open ./out/topology.html
```

Single static binary. No CGO, no system fonts, no runtime deps. Drop
it into a CI image or an agent container and it works.

## The agent loop

```bash
isotopo capabilities          # discover shapes / primitives / layouts (read once)
isotopo validate scene.d2     # JSONPath-located issues with "did you mean"
isotopo render   scene.d2 out # produce SVG + per-element fragments
```

The agent reads `capabilities` at startup to learn what DSL it can
emit, generates a scene, asks `validate` for structural errors and
applies the suggestions, then calls `render`. No human in the loop.

Sample `validate` output for an agent-generated draft with a typo:

```json
{
  "issues": [
    {
      "severity": "error",
      "path": "nodes.scene.parts[0].shape",
      "message": "unknown shape \"cilinder\"",
      "suggest": "cylinder"
    }
  ]
}
```

Exit codes: `0` clean, `2` warnings only, `3` errors. Wire it straight
into a CI loop.

## Why isometric

Flat 2D topology (d2, Mermaid, Graphviz) reads as a list of boxes.
Iso reads as a **system**: the depth axis separates edge / mid-tier
/ data tiers at a glance, and stacked nodes naturally express
replicas and HA.

Hand-authored iso (Figma, FossFlow) scales to about ten elements and
zero people on the diff. iso-topology takes the DSL path: text in,
iso SVG out, source-controlled, agent-generatable.

## Gallery

Every scene below is positioned entirely by `place` relations and
`layout` containers — not a single hand-computed coordinate.

### AI platform (light · hub-and-spoke)

![ai-platform](docs/assets/ai-platform.png)

The AI core ringed by eight capabilities, each top face carrying a
domain glyph plus an iso-projected caption — LLM gateway, vector
store, GPU pool, agents, data lake, streaming, warehouse,
observability. Solid spokes = serving path, dashed = data path.
Source:
[samples/topology/ai-platform/input.yaml](samples/topology/ai-platform/input.yaml).

### LLM serving (dark · layered flow)

![llm-serving](docs/assets/llm-serving.png)

Chat app and CLI flow through the serving gateway — a `layout: grid`
hero board whose cells (router / guardrails / cache / auth) carry
cyan glyphs and captions — into the model plane: a GPU pool and a
model-registry replica stack, with a dotted texture panel floating
behind. Source:
[samples/topology/llm-serving/input.yaml](samples/topology/llm-serving/input.yaml).

### RAG pipeline (dark · dual plane)

![rag-pipeline](docs/assets/rag-pipeline.png)

Ingest & index on the back plane (docs → ETL → embeddings), serving
on the front plane (app → retriever → LLM), the shared vector
database standing between them as a replica stack — the retriever's
ANN query rides a dashed bezier. Source:
[samples/topology/rag-pipeline/input.yaml](samples/topology/rag-pipeline/input.yaml).

### Training compute (light · ghost volumes)

![training-compute](docs/assets/training-compute.png)

Where a training run's GPU hours go: gradient bars carrying glyphs
and hour counts on their top faces, dashed wireframe "ghost" volumes
above showing the per-stage budget ceiling. Source:
[samples/topology/training-compute/input.yaml](samples/topology/training-compute/input.yaml).

### Microservice (.d2 → dagre auto-layout)

![microservice topology](docs/assets/microservice.png)

```d2
user:   User { shape: person }
api:    API Gateway
queue:  Job Queue { shape: queue }
worker: Worker
db:     Database { shape: cylinder }
cache:  Cache    { shape: cylinder }

user   -> api:   request
api    -> queue: enqueue
api    -> cache: lookup
queue  -> worker
worker -> db:    write
worker -> cache: update
```

Source: [samples/topology/microservice/input.d2](samples/topology/microservice/input.d2).

## Two input modes

| Path | Strength | Use when |
|---|---|---|
| `.d2` graph source | auto-layout via dagre or ELK | agents generating topology from graph data, dynamic scenes |
| `.yaml` composite | declarative composition — `layout` containers + `place` relations, no hand-computed coordinates | designer-controlled scenes, fixed templates, infographics |

Both produce the same output structure. Same `Document` model under
the hood: `.d2` runs through `CompileD2 → Translate`, `.yaml` parses
directly. See [DSL_D2.md](docs/reference/dsl-d2.md) and
[DSL_YAML.md](docs/reference/dsl-yaml.md).

## Capabilities

- **23 d2 shapes** mapped to iso primitives (rectangle, cylinder,
  cloud, person, hexagon, queue, oval, …)
- **2 layout engines** for `.d2`: dagre (polyline edges), ELK
  (orthogonal, obstacle-avoidance)
- **Declarative YAML positioning**: `layout: {mode: row|column|grid}`
  containers and `place: {rightOf|inFrontOf: sibling}` relations —
  the solver computes coordinates, validates references, and warns
  on overlaps
- **8 composition primitives**: `group`, `stack`, `layout`, `place`,
  `canvas.grid`, `annotation`, `connector` (orthogonal/bezier),
  brand icons (`iso://brand/kafka` …)
- **Face styling**: per-face gradients, dropShadow, backglow,
  hatch/dot patterns, cornerRadius
- **3 input formats**: `.d2`, `.yaml`, `.json`
- **Two-tier output**: topology SVG + per-element standalone SVG

Full machine-readable list:

```bash
isotopo capabilities | jq '.shapes[].iso_name, .primitives[].name'
```

## Output

```
out/
├── topology.svg              full scene
├── topology.html             SVG side-by-side with editable DSL source
├── topology.<yaml|d2|json>   source copy
└── nodes/
    ├── _index.html           gallery
    ├── <id>.svg              standalone iso element
    ├── <id>.html             embed snippet
    └── <id>.yaml             re-renderable DSL fragment
```

Drop `topology.svg` into any markdown / Notion / slide deck. Each
`nodes/<id>.svg` is a self-contained iso sticker. Embedding recipes
and troubleshooting: [OUTPUTS.md](docs/reference/output-layout.md).

## Use as a Go library

```go
import isotopo "github.com/MarkovWangRR/iso-topology"

doc, _ := isotopo.Parse(yamlBytes)
svg := isotopo.RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
```

Full library API surface: [USAGE.md](docs/reference/cli.md).

## Docs

Organized by purpose, not topic — full index at [docs/README.md](docs/README.md).

**Start here:**

- [Tutorial](docs/getting-started/01-install.md) — 5 steps from install to your first published scene
- [Recipes](docs/agent/RECIPES.md) — task → DSL primitive speed-dial
- [Troubleshooting](docs/guides/troubleshooting.md) — failure modes by symptom

**Reference:**

| Doc | Read when |
|---|---|
| [CLI + library API](docs/reference/cli.md) | subcommands, library entrypoints, signatures |
| [YAML DSL](docs/reference/dsl-yaml.md) | declarative composition (layout/place) — every field |
| [.d2 DSL](docs/reference/dsl-d2.md) | auto-layout path, d2 shape mapping, nested containers |
| [Style / Theme](docs/reference/dsl-theme.md) | palette, stroke, text, effects, cascade |
| [Output layout](docs/reference/output-layout.md) | what's in `out/`, embedding recipes |

**Design:**

- [Why isometric](docs/concepts/why-isometric.md) — design rationale and tradeoffs
- [Extending](docs/guides/extending.md) — add a new shape, primitive, or layout

**Agent integration:**

| Doc | Read when |
|---|---|
| [CAPABILITIES.md](docs/agent/CAPABILITIES.md) | offline-readable capability inventory (generated) |
| [PROMPT_TEMPLATE.md](docs/agent/PROMPT_TEMPLATE.md) | drop-in system prompt for your LLM |
| [dsl.schema.json](docs/agent/schema/dsl.schema.json) | JSON Schema for local lint (no CLI roundtrip) |

## Status

Single-author project. API may evolve; pin to a tag if you depend on
it. The published `oss.terrastruct.com/d2` dependency is locked at
`v0.7.1`.

## Contributing

Issues and PRs welcome. Run `go test ./...` before sending —
`samples/{node,topology}/*/expected.svg` are golden files that catch
unintended output drift across the rendering pipeline.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
