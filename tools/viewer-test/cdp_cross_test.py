#!/usr/bin/env python3
"""Scenario-based CROSS-INTERACTION suite for isotopo Studio
(topology.html template in output.go + `isotopo serve` in cmd/isotopo).

Where cdp_test.py asserts single features, this suite CHAINS flows:
every button is an entry point, and scenarios combine them — load an
example then share it, break the source then download, pin a node then
re-render, resize the pane then reload, open help over a pinned node…

Each scenario is ONE named check whose value is a JSON verdict of
sub-flags; the expectation encodes CORRECT behavior, so a FAIL with a
readable verdict is a recorded page bug (this suite does not fix them).

Run from the repo root:  python3 tools/viewer-test/cdp_cross_test.py
Needs Chrome + `pip install websocket-client`. Serves on :8737.
"""
import json, subprocess, time, urllib.request, websocket, tempfile, sys, os

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
os.chdir(REPO)
SAMPLE = "samples/topology/ai-platform/input.yaml"
with open(SAMPLE) as f:
    SAMPLE_TEXT = f.read()
# The examples menu fetches raw.githubusercontent.com; tests must not
# depend on the network, so a fetch stub serves this marked variant.
EXAMPLE_YAML = SAMPLE_TEXT.replace('label: "GPU Pool"', 'label: "EXAMPLE_MARK_POOL"')

