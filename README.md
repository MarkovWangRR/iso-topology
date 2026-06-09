# iso-topology

Turn textual graph DSL into **2.5D isometric topology diagrams** —
self-contained SVG you can drop into docs, slides, dashboards, or
ship to an LLM.

```bash
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest
echo 'user -> api -> db' > scene.d2
isotopo render scene.d2 ./out
open ./out/topology.html
```

## What problem this solves

Tools like d2 / Mermaid / Graphviz are great at flat 2D topology, but
**iso views read better for system architecture**: the depth axis
lets readers separate logical layers (edge / mid-tier / data) at a
glance, and stacked nodes naturally express replicas / HA.

Authoring iso diagrams by hand (Figma, Sketch, FossFlow) doesn't scale:
- You move boxes pixel by pixel.
- You can't generate them from code, config, or an LLM.
- Source-controlled diff is impossible.

iso-topology takes **text in, iso SVG out**:

| Input | Use when |
|---|---|
| `.d2` graph source | You want auto-layout. Agents generating diagrams from a topology graph. |
| `.yaml` composite | You want precise placement. Designer-level control of every offset. |

Both produce the same output structure: a `topology.svg` for the
whole scene + a per-node SVG/HTML/YAML for every element.

## 30-second tour

```bash
# 1. Install
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest

# 2. Quick d2 render (auto-layout via dagre)
cat > scene.d2 <<'EOF'
user: User { shape: person }
api:  API Gateway
db:   Database { shape: cylinder }

user -> api: request
api  -> db:  query
EOF
isotopo render scene.d2 ./out

# 3. Open the embeddable HTML
open ./out/topology.html
```

Now you have:
- `out/topology.svg` — embeddable SVG of the whole scene
- `out/topology.html` — same SVG + the editable DSL source side-by-side
- `out/nodes/<id>.svg` — standalone SVG per element (drop into a sticker sheet)
- `out/nodes/<id>.html` — per-element embed snippet + YAML fragment

## Designed for agent generation

The DSL is agent-first:

```bash
# Discover what's available without reading source
isotopo capabilities | jq '.shapes[], .primitives[]'

# Validate before render — get JSON of issues with "did you mean"
isotopo validate scene.yaml

# Render
isotopo render scene.yaml ./out
```

Every error from `validate` includes a JSONPath locator and a
nearest-neighbor suggestion, so an agent can self-correct in a loop.

## Docs

| Doc | Read this when |
|---|---|
| [INSTALL.md](docs/INSTALL.md) | Setting up — `go install`, clone & build, library import |
| [USAGE.md](docs/USAGE.md) | CLI subcommand reference + Go library API |
| [DSL_YAML.md](docs/DSL_YAML.md) | YAML composite spec — precise iso placement |
| [DSL_D2.md](docs/DSL_D2.md) | `.d2` input spec — how d2 shapes and containers translate |
| [DSL_THEME.md](docs/DSL_THEME.md) | Style / Theme cascade: Palette, Stroke, Text, Effects |
| [OUTPUTS.md](docs/OUTPUTS.md) | Output directory layout + how to embed in docs / slides |
| [EXTENDING.md](docs/EXTENDING.md) | Add a new shape, primitive, or layout engine |

## Capabilities at a glance

- **23 d2 shapes** mapped (rectangle, cylinder, cloud, person, hexagon, …)
- **2 layout engines** for `.d2`: dagre (polyline), ELK (orthogonal)
- **4 DSL primitives** for YAML: `group` (translucent containers), `stack`
  (auto-replicate N times), `canvas.grid` (iso ground plane), `annotation`
  (screen-space callouts pinned to a part)
- **3 input formats**: `.d2`, `.yaml`, `.json`
- **Two-tier output**: topology-level SVG + per-element standalone SVG

## License

(none specified yet — add one before public distribution)

## Status

Single-author project. API may evolve; pin via tag if you depend on it.
The published d2 dependency is fixed at `v0.7.1`.
