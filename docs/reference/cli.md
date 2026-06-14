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

The CLI is a thin wrapper over the Go engine. The full library API —
parsing, rendering, the stateless **edit** contract (`ApplyOp` /
`EditOp`), types, and the `yamledit` / `iso25d` sub-packages — lives in
the single **[API reference](api.md)**.
