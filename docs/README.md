# iso-topology docs

This tree is organized by purpose, not topic — pick the section that
matches what you're trying to do.

## getting-started/

**You're new and want to render something real, fast.** Walk it in
order — each step builds on the last.

1. [`01-install.md`](getting-started/01-install.md) — install paths
   (CLI + library)
2. [`02-first-scene.md`](getting-started/02-first-scene.md) — your
   first multi-element scene
3. [`03-grouping.md`](getting-started/03-grouping.md) — wrap related
   parts in a labeled container
4. [`04-replicas-annotations.md`](getting-started/04-replicas-annotations.md) —
   stacks, callouts, ground grid
5. [`05-publishing.md`](getting-started/05-publishing.md) — embed in
   docs / slides / LLM context

## guides/

**You have a specific task — recipes that show one solution.**

- [`extending.md`](guides/extending.md) — add a new shape, primitive,
  or layout engine
- [`troubleshooting.md`](guides/troubleshooting.md) — failure modes
  by symptom

## reference/

**You know what you want — look up the exact field name.**

- [`cli.md`](reference/cli.md) — CLI subcommands + Go library API
- [`dsl-yaml.md`](reference/dsl-yaml.md) — YAML composite spec
- [`dsl-d2.md`](reference/dsl-d2.md) — `.d2` input spec (shape
  mapping, nested containers)
- [`dsl-theme.md`](reference/dsl-theme.md) — Style / Theme cascade:
  palette / stroke / text / effects
- [`output-layout.md`](reference/output-layout.md) — output dir
  layout + embedding recipes

## concepts/

**You want to understand the design.**

- [`why-isometric.md`](concepts/why-isometric.md) — design rationale,
  tradeoffs, influences

## agent/

**You're integrating iso-topology into an LLM agent.**

- [`CAPABILITIES.md`](agent/CAPABILITIES.md) — generated machine-readable
  capability inventory (same as `isotopo capabilities`)
- [`RECIPES.md`](agent/RECIPES.md) — task → DSL primitive lookup
- [`PROMPT_TEMPLATE.md`](agent/PROMPT_TEMPLATE.md) — drop-in system
  prompt
- [`schema/dsl.schema.json`](agent/schema/dsl.schema.json) — JSON
  Schema for local lint (no CLI roundtrip)

## How to navigate by question

| If you're asking… | Read |
|---|---|
| "How do I install it?" | [getting-started/01-install.md](getting-started/01-install.md) |
| "Where do I start?" | [getting-started/02-first-scene.md](getting-started/02-first-scene.md) |
| "How do I express N replicas?" | [agent/RECIPES.md](agent/RECIPES.md) |
| "Why isn't my callout showing?" | [guides/troubleshooting.md](guides/troubleshooting.md) |
| "What does palette.left do?" | [reference/dsl-theme.md](reference/dsl-theme.md) |
| "What's the difference between .d2 and .yaml?" | [concepts/why-isometric.md](concepts/why-isometric.md) |
| "How do I add a new iso shape?" | [guides/extending.md](guides/extending.md) |
| "How do I plug this into my agent?" | [agent/PROMPT_TEMPLATE.md](agent/PROMPT_TEMPLATE.md) |

## Documentation style

This tree follows [Diátaxis](https://diataxis.fr/) — four flavors of
docs (tutorial / how-to / reference / explanation) plus a dedicated
agent section. Each flavor has a different goal; mixing them in one
page makes both audiences worse.
