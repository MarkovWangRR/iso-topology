<div align="center">

<img src="docs/assets/logo.jpg" alt="iso-topology logo — text DSL flowing into an isometric block diagram" width="520">

# iso-topology — isometric architecture diagrams as code

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/MarkovWangRR/iso-topology.svg)](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology)

### Figma-grade isometric architecture diagrams — as code, agent-native.

The only diagram-as-code tool where the output is design-grade **isometric**, the
DSL is a machine-checkable contract your agent self-corrects against, and it all
runs deterministically **offline in one static binary**.

`single binary` · `zero runtime deps` · `renders in ms` · `200+ icons incl. real brand logos` · `every sample golden-tested`

English · [简体中文](README.zh-CN.md)

</div>

---

You need a launch-post-quality architecture diagram. Today that's an afternoon in
Figma — pretty, but not diffable, and dead the moment the infra changes — or
Mermaid/D2, which only draw flat flowcharts. **iso-topology gives you the Figma
look from text your agent writes, validates before render, and you commit next to
the code.**

## See it

From this:

```yaml
nodes:
  scene:
    shape: composite
    parts:
      - id: core                          # the hero anchors the scene
        shape: rectangle
        icon: "iso://glyph/sparkles/7C5CFC"
        label: "AI Core"
        style: { effects: { cornerRadius: 14, backglow: { color: "#A78BFA", radius: 46 } } }
      - id: llm
        place: { behind: core, gap: 2.6 }  # ← relations, never coordinates
        icon: "iso://glyph/chat"
        label: "LLM Gateway"
      # …more satellites, each one `place` rule
```

…to this:

![Isometric AI platform architecture diagram rendered from YAML by iso-topology](docs/assets/ai-platform.png)

Every scene in this README is positioned entirely by `place` relations and
`layout` containers — not a single hand-computed coordinate.
[Source](samples/topology/ai-platform/input.yaml).

## Start in 60 seconds

**By hand** — one static binary, no browser or fonts needed:

```bash
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest
echo 'user -> api -> db' > scene.d2     # any d2 graph → auto-layout
isotopo render scene.d2 ./out           # → out/topology.svg (+ html + per-node SVGs)
isotopo serve  scene.d2                 # → localhost:8731 — drag, restyle, export
```

**With your agent** (Claude Code, Cursor, …) — paste this:

```
Install isotopo (go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest),
read `isotopo capabilities` once, then draw my <X> architecture: author YAML using
place/layout relations only (never coordinates), loop `isotopo validate` until exit 0,
then `isotopo render`. Keep the YAML as input.yaml next to the output.
```

