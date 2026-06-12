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
| [`circle`](../../samples/node/circle/input.yaml) | circle — the iso sphere: one ball with equator shading and a top label. |
| [`cloud`](../../samples/node/cloud/input.yaml) | cloud — free-form rounded outline for external systems; no per-face palette. |
| [`composite`](../../samples/node/composite/input.yaml) | composite — a stat card built from three primitives sharing one bbox. |
| [`cylinder`](../../samples/node/cylinder/input.yaml) | cylinder — the database/queue primitive: elliptical top, curved side wall. |
| [`group`](../../samples/node/group/input.yaml) | group — a low-extrusion translucent substrate that wraps nested parts. |
| [`hexprism`](../../samples/node/hexprism/input.yaml) | (no header comment) |
| [`iso-text`](../../samples/node/iso-text/input.yaml) | iso_text — flat text panel (near-zero extrusion); good for titles and captions. |
| [`person`](../../samples/node/person/input.yaml) | person — human actor: sphere head tangent to a hemispherical dome body. |
| [`rectangle`](../../samples/node/rectangle/input.yaml) | rectangle — the bread-and-butter iso box: three faces, top label, extrusion h. |

## samples/topology

| Fixture | Demonstrates |
|---|---|
| [`ai-platform`](../../samples/topology/ai-platform/input.yaml) | ai-platform — an AI platform core and its eight capabilities, as a hub-and-spoke ring. |
| [`gateway-mesh`](../../samples/topology/gateway-mesh/input.yaml) | (no header comment) |
| [`identity-flow`](../../samples/topology/identity-flow/input.yaml) | identity-flow — monochrome film-grain editorial style: how human identities delegate to AI agents which consume machine identities. |
| [`llm-serving`](../../samples/topology/llm-serving/input.yaml) | llm-serving — a dark-mode LLM inference platform. |
| [`microservice`](../../samples/topology/microservice/input.d2) | microservice — minimal .d2 auto-layout path: person → services → database. |
| [`platform-board`](../../samples/topology/platform-board/input.yaml) | platform-board — PCB / circuit-board hero shot, landing-page style. |
| [`rag-pipeline`](../../samples/topology/rag-pipeline/input.yaml) | rag-pipeline — a RAG system on two planes. |
| [`training-compute`](../../samples/topology/training-compute/input.yaml) | training-compute — where the GPU hours of a training run go. |
