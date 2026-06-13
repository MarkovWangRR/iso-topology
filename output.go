package isotopo

import (
	_ "embed"
	"fmt"
	"html"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PartFragments returns one round-trip-able mini-Document per atomic
// part — in the same id-space as RenderParts. Each fragment is itself a
// valid isotopo Document with a single top-level node, so the user can
// drop it in a file and re-render it standalone via `isotopo render`.
// For composite scenes we lift each CompositePart up into its own Node
// (recursing into groups); for non-composite docs each top-level Node
// is its own fragment.
func PartFragments(doc *Document) map[string]*Document {
	out := make(map[string]*Document)
	for id, n := range doc.Nodes {
		if n.Shape != "composite" {
			out[id] = &Document{Theme: doc.Theme, Nodes: map[string]*Node{id: n}}
			continue
		}
		walkAtomicParts(n.Parts, func(p *CompositePart) {
			sub := &Node{
				Shape:   p.Shape,
				Geom:    p.Geom,
				Style:   p.Style,
				Label:   p.Label,
				Icon:    p.Icon,
				Content: p.Content,
			}
			out[p.ID] = &Document{
				Theme: doc.Theme,
				Nodes: map[string]*Node{p.ID: sub},
			}
		})
	}
	return out
}

// MarshalFragmentYAML serializes a per-part fragment Document to YAML
// bytes suitable for writing to disk and re-rendering standalone.
func MarshalFragmentYAML(d *Document) ([]byte, error) {
	return yaml.Marshal(d)
}

// PartIDs returns the sorted id list of every atomic part in the
// document, matching the keys produced by RenderParts / PartFragments.
// v2 — recurses into groups so children show up in the gallery.
func PartIDs(doc *Document) []string {
	seen := map[string]struct{}{}
	for id, n := range doc.Nodes {
		if n.Shape != "composite" {
			seen[id] = struct{}{}
			continue
		}
		walkAtomicParts(n.Parts, func(p *CompositePart) {
			seen[p.ID] = struct{}{}
		})
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// TopologyHTML wraps the topology SVG together with embed-ready snippets
// and the editable source DSL. The HTML is self-contained: a single
// file the user can open, copy embed code from, or hand to a designer.
//
// sourceLang is "yaml", "json", or "d2" — used only to label the source
// block. sourceText is the original DSL bytes; we drop them into a
// <textarea> with copy / download buttons so the user can iterate on
// the topology without losing fidelity to what produced this SVG.
//
//go:embed studio/studio.html
var studioShell string

//go:embed studio/studio.css
var studioCSS string

//go:embed studio/studio.js
var studioJS string

func TopologyHTML(svg, sourceText, sourceLang, sourceFilename string) string {
	// Assemble the Studio page from three embedded assets (studio/*.{html,css,js}):
	// first inline the CSS/JS into the shell, then fill in the per-render data.
	page := strings.NewReplacer("{{CSS}}", studioCSS, "{{JS}}", studioJS).Replace(studioShell)
	base := filepath.Base(sourceFilename)
	return strings.NewReplacer(
		"{{LANG}}", html.EscapeString(sourceLang),
		"{{SVG}}", svg,
		"{{SRC}}", html.EscapeString(sourceText),
		"{{LANGQ}}", fmt.Sprintf("%q", sourceLang),
		"{{PATHQ}}", fmt.Sprintf("%q", sourceFilename),
		"{{FILE}}", html.EscapeString(base),
	).Replace(page)
}

// NodeHTML wraps a single per-part SVG with its embed snippet and DSL
// fragment. The fragment is editable in the same way as the topology
// page — drop it into a YAML doc and render it standalone.
func NodeHTML(id, svg, yamlFragment string) string {
	bg := "#FAFAFB"
	var sb strings.Builder
	fmt.Fprintf(&sb, `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>isotopo · %s</title>
<style>
:root{--bg:%s;--fg:#1F2937;--muted:#6B7280;--border:rgba(127,127,127,0.22);--code:#F4F5F7;}
*{box-sizing:border-box;}
body{margin:0;padding:32px;background:var(--bg);color:var(--fg);font-family:Inter,"Helvetica Neue",Arial,sans-serif;}
h1{margin:0 0 4px;font-size:18px;font-family:ui-monospace,Menlo,monospace;}
h2{margin:24px 0 8px;font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:var(--muted);}
.grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:24px;}
.panel{border:1px solid var(--border);border-radius:10px;background:rgba(255,255,255,0.6);}
.stage{display:flex;align-items:center;justify-content:center;padding:16px;min-height:260px;}
.stage svg{max-width:100%%;height:auto;}
pre,textarea{font:12.5px/1.55 ui-monospace,Menlo,Consolas,monospace;background:var(--code);border:0;border-radius:8px;padding:12px;width:100%%;}
textarea{min-height:240px;resize:vertical;color:var(--fg);}
.row{display:flex;gap:8px;align-items:center;margin:6px 0;}
button{font:12px ui-monospace,Menlo,monospace;border:1px solid var(--border);background:white;padding:6px 10px;border-radius:6px;cursor:pointer;}
button:hover{background:#F1F3F8;}
small{color:var(--muted);}
nav{margin:-12px 0 16px;}
nav a{color:var(--muted);text-decoration:none;border-bottom:1px dotted var(--muted);}
</style></head><body>
<nav><a href="./_index.html">← all nodes</a> · <a href="../topology.html">topology</a></nav>
<h1>%s</h1>
<small>standalone iso element · part of the topology</small>

<div class="grid" style="margin-top:18px;">
  <div class="panel stage">%s</div>
  <div>
    <h2>embed</h2>
    <pre id="embed">&lt;img src="./%s.svg" alt="%s"/&gt;</pre>
    <div class="row"><button onclick="copy('embed')">copy &lt;img&gt;</button></div>

    <h2>fragment · yaml</h2>
    <textarea id="frag" spellcheck="false">%s</textarea>
    <div class="row">
      <button onclick="copy('frag')">copy</button>
      <small>standalone DSL — render with <code>isotopo render %s.yaml out/</code>.</small>
    </div>
  </div>
</div>
<script>
function copy(id){
  const el=document.getElementById(id);
  const text=el.value!==undefined?el.value:el.innerText;
  navigator.clipboard.writeText(text);
}
</script>
</body></html>`,
		html.EscapeString(id),
		bg,
		html.EscapeString(id),
		svg,
		html.EscapeString(id),
		html.EscapeString(id),
		html.EscapeString(yamlFragment),
		html.EscapeString(id),
	)
	return sb.String()
}

// NodesIndexHTML is a tiny gallery linking to every per-part page so
// users can browse the parts of a topology like a sticker sheet.
func NodesIndexHTML(ids []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `<!doctype html>
<html><head><meta charset="utf-8"><title>isotopo · nodes</title>
<style>
body{margin:0;padding:32px;background:#FAFAFB;font-family:Inter,Arial,sans-serif;color:#1F2937;}
h1{margin:0 0 4px;font-size:18px;}
small{color:#6B7280;}
nav{margin:8px 0 18px;}
nav a{color:#6B7280;text-decoration:none;border-bottom:1px dotted #6B7280;}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:14px;}
.card{display:block;background:white;border:1px solid rgba(127,127,127,0.22);border-radius:10px;padding:12px;text-decoration:none;color:inherit;}
.card:hover{border-color:#3B82F6;}
.card h3{margin:0 0 6px;font:11px ui-monospace,Menlo,monospace;opacity:0.72;}
.stage{display:flex;align-items:center;justify-content:center;min-height:170px;}
.stage img{max-width:100%%;max-height:180px;}
</style></head><body>
<nav><a href="../topology.html">← topology</a></nav>
<h1>nodes</h1>
<small>%d standalone iso elements · click a card to copy embed code</small>
<div class="grid" style="margin-top:14px;">
`, len(ids))
	for _, id := range ids {
		fmt.Fprintf(&sb,
			`<a class="card" href="./%s.html"><h3>%s</h3><div class="stage"><img src="./%s.svg" alt="%s"/></div></a>`+"\n",
			id, html.EscapeString(id), id, html.EscapeString(id),
		)
	}
	sb.WriteString(`</div></body></html>`)
	return sb.String()
}
