# Extending isotopo

This is the agent-targeted recipe doc. Each recipe answers "I want to
add X — where do I touch?". Pair every code change with a golden-file
test so drift is caught automatically.

## How to add a new shape

A "shape" is a leaf primitive — rectangle, cylinder, cloud, etc. The
shape ends up rendered by `iso25d.Convert2DTo25D` after going through
`Flatten` (in `flatten.go`) which produces an `iso25d.ConvertOpts`.

1. **Implement the iso geometry** — create `iso25d/shape_<name>.go`:

    - Define `Iso<Name>Opts` (just the parameters the geometry needs).
    - Implement `RenderIso<Name>(o Iso<Name>Opts) string` returning an
      SVG fragment (no `<svg>` wrapper — that's added by the composite).
    - Implement `apply<Name>(o ConvertOpts, target *Iso<Name>Opts)`
      that lowers the dispatcher's `ConvertOpts` into your opts.

2. **Register in the dispatcher** — edit `iso25d/shapes.go`. Add a
   `case "<name>":` arm in `Convert2DTo25D` that calls `apply<Name>`
   then `RenderIso<Name>`.

3. **Wire d2 input** — edit `translate.go::d2ShapeCatalog`. Add every
   d2 shape string that should lower to this iso shape (with its
   height multiplier hint). If the shape is iso-native and has no d2
   counterpart, skip this step — agents will reach it via YAML only.

4. **Make Validate happy** — `CapabilityReport()` reads the catalog,
   so as long as you added to `d2ShapeCatalog` (or to the native
   shapes list in `capabilities.go::buildShapeCaps`), Validate will
   accept the new shape in YAML/D2 inputs.

5. **Add a golden test** — create `samples/node/<name>/input.yaml`
   (single-primitive) or `samples/topology/<name>-demo/input.yaml`
   (multi-element scene), then `go test -update -run TestGolden/.*<name>`
   to snapshot the expected SVG.

## How to add a new DSL primitive

A "primitive" is a composition feature like `group`, `stack`,
`annotation`, `canvas.grid`. Primitives don't map to a single iso
shape — they affect how parts get assembled.

1. **Declare the data** — add the field(s) to the right type in
   `dsl.go` with yaml/json tags. Use pointer types if the field is
   optional and should distinguish "missing" from "zero".

2. **Lower or inject** — choose one:

    - **Lowering primitive** (transforms the parts tree before
      rendering — like `stack` and `group`). Edit `lower.go`:
      extend `lowerCompositeParts` with the new case.

    - **Layer primitive** (post-rendered SVG overlay — like
      `canvas-grid` and `annotation`). Edit `inject.go`: add an
      `inject<Name>` function that splices a `<g data-layer="...">`
      into the rendered SVG.

3. **Wire the renderer** — edit `render.go::renderComposite`. Decide
   the paint order (earlier-injected = behind everything because of
   how `<svg>`-after-tag splicing pushes prior insertions deeper).

4. **Describe it** — edit `capabilities.go::buildPrimitiveCaps`. Add
   a `PrimitiveCap` entry. This is the agent's source of truth.

5. **Validate it** — edit `validate.go`. Add structural checks (does
   the anchor exist? is the side value valid? is the count positive?).
   Use the existing `nearest()` / `nearestID()` for "did you mean".

6. **Add a golden test** — same as for shapes.

## How to add a new layout engine

d2 ships dagre and ELK. To add a third:

1. Implement a `d2graph.LayoutGraph` (see d2 docs).
2. Edit `compile.go::CompileD2`. Extend `layoutResolver`.
3. Edit `dsl.go`: add a `LayoutXxx LayoutEngine` constant.
4. Edit `cmd/isotopo/main.go::parseFlags` to accept the new `--layout xxx`.
5. Edit `capabilities.go::Layouts` so agents discover it.
6. Edit `validate.go` if the layout name appears in DSL anywhere
   (today it's CLI-only, so no validate change needed).
7. Add a golden test under `samples/topology/<name>-layout/input.d2`.

## The agent loop

```
1. read   →  isotopo capabilities   (learn what's available)
2. write  →  emit DSL
3. check  →  isotopo validate <file>   (catch typos before render)
4. render →  isotopo render <file> <out>   (produce SVG)
5. test   →  go test (./...)   (regression check)
```

If step 3 returns suggestions, apply them and re-run step 3. If step
5 fails with golden drift, decide per-case: intended → update golden
with `go test -update`; unintended → revert the change.

## Code structure cheat sheet

```
dsl.go              every public DSL type
parse.go            yaml/json → Document
translate.go        d2target.Diagram → Document
flatten.go          Node + Theme → iso25d.ConvertOpts
theme.go            3-layer style merge
render.go           public API: Render / RenderDocument / RenderParts
lower.go            group + stack expansion
inject.go           canvas-bg / screen-label / connector / annotation
svgutil.go          SVG string parse/edit
output.go           per-part fragment YAML + IDs
capabilities.go     introspection — what does this build support?
validate.go         structural validation with suggestions
compile.go          d2 → laid-out diagram

iso25d/             low-level iso geometry, one shape per file
├── composite.go    iso z-order painter
├── iso.go          IsoBox renderer + shared types
├── rounded.go      rounded-corner panel renderer
├── shapes.go       Convert2DTo25D dispatcher + ConvertOpts type
├── shape_box.go    (lives inside iso.go for historical reasons)
├── shape_cylinder.go
├── shape_sphere.go
├── shape_person.go
├── shape_text.go
└── shape_cloud.go

cmd/isotopo/        the CLI
├── main.go         subcommand routing + flag parsing
```

## Don'ts

- **Don't bake icons / fonts into the binary** — SVG embeds them by
  URL. The default font stack ("Inter, sans-serif") relies on the
  browser/embedder.
- **Don't add new top-level export to the library API casually** —
  every export is part of the public contract. Prefer extending an
  existing function over adding a new one.
- **Don't modify a samples/ golden by hand** — always use
  `go test -update -run TestGolden/<name>` so re-generation matches
  the rest of the pipeline.
