# isotopo Studio — the live diagram workbench

Studio is the interactive page you get from `isotopo serve` (and, in a
degraded static form, from every `isotopo render` output as
`topology.html`). One file, one canvas, sub-second feedback: the YAML
on the right, the rendered isometric scene on the left, and a live
bidirectional mapping between them.

```bash
isotopo serve path/to/input.yaml     # → http://localhost:8731
ISOTOPO_PORT=9000 isotopo serve …    # pick a port
```

## Why it exists

iso-topology is agent-first: most diagrams here are *written by a
coding agent* against the DSL contract. But the loop only closes when
a human can look at the result, point at a node, and say "make this
one violet". Studio is that pointing device. It grew out of three
design positions:

1. **The original file is sacred.** Everything you edit in Studio is
   an in-browser copy. The file on disk is never written — not on
   re-render, not on save, not ever. Your edits persist as a local
   draft (per absolute path) across reloads, with a one-click
   `revert`. Taking changes home is explicit: `Download` (the edited
   YAML) or the canvas exports.
2. **Source and canvas are the same thing seen twice.** Hover a node
   → its YAML block highlights. Click a node → the selection pins.
   Put the caret inside a block → the node glows. Click a validation
   error → the offending line flashes. If you can see it, you can
   find it — in either direction.
3. **Local-first, zero cloud.** Studio is a single self-contained
   HTML page served by the same static binary that renders your
   diagrams. No accounts, no telemetry, no build step. The only
   network calls are the ones you ask for (loading a gallery example,
   sharing a link).

## The tour

| Area | What it does |
|---|---|
| **Canvas** | Scroll to zoom, drag to pan, double-click resets, `⌘0` fits. The backdrop tints itself after the scene's `canvas.background`, so dark scenes sit in a dark room. Zoom readout bottom-right. |
| **Editor** | Syntax-highlighted YAML over a line-number gutter. Tab indents. The pane is drag-resizable (splitter) and remembers its width. |
| **Render** | `Auto` re-renders (debounced) as you type; `⌘↵` forces it. A failed edit keeps the last good render on canvas with a "showing last good render" badge — the canvas never goes blank. |
| **Issues** | Validation problems appear under the editor with did-you-mean suggestions. Click one to jump to (and flash) the offending line. |
| **Export** | `↓ SVG` / `↓ PNG` (2x) capture exactly what the canvas shows — unsaved edits included — named `<file>.edited.svg` when dirty. |
| **Share** | Copies a permalink with the entire YAML deflate-compressed into the URL hash (`#src=…`). Opening it restores the editor state — works on the static page too, no server needed. |
| **Examples** | Loads any official gallery scene into the editor (fetched from GitHub; your file stays untouched). |
| **Browse nodes** | Per-part gallery — every node as a standalone SVG/YAML fragment, served live from the current file. |

Keyboard map lives behind `?` (or the "all shortcuts" footer link).

## Static degradation

`isotopo render out/` writes the same page as `out/topology.html`.
Opened as a file it keeps everything except live re-render — the
status dot reads "Static file" and the Render button stays disabled
until you point `isotopo serve` at the source. Share links still work.

## Endpoints (for tooling)

| Route | Meaning |
|---|---|
| `GET /` | Studio, freshly rendered from the file on disk |
| `GET /topology.svg` | current render of the file on disk |
| `GET /nodes/_index.html`, `/nodes/<id>.svg\|.yaml\|.html` | per-part gallery |
| `POST /api/render?format=yaml` | render the posted source; returns `{"svg","issues"}` |
| `GET /api/ping` | health |

## How it's tested

`tools/viewer-test/cdp_test.py` drives headless Chrome over CDP and
asserts the whole surface end-to-end (27 checks: mapping, pinning,
zoom, drafts, exports, share round-trip, adaptive stage, error
navigation, static degradation). Run it whenever `output.go`'s
template or the serve handlers change:

```bash
python3 tools/viewer-test/cdp_test.py
```
