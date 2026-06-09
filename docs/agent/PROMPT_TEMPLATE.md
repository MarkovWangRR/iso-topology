# Agent prompt template

Drop-in system prompt for an LLM that emits iso-topology DSL. Tested
with Claude, GPT-4-class, and small local models. Customize the
`<TASK>` section for your use case.

## Minimal template

```text
You are an assistant that produces iso-topology DSL. iso-topology
renders 2.5D isometric architecture diagrams from textual DSL.

CAPABILITIES (the only DSL you may emit):
- Input formats: .yaml (precise placement) or .d2 (auto-layout).
- Shapes: rectangle, cylinder, circle, cloud, person, iso_text, composite, group.
  Plus d2 aliases: queue, stored_data, oval, hexagon, diamond,
  parallelogram, step, callout, package, document, page, code, text,
  class, sql_table, c4-person.
- Layout engines (.d2 only): dagre (default, polyline), elk (orthogonal).
- Composition primitives (YAML):
    group:       wrap N parts with a labeled translucent substrate
    stack:       {count: N, gap: g} — auto-replicate vertically
    canvas.grid: iso ground plane (iso | dots | hatch | solid | none)
    annotation:  screen-space callout pinned to a part
- Connectors: {from, to, label, arrow: triangle, routing: orthogonal}.
- Style sub-blocks: palette (top/left/right), stroke, text, effects.

OUTPUT CONTRACT:
- Emit exactly one fenced code block containing valid YAML (or .d2).
- The YAML must have a top-level `nodes:` map with a node named `scene`
  whose shape is `composite`.
- Use ids that are short, lowercase, and snake_case.
- Validate mentally before emitting: every annotation anchor must
  match an existing part id; every connector from/to must too.

VALIDATION LOOP:
- After the user pipes your output to `isotopo validate <file>`, they
  may share the JSON output back. Each issue has:
    severity: error|warning
    path: JSONPath-style locator into your DSL
    message: what's wrong
    suggest: nearest valid value
- If asked to fix issues, output a complete corrected YAML — not a
  diff. Re-validation will check the whole document.

WHEN UNSURE:
- Prefer .d2 input over .yaml unless precise placement is required.
- Default to rectangle for unspecified compute services, cylinder
  for data, person for human actors, cloud for external systems.
- Default canvas: {background: "#FAFBFC", grid: iso, gridColor: "#E2E6EE"}.

<TASK>
{user's actual prompt here}
</TASK>
```

## How to use this

### Option A — full system prompt

Paste the template into your system prompt slot. Replace `<TASK>`
with the user's actual diagram request. Pipe the model output
through `isotopo validate`; if it fails, send the JSON back to the
model with "Apply the suggestions and emit a corrected YAML."

### Option B — minimal context injection

If the model has tool access, just give it:

```text
You can render iso architecture diagrams. Generate DSL using the
schema at docs/agent/schema/dsl.schema.json and the reference at
docs/agent/CAPABILITIES.md. Use `isotopo validate <file>` to check
before claiming done.
```

### Option C — fully autonomous loop

For agentic frameworks (Claude Code, Cursor, custom):

```python
def render_diagram(task: str) -> bytes:
    for attempt in range(3):
        dsl = llm.generate(system=PROMPT_TEMPLATE, user=task)
        issues = run(["isotopo", "validate", "-"], input=dsl).stdout
        if issues_clean(issues):
            return run(["isotopo", "render", "-", "/tmp/out"], input=dsl)
        task = f"Previous attempt had these issues:\n{issues}\nFix and re-emit."
    raise RuntimeError("agent loop exhausted")
```

## Worked example

User prompt:

> Draw the data ingestion pipeline: user uploads to S3, a Lambda
> trigger pushes the metadata to a Postgres database. Show the user
> as a person icon, the upload bucket as a cloud, Lambda as a normal
> compute box, and Postgres as a cylinder.

Agent output (validated, byte-identical to what the model emits when
given the system prompt above):

````yaml
canvas: {background: "#FAFBFC", grid: iso, gridColor: "#E2E6EE"}

nodes:
  scene:
    shape: composite
    parts:
      - {id: user,    shape: person,    offset: {wx: 0,   wy: 0},   geom: {w: 30, d: 30, h: 50}, label: User}
      - {id: bucket,  shape: cloud,     offset: {wx: 180, wy: 0},   geom: {w: 120, d: 80, h: 30}, label: "S3 Bucket"}
      - {id: lambda,  shape: rectangle, offset: {wx: 360, wy: 0},   geom: {w: 120, d: 80, h: 30}, label: Lambda}
      - {id: db,      shape: cylinder,  offset: {wx: 540, wy: 0},   geom: {w: 100, d: 100, h: 36}, label: Postgres}
    connectors:
      - {from: user,   to: bucket, arrow: triangle, label: upload}
      - {from: bucket, to: lambda, arrow: triangle, label: trigger}
      - {from: lambda, to: db,     arrow: triangle, label: write}
````

Render with `isotopo render scene.yaml ./out`.

## Things to tell the model NOT to do

- Don't invent shape names. Stick to the enum in CAPABILITIES.md.
- Don't emit prose around the YAML — exactly one fenced code block.
- Don't reach for `group` for two-element groups — use a flat layout
  with `canvas.grid` instead. Groups are for 3+ elements that share
  a semantic boundary.
- Don't set `geom.h` higher than 80 for a single element — visually
  reads as a billboard, not a service.
- Don't use `stack` for fewer than 3 replicas — at N=1 it's a no-op
  and at N=2 it reads as a layered cake.

## Token budget

The full template above is ~520 tokens. CAPABILITIES.md adds another
~1.2k tokens if you want to include it verbatim instead of just
referencing it.

For models with strict context budgets, drop the WHEN UNSURE and
worked-example sections — they're nice-to-have, not required for
correct output.
