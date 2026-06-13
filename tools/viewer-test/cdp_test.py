#!/usr/bin/env python3
"""Real-browser regression suite for the Studio page (topology.html +
`isotopo serve`). Drives headless Chrome over CDP — no JS toolchain, just
Chrome and `pip install websocket-client`.

Covers the live surface end to end: source↔canvas mapping (hover/pin),
zoom/pan, edit→auto-rerender (in-browser copy), issues panel, dirty/draft,
exports, AND the direct-manipulation layer added later — node-drag commit,
edge waypoint drag, the right-click "Edit details" modal (node/edge/canvas),
delete/duplicate ops, undo/redo, plus static-file degradation.

Run from the repo root:  python3 tools/viewer-test/cdp_test.py
Not part of `go test` (needs Chrome); run it whenever studio/* or
cmd/isotopo's serve handlers change.
"""
import json, subprocess, time, urllib.request, websocket, tempfile, sys, os

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
os.chdir(REPO)
subprocess.run(["pkill", "-f", "isotopo-viewer-test"], capture_output=True)
# A leftover headless Chrome on our debug port (from an aborted run) would
# answer /json with a STALE page and we'd silently test the old build.
for port in ("9223", "9224"):
    subprocess.run(["pkill", "-f", f"remote-debugging-port={port}"], capture_output=True)
