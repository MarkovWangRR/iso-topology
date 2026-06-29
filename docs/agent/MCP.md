# MCP server

`isotopo-mcp` exposes the agent loop as Model Context Protocol tools
over stdio, so Claude Code / Claude Desktop / Cursor / any MCP client
can draw isometric diagrams without shelling out to the CLI.

## Install

```bash
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo-mcp@latest
```

Single static binary, no runtime deps — same as the CLI.

## Register

**Claude Code:**

```bash
claude mcp add isotopo -- isotopo-mcp
```

**Claude Desktop / generic MCP client** — add to the client's MCP
config (e.g. `claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "isotopo": {
      "command": "isotopo-mcp"
    }
  }
}
```

(Use the absolute path from `which isotopo-mcp` if your client
doesn't inherit `$GOPATH/bin` on PATH.)

## Tools

| Tool | Arguments | Returns |
|---|---|---|
| `iso_capabilities` | — | the full machine-readable DSL inventory (shapes, primitives, style keys). Call once per session before emitting DSL. |
| `iso_validate` | `dsl` (required), `format: yaml\|d2\|json` | JSONPath-located issues with `suggest` fixes; `isError: true` when any issue is severity `error`. |
| `iso_evaluate` | `dsl` (required), `format` | layout scorecard JSON — edge crossings, node overlaps, edge-through-node tunnelling, bends, overall readability score — from three views (`readability`, `iso`, `plan`). Aim for 0 crossings / 0 tunnels before rendering. |
| `iso_render` | `dsl` (required), `format`, `output_dir` (optional) | validates first (refuses on errors), then writes `topology.svg`, `topology.html`, source copy, and per-element `nodes/<id>.{svg,yaml}`; returns the file paths. Omitting `output_dir` uses a fresh temp dir. |
| `iso_preview` | `dsl` (required), `selectors` (required), `format`, `projection: \|top`, `output_dir` (optional) | crops ONE node / group / `edge:N` and returns its SVG markup, so an agent can inspect a single element up close. `projection: top` gives the flat plan view. |

## The loop, MCP-shaped

1. `iso_capabilities` → learn the DSL surface.
2. Emit YAML following the positioning rules in
   [PROMPT_TEMPLATE.md](PROMPT_TEMPLATE.md) (or install the
   [`draw-iso-diagram` skill](../../skills/draw-iso-diagram/SKILL.md),
   which encodes them).
3. `iso_validate` → apply `suggest` values until `issues` is empty.
4. `iso_evaluate` → fix layout until `crossings` and `tunnels` are 0.
5. `iso_render` → read back `output_dir/topology.svg`.
6. `iso_preview` (optional) → crop a single node or edge to inspect it
   close up without re-reading the whole scene.

## Protocol notes

Minimal, dependency-free implementation: JSON-RPC 2.0 over
newline-delimited stdio per the MCP spec. Supported methods:
`initialize` (echoes the client's `protocolVersion`), `ping`,
`tools/list`, `tools/call`; notifications are accepted and ignored.
Resources and prompts are not implemented — the capability surface
is deliberately tools-only for now.
