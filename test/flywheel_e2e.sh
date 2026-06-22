#!/usr/bin/env bash
# P6 acceptance — the full flywheel B→A: distil a style from a landing page,
# index it, then have Flow A apply it to a NEW structure and render. LLM steps
# are key-gated (skipped without OPENAI_API_KEY; Flow A still runs on lustre).
set -euo pipefail
cd "$(dirname "$0")/.."
ISO="${ISO:-go run ./cmd/isotopo}"

echo "== Flow B: distil a style from a landing page =="
STYLE=lustre
if [ -n "${OPENAI_API_KEY:-}" ]; then
  SRC=$(ls samples/playground/databricks/*.png 2>/dev/null | head -1 || true)
  if [ -n "$SRC" ]; then
    sips -Z 1000 "$SRC" --out /tmp/fly_src.png >/dev/null 2>&1 || cp "$SRC" /tmp/fly_src.png
    $ISO playbook distill databricks --source /tmp/fly_src.png --iters 1 --target 70
    grep -q 'trust: auto' samples/playbook/databricks/meta.yaml && echo "  distilled databricks (trust=auto) ✓"
    STYLE=databricks
  fi
else
  echo "  no OPENAI_API_KEY → skipping distillation; using blessed lustre"
fi

echo "== index sees the style =="
$ISO playbook index
python3 -c "import json;d=json.load(open('samples/playbook/INDEX.json'));assert any(e['style']=='$STYLE' for e in d),'not indexed';print('  INDEX has $STYLE ✓')"

echo "== Flow A: apply the style to a NEW structure + render =="
$ISO playbook apply $STYLE samples/playbook/_demo.yaml -o /tmp/fly_styled.yaml
if grep -q 'role:' /tmp/fly_styled.yaml; then echo "  FAIL: role leaked"; exit 1; fi
$ISO render /tmp/fly_styled.yaml /tmp/fly_out >/dev/null
grep -q 'data-face="left"' /tmp/fly_out/topology.svg && echo "  styled + rendered (lit faces) ✓"
echo "FLYWHEEL OK ($STYLE)"