time.sleep(0.6)
BIN = f"/tmp/isotopo-viewer-test-{os.getpid()}"
subprocess.run(["go", "build", "-o", BIN, "./cmd/isotopo"], check=True)
serve = subprocess.Popen([BIN, "serve", "samples/topology/ai-platform/input.yaml"],
    env={**os.environ, "ISOTOPO_PORT": "8733"}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(1.2)
CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
tmp = tempfile.mkdtemp()
proc = subprocess.Popen([CHROME, "--headless=new", "--remote-debugging-port=9223", "--remote-allow-origins=*",
    "--window-size=1400,900",
    f"--user-data-dir={tmp}", "--no-first-run", "--disable-gpu", "http://localhost:8733/"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(3)
targets = json.load(urllib.request.urlopen("http://127.0.0.1:9223/json"))
page = [t for t in targets if t["type"] == "page"][0]
ws = websocket.create_connection(page["webSocketDebuggerUrl"], timeout=20)
mid = 0
def ev(expr, timeout=20):
    global mid; mid += 1
    ws.send(json.dumps({"id": mid, "method": "Runtime.evaluate",
        "params": {"expression": expr, "returnByValue": True, "awaitPromise": True}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid:
            r = m["result"].get("result", {})
            return r["value"] if "value" in r else r.get("description")

def cdp(method, params=None):
    global mid; mid += 1
    ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid:
            return m

cdp("Emulation.setDeviceMetricsOverride",
    {"width": 1400, "height": 900, "deviceScaleFactor": 1, "mobile": False})

results = []
def check(name, val, expect=True):
    try:
        ok = (val == expect) if not callable(expect) else expect(val)
    except Exception:
        ok = False
    results.append((name, ok, val))

def J(v):  # parse a JSON string result, tolerant
    try:
        return json.loads(v)
    except Exception:
        return None

time.sleep(1.6)  # let probe() + first render settle

# ── status & header ──────────────────────────────────────────────────────
# One consolidated status: "Rendered HH:MM:SS" when live (no more "Live" pill).
check("status-rendered", ev("document.getElementById('live').textContent"),
      lambda v: "render" in str(v).lower())
check("render-btn-enabled", ev("!document.getElementById('render').disabled"))
check("studio-badge", ev("document.querySelector('header .studio').textContent"),
      lambda v: "studio" in str(v).lower())
check("export-trio", ev("[...document.querySelectorAll('.exportctl button')].map(b=>b.textContent).join('|')"),
      lambda v: "SVG" in v and "PNG" in v and "YAML" in v)
check("no-share-or-copy-toolbar",
      ev("!document.getElementById('share') && !document.getElementById('copybtn')"))

# ── source ↔ canvas mapping ────────────────────────────────────────────────
check("syntax-tokens", ev("""
(()=>JSON.stringify({
  keys: document.querySelectorAll('#hl .tk-k').length>10,
  strings: document.querySelectorAll('#hl .tk-s').length>0,
  taTransparent: getComputedStyle(document.getElementById('src')).color==='rgba(0, 0, 0, 0)'}))()
"""), lambda v: J(v) == {"keys": True, "strings": True, "taTransparent": True})

check("hover-highlight", ev("""
(()=>{const g=document.querySelector('g[data-part-id="gpu_pool"]');
g.dispatchEvent(new Event('mouseenter'));
const hits=[...document.querySelectorAll('#hl .hit')].map(d=>d.textContent).join('\\n');
const glow=g.classList.contains('hi');
g.dispatchEvent(new Event('mouseleave'));
return JSON.stringify({hasHits:hits.length>0, mentionsId:hits.includes('gpu_pool'), glow,
  cleared:document.querySelectorAll('#hl .hit').length===0});})()
"""), lambda v: J(v) == {"hasHits": True, "mentionsId": True, "glow": True, "cleared": True})

check("pin-select", ev("""
(()=>{const g=document.querySelector('g[data-part-id="gpu_pool"]');
g.dispatchEvent(new MouseEvent('click',{bubbles:true})); g.dispatchEvent(new Event('mouseleave'));
const pinned=document.querySelectorAll('#hl .hit').length>0 && g.classList.contains('hi');
document.getElementById('viewport').dispatchEvent(new MouseEvent('click',{bubbles:true}));
return JSON.stringify({pinned, cleared:document.querySelectorAll('#hl .hit').length===0});})()
"""), lambda v: J(v) == {"pinned": True, "cleared": True})

# edge hover highlights the connector's source lines
check("edge-hover-source", ev("""
(()=>{const p=document.querySelector('path[data-connector]'); if(!p) return "noedge";
p.dispatchEvent(new Event('mouseenter'));
const hit=document.querySelectorAll('#hl .hit').length>0;
p.dispatchEvent(new Event('mouseleave'));
return hit;})()
"""))

# ── zoom / pan ─────────────────────────────────────────────────────────────
check("wheel-zoom", ev("""
(()=>{const vp=document.getElementById('viewport');
const b=document.getElementById('zoomer').style.transform;
vp.dispatchEvent(new WheelEvent('wheel',{deltaY:-120,clientX:300,clientY:300,bubbles:true,cancelable:true}));
return document.getElementById('zoomer').style.transform!==b;})()
"""))
check("zoom-ui", ev("""
(()=>{const t0=document.getElementById('zoomer').style.transform;
document.getElementById('zfit').click();
return JSON.stringify({pct:/%$/.test(document.getElementById('zpct').textContent),
  changed:document.getElementById('zoomer').style.transform!==t0});})()
"""), lambda v: J(v) == {"pct": True, "changed": True})

# ── edit → auto re-render (copy semantics) ─────────────────────────────────
check("auto-rerender", ev("""
(async()=>{const src=document.getElementById('src');
src.value=src.value.replace('label: "GPU Pool"','label: "GPU FARM 42"');
src.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(r=>setTimeout(r,1600));
return document.getElementById('zoomer').innerHTML.includes('GPU FARM 42');})()
"""))
check("status-timestamp-after-render", ev("document.getElementById('live').textContent"),
      lambda v: any(c.isdigit() for c in str(v)))

check("adaptive-stage", ev("""
(async()=>{const src=document.getElementById('src');
src.value=src.value.replace('background: "#F6F5FA"','background: "#0B0F14"');
src.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(r=>setTimeout(r,1600));
return document.getElementById('stage').style.getPropertyValue('--stage-bg').indexOf('rgb(')===0;})()
"""))

check("issues-panel", ev("""
(async()=>{const src=document.getElementById('src');
src.value=src.value.replace('shape: rectangle','shape: rectangel');
src.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(r=>setTimeout(r,1600));
const issues=document.getElementById('issues');
return JSON.stringify({shown:issues.classList.contains('show')&&issues.textContent.includes('rectangle'),
  svgKept:document.getElementById('zoomer').innerHTML.includes('GPU FARM 42')});})()
"""), lambda v: J(v) == {"shown": True, "svgKept": True})
check("stale-badge", ev("!document.getElementById('stale').hidden"))
check("issue-jump", ev("""
(()=>{const f=document.querySelector('#issues [data-path]'); if(!f) return JSON.stringify({clicked:false});
f.click(); return JSON.stringify({clicked:true, flashed:document.querySelectorAll('#hl .flash').length>0});})()
"""), lambda v: J(v) == {"clicked": True, "flashed": True})

# fix the broken edit back so later structural tests render cleanly
ev("""(async()=>{const src=document.getElementById('src');
src.value=src.value.replace('shape: rectangel','shape: rectangle');
src.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(r=>setTimeout(r,1500));})()""")

# ── dirty / export ─────────────────────────────────────────────────────────
check("dirty-flag", ev("!document.getElementById('discard').hidden"))
check("export-edited", ev("""
(()=>JSON.stringify({capturesEdit:currentSVG().includes('GPU FARM 42'),
  editedName:exportName('svg').endsWith('.edited.svg')}))()
"""), lambda v: J(v) == {"capturesEdit": True, "editedName": True})

# ── direct manipulation: node drag commit ──────────────────────────────────
check("node-drag-commits", ev("""
(async()=>{const g=document.querySelector('g[data-part-id]');
const id=g.getAttribute('data-part-id'); const before=document.getElementById('src').value;
await commitMove('node', id, 80, 30, 0, 0);
await new Promise(r=>setTimeout(r,500));
const after=document.getElementById('src').value;
return JSON.stringify({wrote:(after.match(/offset:/g)||[]).length>=1, changed:after!==before,
  rendered:!!document.querySelector('g[data-part-id]')});})()
"""), lambda v: J(v) == {"wrote": True, "changed": True, "rendered": True})

# edge waypoint drag commit (per-segment): writes waypoints, endpoints stay
check("edge-waypoint-commits", ev("""
(async()=>{const p=document.querySelector('path[data-connector]'); if(!p) return JSON.stringify({skip:true});
const ci=p.getAttribute('data-connector'); const before=document.getElementById('src').value;
await commitMove('edge', ci, 30, -20, 0, 0, [[40,-20]]);
await new Promise(r=>setTimeout(r,500));
const after=document.getElementById('src').value;
return JSON.stringify({wrote:after.includes('waypoints'), changed:after!==before});})()
"""), lambda v: J(v) in ({"skip": True}, {"wrote": True, "changed": True}))

# undo restores the pre-drag YAML
check("undo-restores", ev("""
(async()=>{const before=document.getElementById('src').value;
await commitMove('node', document.querySelector('g[data-part-id]').getAttribute('data-part-id'), 25, 25, 0, 0);
await new Promise(r=>setTimeout(r,400));
const moved=document.getElementById('src').value;
undo();
await new Promise(r=>setTimeout(r,400));
return JSON.stringify({changed:moved!==before, undone:document.getElementById('src').value===before});})()
"""), lambda v: J(v) == {"changed": True, "undone": True})

# ── right-click "Edit details" modal ───────────────────────────────────────
check("context-menu", ev("""
(()=>{const g=document.querySelector('g[data-part-id]');
g.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,clientX:300,clientY:300}));
const shown=!document.getElementById('ctxmenu').hidden;
const items=[!document.getElementById('ctxedit').hidden,!document.getElementById('ctxdup').hidden,!document.getElementById('ctxdel').hidden];
hideCtx();
return JSON.stringify({shown, items});})()
"""), lambda v: J(v) == {"shown": True, "items": [True, True, True]})

check("detail-edit-node", ev("""
(async()=>{const id=document.querySelector('g[data-part-id]').getAttribute('data-part-id');
openDetail({kind:'node',key:id});
await new Promise(r=>setTimeout(r,500));
const fields=document.querySelectorAll('#detailFields [data-key]').length;
const e=[...document.querySelectorAll('#detailFields [data-key]')].find(x=>x.getAttribute('data-key')==='label');
if(e) e.value='RENAMED_NODE';
applyDetail();
await new Promise(r=>setTimeout(r,700));
return JSON.stringify({fields:fields>0, wrote:document.getElementById('src').value.includes('RENAMED_NODE')});})()
"""), lambda v: J(v) == {"fields": True, "wrote": True})

check("detail-edit-canvas", ev("""
(async()=>{openDetail({kind:'canvas',key:''});
await new Promise(r=>setTimeout(r,500));
const e=[...document.querySelectorAll('#detailFields [data-key]')].find(x=>x.getAttribute('data-key')==='background');
if(e) e.value='#123456';
applyDetail();
await new Promise(r=>setTimeout(r,700));
return document.getElementById('src').value.includes('#123456');})()
"""))

# ── delete / duplicate ops ─────────────────────────────────────────────────
check("duplicate-node", ev("""
(async()=>{const id=document.querySelector('g[data-part-id]').getAttribute('data-part-id');
await opCommit('duplicate',{kind:'node',key:id});
await new Promise(r=>setTimeout(r,700));
return document.getElementById('src').value.includes(id+'_copy');})()
"""))
check("delete-node", ev("""
(async()=>{discardDraft(); await new Promise(r=>setTimeout(r,1200));
const id=document.querySelector('g[data-part-id]').getAttribute('data-part-id');
const before=document.getElementById('src').value;
await opCommit('delete',{kind:'node',key:id});
await new Promise(r=>setTimeout(r,800));
const after=document.getElementById('src').value;
return JSON.stringify({shorter:after.length<before.length, gone:!document.querySelector('g[data-part-id="'+id+'"]')});})()
"""), lambda v: J(v) == {"shorter": True, "gone": True})

# ── editor niceties ────────────────────────────────────────────────────────
check("tab-indent", ev("""
(()=>{const src=document.getElementById('src');
src.focus(); src.selectionStart=src.selectionEnd=0; const b=src.value;
src.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',bubbles:true,cancelable:true}));
return src.value==='  '+b && src.selectionStart===2;})()
"""))

# ── browse-nodes route ─────────────────────────────────────────────────────
try:
    nodes_code = urllib.request.urlopen("http://localhost:8733/nodes/_index.html").status
except Exception:
    nodes_code = 0
check("nodes-gallery-route", nodes_code, 200)

# original file never written
with open("samples/topology/ai-platform/input.yaml") as f:
    check("original-untouched", "RENAMED_NODE" not in f.read())

ws.close(); proc.terminate()

# ── static-file degradation: no /api → Offline + Render disabled ───────────
import http.server, threading, functools
os.makedirs('/tmp/static-out', exist_ok=True)
subprocess.run([BIN, "render", "samples/topology/ai-platform/input.yaml", "/tmp/static-out"],
    cwd=REPO, capture_output=True)
httpd = http.server.HTTPServer(('127.0.0.1', 8799),
    functools.partial(http.server.SimpleHTTPRequestHandler, directory='/tmp/static-out'))
threading.Thread(target=httpd.serve_forever, daemon=True).start()
proc2 = subprocess.Popen([CHROME, "--headless=new", "--remote-debugging-port=9224", "--remote-allow-origins=*",
    f"--user-data-dir={tmp}2", "--no-first-run", "--disable-gpu", "http://127.0.0.1:8799/topology.html"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(3)
targets = json.load(urllib.request.urlopen("http://127.0.0.1:9224/json"))
page = [t for t in targets if t["type"] == "page"][0]
ws = websocket.create_connection(page["webSocketDebuggerUrl"], timeout=20)
time.sleep(1.6)
check("degraded-status", ev("document.getElementById('live').textContent"),
      lambda v: "offline" in str(v).lower())
check("degraded-btn-disabled", ev("document.getElementById('render').disabled"))
ws.close(); proc2.terminate(); httpd.shutdown(); serve.terminate()
try: os.remove(BIN)
except OSError: pass

print()
fails = 0
for name, ok, val in results:
    print(("PASS " if ok else "FAIL ") + name, "" if ok else f"→ {val}")
    fails += 0 if ok else 1
print(f"\n{len(results)-fails}/{len(results)} passed")
sys.exit(1 if fails else 0)
