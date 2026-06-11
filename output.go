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
<title>isotopo · {{FILE}}</title>
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
.brand h1{margin:0;font-size:14.5px;font-weight:650;letter-spacing:-.01em;}
.brand .sub{font-size:10.5px;color:var(--muted);}
.tag{padding:2px 8px;border:1px solid var(--border);border-radius:5px;
  font:10.5px ui-monospace,Menlo,monospace;color:var(--muted);background:white;}
header .spacer{flex:1;}
#live{display:flex;align-items:center;gap:6px;font:11.5px Inter,sans-serif;font-weight:550;color:var(--muted);}
#live::before{content:"";width:7px;height:7px;border-radius:50%;background:#CBD5E1;flex:none;}
#live.on{color:var(--accent-deep);}
#live.on::before{background:var(--accent);animation:pulse 2.2s ease-in-out infinite;}
@keyframes pulse{0%,100%{box-shadow:0 0 0 0 rgba(16,174,185,.35)}50%{box-shadow:0 0 0 5px rgba(16,174,185,0)}}
.grid{flex:1;min-height:0;display:flex;}
.stage-wrap{flex:1.6;min-width:0;position:relative;overflow:hidden;
  background:#F2F4F8 radial-gradient(circle,rgba(15,23,42,.075) 1px,transparent 1.2px);
  background-size:22px 22px;}
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
.zoomctl button:hover{background:var(--accent-soft);color:var(--accent-deep);}
.zoomctl button+button{border-top:1px solid var(--border);}
.side{flex:1;min-width:360px;max-width:560px;display:flex;flex-direction:column;min-height:0;
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
.filetab .path{direction:rtl;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0;flex:0 1 auto;margin-right:-6px;}
.filetab .dot{width:7px;height:7px;border-radius:50%;border:1.5px solid #C2C9D6;background:transparent;flex:none;}
.filetab .dot.on{border-color:#F59E0B;background:#F59E0B;}
.editor{flex:1;min-height:0;position:relative;font:12.5px/1.6 ui-monospace,Menlo,Consolas,monospace;}
.editor .hl,.editor textarea{position:absolute;inset:0;margin:0;padding:14px 16px 14px 60px;white-space:pre;overflow:auto;font:inherit;tab-size:2;}
.editor .gutter{position:absolute;top:0;bottom:0;left:0;width:46px;overflow:hidden;
  padding:14px 0;background:var(--code-bg);border-right:1px solid var(--border);
  color:#B3BCCA;font-size:10.5px;text-align:right;pointer-events:none;}
.editor .gutter div{height:20px;line-height:20px;padding-right:10px;}
.editor textarea::-webkit-scrollbar{width:10px;height:10px;}
.editor textarea::-webkit-scrollbar-thumb{background:#D5DBE5;border-radius:5px;border:2px solid var(--code-bg);}
.editor textarea::-webkit-scrollbar-thumb:hover{background:#BCC5D2;}
.editor textarea::-webkit-scrollbar-track{background:transparent;}
.editor textarea::-webkit-scrollbar-corner{background:transparent;}
.editor .hl{color:transparent;background:var(--code-bg);pointer-events:none;}
.editor .hl .ln{min-height:1.6em;}
.editor .hl .hit{background:var(--accent-soft);box-shadow:inset 3px 0 0 var(--accent);}
.editor .hl .hit-a{border-top-right-radius:6px;}
.editor .hl .hit-b{border-bottom-right-radius:6px;}
.editor textarea{background:transparent;border:0;resize:none;color:#1E293B;outline:none;width:100%;height:100%;caret-color:var(--accent-deep);}
#issues{max-height:140px;overflow:auto;border-top:1px solid var(--border);font:11.5px/1.6 ui-monospace,Menlo,monospace;
  padding:10px 16px;display:none;background:#FFF9F5;}
#issues.show{display:block;}
#issues .err{color:#DC2626;}
#issues .warn{color:#D97706;}
footer{display:flex;gap:18px;align-items:center;padding:8px 20px;border-top:1px solid var(--border);
  font-size:11px;color:var(--muted);background:rgba(255,255,255,.7);}
footer a{color:var(--accent-deep);text-decoration:none;}
footer a:hover{text-decoration:underline;}
kbd{font:10px ui-monospace,Menlo,monospace;border:1px solid var(--border);border-bottom-width:2px;
  border-radius:4px;padding:1px 5px;background:white;color:#475569;}
#render{min-width:78px;}
.filetab #discard{color:var(--accent-deep);cursor:pointer;font-family:Inter,sans-serif;font-size:10.5px;white-space:nowrap;}
.filetab #discard:hover{text-decoration:underline;}
.stale{position:absolute;top:14px;left:50%;transform:translateX(-50%);z-index:4;
  background:#FFF7ED;border:1px solid #FDBA74;color:#9A3412;
  font:11px Inter,sans-serif;padding:5px 13px;border-radius:999px;box-shadow:var(--shadow);}
svg g[data-part-id]{transition:filter .12s;}
svg g[data-part-id].hi{filter:drop-shadow(0 0 3px rgba(16,174,185,.85));}
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
      <h1>iso-topology</h1>
      <span class="sub">isometric diagrams as code</span>
    </div>
  </div>
  <span class="tag">{{LANG}}</span>
  <span class="spacer"></span>
  <span id="live">checking renderer…</span>
</header>
<div class="grid">
  <div class="stage-wrap">
    <div id="viewport"><div id="zoomer">{{SVG}}</div></div>
    <div id="stale" class="stale" hidden>showing last good render</div>
    <div class="exportctl">
      <button onclick="exportSVG()" title="download the current render as SVG">&#8595; SVG</button>
      <button onclick="exportPNG()" title="download the current render as PNG (2x)">&#8595; PNG</button>
    </div>
    <div class="zoomctl">
      <button onclick="zoomBy(1.25)">+</button>
      <button onclick="zoomBy(0.8)">−</button>
      <button onclick="resetView()" title="reset">⤾</button>
    </div>
  </div>
  <div class="side">
    <div class="toolbar">
      <button id="render" onclick="rerender()" title="Cmd/Ctrl+Enter">Render</button>
      <label class="auto"><input type="checkbox" id="auto" checked>Auto</label>
      <span class="spacer" style="flex:1"></span>
      <button id="copybtn" onclick="copySrc()" title="copy the YAML to the clipboard">Copy</button>
      <button id="dl" onclick="downloadCopy()" disabled title="download the edited YAML as a new file">Download</button>
    </div>
    <div class="filetab"><span class="dot" id="dirty" title="unsaved edits stay in this page; the file on disk is never written"></span><span class="path" title="{{DIR}}{{FILE}}">&lrm;{{DIR}}&lrm;</span><b>{{FILE}}</b><span class="spacer" style="flex:1"></span><a id="discard" hidden onclick="discardDraft()">revert</a></div>
    <div class="editor">
      <div class="hl" id="hl"></div>
      <div class="gutter" id="gut"></div>
      <textarea id="src" spellcheck="false">{{SRC}}</textarea>
    </div>
    <div id="issues"></div>
  </div>
</div>
<footer>
  <span>Hover a node to jump to its source</span>
  <span>Scroll to zoom · drag to pan · double-click to reset</span>
  <span><kbd>⌘</kbd>+<kbd>↵</kbd> render</span>
  <span class="spacer" style="flex:1"></span>
  <a href="./topology.svg" target="_blank">Original SVG</a>
  <a href="./nodes/_index.html">Browse nodes</a>
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
function paint(range){
  const lines=srcEl.value.split('\n');
  hlEl.innerHTML=lines.map((l,i)=>{
    const hit=range&&i>=range[0]&&i<=range[1];
    const cls='ln'+(hit?' hit':'')+(hit&&i===range[0]?' hit-a':'')+(hit&&i===range[1]?' hit-b':'');
    return '<div class="'+cls+'">'+esc(l||' ')+'</div>';
  }).join('');
  gutEl.innerHTML=lines.map((_,i)=>'<div>'+(i+1)+'</div>').join('');
  hlEl.scrollTop=srcEl.scrollTop; hlEl.scrollLeft=srcEl.scrollLeft; gutEl.scrollTop=srcEl.scrollTop;
}
srcEl.addEventListener('scroll',()=>{hlEl.scrollTop=srcEl.scrollTop;hlEl.scrollLeft=srcEl.scrollLeft;gutEl.scrollTop=srcEl.scrollTop;});
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
      buildMap(); paint(null); wireHover();
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
  el.innerHTML=list.map(i=>'<div class="'+(i.severity==='error'?'err':'warn')+'">'+
    esc(i.severity+' '+(i.path||'')+' — '+i.message+(i.suggest?' (did you mean '+i.suggest+'?)':''))+'</div>').join('');
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
    clearTimeout(timer); timer=setTimeout(rerender,600);
  }
});
window.addEventListener('keydown',e=>{
  if((e.metaKey||e.ctrlKey)&&e.key==='Enter'){e.preventDefault();rerender();}
});
srcEl.addEventListener('keydown',e=>{
  if(e.key==='Tab'&&!e.shiftKey){
    e.preventDefault();
    srcEl.setRangeText('  ',srcEl.selectionStart,srcEl.selectionEnd,'end');
    srcEl.dispatchEvent(new Event('input'));
  }
});

/* ── export the CURRENT render (incl. unsaved edits) ───────────── */
function currentSVG(){const el=zoomer.querySelector('svg');return el?el.outerHTML:'';}
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
try{
  const d=localStorage.getItem(DRAFTKEY);
  if(d&&d!==ORIGINAL){srcEl.value=d;setDirty(true);}
}catch(_){}
buildMap(); paint(null); wireHover();
probe().then(()=>{if(serverOK&&dirty)rerender();});
if(location.protocol.startsWith('http')) setInterval(probe,5000);
</script>
</body></html>`
	base := filepath.Base(sourceFilename)
	dir := ""
	if d := filepath.Dir(sourceFilename); d != "." {
		dir = d + "/"
	}
	r := strings.NewReplacer(
		"{{LANG}}", html.EscapeString(sourceLang),
		"{{SVG}}", svg,
		"{{SRC}}", html.EscapeString(sourceText),
		"{{LANGQ}}", fmt.Sprintf("%q", sourceLang),
		"{{PATHQ}}", fmt.Sprintf("%q", sourceFilename),
		"{{FILE}}", html.EscapeString(base),
		"{{DIR}}", html.EscapeString(dir),
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
