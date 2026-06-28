# Layout & Routing — master plan to design-grade quality

> Goal: take the auto-layout + connector routing to **industry-top-tier**
> readability for isometric architecture diagrams, via a principled pipeline
> whose objective is measured **in the space the user actually looks at** (the
> iso projection), reusing mature world-plane algorithms and adding the one
> piece nobody else has: a **2.5D occlusion objective with a projection-repair
> loop**.
>
> This doc is both the technical scheme and the phased acceptance plan that
> drives the work. Every phase has a quantitative gate; nothing ships on vibes.

---

## 0. First principles (the whole design in four claims)

1. **The objective lives on the final screen (iso projection), not the world
   plane.** The user reads the projected image. Today `evaluate` scores the flat
   plane and the renderer's router optimizes something else — they disagree, so
   neither is the truth. There must be ONE readability objective, computed on the
   iso projection.

2. **Affine decomposition.** The iso ground projection
   `P(x,y) = ((x−y)·cos30, (x+y)·sin30)` is an invertible linear map, so it
   preserves positions and **edge crossings**. Therefore readability splits
   cleanly:
   - **World-plane (affine-invariant):** placement, ground routing, crossings,
     length, compactness, alignment → **reuse industry SOTA**.
   - **Height-induced (2.5D-only):** occlusion of bodies/labels/icons, the
     `(x+y)`-monotonic "tent", tall-node clearance, ground risers → **the novel
     part we must build**.

3. **Small N changes the calculus.** Architecture diagrams are 10–50 nodes, not
   10⁶. We can afford expensive, near-optimal search (ILP/metaheuristics) on the
   true objective — "fast greedy" is a self-imposed and unnecessary constraint.

4. **Don't reinvent the world-plane core.** The repo already depends on
   `d2` (→ dagre/ELK). Placement and global routing are solved problems; our
   moat is the iso objective + repair loop, not a hand-rolled Sugiyama.

---

## 1. North star — what "top-tier" means, measurably

A single scalar **Readability Score** `R(scene) ∈ [0,1]`, computed on the iso
projection, is the contract for the whole engine. It aggregates:

| Dimension | Measured in | Weight class |
|---|---|---|
| **Occlusion** (body/label/icon/caption hidden) | iso screen (painter-ordered silhouettes) | **dominant** |
| **Edge crossings** | world plane (= iso, affine-invariant) | high |
| **Edge tunnelling** (route through a node) | world plane | high |
| **Bends / route complexity** | route polylines | mid |
| **Compactness / aspect** (near-square, not a bar) | iso bbox | mid |
| **Alignment** (nodes on grid tracks, axis-flush edges) | world | mid |
| **Balance / flow clarity** (rank monotonicity, weight spread) | iso | low |
| **Length** | route polylines | tiebreak |

`R` is the optimization target for placement+routing AND the acceptance metric.
`evaluate` reports its breakdown; the renderer optimizes toward it. They can no
longer diverge — that's the point.

**Top-tier bar (the numbers the engine must hit on the benchmark corpus, §6):**
- Occlusion: **0** body/label occlusions after repair on ≥95% of corpus scenes.
- Crossings: **≤ yFiles/ELK parity** (within +10% of an ELK-routed baseline).
- `R` **rank-correlation ≥ 0.85** with human "good/bad" ordering.
- Determinism: byte-identical output for identical input (golden-stable).
- Performance: **< 150 ms** end-to-end for N ≤ 50.

---

## 2. Target architecture (the pipeline)

```
        ┌─────────────────────────────────────────────────────────┐
        │  R(scene): single readability objective in ISO space     │  ◀── §1
        └─────────────────────────────────────────────────────────┘
                       ▲ measures / drives every stage
 graph ─▶ [1 classify] ─▶ [2 placement] ─▶ [3 global routing] ─▶ [4 project+repair] ─▶ iso SVG
            DAG?mesh?        ELK/dagre |        libavoid-style          detect occlusion
                            stress/cola         (world ground)          → local repair → reproject
                              + constraints
                                  ▲
                            [5 groups joint solve: containment + caption clearance]
```

- **(1) Classify** the graph (DAG-ness, density, has-groups) → pick the placement
  engine. No more "force everything through longest-path ranking".
- **(2) Placement** — adaptive: DAG/flow → full Sugiyama (network-simplex
  layering + Brandes–Köpf x-coords via ELK/dagre); mesh/hub → stress majorization
  / constrained force (cola-style). Height-aware separation constraints baked in.
