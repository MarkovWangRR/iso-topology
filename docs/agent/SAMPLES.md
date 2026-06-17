# Samples index

Generated from `samples/*/*/input.*` header comments — run
`go run ./tools/gen-docs` to refresh. Every fixture is a
golden-tested, copy-paste-ready example; `expected.svg` next to
each input is the rendered output.

Reading order for agents: start with the fixture whose
description matches your task, imitate its structure, then check
[`RECIPES.md`](RECIPES.md) for the primitive-level grammar.

## samples/node

| Fixture | Demonstrates |
|---|---|
| [`array2d`](../../samples/node/array2d/input.yaml) | (no header comment) |
| [`array3d`](../../samples/node/array3d/input.yaml) | (no header comment) |
| [`capsule`](../../samples/node/capsule/input.yaml) | (no header comment) |
| [`circle`](../../samples/node/circle/input.yaml) | circle — the iso sphere: one ball with equator shading and a top label. |
| [`cloud`](../../samples/node/cloud/input.yaml) | cloud — free-form rounded outline for external systems; no per-face palette. |
| [`composite`](../../samples/node/composite/input.yaml) | composite — a stat card built from three primitives sharing one bbox. |
| [`cone`](../../samples/node/cone/input.yaml) | (no header comment) |
| [`custom-path`](../../samples/node/custom-path/input.yaml) | (no header comment) |
| [`cylinder`](../../samples/node/cylinder/input.yaml) | cylinder — the database/queue primitive: elliptical top, curved side wall. |
| [`dome`](../../samples/node/dome/input.yaml) | (no header comment) |
| [`effect-pipeline`](../../samples/node/effect-pipeline/input.yaml) | (no header comment) |
| [`faces-demo`](../../samples/node/faces-demo/input.yaml) | (no header comment) |
| [`frustum`](../../samples/node/frustum/input.yaml) | (no header comment) |
| [`group`](../../samples/node/group/input.yaml) | group — a low-extrusion translucent substrate that wraps nested parts. |
| [`hexprism`](../../samples/node/hexprism/input.yaml) | (no header comment) |
| [`iso-text`](../../samples/node/iso-text/input.yaml) | iso_text — flat text panel (near-zero extrusion); good for titles and captions. |
| [`person`](../../samples/node/person/input.yaml) | person — human actor: sphere head tangent to a hemispherical dome body. |
| [`pyramid`](../../samples/node/pyramid/input.yaml) | (no header comment) |
| [`rack`](../../samples/node/rack/input.yaml) | (no header comment) |
| [`rectangle`](../../samples/node/rectangle/input.yaml) | rectangle — the bread-and-butter iso box: three faces, top label, extrusion h. |
| [`screen`](../../samples/node/screen/input.yaml) | (no header comment) |
| [`wedge`](../../samples/node/wedge/input.yaml) | (no header comment) |

## samples/topology

| Fixture | Demonstrates |
|---|---|
| [`ai-platform`](../../samples/topology/ai-platform/input.yaml) | ai-platform v2 — flagship hero, reimagined with the v3.3 surface pipeline. |
| [`auto-flow`](../../samples/topology/auto-flow/input.yaml) | (no header comment) |
| [`data-fabric`](../../samples/topology/data-fabric/input.yaml) | data-fabric — connector-driven AUTO layout (v4.2). |
| [`devtool-pipeline`](../../samples/topology/devtool-pipeline/input.yaml) | devtool-pipeline — CI/CD neon. |
| [`edge-security`](../../samples/topology/edge-security/input.yaml) | edge-security — zero-trust edge, converted to connector-driven auto-layout (v4.2). |
| [`gateway-mesh`](../../samples/topology/gateway-mesh/input.yaml) | (no header comment) |
| [`identity-flow`](../../samples/topology/identity-flow/input.yaml) | identity-flow — monochrome film-grain editorial style: how human identities delegate to AI agents which consume machine identities. |
| [`inference-board`](../../samples/topology/inference-board/input.yaml) | inference-board — LLM inference PCB, dark green / copper engineering look. |
| [`llm-serving`](../../samples/topology/llm-serving/input.yaml) | llm-serving — a dark-mode LLM inference platform. |
| [`microservice`](../../samples/topology/microservice/input.d2) | microservice — minimal .d2 auto-layout path: person → services → database. |
| [`payment-rails`](../../samples/topology/payment-rails/input.yaml) | payment-rails — fintech print register: a payment authorization flow Converted to connector-driven auto-layout (layout: {mode: auto}, v4.2). |
| [`platform-board`](../../samples/topology/platform-board/input.yaml) | platform-board — PCB / circuit-board hero shot, landing-page style. |
| [`rag-pipeline`](../../samples/topology/rag-pipeline/input.yaml) | rag-pipeline — a RAG system, now under connector-driven auto-layout. |
| [`training-compute`](../../samples/topology/training-compute/input.yaml) | training-compute — where the GPU hours of a training run go. |
| [`vpc-boundary`](../../samples/topology/vpc-boundary/input.yaml) | (no header comment) |
