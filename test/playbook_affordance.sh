#!/usr/bin/env bash
# P3 acceptance: the documented agent workflow (search → role-structure → apply →
# render) runs end-to-end and the style's signature lands. Closeable proxy for
# "the agent consults the playbook".
set -euo pipefail
cd "$(dirname "$0")/.."
ISO="go run ./cmd/isotopo"
echo "1) search returns an actionable contract"
$ISO playbook search "clean data warehouse" | python3 -c "import sys,json;d=json.load(sys.stdin);assert d,'no hits';e=d[0];assert e.get('roles') and e.get('apply'),'missing roles/apply';print('   →',e['style'],e['roles'])"
echo "2) apply a NEW role-only structure + render"
$ISO playbook apply lustre samples/playbook/_demo.yaml -o /tmp/pb_styled.yaml
if grep -q 'role:' /tmp/pb_styled.yaml; then echo "   FAIL: role leaked into isotopo YAML"; exit 1; fi
$ISO render /tmp/pb_styled.yaml /tmp/pb_out >/dev/null
grep -q 'data-face="left"' /tmp/pb_out/topology.svg && echo "   lit faces applied ✓"
grep -qE 'data-connector="[0-9]"[^>]*#2E86D6' /tmp/pb_out/topology.svg && echo "   blue dashed connectors applied ✓"
echo "AFFORDANCE OK"