- **(3) Global routing** — replace the per-edge 2-candidate greedy with a global
  orthogonal router (libavoid/Adaptagrams family) minimizing crossings+bends+
  length+tunnelling jointly, with bus routing for parallel edges.
- **(4) Project + repair** — the novel core: project to iso, measure occlusion
  (height-driven), locally repair (nudge / add clearance / lift label), reproject;
  iterate to convergence. This is "optimize in the space you render".
- **(5) Groups** — constraint-based containment + auto-size (incl. caption
  clearance) solved jointly, not patched after.

Reuse map: (2)+(3) = industry libs; (1)+(4)+(5) + the objective = ours.

---

## 3. The objective function, precisely (Phase 0 deliverable)

`R = 1 − normalized(Σ wᵢ · costᵢ)`, all costs computed after projecting the
solved scene to iso screen space:

- `occl`: Σ over (label/icon/body) of covered-area fraction by later-painted
  opaque silhouettes (convex-hull test, painter order) — reuse the work already
  in `occlusion.go` (silhouette/coverage), generalized to bodies + icons.
- `cross`: count of edge-segment intersections (world plane; affine ⇒ == screen).
- `tunnel`: Σ `routeTunnels` over edges (already exists).
- `bends`: Σ interior corners; `len`: Σ polyline length.
- `aspect`: `|log(W_iso / H_iso)|` penalty (near-square preferred).
- `align`: fraction of nodes off the shared grid tracks; edges not axis-flush.
- `balance`: rank-monotonic violations + visual-weight variance across quadrants.

Weights start from the current `routeCost` ladder (tunnel ≫ cross ≫ bends ≫
len) extended with occlusion as the new dominant term, then **calibrated against
the human-labeled corpus** (fit weights so `R` ranks the corpus like a human).

---

## 4. Phased plan with acceptance gates

Each phase is independently shippable, golden-stable, and gated on a number.
**A phase is not "done" until its gate passes on the benchmark corpus (§6).**

### Phase 0 — Measurement foundation *(you can't optimize what you can't measure)*
- Build the benchmark corpus (§6) + human/heuristic good-bad labels.
- Implement `R(scene)` (§3) computed in iso space; expose via `evaluate`.
- Unify `evaluate` and the renderer onto `R` (single source of truth).
- **Gate:** `R` rank-correlation ≥ 0.80 with human ordering on the corpus;
  occlusion term present and non-zero on known-bad scenes; deterministic.

### Phase 1 — Occlusion objective + projection-repair loop *(highest leverage)*
- Complete iso-screen occlusion detection: bodies, leaf labels, icons, captions
  (generalize existing caption/leaf detectors).
- Repair loop: detect → local fix (nudge node / add clearance / lift label /
  grow group) → reproject → iterate to fixpoint or budget.
- **Gate:** occlusion = 0 after repair on ≥95% of corpus; `R` improves vs P0 on
  every corpus scene; zero golden drift on already-clean scenes; converges in
  ≤ K iterations; deterministic.

### Phase 2 — Global routing *(replace greedy)*
- Integrate/port a libavoid-style global orthogonal router (world ground plane);
  objective = cross + bend + len + tunnel jointly; bus routing for parallel edges.
- Keep the iso post-transforms (ground-hug, riser, anti-tent) as projection
  post-processing.
- **Gate:** on corpus, `cross` and `bends` drop ≥ 30% vs current router;
  `TestAB_RouterImprovesLayout`-style A/B shows strictly-better `R`; within +10%
  of an ELK-routed crossing baseline; deterministic.

### Phase 3 — Adaptive placement *(beyond DAG)*
- Graph-class detector; DAG → ELK/dagre full Sugiyama; mesh/hub → stress/cola
  with height-aware non-overlap + alignment constraints.
- **Gate:** mesh/hub corpus scenes (where longest-path degrades today) improve
  `R` ≥ 25%; DAG scenes ≥ parity; near-square aspect achieved; deterministic.

### Phase 4 — Joint groups + labels
- Constraint-based containment + auto-size including caption clearance; basic
  label placement (avoid overlaps).
- **Gate:** the group-caption-ride class is **0 by construction** on corpus
  (not by post-hoc warning); group+auto produces labeled, non-overlapping
  containers; golden-stable.

