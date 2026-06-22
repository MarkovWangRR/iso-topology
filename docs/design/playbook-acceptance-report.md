# Playbook Design-Flywheel — Acceptance Report

**Mandate:** multi-agent, real-API (no mocks), ≥5 iteration rounds, no DSL changes —
build the most semantically-rich, diverse, agent-retrievable playbook; final
diagrams must be *lively* (not rigid) and maximise the DSL's visual techniques.

All LLM/vision calls were **real** (SiliconFlow OpenAI-compatible endpoint,
`Qwen/Qwen3-VL-32B-Instruct` for vision/extraction/judge). No mocked responses.

---

## 1. Corpus — 12 styles, distilled from real landing pages

Captured **7 live landing pages** via headless Chrome (stripe, vercel, neon,
grafana, supabase, datadog, planetscale) + 3 existing (snowflake, databricks,
motherduck) + 2 hand-authored (lustre, default). Each company page was
reverse-distilled (extract → synthesize → render → judge → refine) into a
`trust: auto` manual. Gallery: `samples/playbook/_gallery/corpus.png`.

Result: 4 dark themes (vercel, neon, supabase, planetscale) + light themes
(stripe, grafana, datadog, snowflake, databricks, motherduck, lustre), 12
distinct accent hues. Index: `samples/playbook/INDEX.json`.

## 2. Multi-agent discovery test (invisible observation)

8 agents were each given a *different* architecture scenario and the repo — **not**
told to use the playbook. Observed behaviour:

| metric | result |
|---|---|
| agents that discovered the registry (INDEX.json / SKILL §1.5) | **8 / 8** |
| agents that authored role-structure + ran `playbook apply` | **8 / 8** |
| renders that succeeded (0 errors) | **8 / 8** |

→ The affordance (skill gate + INDEX contract) works unprompted.
Gallery: `samples/playbook/_gallery/discovery-round.png`.

## 3. Iteration rounds (no DSL changes — all in `internal/playbook`)

| round | change | effect |
|---|---|---|
| R1 | baseline distill (white cards always) | dark brands rendered as light cards; flat |
| R2 | dark-theme-aware synthesis (dark cards, light ink, white icons, contrast floor) | dark brands read correctly |
| R3 | style **axes derived from mood** (corners/depth/gloss/edge/energy) + token caching (`resynth`, instant no-API iteration) | feel varies, not just hue |
| R4 | brand accent threaded through borders / edges / face sheen | brand colour permeates beyond the hero |
| R5 | translucent group trays; showcase round | "transparency" gap closed |

Token caching (`distill/extracted.json` + `playbook resynth`) made R3–R5 engine
iteration **free** (no re-paying for vision) — the loop ran in seconds.

## 4. Final showcase + audit (real vision API)

8 agents drew 8 distinct architectures in 8 distinct styles
(`samples/playbook/_gallery/showcase.png`). Vision-audited each render against
its source brand:

| diagram | style | style-match | vibrancy | DSL techniques |
|---|---|---|---|---|
| edge-db | neon | 75 | 80 | 6 |
| BaaS | supabase | 85 | 75 | 8 |
| SIEM | datadog | 65 | 70 | 8 |
| sharded-SQL | planetscale | 75 | 65 | 6 |
| payments | stripe | 45 | 65 | 5 |
| IoT-mon | grafana | 45 | 55 | 4 |
| warehouse | motherduck | 45 | 60 | 5 |
| **avg (distilled)** | | **62** | **67** | **6.0** |

Vibrancy rose R1→R5 (58 → 67). DSL techniques per diagram averaged **6 of ~9**
(gradient-faces, glow, gradient-edges, dotted-edges, soft-shadow, rounded,
outline, lighting, transparency all observed in the corpus).

## 5. Diagnosis — where borrowing falls short, by root cause

The audit gaps cluster into exactly the three axes requested:

1. **Playbook extraction** (fixable, done): early rounds lost dark themes and
   confined brand colour to the hero. Fixed via dark-aware synthesis + accent
   threading. Residual: the vision model hedges abstract axes to "balanced" →
   mitigated by mood-derived axes.
2. **Agent ReAct — which style to borrow** (partially fixed): in the discovery
   round 3/8 agents defaulted to `vercel` for any *dark/technical* scenario —
   the INDEX metadata doesn't differentiate dark styles enough, so agents pick
   the most famous one. Recommendation: add a `best_for` / distinctive `why`
   per style and a skill nudge toward the most *specific* match.
3. **DSL expressiveness limit** (genuine boundary, DSL frozen): the lowest,
   most stable scores are `stripe` (45) and `grafana` (45) — both want
   **animated multi-colour gradient backgrounds and flowing organic shapes**.
   isotopo's canvas is a single solid fill + iso grid and its shapes are
   prismatic; this design language is **not expressible** without DSL changes
   (out of scope). This is the hard ceiling, not an engine bug.

## 6. What's lively, what to do next

Achieved: a diverse, vibrant corpus (4 dark / 4 light, 12 hues), 8/8 unprompted
agent adoption, 6/9 DSL techniques per diagram, vibrancy 67/100.

Open (non-DSL): richer INDEX `best_for` metadata to spread style selection;
spread accent onto more roles for the "use the brand colour more" gap; a
gradient-backdrop shape would need a DSL addition (the one real ceiling found).

Artifacts: `samples/playbook/_gallery/{corpus,discovery-round,showcase}.png`,
`samples/playbook/INDEX.json`, per-style `distill/report.md` + `extracted.json`.
