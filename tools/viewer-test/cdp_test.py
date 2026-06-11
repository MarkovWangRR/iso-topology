#!/usr/bin/env python3
"""Real-browser regression suite for the interactive viewer
(topology.html + `isotopo serve`). Drives headless Chrome over CDP —
no JS toolchain needed, just Chrome and `pip install websocket-client`.

Asserts the five viewer features end to end:
  hover → source highlight (+ glow + clear), wheel zoom, drag pan,
  edit → auto re-render (against the in-browser copy), broken edit →
  issues panel with suggestion while the last good SVG stays, and
  the static-file degradation path (no server → button disabled).

Run from the repo root:  python3 tools/viewer-test/cdp_test.py
Not part of `go test` (needs Chrome); run it whenever output.go's
template or cmd/isotopo's serve handler changes.
"""
import json, subprocess, time, urllib.request, websocket, tempfile, sys, os

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
os.chdir(REPO)
subprocess.run(["go","build","-o","/tmp/isotopo-viewer-test","./cmd/isotopo"], check=True)
serve = subprocess.Popen(["/tmp/isotopo-viewer-test","serve","samples/topology/ai-platform/input.yaml"],
    env={**os.environ,"ISOTOPO_PORT":"8733"}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(1.2)
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
tmp=tempfile.mkdtemp()
proc=subprocess.Popen([CHROME,"--headless=new","--remote-debugging-port=9223","--remote-allow-origins=*",
    f"--user-data-dir={tmp}","--no-first-run","--disable-gpu","http://localhost:8733/"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(3)
targets=json.load(urllib.request.urlopen("http://127.0.0.1:9223/json"))
page=[t for t in targets if t["type"]=="page"][0]
ws=websocket.create_connection(page["webSocketDebuggerUrl"], timeout=20)
mid=0
def ev(expr, timeout=20):
    global mid; mid+=1
    ws.send(json.dumps({"id":mid,"method":"Runtime.evaluate",
        "params":{"expression":expr,"returnByValue":True,"awaitPromise":True}}))
    while True:
        m=json.loads(ws.recv())
        if m.get("id")==mid:
            r=m["result"].get("result",{})
            if "value" in r: return r["value"]
            return r.get("description")

results=[]
def check(name, val, expect=True):
    ok = (val == expect) if not callable(expect) else expect(val)
    results.append((name, ok, val))

time.sleep(1.5)  # let probe() finish

# T3 live status first
check("live-status", ev("document.getElementById('live').textContent"), lambda v: "live" in str(v).lower())
check("render-btn-enabled", ev("!document.getElementById('render').disabled"))

# T1 hover → highlight
check("hover-highlight", ev("""
(()=>{const g=document.querySelector('g[data-part-id="gpu_pool"]');
g.dispatchEvent(new Event('mouseenter'));
const hits=[...document.querySelectorAll('#hl .hit')].map(d=>d.textContent).join('\\n');
const glow=g.classList.contains('hi');
g.dispatchEvent(new Event('mouseleave'));
const cleared=document.querySelectorAll('#hl .hit').length===0;
return JSON.stringify({hasHits:hits.length>0, mentionsId:hits.includes('gpu_pool'), glow, cleared});})()
"""), lambda v: json.loads(v)=={"hasHits":True,"mentionsId":True,"glow":True,"cleared":True})

# T2 zoom via wheel + pan via drag
check("wheel-zoom", ev("""
(()=>{const vp=document.getElementById('viewport');
const before=document.getElementById('zoomer').style.transform;
vp.dispatchEvent(new WheelEvent('wheel',{deltaY:-120,clientX:300,clientY:300,bubbles:true,cancelable:true}));
const after=document.getElementById('zoomer').style.transform;
return before!==after && after.includes('scale');})()
"""))
check("drag-pan", ev("""
(()=>{const vp=document.getElementById('viewport');
const t0=document.getElementById('zoomer').style.transform;
vp.dispatchEvent(new MouseEvent('mousedown',{clientX:400,clientY:300,bubbles:true}));
window.dispatchEvent(new MouseEvent('mousemove',{clientX:460,clientY:340,bubbles:true}));
window.dispatchEvent(new MouseEvent('mouseup',{bubbles:true}));
return document.getElementById('zoomer').style.transform!==t0;})()
"""))

# T4 edit → auto re-render (copy semantics)
check("auto-rerender", ev("""
(async()=>{const src=document.getElementById('src');
src.value=src.value.replace('label: "GPU Pool"','label: "GPU FARM 42"');
src.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(r=>setTimeout(r,1600));
return document.getElementById('zoomer').innerHTML.includes('GPU FARM 42');})()
"""))

# T5 broken edit → issues panel, svg preserved
check("issues-panel", ev("""
(async()=>{const src=document.getElementById('src');
src.value=src.value.replace('shape: rectangle','shape: rectangel');
src.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(r=>setTimeout(r,1600));
const issues=document.getElementById('issues');
const shown=issues.classList.contains('show') && issues.textContent.includes('rectangle');
const svgKept=document.getElementById('zoomer').innerHTML.includes('GPU FARM 42');
return JSON.stringify({shown, svgKept});})()
"""), lambda v: json.loads(v)=={"shown":True,"svgKept":True})

# T5b broken render → stale badge over the canvas
check("stale-badge", ev("!document.getElementById('stale').hidden"))

# T7 dirty state: edits flip the unsaved flag and arm the download button
check("dirty-flag", ev("""
(()=>{return JSON.stringify({
  dirtyDot: document.getElementById('dirty').classList.contains('on'),
  discardShown: !document.getElementById('discard').hidden,
  dlEnabled: !document.getElementById('dl').disabled});})()
"""), lambda v: json.loads(v)=={"dirtyDot":True,"discardShown":True,"dlEnabled":True})

# T7b export captures the CURRENT (edited) canvas, named .edited.*
check("export-edited-svg", ev("""
(()=>{return JSON.stringify({
  capturesEdit: currentSVG().includes('GPU FARM 42'),
  editedName: exportName('svg').endsWith('.edited.svg'),
  pngName: exportName('png').endsWith('.edited.png')});})()
"""), lambda v: json.loads(v)=={"capturesEdit":True,"editedName":True,"pngName":True})

# T8 Tab inserts indentation instead of moving focus
check("tab-indent", ev("""
(()=>{const src=document.getElementById('src');
src.focus(); src.selectionStart=src.selectionEnd=0;
const before=src.value;
src.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',bubbles:true,cancelable:true}));
return src.value==='  '+before && src.selectionStart===2;})()
"""))

# T9 copy button gives transient feedback (clipboard may be denied headless — either label is fine)
check("copy-feedback", ev("""
(async()=>{const b=document.getElementById('copybtn');
b.click();
await new Promise(r=>setTimeout(r,200));
const during=b.textContent;
await new Promise(r=>setTimeout(r,1400));
return JSON.stringify({changed: during!=='Copy', restored: b.textContent==='Copy'});})()
"""), lambda v: json.loads(v)=={"changed":True,"restored":True})

# T10 draft survives a reload (localStorage), original untouched on disk
ev("location.reload()", timeout=30)
time.sleep(3)
check("draft-restored", ev("""
(()=>{const src=document.getElementById('src');
return JSON.stringify({
  draftKept: src.value.includes('GPU FARM 42'),
  dirtyDot: document.getElementById('dirty').classList.contains('on')});})()
"""), lambda v: json.loads(v)=={"draftKept":True,"dirtyDot":True})
with open("samples/topology/ai-platform/input.yaml") as f:
    check("original-untouched", "GPU FARM 42" not in f.read())

# T11 discard returns to the original and clears the flag
check("discard-draft", ev("""
(async()=>{document.getElementById('discard').click();
await new Promise(r=>setTimeout(r,1200));
const src=document.getElementById('src');
return JSON.stringify({
  restored: src.value.includes('GPU Pool') && !src.value.includes('GPU FARM 42'),
  clean: !document.getElementById('dirty').classList.contains('on'),
  dlDisabled: document.getElementById('dl').disabled,
  draftGone: localStorage.getItem('isotopo-draft:'+PATH)===null});})()
"""), lambda v: json.loads(v)=={"restored":True,"clean":True,"dlDisabled":True,"draftGone":True})

ws.close(); proc.terminate()

# T6 degradation: static server without /api
import http.server, threading, functools, os
os.makedirs('/tmp/static-out', exist_ok=True)
subprocess.run(["/tmp/isotopo-viewer-test","render","samples/topology/ai-platform/input.yaml","/tmp/static-out"],
    cwd="/Users/markovwong/Desktop/CodeProject/iso-topology", capture_output=True)
httpd=http.server.HTTPServer(('127.0.0.1',8799),
    functools.partial(http.server.SimpleHTTPRequestHandler, directory='/tmp/static-out'))
threading.Thread(target=httpd.serve_forever, daemon=True).start()
proc2=subprocess.Popen([CHROME,"--headless=new","--remote-debugging-port=9224","--remote-allow-origins=*",
    f"--user-data-dir={tmp}2","--no-first-run","--disable-gpu","http://127.0.0.1:8799/topology.html"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(3)
targets=json.load(urllib.request.urlopen("http://127.0.0.1:9224/json"))
page=[t for t in targets if t["type"]=="page"][0]
ws=websocket.create_connection(page["webSocketDebuggerUrl"], timeout=20)
time.sleep(1.5)
check("degraded-status", ev("document.getElementById('live').textContent"), lambda v: "offline" in str(v).lower())
check("degraded-btn-disabled", ev("document.getElementById('render').disabled"))
ws.close(); proc2.terminate(); httpd.shutdown(); serve.terminate()

print()
fails=0
for name, ok, val in results:
    print(("PASS " if ok else "FAIL ")+name, "" if ok else f"→ {val}")
    fails += 0 if ok else 1
sys.exit(1 if fails else 0)
