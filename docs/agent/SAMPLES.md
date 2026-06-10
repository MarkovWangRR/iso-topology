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
| [`iso-text`](../../samples/node/iso-text/input.yaml) | iso_text — flat text panel (near-zero extrusion); good for titles and captions. |
| [`person`](../../samples/node/person/input.yaml) | person — human actor: sphere head tangent to a hemispherical dome body. |
| [`rectangle`](../../samples/node/rectangle/input.yaml) | rectangle — the bread-and-butter iso box: three faces, top label, extrusion h. |

## samples/topology

| Fixture | Demonstrates |
|---|---|
| [`build-metrics`](../../samples/topology/build-metrics/input.yaml) | build-metrics — "engineering time" ghost-volume bar chart. |
| [`edge-platform`](../../samples/topology/edge-platform/input.yaml) | edge-platform — dark-mode edge network platform. |
| [`integration-hub`](../../samples/topology/integration-hub/input.yaml) | integration-hub — hub-and-spoke integrations ring. |
| [`microservice`](../../samples/topology/microservice/input.d2) | microservice — minimal .d2 auto-layout path: person → services → database. |
| [`vpc-peering`](../../samples/topology/vpc-peering/input.yaml) | vpc-peering — dark-mode private peering between two VPCs. |
