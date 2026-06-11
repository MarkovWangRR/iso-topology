package isotopo

import (
	"fmt"
	"html"
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
func TopologyHTML(svg, sourceText, sourceLang, sourceFilename string) string {
	tpl := `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>isotopo · topology</title>
<style>
:root{--bg:#FAFAFB;--fg:#1F2937;--muted:#6B7280;--border:rgba(127,127,127,0.22);--code:#F4F5F7;--accent:#6366F1;}
*{box-sizing:border-box;}
html,body{height:100%;}
body{margin:0;display:flex;flex-direction:column;background:var(--bg);color:var(--fg);font-family:Inter,"Helvetica Neue",Arial,sans-serif;}
header{display:flex;align-items:center;gap:10px;padding:10px 18px;border-bottom:1px solid var(--border);}
header h1{margin:0;font-size:15px;font-weight:650;}
header .tag{padding:1px 7px;border:1px solid var(--border);border-radius:4px;font:11px ui-monospace,Menlo,monospace;color:var(--muted);}
header .spacer{flex:1;}
#live{font:11px ui-monospace,Menlo,monospace;color:var(--muted);}
#live.on{color:#059669;}
.grid{flex:1;min-height:0;display:flex;gap:0;}
.stage-wrap{flex:1.6;min-width:0;position:relative;overflow:hidden;border-right:1px solid var(--border);background:
  conic-gradient(from 90deg at 1px 1px,transparent 90deg,rgba(127,127,127,.06) 0) 0 0/24px 24px;}
#viewport{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;cursor:grab;}
#viewport.panning{cursor:grabbing;}
#zoomer{transform-origin:0 0;}
#zoomer svg{display:block;max-width:none;}
.zoomctl{position:absolute;right:14px;bottom:14px;display:flex;gap:6px;}
.zoomctl button{width:30px;}
.side{flex:1;min-width:340px;display:flex;flex-direction:column;min-height:0;}
.toolbar{display:flex;gap:8px;align-items:center;padding:10px 12px;border-bottom:1px solid var(--border);flex-wrap:wrap;}
button{font:12px ui-monospace,Menlo,monospace;border:1px solid var(--border);background:white;padding:6px 10px;border-radius:6px;cursor:pointer;}
button:hover{background:#F1F3F8;}
button:disabled{opacity:.45;cursor:default;}
label.auto{font:11px ui-monospace,Menlo,monospace;color:var(--muted);display:flex;gap:4px;align-items:center;}
.editor{flex:1;min-height:0;position:relative;font:12.5px/1.55 ui-monospace,Menlo,Consolas,monospace;}
.editor .hl,.editor textarea{position:absolute;inset:0;margin:0;padding:14px;white-space:pre;overflow:auto;font:inherit;tab-size:2;}
.editor .hl{color:transparent;background:var(--code);pointer-events:none;}
.editor .hl .ln{min-height:1.55em;}
.editor .hl .hit{background:rgba(99,102,241,0.18);box-shadow:-3px 0 0 var(--accent);}
.editor textarea{background:transparent;border:0;resize:none;color:var(--fg);outline:none;width:100%;height:100%;}
#issues{max-height:130px;overflow:auto;border-top:1px solid var(--border);font:11.5px/1.5 ui-monospace,Menlo,monospace;padding:8px 12px;display:none;}
#issues.show{display:block;}
#issues .err{color:#DC2626;}
#issues .warn{color:#D97706;}
footer{padding:7px 18px;border-top:1px solid var(--border);font-size:11.5px;color:var(--muted);}
footer a{color:var(--muted);}
svg g[data-part-id]{transition:filter .12s;}
svg g[data-part-id].hi{filter:drop-shadow(0 0 5px rgba(99,102,241,.85));}
</style></head><body>
<header>
  <svg width="26" height="26" viewBox="0 0 32 32" aria-label="iso-topology">
    <g stroke-linejoin="round">
      <polygon points="16,4 27,10 16,16 5,10" fill="#7C8CF8"/>
      <polygon points="5,10 16,16 16,29 5,23" fill="#4F46E5"/>
      <polygon points="27,10 16,16 16,29 27,23" fill="#6366F1"/>
      <polygon points="16,10 21,13 16,16 11,13" fill="#EEF2FF"/>
    </g>
  </svg>
  <h1>iso-topology</h1><span class="tag">{{LANG}}</span>
  <span class="spacer"></span>
  <span id="live">checking renderer…</span>
</header>
<div class="grid">
  <div class="stage-wrap">
    <div id="viewport"><div id="zoomer">{{SVG}}</div></div>
    <div class="zoomctl">
      <button onclick="zoomBy(1.25)">+</button>
      <button onclick="zoomBy(0.8)">−</button>
      <button onclick="resetView()" title="reset">⤾</button>
    </div>
  </div>
  <div class="side">
    <div class="toolbar">
      <button id="render" onclick="rerender()" title="Cmd/Ctrl+Enter">re-render</button>
      <label class="auto"><input type="checkbox" id="auto" checked>auto</label>
      <span class="spacer" style="flex:1"></span>
      <button onclick="copySrc()">copy</button>
      <button onclick="downloadCopy()">save edited copy</button>
    </div>
    <div class="editor">
      <div class="hl" id="hl"></div>
      <textarea id="src" spellcheck="false">{{SRC}}</textarea>
    </div>
    <div id="issues"></div>
  </div>
</div>
<footer>
  hover a node to locate its source · scroll to zoom, drag to pan, double-click to reset ·
  edits live in this page (a copy — the original file is never touched) ·
  <a href="./topology.svg" target="_blank">open svg</a> · <a href="./nodes/_index.html">browse nodes</a>
</footer>
<script>
"use strict";
const LANG={{LANGQ}}, FILENAME={{FILEQ}};
const srcEl=document.getElementById('src'), hlEl=document.getElementById('hl');
const zoomer=document.getElementById('zoomer'), viewport=document.getElementById('viewport');

/* ── editor backdrop + SVG↔source hover map ────────────────────── */
let lineMap={};
function esc(t){return t.replace(/&/g,'&amp;').replace(/</g,'&lt;');}
function buildMap(){
  const lines=srcEl.value.split('\n'); lineMap={};
  const idRe=/(?:^|[\s{])id:\s*"?([A-Za-z0-9_~-]+)/;
  for(let i=0;i<lines.length;i++){
    const m=lines[i].match(idRe); if(!m) continue;
    let start=i;
    if(!/^\s*-/.test(lines[i])){
      for(let k=i-1;k>=0;k--){
        if(/^\s*-\s/.test(lines[k])){start=k;break;}
        if(lines[k].trim() && lines[k].search(/\S/)===0) break;
      }
    }
    const ind=lines[start].search(/\S/);
    let end=lines.length-1;
    for(let k=start+1;k<lines.length;k++){
      const t=lines[k]; if(!t.trim()) continue;
      const ki=t.search(/\S/);
      if(ki<ind || (ki===ind && /^\s*-\s/.test(t)) || ki===0){end=k-1;break;}
    }
    if(!(m[1] in lineMap)) lineMap[m[1]]=[start,end];
  }
}
function paint(range){
  const lines=srcEl.value.split('\n');
  hlEl.innerHTML=lines.map((l,i)=>{
    const hit=range&&i>=range[0]&&i<=range[1];
    return '<div class="ln'+(hit?' hit':'')+'">'+esc(l||' ')+'</div>';
  }).join('');
  hlEl.scrollTop=srcEl.scrollTop; hlEl.scrollLeft=srcEl.scrollLeft;
}
srcEl.addEventListener('scroll',()=>{hlEl.scrollTop=srcEl.scrollTop;hlEl.scrollLeft=srcEl.scrollLeft;});
function wireHover(){
  zoomer.querySelectorAll('g[data-part-id]').forEach(g=>{
    g.addEventListener('mouseenter',()=>{
      g.classList.add('hi');
      let id=g.getAttribute('data-part-id');
      if(!(id in lineMap)) id=id.replace(/~\d+$/,'');
      const r=lineMap[id]; if(!r) return;
      paint(r);
      const lh=srcEl.scrollHeight/srcEl.value.split('\n').length;
      srcEl.scrollTop=Math.max(0,r[0]*lh-60);
      hlEl.scrollTop=srcEl.scrollTop;
    });
    g.addEventListener('mouseleave',()=>{g.classList.remove('hi');paint(null);});
  });
}

/* ── zoom & pan ─────────────────────────────────────────────────── */
let scale=1,panX=0,panY=0;
function apply(){zoomer.style.transform='translate('+panX+'px,'+panY+'px) scale('+scale+')';}
function zoomBy(f){scale=Math.min(8,Math.max(0.2,scale*f));apply();}
function resetView(){scale=1;panX=0;panY=0;apply();}
viewport.addEventListener('wheel',e=>{
  e.preventDefault();
  const f=e.deltaY<0?1.1:0.9, r=viewport.getBoundingClientRect();
  const mx=e.clientX-r.left, my=e.clientY-r.top;
  panX=mx-(mx-panX)*f; panY=my-(my-panY)*f;
  scale=Math.min(8,Math.max(0.2,scale*f)); apply();
},{passive:false});
let drag=null;
viewport.addEventListener('mousedown',e=>{drag={x:e.clientX-panX,y:e.clientY-panY};viewport.classList.add('panning');});
window.addEventListener('mousemove',e=>{if(!drag)return;panX=e.clientX-drag.x;panY=e.clientY-drag.y;apply();});
window.addEventListener('mouseup',()=>{drag=null;viewport.classList.remove('panning');});
viewport.addEventListener('dblclick',resetView);

/* ── live re-render against isotopo serve ───────────────────────── */
const liveEl=document.getElementById('live'), renderBtn=document.getElementById('render');
let serverOK=false, timer=null;
async function probe(){
  if(!location.protocol.startsWith('http')){
    liveEl.textContent='static file — run "isotopo serve <input>" for live re-render';
    renderBtn.disabled=true; return;
  }
  try{
    const r=await fetch('/api/ping');
    serverOK=r.ok;
  }catch(_){serverOK=false;}
  liveEl.textContent=serverOK?'live · renderer connected':'renderer unreachable';
  liveEl.classList.toggle('on',serverOK);
  renderBtn.disabled=!serverOK;
}
async function rerender(){
  if(!serverOK) return;
  renderBtn.textContent='rendering…';
  try{
    const r=await fetch('/api/render?format='+encodeURIComponent(LANG),{method:'POST',body:srcEl.value});
    const data=await r.json();
    showIssues(data.issues||[]);
    if(data.svg){
      zoomer.innerHTML=data.svg;
      buildMap(); paint(null); wireHover();
    }
  }catch(e){showIssues([{severity:'error',path:'$',message:String(e)}]);}
  renderBtn.textContent='re-render';
}
function showIssues(list){
  const el=document.getElementById('issues');
  if(!list.length){el.classList.remove('show');el.innerHTML='';return;}
  el.classList.add('show');
  el.innerHTML=list.map(i=>'<div class="'+(i.severity==='error'?'err':'warn')+'">'+
    esc(i.severity+' '+(i.path||'')+' — '+i.message+(i.suggest?' (did you mean '+i.suggest+'?)':''))+'</div>').join('');
}
srcEl.addEventListener('input',()=>{
  buildMap(); paint(null);
  if(document.getElementById('auto').checked && serverOK){
    clearTimeout(timer); timer=setTimeout(rerender,600);
  }
});
window.addEventListener('keydown',e=>{
  if((e.metaKey||e.ctrlKey)&&e.key==='Enter'){e.preventDefault();rerender();}
});

/* ── misc ───────────────────────────────────────────────────────── */
function copySrc(){navigator.clipboard.writeText(srcEl.value);}
function downloadCopy(){
  const blob=new Blob([srcEl.value],{type:'text/plain'});
  const a=document.createElement('a');
  a.href=URL.createObjectURL(blob);
  a.download=FILENAME.replace(/(\.[a-z0-9]+)$/i,'.edited$1');
  a.click();
}
buildMap(); paint(null); wireHover(); probe();
</script>
</body></html>`
	r := strings.NewReplacer(
		"{{LANG}}", html.EscapeString(sourceLang),
		"{{SVG}}", svg,
		"{{SRC}}", html.EscapeString(sourceText),
		"{{LANGQ}}", fmt.Sprintf("%q", sourceLang),
		"{{FILEQ}}", fmt.Sprintf("%q", sourceFilename),
	)
	return r.Replace(tpl)
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