subprocess.run(["pkill", "-f", "isotopo-cross-test"], capture_output=True)
time.sleep(0.5)
BIN = f"/tmp/isotopo-cross-test-{os.getpid()}"
subprocess.run(["go", "build", "-o", BIN, "./cmd/isotopo"], check=True)
serve = subprocess.Popen([BIN, "serve", SAMPLE],
    env={**os.environ, "ISOTOPO_PORT": "8737"},
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(1.2)

CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
tmp = tempfile.mkdtemp()
chrome = subprocess.Popen([CHROME, "--headless=new", "--remote-debugging-port=9226",
    "--remote-allow-origins=*", "--window-size=1400,900",
    f"--user-data-dir={tmp}", "--no-first-run", "--disable-gpu",
    "http://localhost:8737/"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(3)

targets = json.load(urllib.request.urlopen("http://127.0.0.1:9226/json"))
page = [t for t in targets if t["type"] == "page"][0]
ws = websocket.create_connection(page["webSocketDebuggerUrl"], timeout=30)
mid = 0

def ev(expr, timeout=40):
    global mid; mid += 1
    ws.send(json.dumps({"id": mid, "method": "Runtime.evaluate",
        "params": {"expression": expr, "returnByValue": True, "awaitPromise": True}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid:
            r = m["result"].get("result", {})
            if "value" in r: return r["value"]
            return r.get("description")

def cdp(method, params=None):
    global mid; mid += 1
    ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid: return m

def pin_viewport():
    # --window-size is unreliable under headless=new on macOS; pin the
    # viewport at the protocol level so pane/fit math is deterministic.
    cdp("Emulation.setDeviceMetricsOverride",
        {"width": 1400, "height": 900, "deviceScaleFactor": 1, "mobile": False})

SETUP = r"""
window.sleep=ms=>new Promise(r=>setTimeout(r,ms));
window.installClip=function(){
  try{Object.defineProperty(navigator,'clipboard',{configurable:true,
    value:{writeText:t=>{window.__clip=String(t);return Promise.resolve();}}});}catch(e){}
};
window.installDl=function(){
  if(window.__dlPatched) return;
  window.__dlPatched=true; window.__dl=[];
  const orig=HTMLAnchorElement.prototype.click;
  HTMLAnchorElement.prototype.click=function(){
    if(this.download){window.__dl.push({name:this.download,href:this.href});}
    else{orig.call(this);}
  };
};
window.installStub=function(){
  if(window.__stubbed) return; window.__stubbed=true;
  const rf=window.fetch.bind(window);
  window.fetch=function(u,o){
    if(String(u).indexOf('raw.githubusercontent.com')>=0)
      return Promise.resolve(new Response(window.__EX_YAML,{status:200}));
    return rf(u,o);
  };
};
true
"""

def setup_page():
    pin_viewport()
    ev(SETUP)
    ev("window.__EX_YAML=" + json.dumps(EXAMPLE_YAML) + ";true")

def reload_page(set_hash=None, wait=4.0):
    if set_hash is not None:
        ev("location.hash=" + json.dumps(set_hash) + ";true")
    ev("location.reload()", timeout=30)
    time.sleep(wait)
    setup_page()
    time.sleep(0.5)

results = []
def scenario(name, expr, expect):
    raw = ev(expr, timeout=60)
    try:
        d = json.loads(raw) if isinstance(raw, str) else raw
    except Exception:
        d = raw
    try:
        ok = expect(d) if callable(expect) else d == expect
    except Exception:
        ok = False
    results.append((name, ok, d))

def record(name, d, expect):
    try:
        ok = expect(d) if callable(expect) else d == expect
    except Exception:
        ok = False
    results.append((name, ok, d))

setup_page()
time.sleep(1.5)  # let probe() settle

# ── S0 sanity: live server, render armed ─────────────────────────────
scenario("s00-sanity-live", """
(()=>{return JSON.stringify({
  live: document.getElementById('live').textContent==='Live',
  renderArmed: !document.getElementById('render').disabled,
  clean: !document.getElementById('dirty').classList.contains('on')});})()
""", lambda d: d == {"live": True, "renderArmed": True, "clean": True})

# ── S1 zoom chain: ⌘± → percent readout → ⌘0 fit → dblclick reset →
#    wheel+drag → percent-button reset ────────────────────────────────
scenario("s01-zoom-chain", """
(()=>{resetView();
window.dispatchEvent(new KeyboardEvent('keydown',{key:'=',metaKey:true,bubbles:true,cancelable:true}));
const z1=scale;
window.dispatchEvent(new KeyboardEvent('keydown',{key:'=',metaKey:true,bubbles:true,cancelable:true}));
const z2=scale, pct=document.getElementById('zpct').textContent;
window.dispatchEvent(new KeyboardEvent('keydown',{key:'-',metaKey:true,bubbles:true,cancelable:true}));
const z3=scale;
window.dispatchEvent(new KeyboardEvent('keydown',{key:'0',metaKey:true,bubbles:true,cancelable:true}));
const fitMoved=scale!==z3;
viewport.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));
const reset1=scale===1&&panX===0&&panY===0;
viewport.dispatchEvent(new WheelEvent('wheel',{deltaY:-120,clientX:300,clientY:300,bubbles:true,cancelable:true}));
viewport.dispatchEvent(new MouseEvent('mousedown',{clientX:400,clientY:300,bubbles:true}));
window.dispatchEvent(new MouseEvent('mousemove',{clientX:460,clientY:340,bubbles:true}));
window.dispatchEvent(new MouseEvent('mouseup',{bubbles:true}));
const moved=scale!==1||panX!==0;
document.getElementById('zpct').click();
const reset2=scale===1&&panX===0&&panY===0;
return JSON.stringify({z1,z2,pct,z3,fitMoved,reset1,moved,reset2,
  pctNow:document.getElementById('zpct').textContent});})()
""", lambda d: round(d["z1"], 4) == 1.25 and round(d["z2"], 4) == 1.5625
     and d["pct"] == "156%" and round(d["z3"], 4) == 1.25 and d["fitMoved"]
     and d["reset1"] and d["moved"] and d["reset2"] and d["pctNow"] == "100%")

# ── S2 help over a pinned node: pin → footer link opens help → Esc
#    closes → pin + highlight intact ─────────────────────────────────
scenario("s02-help-pin-esc", """
(()=>{const g=document.querySelector('g[data-part-id="agents"]');
g.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const pinned=pinId==='agents';
document.querySelector('footer a[onclick]').click();
const helpShown=!document.getElementById('help').hidden;
window.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
const helpHidden=document.getElementById('help').hidden;
const pinIntact=pinId==='agents'&&document.querySelector('g[data-part-id="agents"]').classList.contains('hi');
const hitsStill=document.querySelectorAll('#hl .hit').length>0;
viewport.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const unpinned=pinId===null;
return JSON.stringify({pinned,helpShown,helpHidden,pinIntact,hitsStill,unpinned});})()
""", lambda d: all(d.values()))

# ── S3 help via "?" key, card click keeps it, backdrop closes it,
#    "?" typed in the editor is guarded ─────────────────────────────
scenario("s03-help-key-backdrop-guard", """
(()=>{if(document.activeElement)document.activeElement.blur();
window.dispatchEvent(new KeyboardEvent('keydown',{key:'?',bubbles:true}));
const opened=!document.getElementById('help').hidden;
document.querySelector('.help-card').dispatchEvent(new MouseEvent('click',{bubbles:true}));
const cardKeepsOpen=!document.getElementById('help').hidden;
const h=document.getElementById('help');
h.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const backdropCloses=h.hidden;
srcEl.focus();
window.dispatchEvent(new KeyboardEvent('keydown',{key:'?',bubbles:true}));
const guarded=document.getElementById('help').hidden;
srcEl.blur();
return JSON.stringify({opened,cardKeepsOpen,backdropCloses,guarded});})()
""", lambda d: all(d.values()))

# ── S4 examples menu vs keyboard/canvas: Esc should close the open
#    menu (UX expectation); outside click does ───────────────────────
scenario("s04-examples-esc-vs-outside-click", """
(()=>{toggleExamples();
const open=!document.getElementById('expanel').hidden;
window.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
const escCloses=document.getElementById('expanel').hidden;
document.getElementById('stage').dispatchEvent(new MouseEvent('click',{bubbles:true}));
const outsideCloses=document.getElementById('expanel').hidden;
return JSON.stringify({open,escCloses,outsideCloses});})()
""", lambda d: all(d.values()))

# ── S5 copy-path button: clipboard gets PATH, icon swaps + restores ──
scenario("s05-copy-path-feedback", """
(async()=>{installClip(); window.__clip=null;
const b=document.getElementById('cppath'), keep=b.innerHTML;
b.click();
await sleep(250);
const swapped=b.innerHTML!==keep;
const copied=window.__clip===PATH&&PATH.startsWith('/');
await sleep(1300);
const restored=b.innerHTML===keep;
return JSON.stringify({copied,swapped,restored});})()
""", lambda d: all(d.values()))

# ── S6 export hygiene: pin a node, export — the exported SVG should
#    NOT carry viewer instrumentation (cursor styles / .hi class) ─────
scenario("s06-export-instrumentation-leak", """
(async()=>{const raw=await (await fetch('/topology.svg')).text();
const rawClean=raw.indexOf('cursor: pointer')<0&&raw.indexOf('class="hi"')<0;
const g=document.querySelector('g[data-part-id="core"]');
g.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const out=currentSVG();
const cursorLeak=out.indexOf('cursor: pointer')>=0||out.indexOf('cursor:pointer')>=0;
const hiLeak=/class="[^"]*\\bhi\\b/.test(out);
viewport.dispatchEvent(new MouseEvent('click',{bubbles:true}));
return JSON.stringify({rawClean,cursorLeak,hiLeak});})()
""", lambda d: d == {"rawClean": True, "cursorLeak": False, "hiLeak": False})

# ── S7 caret reverse-map vs pin: caret lights a node, pin outranks
#    caret, unpin resumes caret sync ─────────────────────────────────
scenario("s07-caret-pin-precedence", """
(()=>{const lines=srcEl.value.split('\\n');
const idx=lines.findIndex(l=>l.indexOf('label: "Vector Store"')>=0);
const pos=lines.slice(0,idx).join('\\n').length+3;
srcEl.focus(); srcEl.selectionStart=srcEl.selectionEnd=pos;
srcEl.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const vsGlow=document.querySelector('g[data-part-id="vector_store"]').classList.contains('hi');
const ga=document.querySelector('g[data-part-id="agents"]');
ga.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const aPinned=pinId==='agents';
srcEl.selectionStart=srcEl.selectionEnd=pos;
srcEl.dispatchEvent(new KeyboardEvent('keyup',{key:'ArrowLeft',bubbles:true}));
const aStill=document.querySelector('g[data-part-id="agents"]').classList.contains('hi');
const vsNot=!document.querySelector('g[data-part-id="vector_store"]').classList.contains('hi');
viewport.dispatchEvent(new MouseEvent('click',{bubbles:true}));
srcEl.dispatchEvent(new KeyboardEvent('keyup',{key:'ArrowLeft',bubbles:true}));
const vsBack=document.querySelector('g[data-part-id="vector_store"]').classList.contains('hi');
srcEl.blur(); glowOnly(null); paint(null);
return JSON.stringify({vsGlow,aPinned,aStill,vsNot,vsBack});})()
""", lambda d: all(d.values()))

# ── S8 pin survives re-render: pin → edit another node → auto render
#    swaps the SVG → pin + painted range survive on the NEW DOM ──────
scenario("s08-pin-survives-rerender", """
(async()=>{const g=document.querySelector('g[data-part-id="gpu_pool"]');
g.dispatchEvent(new MouseEvent('click',{bubbles:true}));
const pinnedBefore=pinId==='gpu_pool'&&g.classList.contains('hi');
srcEl.value=srcEl.value.replace('label: "Agents"','label: "AGENTS_EDIT_MARK"');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(1700);
const rendered=zoomer.innerHTML.indexOf('AGENTS_EDIT_MARK')>=0;
const g2=document.querySelector('g[data-part-id="gpu_pool"]');
const pinSurvives=!!g2&&g2.classList.contains('hi')&&pinId==='gpu_pool';
const rangePainted=document.querySelectorAll('#hl .hit').length>0;
viewport.dispatchEvent(new MouseEvent('click',{bubbles:true}));
discardDraft(); await sleep(1400);
const cleaned=srcEl.value.indexOf('AGENTS_EDIT_MARK')<0;
return JSON.stringify({pinnedBefore,rendered,pinSurvives,rangePainted,cleaned});})()
""", lambda d: all(d.values()))

# ── S9 break → click issue → fix → stale clears → export captures the
#    FIXED render under the .edited name (SVG + PNG) ──────────────────
scenario("s09-break-issue-fix-export", """
(async()=>{installDl();
srcEl.value=srcEl.value.replace('label: "Guardrails"','label: "EXPORT_MARK"');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(1700);
const goodFirst=zoomer.innerHTML.indexOf('EXPORT_MARK')>=0;
srcEl.value=srcEl.value.replace('shape: hexprism','shape: hexprsim');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(1700);
const staleShown=!staleEl.hidden;
const issuesShown=document.getElementById('issues').classList.contains('show');
const first=document.querySelector('#issues [data-path]');
let jumped=false;
if(first){first.click(); jumped=document.querySelectorAll('#hl .flash').length>0;}
srcEl.value=srcEl.value.replace('shape: hexprsim','shape: hexprism');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(1700);
const staleCleared=staleEl.hidden;
const issuesCleared=!document.getElementById('issues').classList.contains('show');
exportSVG(); await sleep(400);
const dsvg=window.__dl[window.__dl.length-1]||{};
let svgHasFix=false;
try{svgHasFix=(await (await fetch(dsvg.href)).text()).indexOf('EXPORT_MARK')>=0;}catch(_){}
exportPNG(); await sleep(1600);
const dpng=window.__dl[window.__dl.length-1]||{};
let pngOk=false;
try{const bl=await (await fetch(dpng.href)).blob(); pngOk=bl.size>1000&&bl.type==='image/png';}catch(_){}
discardDraft(); await sleep(1400);
return JSON.stringify({goodFirst,staleShown,issuesShown,jumped,staleCleared,issuesCleared,
  svgName:dsvg.name||'',svgHasFix,pngName:dpng.name||'',pngOk});})()
""", lambda d: d["goodFirst"] and d["staleShown"] and d["issuesShown"] and d["jumped"]
     and d["staleCleared"] and d["issuesCleared"] and d["svgName"] == "input.edited.svg"
     and d["svgHasFix"] and d["pngName"] == "input.edited.png" and d["pngOk"])

# ── S10 download while the source is broken: Download hands over the
#    broken text; export hands over the LAST GOOD svg (named .edited —
#    name/content mismatch is recorded by the verdict) ────────────────
scenario("s10-download-while-broken", """
(async()=>{installDl();
srcEl.value=srcEl.value.replace('shape: hexprism','shape: hexbroken');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(1700);
const broken=!staleEl.hidden;
const dlEnabled=!document.getElementById('dl').disabled;
downloadCopy(); await sleep(300);
const d1=window.__dl[window.__dl.length-1]||{};
let dlHasBrokenText=false;
try{dlHasBrokenText=(await (await fetch(d1.href)).text()).indexOf('hexbroken')>=0;}catch(_){}
exportSVG(); await sleep(400);
const d2=window.__dl[window.__dl.length-1]||{};
let exportText='';
try{exportText=await (await fetch(d2.href)).text();}catch(_){}
const exportIsLastGood=exportText.indexOf('<svg')>=0&&exportText.indexOf('hexbroken')<0;
discardDraft(); await sleep(1400);
return JSON.stringify({broken,dlEnabled,dlName:d1.name||'',dlHasBrokenText,
  exportName:d2.name||'',exportIsLastGood});})()
""", lambda d: d["broken"] and d["dlEnabled"] and d["dlName"] == "input.edited.yaml"
     and d["dlHasBrokenText"] and d["exportName"] == "input.edited.svg" and d["exportIsLastGood"])

# ── S11 Tab chain: Tab indents a top-level key → YAML breaks → issues
#    + stale → discard recovers everything ───────────────────────────
scenario("s11-tab-breaks-discard-recovers", """
(async()=>{const p=srcEl.value.indexOf('canvas:');
srcEl.focus(); srcEl.selectionStart=srcEl.selectionEnd=p;
srcEl.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',bubbles:true,cancelable:true}));
const inserted=srcEl.value.indexOf('  canvas:')>=0;
const dirtyOn=document.getElementById('dirty').classList.contains('on');
await sleep(1700);
const staleShown=!staleEl.hidden;
const issuesShown=document.getElementById('issues').classList.contains('show');
srcEl.blur();
discardDraft(); await sleep(1400);
const recovered=staleEl.hidden&&!document.getElementById('issues').classList.contains('show')
  &&!document.getElementById('dirty').classList.contains('on');
return JSON.stringify({inserted,dirtyOn,staleShown,issuesShown,recovered});})()
""", lambda d: all(d.values()))

# ── S12 empty doc: a parseable doc with no nodes should still give the
#    user a signal (issue or message), not a silent stale badge ───────
scenario("s12-empty-doc-feedback", """
(async()=>{const api=await (await fetch('/api/render?format=yaml',{method:'POST',body:'theme: {}\\n'})).json();
const apiSvgEmpty=!api.svg;
const apiIssueCount=(api.issues||[]).length;
srcEl.value='theme: {}\\n';
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(1700);
const staleShown=!staleEl.hidden;
const issuesShown=document.getElementById('issues').classList.contains('show');
discardDraft(); await sleep(1400);
return JSON.stringify({apiSvgEmpty,apiIssueCount,staleShown,issuesShown});})()
""", lambda d: not d["apiSvgEmpty"] or d["apiIssueCount"] > 0)

# ── S13 debounce race: edit (arms 600ms timer) then uncheck Auto
#    immediately — render should NOT fire, but the armed timer does ───
scenario("s13-auto-uncheck-debounce-race", """
(async()=>{srcEl.value=srcEl.value.replace('label: "Data Lake"','label: "RACE_MARK"');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
document.getElementById('auto').checked=false;
await sleep(1500);
const renderedAnyway=zoomer.innerHTML.indexOf('RACE_MARK')>=0;
document.getElementById('auto').checked=true;
discardDraft(); await sleep(1400);
return JSON.stringify({renderedAnyway});})()
""", lambda d: d == {"renderedAnyway": False})

# ── S14 share while help is open: permalink lands in the clipboard,
#    round-trips to the editor text, help stays open ──────────────────
scenario("s14-share-while-help-open", """
(async()=>{installClip(); window.__clip=null;
toggleHelp();
await shareLink();
const url=window.__clip||'';
const labelWas=document.getElementById('share').textContent;
const helpStill=!document.getElementById('help').hidden;
toggleHelp();
await sleep(1500);
const labelRestored=document.getElementById('share').textContent==='Share';
let roundtrip=false;
try{roundtrip=(await inflateText(url.split('#src=')[1]))===srcEl.value;}catch(_){}
return JSON.stringify({hasHash:url.indexOf('#src=')>=0,labelWas,helpStill,labelRestored,roundtrip});})()
""", lambda d: d["hasHash"] and d["labelWas"] == "Link copied" and d["helpStill"]
     and d["labelRestored"] and d["roundtrip"])

# ── S15 example load with Auto OFF: editor swaps, canvas stays, manual
#    Render button catches up ─────────────────────────────────────────
scenario("s15-example-auto-off-manual-render", """
(async()=>{installStub();
document.getElementById('auto').checked=false;
await loadExample('rag-pipeline');
const loaded=srcEl.value.indexOf('EXAMPLE_MARK_POOL')>=0;
const dirtyOn=document.getElementById('dirty').classList.contains('on');
await sleep(1700);
const notAutoRendered=zoomer.innerHTML.indexOf('EXAMPLE_MARK_POOL')<0;
document.getElementById('render').click();
await sleep(1500);
const manualRendered=zoomer.innerHTML.indexOf('EXAMPLE_MARK_POOL')>=0;
document.getElementById('auto').checked=true;
discardDraft(); await sleep(1400);
return JSON.stringify({loaded,dirtyOn,notAutoRendered,manualRendered});})()
""", lambda d: all(d.values()))

# ── S16 footer links resolve in serve mode ───────────────────────────
def _status(url):
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            return r.status, r.read(200).decode("utf-8", "replace")
    except Exception:
        return 0, ""
svg_status, svg_head = _status("http://localhost:8737/topology.svg")
nodes_status, _ = _status("http://localhost:8737/nodes/_index.html")
anchors = ev("""JSON.stringify({
  orig: !!document.querySelector('footer a[href="./topology.svg"]'),
  nodes: !!document.querySelector('footer a[href="./nodes/_index.html"]'),
  about: !!document.querySelector('footer a[href*="studio.md"]')})""")
record("s16-footer-links-alive",
    {"svgStatus": svg_status, "svgIsSvg": "<svg" in svg_head or "<?xml" in svg_head,
     "nodesStatus": nodes_status, **json.loads(anchors)},
    lambda d: d["svgStatus"] == 200 and d["svgIsSvg"] and d["nodesStatus"] == 200
     and d["orig"] and d["nodes"] and d["about"])

# ── S17 draft → reload → restored AND auto re-rendered after probe ───
ev("""(async()=>{srcEl.value=srcEl.value.replace('label: "Streaming"','label: "DRAFT_RELOAD_MARK"');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(900); return true;})()""")
reload_page()
scenario("s17-draft-reload-autorender", """
(async()=>{await sleep(1500);
const draftKept=srcEl.value.indexOf('DRAFT_RELOAD_MARK')>=0;
const dirtyOn=document.getElementById('dirty').classList.contains('on');
const canvasUpdated=zoomer.innerHTML.indexOf('DRAFT_RELOAD_MARK')>=0;
discardDraft(); await sleep(1400);
const cleaned=srcEl.value.indexOf('DRAFT_RELOAD_MARK')<0
  &&localStorage.getItem(DRAFTKEY)===null;
return JSON.stringify({draftKept,dirtyOn,canvasUpdated,cleaned});})()
""", lambda d: all(d.values()))

# ── S18 splitter → persist → reload → width survives → fit respects
#    the resized viewport ────────────────────────────────────────────
pre = ev("""
(()=>{const side=document.querySelector('.side'), sp=document.getElementById('split');
const w0=side.getBoundingClientRect().width;
sp.dispatchEvent(new MouseEvent('mousedown',{clientX:900,bubbles:true,cancelable:true}));
window.dispatchEvent(new MouseEvent('mousemove',{clientX:900+(w0-480),bubbles:true}));
window.dispatchEvent(new MouseEvent('mouseup',{bubbles:true}));
const w1=Math.round(side.getBoundingClientRect().width);
return JSON.stringify({w1,saved:+localStorage.getItem('isotopo-pane')===w1});})()
""")
pre = json.loads(pre)
reload_page()
post = ev("""
(()=>{const side=document.querySelector('.side');
const wRestored=Math.round(side.getBoundingClientRect().width);
fitView();
const svg=zoomer.querySelector('svg');
const r=viewport.getBoundingClientRect();
const w=parseFloat(svg.getAttribute('width'))||svg.viewBox.baseVal.width;
const h=parseFloat(svg.getAttribute('height'))||svg.viewBox.baseVal.height;
const expected=Math.min(4,Math.max(0.2,Math.min((r.width-72)/w,(r.height-72)/h)));
const fitOK=Math.abs(scale-expected)<0.001;
const s1=scale;
setPaneWidth(wRestored+250); fitView();
const shrank=scale<=s1;
localStorage.removeItem('isotopo-pane');
side.style.removeProperty('flex'); side.style.removeProperty('max-width');
resetView();
return JSON.stringify({wRestored,fitOK,s1,s2:scale,shrank});})()
""")
post = json.loads(post)
# MouseEvent.clientX is an integer (spec: long), so the drag target can
# land within ±2px of 480 depending on the fractional starting width.
record("s18-pane-resize-reload-fit", {**pre, **post},
    lambda d: abs(d["w1"] - 480) <= 2 and d["saved"] and d["wRestored"] == d["w1"]
     and d["fitOK"] and d["shrank"])

# ── S19 invalid #src= hash degrades gracefully ───────────────────────
reload_page(set_hash="src=!!notbase64!!")
scenario("s19-invalid-hash-graceful", """
(async()=>{await sleep(1200);
const r=JSON.stringify({
  editorOriginal: srcEl.value.indexOf('GPU Pool')>=0,
  clean: !document.getElementById('dirty').classList.contains('on'),
  noStale: staleEl.hidden,
  live: document.getElementById('live').textContent==='Live'});
history.replaceState(null,'',location.pathname);
return r;})()
""", lambda d: all(d.values()))

# ── S20 the big chain: load example → edit → share → second edit
#    (draft v2) → open the share URL → hash must outrank the draft →
#    render catches up → discard ─────────────────────────────────────
s20a = ev("""
(async()=>{installStub(); installClip(); window.__clip=null;
await loadExample('llm-serving');
const afterLoad=srcEl.value.indexOf('EXAMPLE_MARK_POOL')>=0;
await sleep(1700);
const renderedExample=zoomer.innerHTML.indexOf('EXAMPLE_MARK_POOL')>=0;
srcEl.value=srcEl.value.replace('EXAMPLE_MARK_POOL','SHARE_V1_MARK');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(900);
await shareLink();
const url=window.__clip||'';
srcEl.value=srcEl.value.replace('SHARE_V1_MARK','DRAFT_V2_MARK');
srcEl.dispatchEvent(new Event('input',{bubbles:true}));
await sleep(400);
const draftIsV2=(localStorage.getItem(DRAFTKEY)||'').indexOf('DRAFT_V2_MARK')>=0;
return JSON.stringify({afterLoad,renderedExample,shareUrlHasHash:url.indexOf('#src=')>=0,
  draftIsV2,hash:(url.split('#')[1]||'')});})()
""", timeout=60)
s20a = json.loads(s20a)
reload_page(set_hash=s20a.get("hash") or "src=", wait=4.5)
s20b = ev("""
(async()=>{await sleep(1800);
const editorIsV1=srcEl.value.indexOf('SHARE_V1_MARK')>=0;
const editorNotV2=srcEl.value.indexOf('DRAFT_V2_MARK')<0;
const canvasIsV1=zoomer.innerHTML.indexOf('SHARE_V1_MARK')>=0;
const dirtyOn=document.getElementById('dirty').classList.contains('on');
discardDraft(); await sleep(1400);
const discardRestored=srcEl.value.indexOf('GPU Pool')>=0&&srcEl.value.indexOf('SHARE_V1_MARK')<0;
const draftGone=localStorage.getItem(DRAFTKEY)===null;
const hashStill=location.hash.indexOf('#src=')===0;
return JSON.stringify({editorIsV1,editorNotV2,canvasIsV1,dirtyOn,discardRestored,draftGone,hashStill});})()
""", timeout=60)
s20b = json.loads(s20b)
record("s20-hash-outranks-draft-chain",
    {k: v for k, v in {**s20a, **s20b}.items() if k != "hash"},
    lambda d: d["afterLoad"] and d["renderedExample"] and d["shareUrlHasHash"]
     and d["draftIsV2"] and d["editorIsV1"] and d["editorNotV2"] and d["canvasIsV1"]
     and d["dirtyOn"] and d["discardRestored"] and d["draftGone"])

# ── S21 discard vs lingering hash: after revert, a reload of the SAME
#    URL must not resurrect the discarded content ─────────────────────
reload_page(wait=4.5)  # hash still in the URL from S20
scenario("s21-discard-vs-hash-resurrection", """
(async()=>{await sleep(1200);
const resurrected=srcEl.value.indexOf('SHARE_V1_MARK')>=0;
history.replaceState(null,'',location.pathname);
discardDraft(); await sleep(1400);
return JSON.stringify({resurrected});})()
""", lambda d: d == {"resurrected": False})

# ── S22 sample file on disk untouched by the whole session ───────────
with open(SAMPLE) as f:
    txt = f.read()
record("s22-original-untouched",
    {"unchanged": txt == SAMPLE_TEXT,
     "noMarkers": all(m not in txt for m in
        ["EXAMPLE_MARK_POOL", "SHARE_V1_MARK", "DRAFT_V2_MARK", "RACE_MARK",
         "DRAFT_RELOAD_MARK", "AGENTS_EDIT_MARK", "EXPORT_MARK", "hexbroken"])},
    lambda d: all(d.values()))

# ── teardown + report ────────────────────────────────────────────────
try: ws.close()
except Exception: pass
chrome.terminate(); serve.terminate()
try: os.remove(BIN)
except OSError: pass

print()
fails = 0
for name, ok, val in results:
    print(("PASS " if ok else "FAIL ") + name)
    if not ok:
        print("      verdict:", json.dumps(val))
        fails += 1
sys.exit(1 if fails else 0)
