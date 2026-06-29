# Agent-loop harness optimization — plan & acceptance

> Goal: make the diagram **react loop** (write DSL → validate/render → judge →
> adjust) converge in **as few blind iterations as possible** — ideally zero.
> Same discipline as the layout-engine master plan: one tracked number, every
> layer gated quantitatively, validated on the synthetic corpus **and** all real
> `samples/topology` demos, deterministic and golden-stable. Nothing ships on
> vibes; no churn without measured gain (the P2.2 precedent).

---

## 0. The problem (why the loop stalls)

The highest-frequency wall is the **terminal "did it render correctly?" step** of
every react cycle, for two reasons:

- **Wall A — the signal didn't predict the visual.** Feedback was scored in the
  flat plane while the user reads the iso projection, so `validate`/`evaluate`
  could pass while the render was occluded. *Largely closed already* by the
  readability objective `R` measured in iso space.
- **Wall B — the agent can't cheaply, reliably SEE the output.** SVG must be
  rasterized to be judged, and the ad-hoc `qlmanage→magick` path mis-crops and
  lies. So the agent iterates blind, re-doing fixes it cannot confirm.

This plan attacks both, in four layers, ordered by ROI.

---

## 1. North star — one tracked metric

**Blind-Iteration Count (BIC):** the number of `render` calls an agent makes
before a scene reaches **acceptance** — `occlusion = 0 ∧ overlap = 0 ∧ R ≥ R*` —
under a fixed *"apply exactly what the tool tells you"* policy. Measured by a
loop simulator (§3) over the corpus + the 13 real demos.

- **Today:** unbounded / human-in-the-loop for the handled defect classes.
- **Target:** **BIC ≤ 1** for every defect class the engine handles; **BIC = 0**
  once a class is correct-by-construction (Layer 4).

Invariants every layer must hold (non-negotiable, tested):
- **Deterministic** — byte-identical output for identical input.
- **No-op / no-regression on clean scenes** — golden-stable.
- **Validated twice** — synthetic corpus (`samples/bench`) *and* all 13
  `samples/topology` demos; 0 regressions.

---

## 2. The four layers

### Layer 1 — Self-repairing render + a "what I fixed" report
*Eliminate the iteration: the cheapest iteration is the one that never happens.*

- **Deliver:** `render` runs the projection-repair loop **by default** (opt out
  with `--no-repair`) and emits a structured `repaired:` block — one line per
  defect: `{kind, owner, action, delta}`.
- **Dev tasks:** wire `RepairScene` into `renderFile` before lowering; emit the
  repair report to stderr/JSON; keep the strict no-op on clean scenes.
- **Acceptance gate:**
  - On corpus + 13 demos, every scene whose defects are in
    `{caption-ride, world-overlap}` reaches `occlusion=0 ∧ overlap=0` in the
    **single default render** → **BIC = 0** for those classes.
  - Zero golden drift on clean scenes (repair no-op, byte-identical).
  - The report lists *exactly* the changes applied; deterministic.

### Layer 2 — One authoritative render report (signal + located defects + patches)
*Make the unavoidable iteration truthful: hand the fix over, don't make the agent guess.*

- **Deliver:** a single `render --report` payload:
  ```
  { svg, readability:{ R, breakdown:{occl,cross,tunnel,overlap,bends} },
    defects:[ { kind, severity, location:{partOrEdgeId, worldBBox},
                suggestion, patch } ] }
  ```
  Each residual defect carries a **machine-applicable `patch`** — `{target id,
  field path, op, value}` — not just prose.
- **Dev tasks:** unify `validate` + `evaluate` + occlusion detectors into one
  report builder; upgrade the existing `Issue` suggestions (which already say
  e.g. *"raise this group's front padding"*) into structured patches; add
  `isotopo apply-patch` so patches round-trip into DSL.
- **Acceptance gate:**
  - **Patch actionability ≥ 95%:** for every defect that emits a patch, applying
    it then re-rendering **clears that defect** (a test applies each suggested
    patch and re-evaluates).
  - One call, deterministic, covers **100%** of the defects `R` counts.
  - Loop simulator under the apply-patch policy: **BIC ≤ 1** on corpus + demos
    for every handled class.

### Layer 3 — Native deterministic snapshot (+ annotated overlay)
*Kill Wall B: when the agent must look, it sees the truth, with defects pointed at.*

- **Deliver:** `isotopo snapshot` rasterizes SVG→PNG **in-process** with
  viewport == the SVG `viewBox`, fixed DPI, **no trim**; `--annotate` draws
  boxes on detected defects (occluded labels, overlaps, tunnelled edges).
- **Dev tasks:** in-process deterministic SVG→raster; assert output geometry ==
  viewBox; overlay driven by the same silhouette/occlusion/overlap detectors at
  correct screen coords; replace the `qlmanage` path in the agent skill/docs.
- **Acceptance gate:**
  - **Fidelity:** snapshot content bbox == projected `viewBox` within 1px;
    byte-stable across runs (no mis-crop, ever).
  - **Localization:** annotated boxes hit **100%** of detected defects at correct
    screen coordinates (tested on known-bad scenes).
  - Agent guidance no longer references `qlmanage`/`magick`.

