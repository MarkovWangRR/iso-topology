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
| [`d2-containers`](../../samples/topology/d2-containers/input.d2) | d2 nested containers → iso group, automatic. |
| [`d2-shape-zoo`](../../samples/topology/d2-shape-zoo/input.d2) | Shape zoo — every d2 shape the catalog maps, all in one .d2 source. |
| [`k8s-litellm`](../../samples/topology/k8s-litellm/input.yaml) | k8s-litellm — Kubernetes cluster running LiteLLM proxy pods, routing to external LLM providers (replica of a reference diagram). |
| [`kubernetes-cluster`](../../samples/topology/kubernetes-cluster/input.yaml) | Kubernetes Cluster + LiteLLM Proxy Pods — staggered layout reproduction. |
| [`layout-demo`](../../samples/topology/layout-demo/input.yaml) | layout-demo — the v2.2 declarative layout primitives, end to end, with ZERO hand-authored coordinates. |
| [`microservice`](../../samples/topology/microservice/input.d2) | microservice — minimal .d2 auto-layout path: person → services → database. |
| [`multi-region`](../../samples/topology/multi-region/input.yaml) | Multi-region — replica of a dark-mode multi-region database architecture diagram (green accent on near-black background, Region A / Region B substrates each … |
| [`starrocks`](../../samples/topology/starrocks/input.yaml) | StarRocks — replica of the unified-lakehouse marketing diagram. |
| [`v2-showcase`](../../samples/topology/v2-showcase/input.yaml) | v2 primitives showcase — exercises every new DSL feature in one scene: |
| [`vite-plus`](../../samples/topology/vite-plus/input.yaml) | VITE+ — replica of the official VITE+ dev-tool board. |