### Phase 5 — Near-optimal polish *(small-N affordance)*
- Metaheuristic (simulated annealing / large-neighborhood search) on `R` for
  final refinement, seeded by P2/P3 output; time-boxed for N ≤ 50.
- **Gate:** `R` within X% of a computed lower bound (or beats P4 by ≥ 10% with
  no regressions); still < 150 ms for N ≤ 50; deterministic (fixed seed).

---

## 5. How this drives itself (the control loop)

- **One number, `R`** — every phase moves it; every PR reports its delta on the
  corpus. Regressions block.
- **Benchmark corpus is the CI gate** — `go test` runs `R` over the corpus and
  fails on any phase-gate regression (extends the existing golden + planab A/B
  harness).
- **Adversarial scenes** — each fixed bug (caption ride, array poke-through,
  faces contrast, iso occlusion) becomes a permanent corpus entry that must keep
  `R` high — so old bugs can't silently return.
- **No phase skips its gate.** The plan is a ratchet: `R` only goes up.

---

## 6. Benchmark corpus (the measurement substrate)

~40–60 scenes spanning: DAG flows, hub-and-spoke, dense mesh, deep nesting,
tall-node stacks, long captions, mixed shapes (arrays/prisms/racks), light &
dark themes — each with (a) the input, (b) a human good/bad label or pairwise
ranking, (c) the current `R`. Seeded from `samples/topology/` + `style_refer/`
+ the adversarial regressions already written. Lives in `samples/bench/`.

---

## Progress log

- **Phase 0 — DONE.** `Readability(doc) → R` implemented (`readability.go`),
  computed in iso space (occlusion + EvaluateIso crossings/tunnels/overlaps/
  bends/length); exposed via `isotopo evaluate` (`readability` field).
  Benchmark corpus seeded at `samples/bench/` (good-flow, good-grid;
  bad-occlusion, bad-overlap, bad-both) with `labels.txt`. **Gate met**:
  `TestReadabilityRanksCorpus` — worst good R=0.872 ≫ best bad R=0.200 (clean
  separation). Weight calibration: bends/length demoted to tiebreaks (they scale
  with size, not quality); occlusion/overlap dominate.
  - **Surfaced gaps (feed forward):** straight-edge **crossings and tunnelling
    are not detected** by EvaluateIso (router avoids orthogonal crossings; straight
    routes aren't scored), so a reliable crossing/tunnel corpus scene must wait on
    **P2** (global routing + detection). Body-vs-body occlusion (node fully hidden)
    is not yet in the occlusion term — **P1**.
- **Phase 1 — IN PROGRESS.**
  - **Projection-repair loop v1 (caption clearance) — DONE** (`repair.go`,
    `RepairScene`): detect iso-screen caption occlusion → widen the offending
    group's front padding → re-check → converge. `isotopo render --repair` runs
    it. Demonstrated: bad-occlusion auto-fixes in 1 iteration, occlusion 1→0,
    **R 0.167 → 1.000**; strict no-op (0 iters) on clean scenes (golden-safe).
    Tests: `TestRepair_FixesCaptionOcclusion`, `TestRepair_NoOpOnClean`.
  - **Fixed a real side-effect bug en route:** `EvaluateIso` (hence
    `Readability`) was solving + clearing the caller's Layout in place; now it
    deep-clones (`cloneSceneForEval`). `TestEvaluate_DoesNotMutateDoc` locks it.
  - **Remaining P1:** body-vs-body occlusion detection + an overlap-removal
    repair (bad-overlap / the overlap half of bad-both still need it), then the
    P1 gate (occlusion 0 after repair on ≥95% corpus).
- **Next:** overlap-removal repair, then body-occlusion detection.

## 7. Honest caveats

- "Optimal" is NP-hard (crossing/area minimization); the goal is **objective in
  the right space + mature components + near-optimal search at small N**, not a
  proven global optimum.
- The risk is in **(4) repair-loop convergence** and **(3) router integration**
  (Go port or cgo). Both are de-risked by the corpus: if `R` doesn't improve,
  the change doesn't ship.
- Sequencing is deliberate: **measure (P0) → the one thing only we can do (P1) →
  borrow the strong components (P2/P3) → joint constraints (P4) → polish (P5)**.
  Value lands early (P1 fixes the whole occlusion bug class structurally), and
  every step is independently shippable.
