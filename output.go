package isotopo

import (
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
func TopologyHTML(svg, sourceText, sourceLang, sourceFilename string) string {
	tpl := `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>isotopo Studio · {{FILE}}</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 68 56'%3E%3Cpolygon points='4,34 22,24 40,34 22,44' fill='%232FA9B8' opacity='.35'/%3E%3Cpolygon points='4,30 22,20 40,30 22,40' fill='%2336BCC6' opacity='.65'/%3E%3Cpolygon points='4,26 22,16 40,26 22,36' fill='%234ED2D9'/%3E%3Cpath d='M38 30 C46 34 44 40 50 42' fill='none' stroke='%233CC4CC' stroke-width='5' stroke-linecap='round'/%3E%3Cpolygon points='46,36 56,30 66,36 56,42' fill='%238FE9EC'/%3E%3Cpolygon points='46,36 56,42 56,54 46,48' fill='%232596A6'/%3E%3Cpolygon points='66,36 56,42 56,54 66,48' fill='%2349CDD6'/%3E%3C/svg%3E">
<style>
:root{
  --bg:#F6F8FA;--panel:#FFFFFF;--fg:#0F172A;--muted:#64748B;
  --border:#E6E9F0;--accent:#10AEB9;--accent-deep:#0B8C97;
  --accent-soft:rgba(16,174,185,.10);--code-bg:#FBFCFE;
  --shadow:0 1px 2px rgba(15,23,42,.04),0 10px 28px -14px rgba(15,23,42,.14);
}
*{box-sizing:border-box;}
html,body{height:100%;}
body{margin:0;display:flex;flex-direction:column;background:var(--bg);color:var(--fg);
  font-family:Inter,-apple-system,"SF Pro Text","Helvetica Neue",Arial,sans-serif;
  -webkit-font-smoothing:antialiased;}
header{display:flex;align-items:center;gap:12px;padding:11px 20px;
  background:rgba(255,255,255,.82);backdrop-filter:blur(14px) saturate(1.4);
  border-bottom:1px solid var(--border);position:relative;z-index:5;}
.brand{display:flex;align-items:center;gap:11px;}
.brand .word{display:flex;flex-direction:column;line-height:1.15;}
.brand h1{margin:0;font-size:14.5px;font-weight:650;letter-spacing:-.01em;display:flex;align-items:center;gap:7px;}
.studio{font:9px Inter,sans-serif;font-weight:700;letter-spacing:.14em;color:var(--accent-deep);
  background:var(--accent-soft);padding:2.5px 6px;border-radius:4px;}
.brand .sub{font-size:10.5px;color:var(--muted);}
.pagedesc{font:11px Inter,sans-serif;color:var(--muted);
  margin-left:4px;padding-left:14px;border-left:1px solid var(--border);}
header .spacer{flex:1;}
#live{display:flex;align-items:center;gap:6px;font:11.5px Inter,sans-serif;font-weight:550;color:var(--muted);}
#live::before{content:"";width:7px;height:7px;border-radius:50%;background:#CBD5E1;flex:none;}
#live.on{color:var(--accent-deep);}
#live.on::before{background:var(--accent);animation:pulse 2.2s ease-in-out infinite;}
@keyframes pulse{0%,100%{box-shadow:0 0 0 0 rgba(16,174,185,.35)}50%{box-shadow:0 0 0 5px rgba(16,174,185,0)}}
.grid{flex:1;min-height:0;display:flex;}
.stage-wrap{flex:1.6;min-width:0;position:relative;overflow:hidden;
  background:var(--stage-bg,#F2F4F8) radial-gradient(circle,var(--stage-dot,rgba(15,23,42,.075)) 1px,transparent 1.2px);
  background-size:22px 22px;transition:background-color .35s;}
#viewport{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;cursor:grab;}
#viewport.panning{cursor:grabbing;}
#zoomer{transform-origin:0 0;filter:drop-shadow(0 18px 30px rgba(15,23,42,.10));}
#zoomer svg{display:block;max-width:none;}
.exportctl{position:absolute;right:16px;top:14px;display:flex;gap:1px;
  background:white;border:1px solid var(--border);border-radius:8px;overflow:hidden;box-shadow:var(--shadow);}
.exportctl button{border:0;border-radius:0;background:white;font:11px Inter,sans-serif;font-weight:550;
  color:#334155;padding:7px 12px;}
.exportctl button:hover{background:var(--accent-soft);color:var(--accent-deep);}
.exportctl button+button{border-left:1px solid var(--border);}
.zoomctl{position:absolute;right:16px;bottom:16px;display:flex;flex-direction:column;gap:1px;
  background:white;border:1px solid var(--border);border-radius:8px;overflow:hidden;box-shadow:var(--shadow);}
.zoomctl button{width:34px;height:32px;border:0;border-radius:0;background:white;font-size:14px;color:#334155;}
.zoomctl #zpct{width:46px;font:10.5px Inter,sans-serif;font-weight:600;color:var(--muted);}
.zoomctl button:hover{background:var(--accent-soft);color:var(--accent-deep);}
.zoomctl button+button{border-top:1px solid var(--border);}
.splitter{flex:none;width:7px;margin:0 -3px;cursor:col-resize;position:relative;z-index:6;}
.splitter::after{content:"";position:absolute;left:3px;top:0;bottom:0;width:1px;background:transparent;transition:background .15s;}
.splitter:hover::after,.splitter.active::after{background:var(--accent);}
.side{flex:1;min-width:320px;max-width:560px;display:flex;flex-direction:column;min-height:0;
  background:var(--panel);border-left:1px solid var(--border);}
.toolbar{display:flex;gap:8px;align-items:center;padding:10px 14px;border-bottom:1px solid var(--border);flex-wrap:wrap;}
button{font:12px Inter,sans-serif;font-weight:550;border:1px solid var(--border);background:white;
  color:#334155;padding:7px 13px;border-radius:6px;cursor:pointer;
  transition:background .15s,border-color .15s,box-shadow .15s;}
button:hover{background:#F4F6FA;border-color:#D6DAE3;}
button:focus-visible{outline:2px solid var(--accent);outline-offset:1px;}
button:disabled{opacity:.45;cursor:default;}
#render{background:var(--accent);border-color:transparent;color:white;}
#render:hover{background:var(--accent-deep);}
label.auto{font:11.5px Inter,sans-serif;color:var(--muted);display:flex;gap:5px;align-items:center;}
label.auto input{accent-color:var(--accent);}
.filetab{display:flex;align-items:center;gap:8px;padding:8px 16px;border-bottom:1px solid var(--border);
  background:var(--code-bg);font:11px ui-monospace,Menlo,monospace;color:var(--muted);}
.filetab b{color:#334155;font-weight:600;flex:none;}
.filetab .path{overflow:hidden;white-space:nowrap;min-width:0;flex:0 1 auto;margin-right:-6px;}
.filetab .iconbtn{border:0;background:transparent;padding:3px;min-width:0;display:flex;align-items:center;
  color:#9AA4B5;border-radius:4px;cursor:pointer;flex:none;}
.filetab .iconbtn:hover{background:var(--accent-soft);color:var(--accent-deep);}
.filetab .dot{width:7px;height:7px;border-radius:50%;border:1.5px solid #C2C9D6;background:transparent;flex:none;}
.filetab .dot.on{border-color:#F59E0B;background:#F59E0B;}
.editor{flex:1;min-height:0;position:relative;font:11.5px/18px ui-monospace,Menlo,Consolas,monospace;}
.editor .hl,.editor textarea{position:absolute;inset:0;margin:0;padding:14px 16px 14px 60px;white-space:pre;overflow:auto;font:inherit;tab-size:2;}
.editor .gutter{position:absolute;top:0;bottom:0;left:0;width:46px;overflow:hidden;
  padding:14px 0;background:var(--code-bg);border-right:1px solid var(--border);
  color:#B3BCCA;font-size:10px;text-align:right;pointer-events:none;}
.editor .gutter div{height:18px;line-height:18px;padding-right:10px;}
.editor textarea::-webkit-scrollbar{width:10px;height:10px;}
.editor textarea::-webkit-scrollbar-thumb{background:#D5DBE5;border-radius:5px;border:2px solid var(--code-bg);}
.editor textarea::-webkit-scrollbar-thumb:hover{background:#BCC5D2;}
.editor textarea::-webkit-scrollbar-track{background:transparent;}
.editor textarea::-webkit-scrollbar-corner{background:transparent;}
.editor .hl{color:#334155;background:var(--code-bg);pointer-events:none;}
.tk-k{color:#0F172A;font-weight:600;}
.tk-s{color:#0E9AA5;}
.tk-u{color:#7C3AED;}
.tk-n{color:#B45309;}
.tk-b{color:#7C3AED;}
.tk-c{color:#94A3B8;font-style:italic;}
.tk-d{color:#94A3B8;}
.hl .flash{animation:flashln 1.4s ease-out;}
@keyframes flashln{0%{background:rgba(245,158,11,.35)}100%{background:transparent}}
.editor .hl .ln{min-height:18px;}
.editor .hl .hit{background:var(--accent-soft);box-shadow:inset 3px 0 0 var(--accent);}
.editor .hl .hit-a{border-top-right-radius:6px;}
.editor .hl .hit-b{border-bottom-right-radius:6px;}
.editor textarea{background:transparent;border:0;resize:none;color:transparent;outline:none;width:100%;height:100%;caret-color:var(--accent-deep);}
.editor textarea::selection{background:rgba(16,174,185,.22);}
#issues{max-height:140px;overflow:auto;border-top:1px solid var(--border);font:11.5px/1.6 ui-monospace,Menlo,monospace;
  padding:10px 16px;display:none;background:#FFF9F5;}
#issues.show{display:block;}
#issues .err{color:#DC2626;}
#issues .warn{color:#D97706;}
#issues .err,#issues .warn{cursor:pointer;}
#issues .err:hover,#issues .warn:hover{text-decoration:underline;}
footer{display:flex;gap:18px;align-items:center;padding:8px 20px;border-top:1px solid var(--border);
  font-size:11px;color:var(--muted);background:rgba(255,255,255,.7);}
footer a{color:var(--accent-deep);text-decoration:none;}
footer a:hover{text-decoration:underline;}
kbd{font:10px ui-monospace,Menlo,monospace;border:1px solid var(--border);border-bottom-width:2px;
  border-radius:4px;padding:1px 5px;background:white;color:#475569;}
#help[hidden]{display:none;}
#help{position:fixed;inset:0;background:rgba(15,23,42,.35);z-index:50;display:flex;
  align-items:center;justify-content:center;backdrop-filter:blur(3px);}
.help-card{background:white;border-radius:14px;box-shadow:0 24px 64px -16px rgba(15,23,42,.35);
  padding:22px 26px;min-width:340px;}
.help-card h2{margin:0 0 14px;font-size:14px;font-weight:650;}
.hrow{display:flex;justify-content:space-between;gap:40px;font:12px Inter,sans-serif;
  color:#475569;padding:6px 0;border-top:1px solid var(--border);}
#render{min-width:78px;}
.filetab #discard{color:var(--accent-deep);cursor:pointer;font-family:Inter,sans-serif;font-size:10.5px;white-space:nowrap;}
.filetab #discard:hover{text-decoration:underline;}
.stale{position:absolute;top:14px;left:50%;transform:translateX(-50%);z-index:4;
  background:#FFF7ED;border:1px solid #FDBA74;color:#9A3412;
  font:11px Inter,sans-serif;padding:5px 13px;border-radius:999px;box-shadow:var(--shadow);}
svg g[data-part-id]{transition:filter .12s;}
svg g[data-part-id].hi{filter:drop-shadow(0 0 3px rgba(16,174,185,.85));}
svg g[data-part-id]{transition:filter .12s;}
svg g[data-part-id]:hover{filter:drop-shadow(0 0 5px var(--accent));}
svg g[data-part-id].dragging{filter:drop-shadow(0 0 9px var(--accent));opacity:.82;}
svg path[data-connector]:hover{stroke:var(--accent-deep);filter:drop-shadow(0 0 3px var(--accent)) drop-shadow(0 0 7px var(--accent));}
svg path[data-connector].dragging{stroke:var(--accent);filter:drop-shadow(0 0 5px var(--accent)) drop-shadow(0 0 11px var(--accent));}
</style></head><body>
<header>
  <div class="brand">
    <svg width="34" height="30" viewBox="0 0 68 56" aria-label="iso-topology">
      <defs>
        <linearGradient id="lgcube" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="#52D7DE"/><stop offset="1" stop-color="#2FA9B8"/>
        </linearGradient>
      </defs>
      <!-- stacked plates -->
      <polygon points="4,26 22,16 40,26 22,36" fill="#2FA9B8" opacity=".35" transform="translate(0,8)"/>
      <polygon points="4,26 22,16 40,26 22,36" fill="#36BCC6" opacity=".65" transform="translate(0,4)"/>
      <polygon points="4,26 22,16 40,26 22,36" fill="#4ED2D9"/>
      <!-- flow into the cube -->
      <path d="M38 30 C46 34 44 40 50 42" fill="none" stroke="#3CC4CC" stroke-width="5" stroke-linecap="round"/>
      <!-- cube -->
      <polygon points="46,36 56,30 66,36 56,42" fill="#8FE9EC"/>
      <polygon points="46,36 56,42 56,54 46,48" fill="#2596A6"/>
      <polygon points="66,36 56,42 56,54 66,48" fill="url(#lgcube)"/>
    </svg>
    <div class="word">
      <h1>iso-topology <span class="studio">STUDIO</span></h1>
      <span class="sub">isometric diagrams as code</span>
    </div>
  </div>
  <span class="pagedesc">Preview &middot; Edit &middot; Export</span>
  <span class="spacer"></span>
  <span id="live">checking renderer…</span>
</header>
<div class="grid">
  <div class="stage-wrap" id="stage">
    <div id="viewport"><div id="zoomer">{{SVG}}</div></div>
    <div id="stale" class="stale" hidden>showing last good render</div>
    <div class="exportctl">
      <button onclick="exportSVG()" title="download exactly what the canvas shows (last good render if the source is broken)">&#8595; SVG</button>
      <button onclick="exportPNG()" title="download the current render as PNG (2x)">&#8595; PNG</button>
    </div>
    <div class="zoomctl">
      <button onclick="zoomBy(1.25)" title="zoom in (⌘+)">+</button>
      <button onclick="zoomBy(0.8)" title="zoom out (⌘−)">−</button>
      <button id="zfit" onclick="fitView()" title="fit to window (⌘0)">⤢</button>
      <button id="zpct" onclick="resetView()" title="reset to 100%">100%</button>
    </div>
  </div>
  <div class="splitter" id="split" title="drag to resize the editor"></div>
  <div class="side">
    <div class="toolbar">
      <button id="render" onclick="rerender()" title="Cmd/Ctrl+Enter">Render</button>
      <label class="auto"><input type="checkbox" id="auto" checked>Auto</label>
      <span class="spacer" style="flex:1"></span>
      <button id="copybtn" onclick="copySrc()" title="copy the YAML to the clipboard">Copy</button>
      <button id="dl" onclick="downloadCopy()" disabled title="download the edited YAML as a new file">Download</button>
      <button id="share" onclick="shareLink()" title="copy a permalink with the YAML embedded in the URL">Share</button>
    </div>
    <div class="filetab"><span class="dot" id="dirty" title="unsaved edits stay in this page; the file on disk is never written"></span><span class="path" id="fpath"></span><b>{{FILE}}</b><button class="iconbtn" id="cppath" onclick="copyPath()" title="copy full path"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg></button><span class="spacer" style="flex:1"></span><a id="discard" hidden onclick="discardDraft()">revert</a></div>
    <div class="editor">
      <div class="hl" id="hl"></div>
      <div class="gutter" id="gut"></div>
      <textarea id="src" spellcheck="false">{{SRC}}</textarea>
    </div>
    <div id="issues"></div>
  </div>
</div>
<div id="help" hidden>
  <div class="help-card">
    <h2>Keyboard shortcuts</h2>
    <div class="hrow"><span>Re-render</span><span><kbd>⌘</kbd>+<kbd>↵</kbd></span></div>
    <div class="hrow"><span>Fit to window</span><span><kbd>⌘</kbd>+<kbd>0</kbd></span></div>
    <div class="hrow"><span>Zoom in / out</span><span><kbd>⌘</kbd>+<kbd>+</kbd> / <kbd>⌘</kbd>+<kbd>−</kbd></span></div>
    <div class="hrow"><span>Indent</span><span><kbd>Tab</kbd></span></div>
    <div class="hrow"><span>This panel</span><span><kbd>?</kbd></span></div>
    <div class="hrow"><span>Pin a node</span><span>click it on the canvas</span></div>
    <div class="hrow"><span>Jump to an error</span><span>click it in the issues panel</span></div>
  </div>
</div>
<footer>
  <span>Hover a node to jump to its source &middot; click to pin</span>
  <span>Scroll to zoom · drag to pan · double-click to reset</span>
  <span><kbd>⌘</kbd>+<kbd>↵</kbd> render &middot; <a onclick="toggleHelp()" style="cursor:pointer">all shortcuts</a></span>
  <span class="spacer" style="flex:1"></span>
  <a href="./topology.svg" target="_blank">Original SVG</a>
  <a href="./nodes/_index.html">Browse nodes</a>
  <a href="https://github.com/MarkovWangRR/iso-topology/blob/main/docs/guides/studio.md" target="_blank">About Studio</a>
</footer>
<script>
"use strict";
const LANG={{LANGQ}}, PATH={{PATHQ}};
const FILENAME=PATH.split('/').pop();
const srcEl=document.getElementById('src'), hlEl=document.getElementById('hl'), gutEl=document.getElementById('gut');
const ORIGINAL=srcEl.value, DRAFTKEY='isotopo-draft:'+PATH;
const staleEl=document.getElementById('stale');
let dirty=false;
function setDirty(d){
  dirty=d;
  document.getElementById('dirty').classList.toggle('on',d);
  document.getElementById('discard').hidden=!d;
  document.getElementById('dl').disabled=!d;
}
function discardDraft(){
  try{localStorage.removeItem(DRAFTKEY);}catch(_){}
  if(location.hash.indexOf('#src=')===0){
    try{history.replaceState(null,'',location.pathname+location.search);}catch(_){}
  }
  srcEl.value=ORIGINAL; setDirty(false);
  buildMap(); paint(null);
  if(serverOK) rerender();
}
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
/* one-line YAML tokenizer: comments, keys, strings (iso:// URIs get
   their own class), numbers, booleans, list dashes. Escapes as it goes
   so layout stays byte-identical with the textarea overlay. */
function tokLine(l){
  if(!l) return ' ';
  let out='';
  const km=l.match(/^(\s*)(- )?([A-Za-z_][\w.-]*)(:)(?=\s|$)/);
  let rest=l;
  if(km){
    out+=esc(km[1])+(km[2]?'<span class="tk-d">- </span>':'')+
      '<span class="tk-k">'+esc(km[3])+'</span>:';
    rest=l.slice(km[0].length);
  }else{
    const dm=l.match(/^(\s*)(- )/);
    if(dm){out+=esc(dm[1])+'<span class="tk-d">- </span>';rest=l.slice(dm[0].length);}
    else rest=l;
  }
  const re=/("(?:[^"\\]|\\.)*"?)|(#.*$)|(\b(?:true|false|null)\b)|(-?\d+(?:\.\d+)?\b)/g;
  let last=0,m;
  while((m=re.exec(rest))){
    out+=esc(rest.slice(last,m.index));
    if(m[1])      out+='<span class="'+(m[1].indexOf('iso://')>=0?'tk-u':'tk-s')+'">'+esc(m[1])+'</span>';
    else if(m[2]) out+='<span class="tk-c">'+esc(m[2])+'</span>';
    else if(m[3]) out+='<span class="tk-b">'+esc(m[3])+'</span>';
    else          out+='<span class="tk-n">'+esc(m[4])+'</span>';
    last=m.index+m[0].length;
  }
  out+=esc(rest.slice(last));
  return out;
}
let flashLine=-1;
function paint(range){
  const lines=srcEl.value.split('\n');
  hlEl.innerHTML=lines.map((l,i)=>{
    const hit=range&&i>=range[0]&&i<=range[1];
    const cls='ln'+(hit?' hit':'')+(hit&&i===range[0]?' hit-a':'')+(hit&&i===range[1]?' hit-b':'')+(i===flashLine?' flash':'');
    return '<div class="'+cls+'">'+tokLine(l)+'</div>';
  }).join('');
  gutEl.innerHTML=lines.map((_,i)=>'<div>'+(i+1)+'</div>').join('');
  hlEl.scrollTop=srcEl.scrollTop; hlEl.scrollLeft=srcEl.scrollLeft; gutEl.scrollTop=srcEl.scrollTop;
}
srcEl.addEventListener('scroll',()=>{hlEl.scrollTop=srcEl.scrollTop;hlEl.scrollLeft=srcEl.scrollLeft;gutEl.scrollTop=srcEl.scrollTop;});
let pinId=null;
function rangeFor(id){
  if(!(id in lineMap)) id=id.replace(/~\d+$/,'');
  return lineMap[id]||null;
}
function scrollToLine(ln){
  const lh=srcEl.scrollHeight/srcEl.value.split('\n').length;
  srcEl.scrollTop=Math.max(0,ln*lh-60);
  hlEl.scrollTop=srcEl.scrollTop; gutEl.scrollTop=srcEl.scrollTop;
}
function glowOnly(id){
  zoomer.querySelectorAll('g[data-part-id]').forEach(g=>{
    const gid=g.getAttribute('data-part-id');
    g.classList.toggle('hi', gid===id || (id!==null && gid.replace(/~\d+$/,'')===id));
  });
}
function wireHover(){
  zoomer.querySelectorAll('g[data-part-id]').forEach(g=>{
    const id=g.getAttribute('data-part-id');
    g.style.cursor='move';
    g.addEventListener('mouseenter',()=>{
      g.classList.add('hi');
      const r=rangeFor(id); if(!r) return;
      paint(r); scrollToLine(r[0]);
    });
    g.addEventListener('mouseleave',()=>{
      if(pinId!==id) g.classList.remove('hi');
      const pr=pinId?rangeFor(pinId):null;
      paint(pr);
    });
    g.addEventListener('click',e=>{
      e.stopPropagation();
      pinId = (pinId===id)?null:id;
      glowOnly(pinId);
      const pr=pinId?rangeFor(pinId):null;
      paint(pr);
      if(pr) scrollToLine(pr[0]);
    });
  });
  if(pinId) glowOnly(pinId);
}

/* ── drag-to-edit: move a node (offset) or shift an edge (bend) ──── */
const C30=0.8660254037844387, S30=0.5;
let nodeDrag=null, edgeDrag=null;
function screenToWorldDelta(dsx,dsy){
  // invert the iso ground-plane projection (sx=(wx-wy)c30, sy=(wx+wy)s30)
  return [dsx/(2*C30)+dsy/(2*S30), -dsx/(2*C30)+dsy/(2*S30)];
}
/* ── per-segment edge editing (drawio-style orthogonal waypoints) ────
   The renderer tags each connector with data-route="sx,sy,wx,wy ..." —
   every corner in pre-clip SVG-user AND world coords, source→target. We
   hit-test which segment the pointer grabbed, move ONLY that segment along
   its perpendicular world axis (inserting a corner at a docked endpoint so
   the endpoints never move), and post the resulting interior waypoint list.
   All structural math is in world coords; screen is only for hit-test and
   live preview, so it round-trips exactly with what the server re-renders. */
function parseRoute(p){
  const a=p.getAttribute('data-route'); if(!a) return null;
  const r=a.trim().split(/\s+/).map(t=>{const n=t.split(',').map(Number);
    return {sx:n[0],sy:n[1],wx:n[2],wy:n[3]};});
  return r.length>=2 ? r : null;
}
function distToSeg(px,py,ax,ay,bx,by){
  const vx=bx-ax,vy=by-ay,wx=px-ax,wy=py-ay;
  const L=vx*vx+vy*vy; let t=L>0?(wx*vx+wy*vy)/L:0; t=Math.max(0,Math.min(1,t));
  const dx=px-(ax+t*vx),dy=py-(ay+t*vy); return Math.hypot(dx,dy);
}
function nearestSegment(p,route,cx,cy){
  // cursor → SVG-user coords (the space data-route's sx,sy live in)
  const m=p.getScreenCTM(); if(!m) return {j:0,axisIsX:Math.abs(route[1].wx-route[0].wx)>=Math.abs(route[1].wy-route[0].wy)};
  const pt=p.ownerSVGElement.createSVGPoint(); pt.x=cx; pt.y=cy;
  const u=pt.matrixTransform(m.inverse());
  let best=0,bd=Infinity;
  for(let i=0;i<route.length-1;i++){
    const d=distToSeg(u.x,u.y,route[i].sx,route[i].sy,route[i+1].sx,route[i+1].sy);
    if(d<bd){bd=d;best=i;}
  }
  const a=route[best],b=route[best+1];
  return {j:best, axisIsX: Math.abs(b.wx-a.wx)>=Math.abs(b.wy-a.wy)};
}
// Move segment j perpendicular by delta (world units), keeping the two
// docked endpoints (index 0 and last) fixed by inserting corners. Returns a
// fresh world-corner list. axisIsX: segment runs along world-x, moves in wy.
function editSegment(route,j,axisIsX,delta){
  const W=route.map(r=>({wx:r.wx,wy:r.wy})); const N=W.length, k=axisIsX?'wy':'wx';
  const isStart=j===0, isEnd=j+1===N-1;
  if(!isStart && !isEnd){ W[j][k]+=delta; W[j+1][k]+=delta; return W; }
  if(isStart && isEnd){
    const A={wx:W[0].wx,wy:W[0].wy}, B={wx:W[1].wx,wy:W[1].wy}; A[k]+=delta; B[k]+=delta;
    return [W[0],A,B,W[1]];
  }
  if(isStart){
    const ins={wx:W[0].wx,wy:W[0].wy}; ins[k]+=delta; W[1][k]+=delta;
    return [W[0],ins,...W.slice(1)];
  }
  const ins={wx:W[N-1].wx,wy:W[N-1].wy}; ins[k]+=delta; W[N-2][k]+=delta;
  return [...W.slice(0,N-1),ins,W[N-1]];
}
function normalizeW(W){
  const eps=0.5; let out=[W[0]];
  for(let i=1;i<W.length;i++){const a=out[out.length-1];
    if(Math.abs(W[i].wx-a.wx)<eps && Math.abs(W[i].wy-a.wy)<eps) continue; out.push(W[i]);}
  for(let ch=true;ch && out.length>2;){ch=false;
    for(let i=1;i<out.length-1;i++){const a=out[i-1],b=out[i],c=out[i+1];
      if((Math.abs(a.wx-b.wx)<eps&&Math.abs(b.wx-c.wx)<eps)||(Math.abs(a.wy-b.wy)<eps&&Math.abs(b.wy-c.wy)<eps)){
        out.splice(i,1); ch=true; break;}}}
  return out;
}
// world corner → SVG-user coords, anchored on route[0]'s exact pairing.
function worldToUser(w,ref){const dx=w.wx-ref.wx,dy=w.wy-ref.wy;
  return [ref.sx+(dx-dy)*C30, ref.sy+(dx+dy)*S30];}
function routeToPathD(W,ref){
  return W.map((w,i)=>{const u=worldToUser(w,ref);return (i?'L ':'M ')+u[0].toFixed(2)+','+u[1].toFixed(2);}).join(' ');
}
// Live edge-follow: shift a path's FROM end (first coord pair) and/or TO end
// (last pair) by (dx,dy) user units. Used while dragging a node so its docked
// connector endpoints track it — the node translates rigidly by the same
// (dx,dy), so the glued endpoint stays on its face and the line rubber-bands.
function shiftEnds(d,moveFrom,moveTo,dx,dy){
  const re=/-?\d*\.?\d+/g, nums=[]; let m;
  while((m=re.exec(d))) nums.push({v:parseFloat(m[0]),i:m.index,len:m[0].length});
  if(nums.length<4) return d;
  const edits=[];
  if(moveFrom){edits.push([0,dx],[1,dy]);}
  if(moveTo){edits.push([nums.length-2,dx],[nums.length-1,dy]);}
  edits.sort((a,b)=>b[0]-a[0]);   // right-to-left so earlier offsets stay valid
  let s=d;
  for(const ed of edits){const t=nums[ed[0]]; s=s.slice(0,t.i)+(t.v+ed[1]).toFixed(2)+s.slice(t.i+t.len);}
  return s;
}
async function commitMove(kind,key,dwx,dwy,dropX,dropY,wp){
  if(!serverOK) return;
  let qp = kind==='node' ? 'kind=node&id='+encodeURIComponent(key) : 'kind=edge&ci='+key;
  if(wp) qp += '&wp='+encodeURIComponent(JSON.stringify(wp));
  try{
    const r=await fetch('/api/move?'+qp+'&dwx='+dwx+'&dwy='+dwy+'&format='+encodeURIComponent(LANG),
      {method:'POST',body:srcEl.value});
    if(!r.ok) return;
    const data=await r.json();
    if(typeof data.yaml==='string'){ srcEl.value=data.yaml; setDirty(srcEl.value!==ORIGINAL);
      try{localStorage.setItem(DRAFTKEY,srcEl.value);}catch(_){} }
    if(data.svg){
      // For an edge bend no node should move; record a stable node's screen
      // position now so we can hold the whole scene still after re-render
      // (the bend can extend bounds and reframe the viewBox).
      let edgeAnchor=null;
      if(kind==='edge'){
        const ref=zoomer.querySelector('g[data-part-id]');
        if(ref){const c=ref.getBoundingClientRect();
          edgeAnchor={id:ref.getAttribute('data-part-id'),x:c.x+c.width/2,y:c.y+c.height/2};}
      }
      zoomer.innerHTML=data.svg;
      buildMap(); paint(pinId?rangeFor(pinId):null);
      wireHover(); wireDrag(); adaptStage();
      // Re-render recomputes the viewBox (and the flex-centred SVG can
      // resize), which would slide the whole scene and read as a
      // teleport. Re-anchor on the dropped element: pan so it sits
      // exactly under the cursor, so the move feels direct.
      if(kind==='node' && dropX!=null){
        const g=zoomer.querySelector('g[data-part-id="'+(window.CSS&&CSS.escape?CSS.escape(key):key)+'"]');
        if(g){const c=g.getBoundingClientRect();
          panX+=dropX-(c.x+c.width/2); panY+=dropY-(c.y+c.height/2); apply();}
      }else if(kind==='edge' && edgeAnchor){
        // Hold the scene still: pan so the reference node returns to where
        // it sat before the re-render. The bend already places the line at
        // the drop point relative to the (unmoving) nodes, so the line
        // lands under the cursor and nothing else jumps.
        const ref=zoomer.querySelector('g[data-part-id="'+(window.CSS&&CSS.escape?CSS.escape(edgeAnchor.id):edgeAnchor.id)+'"]');
        if(ref){const c=ref.getBoundingClientRect();
          panX+=edgeAnchor.x-(c.x+c.width/2); panY+=edgeAnchor.y-(c.y+c.height/2); apply();}
      }
    }
    showIssues(data.issues||[]);
  }catch(_){}
}
function wireDrag(){
  // Attach unconditionally — probe() may not have resolved at first
  // paint; commitMove() guards on serverOK at drop time instead.
  zoomer.querySelectorAll('g[data-part-id]').forEach(g=>{
    // Hover affordance: 4-way 'move' arrows — clearly distinct from the
    // canvas pan cursor (open-hand 'grab'), so a movable object reads as
    // movable the instant the pointer is over it.
    g.style.cursor='move';
    g.addEventListener('mousedown',e=>{
      e.preventDefault();   // suppress the browser's native SVG image-drag
      e.stopPropagation();  // don't let the viewport start a pan
      g.style.cursor='grabbing';
      document.body.style.cursor='grabbing';  // hold feedback if pointer outruns the element
      g.classList.add('dragging');
      const did=g.getAttribute('data-part-id').replace(/~\d+$/,'');
      // connectors docked to this node, so they can live-follow the drag
      const edges=[];
      zoomer.querySelectorAll('path[data-connector]').forEach(p=>{
        const mf=p.getAttribute('data-from')===did, mt=p.getAttribute('data-to')===did;
        if(mf||mt) edges.push({el:p,mf,mt,baseD:p.getAttribute('d')});
      });
      nodeDrag={el:g,id:did,x:e.clientX,y:e.clientY,base:g.getAttribute('transform')||'',edges};
    });
  });
  zoomer.querySelectorAll('path[data-connector]').forEach(p=>{
    p.setAttribute('stroke-width', Math.max(parseFloat(p.getAttribute('stroke-width')||'1.4'),3));
    p.style.cursor='move';
    p.addEventListener('mousedown',e=>{
      e.preventDefault();
      e.stopPropagation();
      p.style.cursor='grabbing';
      document.body.style.cursor='grabbing';
      p.classList.add('dragging');
      const route=parseRoute(p);
      const seg=route?nearestSegment(p,route,e.clientX,e.clientY):null;
      edgeDrag={el:p,ci:p.getAttribute('data-connector'),x:e.clientX,y:e.clientY,
        base:p.getAttribute('transform')||'',baseD:p.getAttribute('d'),route,seg,wp:null};
    });
  });
}
// Live follow by COMPOSING with the element's existing transform
// attribute. A node <g> already carries transform="translate(x y)";
// setting CSS style.transform would OVERRIDE that attribute in SVG and
// teleport the node to the origin — the real-browser "completely
// unusable" bug. So we add the drag delta to the attribute's own
// translate instead. Lengths are SVG user units → screen px / scale.
function liveTranslate(el, base, dx, dy){
  const m = base.match(/translate\(\s*(-?[\d.]+)[ ,]+(-?[\d.]+)\s*\)/);
  if(m){
    const nx=parseFloat(m[1])+dx, ny=parseFloat(m[2])+dy;
    el.setAttribute('transform', base.slice(0,m.index)+'translate('+nx+' '+ny+')'+base.slice(m.index+m[0].length));
  }else{
    el.setAttribute('transform', ('translate('+dx+' '+dy+') ')+base);
  }
}
window.addEventListener('mousemove',e=>{
  const d=nodeDrag||edgeDrag; if(!d) return;
  if(edgeDrag && d.route && d.seg){
    // Per-segment preview: move ONLY the grabbed segment, rebuilt from world
    // corners so the docked endpoints stay put and the line stays iso-clean.
    const wd=screenToWorldDelta((e.clientX-d.x)/scale,(e.clientY-d.y)/scale);
    const delta=d.seg.axisIsX?wd[1]:wd[0];
    const edited=normalizeW(editSegment(d.route,d.seg.j,d.seg.axisIsX,delta));
    d.el.setAttribute('d', routeToPathD(edited,d.route[0]));
    d.wp=edited;
    return;
  }
  const dx=(e.clientX-d.x)/scale, dy=(e.clientY-d.y)/scale;
  liveTranslate(d.el, d.base, dx, dy);
  // node drag: docked connector endpoints track the node in real time
  if(nodeDrag && d.edges) d.edges.forEach(ed=>ed.el.setAttribute('d', shiftEnds(ed.baseD, ed.mf, ed.mt, dx, dy)));
});
window.addEventListener('mouseup',e=>{
  const d=nodeDrag||edgeDrag, kind=nodeDrag?'node':(edgeDrag?'edge':null);
  if(!kind) return;
  nodeDrag=null; edgeDrag=null;
  // restore the pre-drag transform; commit re-renders the SVG fresh
  d.el.setAttribute('transform', d.base);
  if(d.baseD!=null) d.el.setAttribute('d', d.baseD);
  // restore live-followed edges; the commit re-render replaces them cleanly,
  // and a below-threshold release leaves nothing changed.
  if(d.edges) d.edges.forEach(ed=>ed.el.setAttribute('d', ed.baseD));
  d.el.classList.remove('dragging'); d.el.style.cursor='move'; document.body.style.cursor='';
  const wd=screenToWorldDelta((e.clientX-d.x)/scale,(e.clientY-d.y)/scale);
  if(Math.abs(wd[0])<=2 && Math.abs(wd[1])<=2) return;   // below drag threshold
  if(kind==='edge' && d.wp){
    // interior corners (drop the docked endpoints) become the waypoint list
    const interior=d.wp.slice(1,d.wp.length-1).map(w=>[Math.round(w.wx),Math.round(w.wy)]);
    commitMove('edge', d.ci, 0, 0, e.clientX, e.clientY, interior);
  }else{
    commitMove(kind, kind==='node'?d.id:d.ci, Math.round(wd[0]), Math.round(wd[1]), e.clientX, e.clientY);
  }
});

/* reverse mapping: caret inside a part's block lights the node up */
function caretSync(){
  const line=srcEl.value.slice(0,srcEl.selectionStart).split('\n').length-1;
  let hitId=null;
  for(const id in lineMap){
    const r=lineMap[id];
    if(line>=r[0]&&line<=r[1]){hitId=id;break;}
  }
  if(!pinId) glowOnly(hitId);
}
srcEl.addEventListener('keyup',caretSync);
srcEl.addEventListener('click',caretSync);

/* ── zoom & pan ─────────────────────────────────────────────────── */
let scale=1,panX=0,panY=0;
function apply(){
  zoomer.style.transform='translate('+panX+'px,'+panY+'px) scale('+scale+')';
  const z=document.getElementById('zpct');
  if(z) z.textContent=Math.round(scale*100)+'%';
}
function zoomBy(f){
  const r=viewport.getBoundingClientRect();
  const cx=r.width/2, cy=r.height/2;
  const ns=Math.min(8,Math.max(0.2,scale*f));
  const eff=ns/scale;
  panX=cx-(cx-panX)*eff; panY=cy-(cy-panY)*eff;
  scale=ns; apply();
}
function resetView(){scale=1;panX=0;panY=0;apply();}
function fitView(){
  const svg=zoomer.querySelector('svg'); if(!svg) return;
  const r=viewport.getBoundingClientRect();
  const w=parseFloat(svg.getAttribute('width'))||svg.viewBox.baseVal.width;
  const h=parseFloat(svg.getAttribute('height'))||svg.viewBox.baseVal.height;
  if(!w||!h) return;
  const s=Math.min((r.width-72)/w,(r.height-72)/h);
  scale=Math.min(4,Math.max(0.2,s));
  panX=w*(1-scale)/2; panY=h*(1-scale)/2;
  apply();
}
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
viewport.addEventListener('click',e=>{
  if(e.target.closest && e.target.closest('g[data-part-id]')) return;
  if(pinId!==null){pinId=null;glowOnly(null);paint(null);}
});

/* ── adaptive stage: tint the backdrop after the scene's canvas ── */
function hexRGB(c){
  if(!c) return null;
  c=c.trim();
  let m=c.match(/^#([0-9a-f]{3})$/i);
  if(m) return [parseInt(m[1][0]+m[1][0],16),parseInt(m[1][1]+m[1][1],16),parseInt(m[1][2]+m[1][2],16)];
  m=c.match(/^#([0-9a-f]{6})/i);
  if(m) return [parseInt(m[1].slice(0,2),16),parseInt(m[1].slice(2,4),16),parseInt(m[1].slice(4,6),16)];
  return null;
}
function adaptStage(){
  const st=document.getElementById('stage');
  const r=zoomer.querySelector('rect[data-layer="canvas-bg"]');
  const rgb=r?hexRGB(r.getAttribute('fill')):null;
  if(!rgb){st.style.removeProperty('--stage-bg');st.style.removeProperty('--stage-dot');return;}
  const lum=(0.2126*rgb[0]+0.7152*rgb[1]+0.0722*rgb[2])/255;
  if(lum<0.45){
    const mix=rgb.map(v=>Math.round(v*0.82));
    st.style.setProperty('--stage-bg','rgb('+mix.join(',')+')');
    st.style.setProperty('--stage-dot','rgba(255,255,255,.07)');
  }else{
    const mix=rgb.map(v=>Math.round(v*0.96));
    st.style.setProperty('--stage-bg','rgb('+mix.join(',')+')');
    st.style.setProperty('--stage-dot','rgba(15,23,42,.08)');
  }
}

/* ── issues → source navigation ─────────────────────────────────── */
function pathToLine(path){
  const segs=String(path||'').replace(/\[(\d+)\]/g,'.$1').split('.').filter(Boolean);
  const lines=srcEl.value.split('\n');
  let pos=0, indent=-1;
  for(const sg of segs){
    if(/^\d+$/.test(sg)){
      let n=+sg, itemIndent=-1, found=-1;
      for(let i=pos+1;i<lines.length;i++){
        const t=lines[i]; if(!t.trim()) continue;
        const ind=t.search(/\S/);
        if(ind<=indent) break;
        if(/^\s*-(\s|$)/.test(t)){
          if(itemIndent<0) itemIndent=ind;
          if(ind===itemIndent){ if(n===0){found=i;break;} n--; }
        }
      }
      if(found<0) return pos;
      pos=found; indent=lines[found].search(/\S/);
      continue;
    }
    const re=new RegExp('^\\s*(?:- )?'+sg.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'\\s*:');
    let found=-1;
    for(let i=pos;i<lines.length;i++){
      if(re.test(lines[i])){found=i;break;}
    }
    if(found<0) return pos;
    pos=found; indent=lines[found].search(/\S/);
  }
  return pos;
}
function jumpToIssue(path){
  const ln=pathToLine(path);
  flashLine=ln;
  paint(pinId?rangeFor(pinId):null);
  scrollToLine(ln);
  setTimeout(()=>{flashLine=-1;paint(pinId?rangeFor(pinId):null);},1500);
}

/* ── live re-render against isotopo serve ───────────────────────── */
const liveEl=document.getElementById('live'), renderBtn=document.getElementById('render');
document.getElementById('auto').addEventListener('change',e=>{
  if(!e.target.checked) clearTimeout(timer);
});
let serverOK=false, timer=null;
async function probe(){
  if(!location.protocol.startsWith('http')){
    liveEl.textContent='Static file';
    liveEl.title='run "isotopo serve <input>" for live re-render';
    renderBtn.disabled=true; return;
  }
  try{
    const r=await fetch('/api/ping');
    serverOK=r.ok;
  }catch(_){serverOK=false;}
  liveEl.textContent=serverOK?'Live':'Offline';
  liveEl.title=serverOK?'renderer connected':'renderer unreachable — restart isotopo serve';
  liveEl.classList.toggle('on',serverOK);
  renderBtn.disabled=!serverOK;
}
async function rerender(){
  if(!serverOK) return;
  renderBtn.textContent='Rendering…';
  try{
    const r=await fetch('/api/render?format='+encodeURIComponent(LANG),{method:'POST',body:srcEl.value});
    const data=await r.json();
    showIssues(data.issues||[]);
    if(data.svg){
      zoomer.innerHTML=data.svg;
      buildMap(); paint(pinId?rangeFor(pinId):null); wireHover(); wireDrag(); adaptStage();
      staleEl.hidden=true;
    }else{
      staleEl.hidden=false;
    }
  }catch(e){
    showIssues([{severity:'error',path:'$',message:String(e)}]);
    staleEl.hidden=false;
    probe();
  }
  renderBtn.textContent='Render';
}
function showIssues(list){
  const el=document.getElementById('issues');
  if(!list.length){el.classList.remove('show');el.innerHTML='';return;}
  el.classList.add('show');
  el.innerHTML=list.map(i=>'<div class="'+(i.severity==='error'?'err':'warn')+'" data-path="'+esc(i.path||'')+'" title="click to jump to this line">'+
    esc(i.severity+' '+(i.path||'')+' — '+i.message+(i.suggest?' (did you mean '+i.suggest+'?)':''))+'</div>').join('');
  el.querySelectorAll('[data-path]').forEach(d=>{
    d.addEventListener('click',()=>jumpToIssue(d.getAttribute('data-path')));
  });
}
srcEl.addEventListener('input',()=>{
  buildMap(); paint(null);
  const edited=srcEl.value!==ORIGINAL;
  setDirty(edited);
  try{
    if(edited) localStorage.setItem(DRAFTKEY,srcEl.value);
    else localStorage.removeItem(DRAFTKEY);
  }catch(_){}
  if(document.getElementById('auto').checked && serverOK){
    clearTimeout(timer); timer=setTimeout(()=>{if(document.getElementById('auto').checked)rerender();},600);
  }
});
window.addEventListener('keydown',e=>{
  if((e.metaKey||e.ctrlKey)&&e.key==='Enter'){e.preventDefault();rerender();return;}
  if((e.metaKey||e.ctrlKey)&&e.key==='0'){e.preventDefault();fitView();return;}
  if((e.metaKey||e.ctrlKey)&&(e.key==='='||e.key==='+')){e.preventDefault();zoomBy(1.25);return;}
  if((e.metaKey||e.ctrlKey)&&e.key==='-'){e.preventDefault();zoomBy(0.8);return;}
  if(e.key==='Escape'){document.getElementById('help').hidden=true;return;}
  if(e.key==='?'&&document.activeElement!==srcEl){toggleHelp();}
});
function toggleHelp(){
  const h=document.getElementById('help');
  h.hidden=!h.hidden;
}
document.getElementById('help').addEventListener('click',e=>{
  if(e.target.id==='help') e.target.hidden=true;
});
srcEl.addEventListener('keydown',e=>{
  if(e.key==='Tab'&&!e.shiftKey){
    e.preventDefault();
    srcEl.setRangeText('  ',srcEl.selectionStart,srcEl.selectionEnd,'end');
    srcEl.dispatchEvent(new Event('input'));
  }
});

/* ── path display: middle-truncate by segment to fit ───────────── */
const fpathEl=document.getElementById('fpath');
function renderPath(){
  const segs=PATH.split('/').filter(Boolean);
  segs.pop(); // basename is rendered separately in bold
  fpathEl.title=PATH;
  const set=k=>{
    if(k>=segs.length){fpathEl.textContent='/'+segs.map(x=>x+'/').join('');return;}
    if(k<=0){fpathEl.textContent='…/';return;}
    const nt=Math.ceil(k/2), nh=k-nt;
    const head=segs.slice(0,nh).map(x=>x+'/').join('');
    const tail=segs.slice(-nt).map(x=>x+'/').join('');
    fpathEl.textContent=(nh>0?'/'+head:'')+'…/'+tail;
  };
  let k=segs.length;
  set(k);
  while(k>0 && fpathEl.scrollWidth>fpathEl.clientWidth){k--;set(k);}
}
window.addEventListener('resize',renderPath);

/* ── editor pane drag-resize ────────────────────────────────────── */
const sideEl=document.querySelector('.side'), splitEl=document.getElementById('split');
function setPaneWidth(w){
  w=Math.min(Math.max(w,320),Math.max(360,window.innerWidth*0.7));
  sideEl.style.flex='0 0 '+w+'px';
  sideEl.style.maxWidth='none';
}
let paneDrag=null;
splitEl.addEventListener('mousedown',e=>{
  e.preventDefault();
  paneDrag={x:e.clientX,w:sideEl.getBoundingClientRect().width};
  splitEl.classList.add('active');
  document.body.style.cursor='col-resize';
  document.body.style.userSelect='none';
});
window.addEventListener('mousemove',e=>{
  if(!paneDrag)return;
  setPaneWidth(paneDrag.w-(e.clientX-paneDrag.x));
  renderPath();
});
window.addEventListener('mouseup',()=>{
  if(!paneDrag)return;
  paneDrag=null;
  splitEl.classList.remove('active');
  document.body.style.cursor='';
  document.body.style.userSelect='';
  try{localStorage.setItem('isotopo-pane',String(Math.round(sideEl.getBoundingClientRect().width)));}catch(_){}
});
try{
  const pw=parseInt(localStorage.getItem('isotopo-pane'),10);
  if(pw>0) setPaneWidth(pw);
}catch(_){}
async function copyPath(){
  const b=document.getElementById('cppath'), keep=b.innerHTML;
  try{
    await navigator.clipboard.writeText(PATH);
    b.innerHTML='<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
    setTimeout(()=>{b.innerHTML=keep;},1200);
  }catch(_){}
}

/* ── export the CURRENT render (incl. unsaved edits) ───────────── */
function currentSVG(){
  const el=zoomer.querySelector('svg'); if(!el) return '';
  const c=el.cloneNode(true);
  c.querySelectorAll('[data-part-id]').forEach(g=>{
    g.classList.remove('hi');
    if(!g.getAttribute('class')) g.removeAttribute('class');
    g.style.removeProperty('cursor');
    if(!g.getAttribute('style')) g.removeAttribute('style');
  });
  return c.outerHTML;
}
function exportName(ext){
  return FILENAME.replace(/\.[a-z0-9]+$/i,'')+(dirty?'.edited':'')+'.'+ext;
}
function downloadBlob(blob,name){
  const a=document.createElement('a');
  a.href=URL.createObjectURL(blob); a.download=name; a.click();
  setTimeout(()=>URL.revokeObjectURL(a.href),5000);
}
function exportSVG(){
  const sv=currentSVG(); if(!sv) return;
  downloadBlob(new Blob([sv],{type:'image/svg+xml'}),exportName('svg'));
}
function exportPNG(){
  const sv=currentSVG(); if(!sv) return;
  const url=URL.createObjectURL(new Blob([sv],{type:'image/svg+xml'}));
  const img=new Image();
  img.onload=()=>{
    const c=document.createElement('canvas');
    c.width=img.width*2; c.height=img.height*2;
    const ctx=c.getContext('2d'); ctx.scale(2,2); ctx.drawImage(img,0,0);
    c.toBlob(b=>downloadBlob(b,exportName('png')),'image/png');
    URL.revokeObjectURL(url);
  };
  img.src=url;
}

/* ── misc ───────────────────────────────────────────────────────── */
async function copySrc(){
  const b=document.getElementById('copybtn');
  try{await navigator.clipboard.writeText(srcEl.value);b.textContent='Copied';}
  catch(_){b.textContent='Copy failed';}
  setTimeout(()=>{b.textContent='Copy';},1200);
}
function downloadCopy(){
  const blob=new Blob([srcEl.value],{type:'text/plain'});
  const a=document.createElement('a');
  a.href=URL.createObjectURL(blob);
  a.download=FILENAME.replace(/(\.[a-z0-9]+)$/i,'.edited$1');
  a.click();
}
/* ── share: YAML deflated into the URL hash ─────────────────────── */
async function pipeThrough(stream,bytes){
  const w=stream.writable.getWriter();
  w.write(bytes); w.close();
  const out=[];
  const rd=stream.readable.getReader();
  for(;;){const {done,value}=await rd.read(); if(done)break; out.push(value);}
  let n=0; out.forEach(c=>n+=c.length);
  const all=new Uint8Array(n); let o=0;
  out.forEach(c=>{all.set(c,o);o+=c.length;});
  return all;
}
async function deflateText(t){
  const b=await pipeThrough(new CompressionStream('deflate-raw'),new TextEncoder().encode(t));
  let s2=''; b.forEach(v=>{s2+=String.fromCharCode(v);});
  return btoa(s2).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
}
async function inflateText(b64){
  const s2=atob(b64.replace(/-/g,'+').replace(/_/g,'/'));
  const bytes=new Uint8Array(s2.length);
  for(let i=0;i<s2.length;i++) bytes[i]=s2.charCodeAt(i);
  const out=await pipeThrough(new DecompressionStream('deflate-raw'),bytes);
  return new TextDecoder().decode(out);
}
async function shareLink(){
  const b=document.getElementById('share');
  try{
    const h=await deflateText(srcEl.value);
    const url=location.origin+location.pathname+'#src='+h;
    await navigator.clipboard.writeText(url);
    b.textContent='Link copied';
  }catch(_){b.textContent='Share failed';}
  setTimeout(()=>{b.textContent='Share';},1400);
}

try{
  const d=localStorage.getItem(DRAFTKEY);
  if(d&&d!==ORIGINAL){srcEl.value=d;setDirty(true);}
}catch(_){}
/* a #src= permalink outranks the draft — explicit intent wins */
if(location.hash.indexOf('#src=')===0){
  inflateText(location.hash.slice(5)).then(t=>{
    srcEl.value=t; setDirty(t!==ORIGINAL);
    buildMap(); paint(null);
    if(serverOK) rerender();
  }).catch(()=>{});
}
buildMap(); paint(null); wireHover(); wireDrag(); renderPath(); adaptStage();
probe().then(()=>{if(serverOK&&dirty)rerender();});
if(location.protocol.startsWith('http')) setInterval(probe,5000);
</script>
</body></html>`
	base := filepath.Base(sourceFilename)
	r := strings.NewReplacer(
		"{{LANG}}", html.EscapeString(sourceLang),
		"{{SVG}}", svg,
		"{{SRC}}", html.EscapeString(sourceText),
		"{{LANGQ}}", fmt.Sprintf("%q", sourceLang),
		"{{PATHQ}}", fmt.Sprintf("%q", sourceFilename),
		"{{FILE}}", html.EscapeString(base),
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
