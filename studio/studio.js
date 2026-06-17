
"use strict";
const LANG={{LANGQ}}, PATH={{PATHQ}};
const FILENAME=PATH.split('/').pop();
const srcEl=document.getElementById('src'), hlEl=document.getElementById('hl'), gutEl=document.getElementById('gut');
const ORIGINAL=srcEl.value, DRAFTKEY='isotopo-draft:'+PATH;
const staleEl=document.getElementById('stale');
let dirty=false;
function setDirty(d){
  dirty=d;
  document.getElementById('discard').hidden=!d;
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
let lineMap={}, connMap=[];
function esc(t){return t.replace(/&/g,'&amp;').replace(/</g,'&lt;');}
function buildMap(){
  const lines=srcEl.value.split('\n'); lineMap={}; connMap=[];
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
  // connMap[ci] = [startLine,endLine] of the ci-th connector, mirroring the
  // server's findConnectorLine (the FIRST connectors: block, in order), so
  // data-connector ci maps straight to its source lines.
  let c0=-1,cInd=0;
  for(let i=0;i<lines.length;i++){
    const m=lines[i].match(/^(\s*)connectors:\s*(#.*)?$/);
    if(m){c0=i;cInd=m[1].length;break;}
  }
  if(c0>=0){
    const starts=[]; let itemInd=-1, blockEnd=lines.length-1;
    for(let i=c0+1;i<lines.length;i++){
      const t=lines[i]; if(!t.trim()) continue;
      const ind=t.search(/\S/);
      if(ind<=cInd){blockEnd=i-1;break;}
      if(/^\s*-/.test(t)){ if(itemInd<0) itemInd=ind; if(ind===itemInd) starts.push(i); }
    }
    for(let k=0;k<starts.length;k++)
      connMap.push([starts[k], k+1<starts.length?starts[k+1]-1:blockEnd]);
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
let pinId=null, pinCi=null;   // pinned node id OR connector index (exclusive)
function rangeFor(id){
  if(!(id in lineMap)) id=id.replace(/~\d+$/,'');
  return lineMap[id]||null;
}
// line range of whatever is currently pinned (node or edge), or null
function pinnedRange(){return pinId?rangeFor(pinId):(pinCi!=null?connMap[pinCi]:null);}
function glowEdge(ci){
  zoomer.querySelectorAll('path[data-connector]').forEach(p=>{
    p.classList.toggle('pinned', ci!=null && +p.getAttribute('data-connector')===ci);
  });
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
/* ── drag-to-connect: draw a new connector by dragging from a node ────────
   Hovering a node shows 4 handle dots at the iso face centres (top, left,
   right) and the nadir vertex. Positions are computed from the actual polygon
   vertices via getScreenCTM() — NOT from the DOM bounding box — so they sit
   exactly on the isometric surface regardless of zoom or scroll.
   Handles live on a dedicated SVG overlay so they don't interfere with the
   existing node SVG structure. ──────────────────────────────────────────── */
let connDrag=null;       // {fromId, startX, startY, line} while dragging
let connTarget=null;     // id of the node the pointer is currently over

const HANDLE_R=5, HANDLE_COL='#2563EB', HANDLE_HOV='#1D4ED8';

// Overlay SVG that lives on top of zoomer for handle dots + rubber-band line
let connOverlay=null;
function ensureOverlay(){
  if(connOverlay) return;
  connOverlay=document.createElementNS('http://www.w3.org/2000/svg','svg');
  connOverlay.setAttribute('id','conn-overlay');
  connOverlay.style.cssText='position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;overflow:visible;';
  document.getElementById('zoomer').parentElement.appendChild(connOverlay);
}
function clearHandles(){
  connOverlay&&connOverlay.querySelectorAll('.conn-handle').forEach(h=>h.remove());
}

// Convert a polygon's centroid from SVG user-units → overlay-relative px.
function polyFaceCentroid(polygon, stageRect){
  if(!polygon) return null;
  const pts=polygon.points;
  if(!pts||pts.numberOfItems===0) return null;
  let ux=0, uy=0;
  for(let i=0;i<pts.numberOfItems;i++){ ux+=pts.getItem(i).x; uy+=pts.getItem(i).y; }
  ux/=pts.numberOfItems; uy/=pts.numberOfItems;
  const svgPt=polygon.ownerSVGElement.createSVGPoint();
  svgPt.x=ux; svgPt.y=uy;
  const screen=svgPt.matrixTransform(polygon.getScreenCTM());
  return [screen.x-stageRect.left, screen.y-stageRect.top];
}

// Return the nadir (lowest screen point) across all face polygons —
// the bottom-most vertex of the silhouette.
function nadirPoint(g, stageRect){
  let best=null;
  g.querySelectorAll('polygon[data-face]').forEach(poly=>{
    const pts=poly.points;
    const ctm=poly.getScreenCTM();
    const svg=poly.ownerSVGElement;
    for(let i=0;i<pts.numberOfItems;i++){
      const p=svg.createSVGPoint();
      p.x=pts.getItem(i).x; p.y=pts.getItem(i).y;
      const sc=p.matrixTransform(ctm);
      if(!best||sc.y>best[1]) best=[sc.x-stageRect.left, sc.y-stageRect.top];
    }
  });
  return best;
}

function showHandles(g){
  ensureOverlay();
  clearHandles();
  const stage=document.getElementById('zoomer').parentElement.getBoundingClientRect();

  // Compute anchor positions from actual iso face geometry.
  const pts=[
    polyFaceCentroid(g.querySelector('polygon[data-face="top"]'),   stage),
    polyFaceCentroid(g.querySelector('polygon[data-face="right"]'),  stage),
    nadirPoint(g, stage),
    polyFaceCentroid(g.querySelector('polygon[data-face="left"]'),   stage),
  ].filter(Boolean); // drop nulls (shapes that lack a face, e.g. sphere)

  pts.forEach(([cx,cy],side)=>{
    const c=document.createElementNS('http://www.w3.org/2000/svg','circle');
    c.setAttribute('cx',cx); c.setAttribute('cy',cy);
    c.setAttribute('r',HANDLE_R);
    c.setAttribute('fill',HANDLE_COL);
    c.setAttribute('stroke','#fff'); c.setAttribute('stroke-width','1.5');
    c.setAttribute('class','conn-handle');
    c.style.cssText='pointer-events:all;cursor:crosshair;';
    c.dataset.side=side;
    c.dataset.cx=cx; c.dataset.cy=cy;
    c.addEventListener('mouseenter',()=>c.setAttribute('fill',HANDLE_HOV));
    c.addEventListener('mouseleave',()=>c.setAttribute('fill',HANDLE_COL));
    c.addEventListener('mousedown',ev=>{
      if(ev.button!==0) return;
      ev.preventDefault(); ev.stopPropagation();
      const fromId=g.getAttribute('data-part-id').replace(/~\d+$/,'');
      // Rubber-band line
      const line=document.createElementNS('http://www.w3.org/2000/svg','polyline');
      line.setAttribute('points',cx+','+cy+' '+cx+','+cy);
      line.setAttribute('stroke',HANDLE_COL); line.setAttribute('stroke-width','1.8');
      line.setAttribute('fill','none'); line.setAttribute('stroke-dasharray','5,3');
      line.setAttribute('id','conn-rubber');
      connOverlay.appendChild(line);
      connDrag={fromId, startX:cx, startY:cy, line};
      document.body.style.cursor='crosshair';
    });
    connOverlay.appendChild(c);
  });
}

window.addEventListener('mousemove',ev=>{
  if(!connDrag) return;
  const stage=document.getElementById('zoomer').parentElement.getBoundingClientRect();
  const ex=ev.clientX-stage.left, ey=ev.clientY-stage.top;
  connDrag.line.setAttribute('points',connDrag.startX+','+connDrag.startY+' '+ex+','+ey);
},true);

window.addEventListener('mouseup',async ev=>{
  if(!connDrag) return;
  const drag=connDrag; connDrag=null;
  drag.line.remove(); clearHandles();
  document.body.style.cursor='';
  const toId=connTarget;
  connTarget=null;
  if(!toId||toId===drag.fromId||!serverOK) return;
  pushUndo();
  try{
    const r=await fetch('/api/op?op=add-edge&from='+encodeURIComponent(drag.fromId)+'&to='+encodeURIComponent(toId)+'&format='+encodeURIComponent(LANG),
      {method:'POST',body:srcEl.value});
    if(!r.ok) return;
    const data=await r.json();
    if(typeof data.yaml==='string'){ srcEl.value=data.yaml; setDirty(srcEl.value!==ORIGINAL);
      try{localStorage.setItem(DRAFTKEY,srcEl.value);}catch(_){} }
    if(data.svg){ zoomer.innerHTML=data.svg; buildMap(); paint(pinnedRange()); wireHover(); wireDrag(); adaptStage(); markRendered(); }
    showIssues(data.issues||[]);
  }catch(_){}
},true);

function wireHover(){
  zoomer.querySelectorAll('g[data-part-id]').forEach(g=>{
    const id=g.getAttribute('data-part-id');
    g.style.cursor='move';
    g.addEventListener('mouseenter',()=>{
      g.classList.add('hi');
      if(connDrag){ connTarget=id.replace(/~\d+$/,''); return; }
      showHandles(g);
      const r=rangeFor(id); if(!r) return;
      paint(r); scrollToLine(r[0]);
    });
    g.addEventListener('mouseleave',()=>{
      if(connDrag){ if(connTarget===id.replace(/~\d+$/,'')) connTarget=null; return; }
      clearHandles();
      if(pinId!==id) g.classList.remove('hi');
      paint(pinnedRange());
    });
    g.addEventListener('click',e=>{
      e.stopPropagation();
      pinId = (pinId===id)?null:id; pinCi=null;   // pinning a node clears any edge pin
      glowOnly(pinId); glowEdge(null);
      const pr=pinnedRange();
      paint(pr);
      if(pr) scrollToLine(pr[0]);
    });
  });
  // Edges: hover highlights + scrolls to the connector's source lines;
  // click PINS it (persistent highlight) just like a node. The line itself
  // glows via CSS (.pinned / :hover).
  zoomer.querySelectorAll('path[data-connector]').forEach(p=>{
    const ci=+p.getAttribute('data-connector');
    p.addEventListener('mouseenter',()=>{
      const r=connMap[ci]; if(!r) return;
      paint(r); scrollToLine(r[0]);
    });
    p.addEventListener('mouseleave',()=>{
      paint(pinnedRange());
    });
    p.addEventListener('click',e=>{
      e.stopPropagation();
      pinCi = (pinCi===ci)?null:ci; pinId=null;   // pinning an edge clears any node pin
      glowOnly(null); glowEdge(pinCi);
      const pr=pinnedRange();
      paint(pr);
      if(pr) scrollToLine(pr[0]);
    });
  });
  if(pinId) glowOnly(pinId);
  if(pinCi!=null) glowEdge(pinCi);
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
const SNAP_STEP=40;   // one grid cell (world units)
async function commitMove(kind,key,dwx,dwy,dropX,dropY,wp){
  if(!serverOK) return;
  pushUndo();
  const snapEl=document.getElementById('snap');
  const snap=(kind==='node' && snapEl && snapEl.checked) ? SNAP_STEP : 0;
  let qp = kind==='node' ? 'kind=node&id='+encodeURIComponent(key) : 'kind=edge&ci='+key;
  if(wp) qp += '&wp='+encodeURIComponent(JSON.stringify(wp));
  try{
    const r=await fetch('/api/move?'+qp+'&dwx='+dwx+'&dwy='+dwy+'&snap='+snap+'&format='+encodeURIComponent(LANG)+projQS(),
      {method:'POST',body:srcEl.value});
    if(!r.ok) return;
    const data=await r.json();
    if(typeof data.yaml==='string'){ srcEl.value=data.yaml; setDirty(srcEl.value!==ORIGINAL);
      try{localStorage.setItem(DRAFTKEY,srcEl.value);}catch(_){} }
    if(data.svg){
      // A snapped node lands on the grid, NOT under the cursor — so (like an
      // edge bend) hold the scene still on a stable OTHER node instead of
      // drop-anchoring to the release point. Record it before the re-render.
      const holdScene = kind==='edge' || snap>0;
      let anchor=null;
      if(holdScene){
        const ref=[...zoomer.querySelectorAll('g[data-part-id]')].find(g=>g.getAttribute('data-part-id')!==key)
                  || zoomer.querySelector('g[data-part-id]');
        if(ref){const c=ref.getBoundingClientRect();
          anchor={id:ref.getAttribute('data-part-id'),x:c.x+c.width/2,y:c.y+c.height/2};}
      }
      zoomer.innerHTML=data.svg;
      buildMap(); paint(pinnedRange());
      wireHover(); wireDrag(); adaptStage(); markRendered();
      if(!holdScene && kind==='node' && dropX!=null){
        // free drag: pan so the moved node sits exactly under the cursor.
        const g=zoomer.querySelector('g[data-part-id="'+(window.CSS&&CSS.escape?CSS.escape(key):key)+'"]');
        if(g){const c=g.getBoundingClientRect();
          panX+=dropX-(c.x+c.width/2); panY+=dropY-(c.y+c.height/2); apply();}
      }else if(anchor){
        // hold the scene still: return the reference node to where it sat.
        const ref=zoomer.querySelector('g[data-part-id="'+(window.CSS&&CSS.escape?CSS.escape(anchor.id):anchor.id)+'"]');
        if(ref){const c=ref.getBoundingClientRect();
          panX+=anchor.x-(c.x+c.width/2); panY+=anchor.y-(c.y+c.height/2); apply();}
      }
    }
    showIssues(data.issues||[]);
  }catch(_){}
}
/* ── right-click → context menu → "Edit details" detail editor ──────── */
const ctxmenu=document.getElementById('ctxmenu');
const detailModal=document.getElementById('detailModal');
const detailFields=document.getElementById('detailFields');
const detailTitle=document.getElementById('detailTitle');
let ctxTarget=null, detailTarget=null;
function escAttr(t){return String(t).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/"/g,'&quot;');}
// A part is a container (group/boundary/composite lane) if its source block
// holds a nested `parts:` key. Containers can't be duplicated (children would
// get colliding ids) or deleted from the canvas (it'd remove the whole lane),
// matching the server-side guards in ApplyOp — so we hide those items.
function isContainerId(id){
  const r=lineMap[id]; if(!r) return false;
  const lines=srcEl.value.split('\n');
  for(let i=r[0];i<=r[1]&&i<lines.length;i++){
    if(/^\s*parts:\s*(#.*)?$/.test(lines[i])) return true;
  }
  return false;
}
function showCtx(x,y,kind,key){
  ctxTarget={kind,key};
  // Add node only from the empty canvas; Duplicate leaf nodes only; Delete
  // leaf nodes/edges; canvas itself is edit + add only; containers (lanes) are
  // edit-only — structural ops on them corrupt or wipe a whole group.
  const container = kind==='node' && isContainerId(key);
  document.getElementById('ctxadd').hidden = kind!=='canvas';
  document.getElementById('ctxdup').hidden = kind!=='node' || container;
  document.getElementById('ctxdel').hidden = kind==='canvas' || container;
  ctxmenu.style.left=Math.min(x,innerWidth-160)+'px';
  ctxmenu.style.top=Math.min(y,innerHeight-110)+'px';
  ctxmenu.hidden=false;
}
function hideCtx(){ctxmenu.hidden=true;}
document.addEventListener('mousedown',e=>{ if(!ctxmenu.hidden && !ctxmenu.contains(e.target)) hideCtx(); });
document.addEventListener('scroll',hideCtx,true);
document.getElementById('ctxadd').addEventListener('click',()=>{ opCommit('add',{kind:'node',key:''}); });
document.getElementById('ctxedit').addEventListener('click',()=>{ if(ctxTarget) openDetail(ctxTarget); });
document.getElementById('ctxdup').addEventListener('click',()=>{ if(ctxTarget) opCommit('duplicate',ctxTarget); });
document.getElementById('ctxdel').addEventListener('click',()=>{ if(ctxTarget) opCommit('delete',ctxTarget); });
// Structural op (delete/duplicate): same write-back shape as commitMove.
async function opCommit(op,t){
  hideCtx();
  if(!serverOK) return;
  pushUndo();
  try{
    const r=await fetch('/api/op?op='+op+'&'+qpFor(t)+'&format='+encodeURIComponent(LANG)+projQS(),{method:'POST',body:srcEl.value});
    if(!r.ok) return;
    const data=await r.json();
    if(typeof data.yaml==='string'){ srcEl.value=data.yaml; setDirty(srcEl.value!==ORIGINAL);
      try{localStorage.setItem(DRAFTKEY,srcEl.value);}catch(_){} }
    if(data.svg){ zoomer.innerHTML=data.svg; buildMap(); paint(pinnedRange()); wireHover(); wireDrag(); adaptStage(); markRendered(); }
    showIssues(data.issues||[]);
  }catch(_){}
}
/* ── undo / redo of STRUCTURAL edits (drag, detail, delete, duplicate).
   Each commit snapshots the pre-change YAML; ⌘Z/⌘⇧Z restore it. Plain
   typing in the editor keeps the textarea's own native undo. ────────────── */
let undoStack=[], redoStack=[];
function pushUndo(){ undoStack.push(srcEl.value); if(undoStack.length>100) undoStack.shift(); redoStack=[]; }
function applyHistory(v){
  srcEl.value=v; setDirty(v!==ORIGINAL);
  try{ v===ORIGINAL?localStorage.removeItem(DRAFTKEY):localStorage.setItem(DRAFTKEY,v); }catch(_){}
  buildMap(); paint(pinnedRange()); if(serverOK) rerender();
}
function undo(){ if(!undoStack.length) return; redoStack.push(srcEl.value); applyHistory(undoStack.pop()); }
function redo(){ if(!redoStack.length) return; undoStack.push(srcEl.value); applyHistory(redoStack.pop()); }
function qpFor(t){
  if(t.kind==='node') return 'kind=node&id='+encodeURIComponent(t.key);
  if(t.kind==='edge') return 'kind=edge&ci='+t.key;
  return 'kind=canvas';
}
/* ── detail form rendering: grouped sections, illustrated choice tiles,
   compact inline rows, and the raw YAML key shown alongside each label so
   the friendly form stays anchored to the source. ─────────────────────── */
function renderFields(fields){
  // Master-detail layout: groups become TABS in a left rail; the selected
  // tab's fields show in the right pane. ALL panes are rendered up front
  // (inactive ones merely hidden), so switching tabs never discards unsaved
  // edits in the others — applyDetail still reads every [data-key] in the DOM.
  fields.forEach((f,i)=>f._id='df_'+i);
  const order=[], by={};
  fields.forEach(f=>{
    const g=f.group||'General';
    if(!(g in by)){ by[g]={name:g, items:[]}; order.push(g); }
    by[g].items.push(f);
  });
  const tabs=order.map((g,i)=>
    '<button type="button" class="df-tab'+(i===0?' on':'')+'" data-tab="t'+i+'">'+esc(g)+'</button>'
  ).join('');
  const panes=order.map((g,i)=>
    '<div class="df-pane" data-pane="t'+i+'"'+(i===0?'':' hidden')+'>'+groupBody(by[g].items)+'</div>'
  ).join('');
  return '<div class="df-tabs"><div class="df-rail">'+tabs+'</div><div class="df-panes">'+panes+'</div></div>';
}
function groupBody(items){
  let html='', inlineBuf=[];
  const flush=()=>{ if(inlineBuf.length){ html+='<div class="df-inline">'+inlineBuf.join('')+'</div>'; inlineBuf=[]; } };
  items.forEach(f=>{
    if(f.inline){ inlineBuf.push(cellHTML(f,f._id)); }
    else { flush(); html+=rowHTML(f,f._id); }
  });
  flush();
  return html;
}
function rowHTML(f,id){
  return '<div class="df-row">'+
    '<div class="df-labelline"><label class="df-k" for="'+id+'">'+esc(f.label)+'</label><code class="df-path">'+esc(f.key)+'</code></div>'+
    (f.desc?'<div class="df-desc">'+esc(f.desc)+'</div>':'')+
    fieldInput(f,id)+'</div>';
}
function cellHTML(f,id){
  return '<div class="df-cell"><label class="df-ck" for="'+id+'" title="'+escAttr(f.key)+'">'+esc(f.label)+'</label>'+fieldInput(f,id)+'</div>';
}
function fieldInput(f,id){
  const key=escAttr(f.key), orig=escAttr(f.value);
  if(f.type==='choice') return choiceHTML(f,key,orig);
  if(f.type==='color')  return colorHTML(f,id,key,orig);
  if(f.type==='icon')   return iconHTML(f,id,key,orig);
  const t=f.type==='number'?'number':'text';
  // Empty field → muted placeholder of what blank means: the resolved value
  // (effWord+eff) when there is one, else the field's default/off state (empty).
  // Editable value stays empty so an untouched field keeps deferring.
  const ph=f.value?'':(f.eff?effWord(f.key)+' '+f.eff:(f.empty||''));
  return '<input type="'+t+'" id="'+id+'" data-key="'+key+'" data-orig="'+orig+'" value="'+escAttr(f.value)+'" placeholder="'+escAttr(ph)+'">';
}
function choiceHTML(f,key,orig){
  const opts=(f.options||[]).slice(), cur=f.value||'';
  // case-insensitive match; keep an out-of-list / odd-case value as its own tile
  if(cur && !opts.some(o=>o.toLowerCase()===cur.toLowerCase())) opts.unshift(cur);
  const tiles=opts.map(o=>{
    const on=(o.toLowerCase()===cur.toLowerCase())?' on':'';   // '' (solid) matches cur='' too
    return '<button type="button" class="df-tile'+on+'" data-val="'+escAttr(o)+'">'+optGlyph(f.key,o)+'<span>'+esc(optLabel(f.key,o))+'</span></button>';
  }).join('');
  // inherited (no own value) → note the resolved choice so the blank tile isn't misread as "off"
  const hint=(!f.value&&f.eff)?'<div class="df-inherit">'+effWord(f.key)+' '+esc(f.eff)+'</div>':'';
  return '<div class="df-choice" data-key="'+key+'" data-orig="'+orig+'" data-val="'+escAttr(cur)+'">'+tiles+'</div>'+hint;
}
// hint verb for an empty field's resolved value: geometry is solver-computed
// ("auto"), style cascades from shape/preset/theme ("inherits"), and the rest
// (edge routing/stroke) fall back to a fixed render default ("default").
function effWord(key){
  if(key.indexOf('geom.')===0) return 'auto';
  // built-in render defaults (no cascade source) read as "default"
  if(key==='style.text.orient'||key==='style.effects.opacity') return 'default';
  if(key.indexOf('style.')===0) return 'inherits';
  return 'default';
}
function colorHTML(f,id,key,orig){
  // When the field has no own value, the colour is inherited (preset/theme) or
  // a gradient — paint the swatch with the EFFECTIVE colour the server resolved
  // (f.eff) so the picker matches the canvas, and keep the editable value empty
  // so an untouched field keeps inheriting.
  const eff=/^#[0-9a-fA-F]{6}$/.test(f.eff||'')?f.eff:'';
  const hex=/^#[0-9a-fA-F]{6}$/.test(f.value)?f.value:(eff||'#cccccc');
  const ph=f.eff?(effWord(f.key)+' '+f.eff):(f.empty||'unset');
  return '<span class="df-color"><input type="color" data-sync="'+id+'" value="'+hex+'">'+
    '<input type="text" id="'+id+'" data-key="'+key+'" data-orig="'+orig+'" value="'+escAttr(f.value)+'" placeholder="'+escAttr(ph)+'"></span>';
}
function iconHTML(f,id,key,orig){
  const FILE=' accept="image/svg+xml,image/png,image/jpeg,image/gif,image/webp,.svg,.png,.jpg,.jpeg,.gif,.webp"';
  if(/^data:/.test(f.value)){
    return '<span class="df-icon"><span class="df-chip">Embedded image</span>'+
      '<input type="hidden" id="'+id+'" data-key="'+key+'" data-orig="'+orig+'" value="'+escAttr(f.value)+'">'+
      '<button type="button" class="df-browse" data-pick="'+id+'">Replace…</button>'+
      '<button type="button" class="df-clear" data-clear="'+id+'">Clear</button>'+
      '<input type="file" class="df-file" data-pick="'+id+'"'+FILE+' hidden></span>';
  }
  return '<span class="df-icon"><input type="text" id="'+id+'" data-key="'+key+'" data-orig="'+orig+'" value="'+escAttr(f.value)+'" placeholder="iso://… or pick a file">'+
    '<button type="button" class="df-browse" data-pick="'+id+'">Browse…</button>'+
    '<input type="file" class="df-file" data-pick="'+id+'"'+FILE+' hidden></span>';
}
// optGlyph returns a small inline SVG illustrating an enum value, so the
// choice tiles read at a glance instead of being bare words.
function optLabel(key,val){
  const L={'stroke.dash:':'Solid','stroke.dash:6 4':'Dashed','stroke.dash:1 5':'Dotted'};
  if(key.endsWith('.dir')&&val==='') return 'default';
  return L[key+':'+val] || (val||'(none)');
}
function optGlyph(key,val){
  const G={
    'stroke.dash:':'<path d="M3 10 17 10"/>',
    'stroke.dash:6 4':'<path d="M3 10 17 10" stroke-dasharray="5 3"/>',
    'stroke.dash:1 5':'<path d="M3 10 17 10" stroke-dasharray="0.5 4"/>',
    'grid:none':'<rect x="3" y="3" width="14" height="14" rx="2"/>',
    'grid:iso':'<path d="M10 3 17 10 10 17 3 10Z"/><path d="M10 7 13 10 10 13 7 10Z"/>',
    'grid:dots':'<circle cx="6" cy="6" r="1.4"/><circle cx="14" cy="6" r="1.4"/><circle cx="10" cy="10" r="1.4"/><circle cx="6" cy="14" r="1.4"/><circle cx="14" cy="14" r="1.4"/>',
    'grid:hatch':'<path d="M3 13 13 3M7 17 17 7"/>',
    'grid:solid':'<rect x="3" y="3" width="14" height="14" rx="2" fill="currentColor" stroke="none"/>',
    'arrow:none':'<path d="M3 10 17 10"/>',
    'arrow:triangle':'<path d="M3 10 12 10"/><path d="M11 6 17 10 11 14Z" fill="currentColor" stroke="none"/>',
    'routing:orthogonal':'<path d="M3 5 10 5 10 15 17 15"/>',
    'routing:straight':'<path d="M3 16 17 4"/>',
    'routing:bezier':'<path d="M3 16 C9 16 11 4 17 4"/>',
    'shape:rectangle':'<path d="M3 8 10 4 17 8 10 12Z"/><path d="M3 8 3 12 10 16 10 12"/><path d="M17 8 17 12 10 16"/>',
    'shape:cylinder':'<ellipse cx="10" cy="6" rx="6" ry="2.4"/><path d="M4 6 4 14 A6 2.4 0 0 0 16 14 L16 6"/>',
    'shape:circle':'<circle cx="10" cy="10" r="6"/>',
    'shape:cloud':'<path d="M6.5 15 A3 3 0 0 1 7 9 A4 4 0 0 1 14.5 10 A2.6 2.6 0 0 1 14 15Z"/>',
    'shape:person':'<circle cx="10" cy="6.5" r="2.6"/><path d="M5 16 A5 5 0 0 1 15 16"/>',
    'shape:prism':'<path d="M10 3 17 8 14.5 16 5.5 16 3 8Z"/>',
    'shape:hexprism':'<path d="M6 4 14 4 17 10 14 16 6 16 3 10Z"/>',
    'shape:triprism':'<path d="M10 3 17 16 3 16Z"/>',
    'shape:octprism':'<path d="M7 3 13 3 17 7 17 13 13 17 7 17 3 13 3 7Z"/>',
    'shape:diamond':'<path d="M10 3 16 10 10 17 4 10Z"/>',
    'shape:group':'<rect x="3" y="3" width="14" height="14" rx="2" stroke-dasharray="3 2"/>',
    'shape:boundary':'<rect x="3" y="3" width="14" height="14" rx="3" stroke-dasharray="3 2.5"/>',
    'shape:text':'<path d="M5 5 15 5M10 5 10 15"/>',
    'shape:array1d':'<rect x="2" y="8" width="4" height="4"/><rect x="8" y="8" width="4" height="4"/><rect x="14" y="8" width="4" height="4"/>',
    'shape:array2d':'<rect x="3" y="3" width="4" height="4"/><rect x="8" y="3" width="4" height="4"/><rect x="13" y="3" width="4" height="4"/><rect x="3" y="8" width="4" height="4"/><rect x="8" y="8" width="4" height="4"/><rect x="13" y="8" width="4" height="4"/><rect x="3" y="13" width="4" height="4"/><rect x="8" y="13" width="4" height="4"/><rect x="13" y="13" width="4" height="4"/>',
    'shape:array3d':'<rect x="6" y="6" width="4" height="4"/><rect x="11" y="6" width="4" height="4"/><rect x="6" y="11" width="4" height="4"/><rect x="11" y="11" width="4" height="4"/><path d="M8 4 17 4 17 13M13 4 13 8"/>',
    'shape:browser-panel':'<rect x="3" y="4" width="14" height="12" rx="1.5"/><path d="M3 8 17 8"/><circle cx="5.6" cy="6" r="0.7" fill="currentColor" stroke="none"/><circle cx="8" cy="6" r="0.7" fill="currentColor" stroke="none"/>',
    'shape:capsule':'<rect x="3" y="6" width="14" height="8" rx="4"/>',
    'shape:cone':'<path d="M10 3 5 14M10 3 15 14"/><ellipse cx="10" cy="14" rx="5" ry="2"/>',
    'shape:custom_path':'<path d="M4 12 C5 5 9 5 10 9 C11 13 15 13 16 7"/>',
    'shape:dome':'<path d="M4 14 A6 6 0 0 1 16 14"/><path d="M3 14 17 14"/>',
    'shape:frustum':'<path d="M6 5 14 5 17 15 3 15Z"/>',
    'shape:pyramid':'<path d="M10 2 4 11 10 14 16 11Z"/><path d="M4 11 10 14 16 11"/>',
    'shape:rack':'<rect x="5" y="3" width="10" height="14" rx="1"/><path d="M5 7 15 7M5 11 15 11M5 14 15 14"/>',
    'shape:screen':'<rect x="3" y="3" width="14" height="10" rx="1"/><path d="M10 13 10 16M7 16 13 16"/>',
    'shape:torus':'<circle cx="10" cy="10" r="6.5"/><circle cx="10" cy="10" r="2.4"/>',
    'shape:wedge':'<path d="M3 15 12 15 12 6Z"/><path d="M12 6 17 8 17 17 12 15"/><path d="M3 15 8 17 17 17"/>'
  };
  // Gradient direction → an actual arrow (matched by VALUE, since the key
  // varies per face: top/left/rightGradient.dir).
  const DIR={
    '':'<path d="M4 10 16 10"/>',
    'down':'<path d="M10 3 10 16M6 12 10 16 14 12"/>',
    'up':'<path d="M10 17 10 4M6 8 10 4 14 8"/>',
    'left':'<path d="M17 10 4 10M8 6 4 10 8 14"/>',
    'right':'<path d="M3 10 16 10M12 6 16 10 12 14"/>',
    'diag':'<path d="M5 5 15 15M15 9 15 15 9 15"/>'
  };
  let inner=G[key+':'+val];
  if(inner===undefined && key.endsWith('.dir')) inner=DIR[val];
  // No meaningful glyph (weight / align / mode / orient / pattern …) → render
  // a label-only tile rather than a row of identical, meaningless circles.
  if(inner===undefined) return '';
  return '<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">'+inner+'</svg>';
}
function wireDetailInputs(){
  // Tab rail → swap the visible pane (panes stay in the DOM, so edits persist).
  const rail=detailFields.querySelector('.df-rail');
  if(rail) rail.querySelectorAll('.df-tab').forEach(tab=>{
    tab.addEventListener('click',()=>{
      rail.querySelectorAll('.df-tab').forEach(x=>x.classList.remove('on'));
      tab.classList.add('on');
      const id=tab.getAttribute('data-tab');
      detailFields.querySelectorAll('.df-pane').forEach(p=>{ p.hidden=(p.getAttribute('data-pane')!==id); });
    });
  });
  detailFields.querySelectorAll('input[type="color"][data-sync]').forEach(cp=>{
    cp.addEventListener('input',()=>{const t=document.getElementById(cp.getAttribute('data-sync')); if(t)t.value=cp.value;});
  });
  detailFields.querySelectorAll('.df-choice').forEach(ch=>{
    ch.querySelectorAll('.df-tile').forEach(t=>{
      t.addEventListener('click',()=>{
        ch.querySelectorAll('.df-tile').forEach(x=>x.classList.remove('on'));
        t.classList.add('on'); ch.setAttribute('data-val',t.getAttribute('data-val'));
      });
    });
  });
  detailFields.querySelectorAll('.df-browse[data-pick]').forEach(btn=>{
    const pid=btn.getAttribute('data-pick');
    const file=detailFields.querySelector('.df-file[data-pick="'+pid+'"]');
    const text=document.getElementById(pid);
    btn.addEventListener('click',()=>file.click());
    file.addEventListener('change',()=>{const f=file.files&&file.files[0]; if(!f)return; const rd=new FileReader(); rd.onload=()=>{text.value=rd.result;}; rd.readAsDataURL(f);});
  });
  detailFields.querySelectorAll('.df-clear[data-clear]').forEach(b=>{
    b.addEventListener('click',()=>{const t=document.getElementById(b.getAttribute('data-clear')); if(t)t.value='';});
  });
}
// a choice tile-group exposes its value via data-val; everything else via .value
function fieldVal(el){ return el.classList&&el.classList.contains('df-choice') ? el.getAttribute('data-val') : el.value; }
async function openDetail(t){
  hideCtx();
  if(!serverOK) return;
  const qp = qpFor(t);
  try{
    const r=await fetch('/api/fields?'+qp+'&format='+encodeURIComponent(LANG),{method:'POST',body:srcEl.value});
    if(!r.ok) return;
    const data=await r.json();
    detailTarget=t;
    const fields=data.fields||[];
    if(t.kind==='node'){
      detailTitle.textContent='Edit node — '+t.key;
    }else if(t.kind==='canvas'){
      detailTitle.textContent='Edit canvas & background';
    }else{
      const fv=k=>{const f=fields.find(x=>x.key===k);return f?f.value:'';};
      detailTitle.textContent='Edit edge — '+(fv('from')||'?')+' → '+(fv('to')||'?');
    }
    if(!fields.length){
      detailFields.innerHTML='<div class="df-desc" style="padding:8px 0">No editable fields.</div>';
    }else{
      detailFields.innerHTML=renderFields(fields);
      wireDetailInputs();
    }
    detailModal.hidden=false;
  }catch(_){}
}
function closeDetail(){detailModal.hidden=true; detailTarget=null;}
detailModal.addEventListener('mousedown',e=>{ if(e.target===detailModal) closeDetail(); });
async function applyDetail(){
  if(!detailTarget||!serverOK) return;
  const t=detailTarget, changes={};
  detailFields.querySelectorAll('[data-key]').forEach(el=>{
    if(el.disabled) return;
    const v=fieldVal(el);
    if(v!==el.getAttribute('data-orig')) changes[el.getAttribute('data-key')]=v;
  });
  if(!Object.keys(changes).length){ closeDetail(); return; }
  pushUndo();
  const qp = qpFor(t);
  try{
    const r=await fetch('/api/edit?'+qp+'&f='+encodeURIComponent(JSON.stringify(changes))+'&format='+encodeURIComponent(LANG)+projQS(),
      {method:'POST',body:srcEl.value});
    if(!r.ok) return;
    const data=await r.json();
    if(typeof data.yaml==='string'){ srcEl.value=data.yaml; setDirty(srcEl.value!==ORIGINAL);
      try{localStorage.setItem(DRAFTKEY,srcEl.value);}catch(_){} }
    if(data.svg){ zoomer.innerHTML=data.svg; buildMap(); paint(pinnedRange()); wireHover(); wireDrag(); adaptStage(); markRendered(); }
    showIssues(data.issues||[]);
    closeDetail();
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
      if(e.button!==0) return;   // left-button drags only; right-click = context menu
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
    g.addEventListener('contextmenu',e=>{
      e.preventDefault(); e.stopPropagation();
      showCtx(e.clientX,e.clientY,'node',g.getAttribute('data-part-id').replace(/~\d+$/,''));
    });
    // Double-click = edit details, same as right-click → Edit (a no-move
    // release is below the 2px drag threshold, so it never commits a move).
    g.addEventListener('dblclick',e=>{
      e.preventDefault(); e.stopPropagation();
      openDetail({kind:'node',key:g.getAttribute('data-part-id').replace(/~\d+$/,'')});
    });
  });
  zoomer.querySelectorAll('path[data-connector]').forEach(p=>{
    p.setAttribute('stroke-width', Math.max(parseFloat(p.getAttribute('stroke-width')||'1.4'),3));
    p.style.cursor='move';
    p.addEventListener('contextmenu',e=>{
      e.preventDefault(); e.stopPropagation();
      showCtx(e.clientX,e.clientY,'edge',p.getAttribute('data-connector'));
    });
    p.addEventListener('dblclick',e=>{
      e.preventDefault(); e.stopPropagation();
      openDetail({kind:'edge',key:p.getAttribute('data-connector')});
    });
    p.addEventListener('mousedown',e=>{
      if(e.button!==0) return;   // left-button drags only; right-click = context menu
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
  // live drop-target feedback: highlight the group the node would land in, so
  // it's obvious BEFORE releasing whether the re-home was recognised.
  if(nodeDrag) highlightDropTarget(containerUnder(e.clientX, e.clientY, d.id));
});
// Mark one group slab as the active drop target (clears any previous one).
let dropTargetId=null;
function highlightDropTarget(id){
  if(id===dropTargetId) return;
  dropTargetId=id;
  zoomer.querySelectorAll('g[data-part-id].drop-target').forEach(g=>g.classList.remove('drop-target'));
  if(id){
    const g=zoomer.querySelector('g[data-part-id="'+(window.CSS&&CSS.escape?CSS.escape(id):id)+'"]');
    if(g) g.classList.add('drop-target');
  }
}
window.addEventListener('mouseup',e=>{
  const d=nodeDrag||edgeDrag, kind=nodeDrag?'node':(edgeDrag?'edge':null);
  highlightDropTarget(null);   // clear the drop-target glow regardless of outcome
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
  }else if(kind==='node'){
    // Position first, then re-home: drop onto a group's slab joins it; drop on
    // bare canvas returns it to the scene root. The server no-ops the reparent
    // when the parent is unchanged, so an in-group nudge keeps its offset.
    const target=containerUnder(e.clientX, e.clientY, d.id);
    commitMove(kind, d.id, Math.round(wd[0]), Math.round(wd[1]), e.clientX, e.clientY)
      .then(()=>reparentNode(d.id, target||''));
  }else{
    commitMove(kind, d.ci, Math.round(wd[0]), Math.round(wd[1]), e.clientX, e.clientY);
  }
});

// containerUnder returns the id of the innermost container (group/lane) slab
// whose on-screen box contains (cx,cy), excluding the dragged node, or null
// (→ scene root). Used to re-home a dragged node into / out of a group.
function containerUnder(cx, cy, exceptId){
  // Hit-test the ACTUAL painted geometry (iso slabs are rhombi — their
  // axis-aligned boxes overlap heavily, so a bbox test highlights the wrong
  // lane). Walk the paint stack at the cursor: the first container slab whose
  // pixels are under the cursor (skipping the dragged node + leaf nodes that
  // sit on top of a slab) is the drop target.
  for(const el of document.elementsFromPoint(cx, cy)){
    const g=el.closest && el.closest('g[data-part-id]');
    if(!g) continue;
    const id=g.getAttribute('data-part-id').replace(/~\d+$/,'');
    if(id===exceptId) continue;        // the node being dragged
    if(isContainerId(id)) return id;   // a group slab under the cursor → target
    // a leaf node on top of a slab: keep descending to the slab behind it
  }
  return null;
}
async function reparentNode(id, target){
  if(!serverOK) return;
  pushUndo();
  try{
    const r=await fetch('/api/op?op=reparent&id='+encodeURIComponent(id)+'&target='+encodeURIComponent(target)+'&format='+encodeURIComponent(LANG)+projQS(),{method:'POST',body:srcEl.value});
    if(!r.ok){ redoStack=[]; undoStack.pop(); return; }   // 422 (e.g. into own child) → drop the no-op undo entry
    const data=await r.json();
    if(typeof data.yaml==='string'){ srcEl.value=data.yaml; setDirty(srcEl.value!==ORIGINAL); try{localStorage.setItem(DRAFTKEY,srcEl.value);}catch(_){} }
    if(data.svg){ zoomer.innerHTML=data.svg; buildMap(); paint(pinnedRange()); wireHover(); wireDrag(); adaptStage(); markRendered(); }
  }catch(_){ undoStack.pop(); }
}

/* reverse mapping: caret inside a part's block lights the node up */
function caretSync(){
  const line=srcEl.value.slice(0,srcEl.selectionStart).split('\n').length-1;
  let hitId=null;
  for(const id in lineMap){
    const r=lineMap[id];
    if(line>=r[0]&&line<=r[1]){hitId=id;break;}
  }
  if(!pinId && pinCi==null) glowOnly(hitId);
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
  if(pinId!==null||pinCi!==null){pinId=null;pinCi=null;glowOnly(null);glowEdge(null);paint(null);}
});
// Right-click the empty canvas → edit the whole-image background/grid. Node
// and edge contextmenu handlers stopPropagation, so this fires only on bare
// canvas (or a node/edge face that isn't draggable, which is fine).
viewport.addEventListener('contextmenu',e=>{
  if(e.target.closest && (e.target.closest('g[data-part-id]')||e.target.closest('path[data-connector]'))) return;
  e.preventDefault();
  showCtx(e.clientX,e.clientY,'canvas','');
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
  paint(pinnedRange());
  scrollToLine(ln);
  setTimeout(()=>{flashLine=-1;paint(pinnedRange());},1500);
}

/* ── live re-render against isotopo serve ───────────────────────── */
const liveEl=document.getElementById('live'), renderBtn=document.getElementById('render');
document.getElementById('auto').addEventListener('change',e=>{
  if(!e.target.checked) clearTimeout(timer);
});
let serverOK=false, timer=null;
// One consolidated status (top-right). It replaces the old renderer pill AND
// the file-tab dirty dot: it reports connection + freshness in one place,
// stamping the time of the last successful render so the user can see how
// current the canvas is (auto-render is on by default).
function nowTime(){const d=new Date(),p=n=>String(n).padStart(2,'0');return p(d.getHours())+':'+p(d.getMinutes())+':'+p(d.getSeconds());}
function setStatus(cls,text,title){liveEl.className=cls;liveEl.textContent=text;liveEl.title=title||'';}
function markRendered(){setStatus('ok','Rendered '+nowTime(),'canvas is in sync with the code as of this time');}
async function probe(){
  if(!location.protocol.startsWith('http')){
    setStatus('','Static file','run "isotopo serve <input>" for live re-render');
    renderBtn.disabled=true; return;
  }
  const was=serverOK;
  try{
    const r=await fetch('/api/ping');
    serverOK=r.ok;
  }catch(_){serverOK=false;}
  renderBtn.disabled=!serverOK;
  if(!serverOK){
    setStatus('err','Offline','renderer unreachable — is "isotopo serve" still running?');
  }else if(!was){
    markRendered(); // recovered (or first probe): the shown canvas is current
  }
}
// View mode: 'iso' (2.5D, default) or 'top' (flat plan). It's a PREVIEW
// override sent as ?projection= on every render/edit round-trip — the source
// the editor holds is never rewritten, so toggling back is lossless.
let VIEWMODE='iso';
function projQS(){ return VIEWMODE==='top' ? '&projection=top' : ''; }
function toggleView(){
  VIEWMODE = VIEWMODE==='top' ? 'iso' : 'top';
  const b=document.getElementById('viewtoggle');
  if(b){ b.textContent = VIEWMODE==='top' ? '◰ Plan' : '◳ Iso'; b.classList.toggle('on', VIEWMODE==='top'); }
  if(serverOK) rerender();
}
async function rerender(){
  if(!serverOK) return;
  renderBtn.textContent='Rendering…';
  setStatus('','Rendering…','');
  try{
    const r=await fetch('/api/render?format='+encodeURIComponent(LANG)+projQS(),{method:'POST',body:srcEl.value});
    const data=await r.json();
    showIssues(data.issues||[]);
    if(data.svg){
      zoomer.innerHTML=data.svg;
      buildMap(); paint(pinnedRange()); wireHover(); wireDrag(); adaptStage();
      staleEl.hidden=true;
      markRendered();
    }else{
      staleEl.hidden=false;
      setStatus('err','Render failed','the source has errors — see the issues panel');
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
  // ⌘Z / ⌘⇧Z undo-redo of structural edits — only when not typing in the
  // editor (the textarea keeps its own native text undo).
  if((e.metaKey||e.ctrlKey)&&(e.key==='z'||e.key==='Z')&&document.activeElement!==srcEl){
    e.preventDefault(); e.shiftKey?redo():undo(); return;
  }
  if((e.metaKey||e.ctrlKey)&&e.key==='Enter'){e.preventDefault();rerender();return;}
  if((e.metaKey||e.ctrlKey)&&e.key==='0'){e.preventDefault();fitView();return;}
  if((e.metaKey||e.ctrlKey)&&(e.key==='='||e.key==='+')){e.preventDefault();zoomBy(1.25);return;}
  if((e.metaKey||e.ctrlKey)&&e.key==='-'){e.preventDefault();zoomBy(0.8);return;}
  if(e.key==='Escape'){document.getElementById('help').hidden=true;hideCtx();closeDetail();return;}
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

/* ── editor panel toggle (collapse/expand) ──────────────────────── */
const gridEl=document.getElementById('grid');
const sideEl=document.querySelector('.side'), splitEl=document.getElementById('split');
const toggleBtn=document.getElementById('paneltoggle');
let panelOpen=false;
function updateToggleIcon(){
  toggleBtn.textContent=panelOpen?'◀':'▶';
  toggleBtn.title=panelOpen?'hide editor (⌘E)':'show editor (⌘E)';
}
function setPanel(open,save){
  panelOpen=open;
  sideEl.classList.toggle('collapsed',!open);
  splitEl.style.display=open?'':'none';
  gridEl.classList.toggle('collapsed',!open);
  updateToggleIcon();
  if(save){try{localStorage.setItem('isotopo-panel',open?'1':'0');}catch(_){}}
}
function togglePanel(){setPanel(!panelOpen,true);}
window.addEventListener('keydown',e=>{
  if((e.metaKey||e.ctrlKey)&&e.key==='e'){e.preventDefault();togglePanel();}
});
(function(){
  let v='0';
  try{v=localStorage.getItem('isotopo-panel')??'0';}catch(_){}
  setPanel(v==='1',false);
})();

/* ── editor pane drag-resize ────────────────────────────────────── */
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
function downloadCopy(){
  const blob=new Blob([srcEl.value],{type:'text/plain'});
  const a=document.createElement('a');
  a.href=URL.createObjectURL(blob);
  a.download=FILENAME.replace(/(\.[a-z0-9]+)$/i,'.edited$1');
  a.click();
}
/* ── #src= permalink decode (kept so older shared links still load) ─ */
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
async function inflateText(b64){
  const s2=atob(b64.replace(/-/g,'+').replace(/_/g,'/'));
  const bytes=new Uint8Array(s2.length);
  for(let i=0;i<s2.length;i++) bytes[i]=s2.charCodeAt(i);
  const out=await pipeThrough(new DecompressionStream('deflate-raw'),bytes);
  return new TextDecoder().decode(out);
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