### Layer 4 — Correct-by-construction (joint groups + labels solve)
*Retire repair: make the defect classes 0 before any repair runs.*

- **Deliver:** caption clearance, container auto-size, and neighbour-label
  (screen-occlusion) avoidance satisfied **in the layout solve** (master-plan
  P4), so the handled defect classes are **0 by construction**.
- **Dev tasks:** constraint-based containment + caption-reserved footprint as a
  joint pass (generalize `ensureGroupFootprint`); add neighbour-label avoidance
  — the open **clickhouse-hub** title-vs-node screen-occlusion class.
- **Acceptance gate:**
  - **Construction-zero:** on corpus + demos the handled classes are **0 before
    repair** (repair becomes a no-op everywhere) → **BIC = 0**.
  - `R` ≥ parity vs Layer-1 output on **every** scene; golden-stable; deterministic.
  - clickhouse-hub neighbour-label occlusion = 0 by construction.

---

## 3. Measurement harness (how each layer is proven)

- **Loop simulator** — `simulateLoop(scene, policy) → {BIC, finalR}`: render →
  read the report → apply the tool's suggested patches → repeat until acceptance
  or a budget. This is **the** acceptance driver for BIC; run over `samples/bench`
  + all 13 `samples/topology` demos.
- **Existing gates reused:** corpus R-ranking gate, the real-demo acceptance scan
  (improved / no-op / **REGRESS=0**), golden stability.
- **The ratchet:** a layer ships only if it **lowers BIC** (or achieves
  construction-zero) with **0 regressions**. No measured gain → don't ship
  (exactly как the crossing-aware router was reverted).

---

## 4. Sequencing & ROI

1. **L1 + L2 together** — highest ROI: self-repair crushes iteration *frequency*,
   the report+patches crush per-iteration *cost*. Together they take BIC ≤ 1 for
   every handled class.
2. **L3** — perception backstop for when looking is unavoidable.
3. **L4** — the deeper, longer move that retires repair and closes the last
   open class (clickhouse neighbour-label).

---

## 5. Definition of done (whole plan)

- **BIC ≤ 1** on every corpus + real-demo scene for handled defect classes;
  **BIC = 0** for classes promoted to construction-zero (L4), incl. the
  clickhouse-hub class.
- A **single authoritative render report** with **≥ 95% patch actionability**.
- A **deterministic snapshot** primitive that has replaced `qlmanage` in agent
  guidance, with annotated defect overlays.
- Every layer **deterministic, golden-stable, 0 regressions**, validated on the
  corpus and all 13 real demos.

---

## 6. Progress log
- **L1 — DONE.** `render` runs the projection-repair loop **by default**
  (`--no-repair` opts out) and emits a `repaired (N fix(es), K iters): …` report
  listing each cleared occlusion / separated overlap (`RepairAndReport`). Gate
  met (`TestBIC_HandledClassesZero`): a single repaired render drives **BIC = 0**
  on 21/22 defective scenes across the corpus + all real demos — every handled
  class (caption-rides, world-overlaps) cleared in one call. The lone residual is
  clickhouse-hub's neighbour-label screen-occlusion (the L4 target). Clean scenes
  stay no-op; golden-stable (goldens render via the library, not the CLI).
- **L2 — DONE.** `render --report` emits a single `report.json` (`RenderReport`):
  the readability breakdown plus every occlusion located by part id, and a
  **machine-applicable `patch`** (`{target, field, value}`) on each in-group
  caption-ride — the front-padding value the repair loop converges to, so
  applying it is guaranteed to clear the ride. `ApplyPatch` round-trips a patch
  into the document. Gate met (`TestL2_PatchActionability`): **patch
  actionability 10/10 (100%)** across the corpus + real demos. The report is
  built **before** rendering (rendering's `applyLayout` clears group `Layout`
  in place, which would strip the patches). Neighbour-occlusions are located but
  carry no auto-patch yet (L4).
- **L3.1 — DONE.** `isotopo snapshot <in> <out>` renders **and** rasterizes to a
  faithful `topology.png` via **resvg** (ImageMagick fallback): viewport == the
  SVG `viewBox`, **no trim**, so geometry is preserved 1:1 and text/gradients
  render truthfully. This retires the `qlmanage→magick→trim` path that mis-crops
  and lies. Gate met (`TestSnapshot_FaithfulDeterministic`, skipped where no
  rasterizer): PNG dims == SVG dims exactly; **byte-identical across runs**
  (deterministic). Verified visually faithful on clickhouse-hub.
- **Remaining — L3.2:** annotated overlay (boxes on detected defects at their
  screen coords) — needs the detectors to expose screen bboxes; the faithful
  snapshot + the L2 report (defect located by part id) already let the agent find
  a defect, so this is a refinement.
- **Next — L4:** correct-by-construction (joint groups+labels solve) + the
  neighbour-label repair that closes the clickhouse-hub class — the higher lever.