The full battle-tested system prompt (positioning rules + output contract, generated
from code so it can't go stale) is [PROMPT_TEMPLATE.md](docs/agent/PROMPT_TEMPLATE.md).
There's also an installable [`draw-iso-diagram` skill](skills/README.md), an
[MCP server](docs/agent/MCP.md) (`claude mcp add isotopo -- isotopo-mcp`), and a
generated [`llms.txt`](llms.txt).

## The loop your agent runs

iso-topology speaks contract, not vibes — your agent discovers the DSL, emits a
scene, gets machine-checkable feedback, and converges with no human in the loop:

```bash
isotopo capabilities          # machine-readable DSL inventory — read once
isotopo validate scene.yaml   # JSONPath-located issues + "did you mean" fixes (exit 0/2/3)
isotopo evaluate scene.yaml   # layout scorecard: crossings / overlaps / edge-tunnelling
isotopo render   scene.yaml out
isotopo preview  scene.yaml out.svg core edge:3   # crop ONE node / group / edge
```

`validate` is a JSON-schema lint with fix suggestions; `evaluate` scores layout
quality from a deterministic plan view; both let an agent self-correct before it ever
looks at pixels. Output is byte-deterministic — same input, same SVG — which is what
makes the golden-file tests and clean git diffs work.

## Studio — point, drag, restyle

```bash
isotopo serve input.yaml        # → http://localhost:8731
```

![isotopo Studio — rendered isometric scene on the left, editable YAML on the right](docs/assets/studio.png)

The rendered scene on the left, its YAML on the right, sub-second feedback between.
**Drag to lay out**, drag a node in/out of a group, **right-click to restyle**,
add / delete / duplicate / connect nodes, toggle ◳ iso / ◰ plan, undo/redo, and
export SVG / PNG / YAML. Every edit is an in-browser copy — the file on disk is never
touched until you hit save. Tour: [docs/guides/studio.md](docs/guides/studio.md).

## What you can draw

- **18 iso primitives** — boxes, cylinders, prisms (tri/hex/oct), racks, arrays
  (1D/2D/3D), clouds, people, boundaries (VPC/zone), wedges, custom polygons —
  picked by *role*, not all rectangles.
- **Coordinate-free layout** — `place: {rightOf|behind|above…, gap}` relations and
  `layout: {mode: row|column|grid|ring|auto}` containers (auto = connector-driven
  DAG). A solver computes positions, validates references, and warns on overlap.
- **Design-grade styling** — per-face solid or gradient fills, translucent **glass**,
  **grain**, `faceSplit` lighting, dot/hatch patterns (iso-projected), drop shadows,
  backglow, rounded corners, wireframe.
- **Connectors that read as systems** — `orthogonal` (rides the iso grid) / `straight`
  / `bezier`, with dashes, gradients, waypoints, and labels.
- **200+ recolorable icons** — 150+ real brand logos (Simple Icons, CC0) plus 35
  concept glyphs; tint any with `/RRGGBB` or `/light`.
- **Reusable looks** — `theme.presets`, an 8-family [style playbook](samples/playbook),
  and 28 ready-to-copy [style references](samples/style_refer).

Full machine-readable inventory: `isotopo capabilities` (or
[CAPABILITIES.md](docs/agent/CAPABILITIES.md)).

## Gallery

| | |
|---|---|
| **ClickHouse ecosystem** — dark frosted-glass hub-and-spoke, one neon accent.<br>[Source](samples/topology/clickhouse-hub/input.yaml) | **DuckDB data architecture** — hand-drawn: warm paper, iso grid, film grain.<br>[Source](samples/topology/duckdb-handdrawn/input.yaml) |
| ![ClickHouse ecosystem isometric diagram](docs/assets/clickhouse-hub.png) | ![DuckDB hand-drawn isometric diagram](docs/assets/duckdb-handdrawn.png) |
| **Training compute** — light canvas, dashed budget "ghost" volumes.<br>[Source](samples/topology/training-compute/input.yaml) | **Microservice from 3 lines of d2** — auto-layout lifts a graph to 2.5D.<br>[Source](samples/topology/microservice/input.d2) |
| ![Isometric training-compute bar chart](docs/assets/training-compute.png) | ![Auto-laid-out microservice topology](docs/assets/microservice.png) |

## Why isometric, and why this tool

Flat 2D reads as a list of boxes; iso reads as a **system** — the depth axis
separates edge / mid-tier / data layers at a glance, and stacked nodes express
replicas and HA. Hand-drawing that in Figma scales to ~ten elements and zero people
on the diff.

| | Mermaid / D2 | Figma / draw.io | **iso-topology** |
|---|---|---|---|
| Visual register | flat flowchart | design-grade | **design-grade isometric** |
| Text source, git-diffable | ✓ | ✗ | ✓ |
| Agent discovers + validates the DSL | ✗ | ✗ | **✓ (`capabilities` + `validate`)** |
| Layout without hand-tuned coordinates | ✓ | ✗ | ✓ (`place` / `layout` solver) |
| Offline single binary | partial | ✗ | ✓ |

## Use as a Go library

```go
import isotopo "github.com/MarkovWangRR/iso-topology"

svg, issues, _ := isotopo.RenderSource("yaml", yamlBytes)            // DSL → one SVG

op := isotopo.EditOp{Kind: "move", Target: "node", ID: "api", DWX: 50}
newSrc, svg, issues, _ := isotopo.ApplyOp("yaml", yamlBytes, op)     // stateless canvas edit, comments preserved
```

The stateless edit contract powers Studio and client-side WASM editors. Full surface:
the [API reference](docs/reference/api.md) ([godoc](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology)).

## Docs

Full index at [docs/README.md](docs/README.md).

- **Start:** [Tutorial](docs/getting-started/01-install.md) · [Recipes](docs/agent/RECIPES.md) · [Scene design](docs/guides/scene-design.md)
- **Reference:** [API](docs/reference/api.md) · [CLI](docs/reference/cli.md) · [YAML DSL](docs/reference/dsl-yaml.md) · [d2 DSL](docs/reference/dsl-d2.md) · [Style/Theme](docs/reference/dsl-theme.md)
- **Agents:** [CAPABILITIES](docs/agent/CAPABILITIES.md) · [PROMPT_TEMPLATE](docs/agent/PROMPT_TEMPLATE.md) · [SAMPLES](docs/agent/SAMPLES.md) · [schema](docs/agent/schema/dsl.schema.json) · [MCP](docs/agent/MCP.md) · [skills](skills/README.md)

## Status & contributing

Single-author project, moving fast — pin a tag if you depend on it (the `d2`
dependency is locked at `v0.7.1`). Issues and PRs welcome; run `go test ./...` first —
`samples/*/*/expected.svg` are golden files that catch output drift. Drew something
cool? Open an issue with your scene + SVG and the best ones get linked here. ⭐ helps
others find it.

## License

Apache License 2.0 — see [LICENSE](LICENSE). Rendered output is yours to use
commercially; bundled brand logos are CC0 from [Simple Icons](https://simpleicons.org).
</content>
