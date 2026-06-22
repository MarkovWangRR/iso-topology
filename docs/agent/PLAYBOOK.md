# Playbook registry — reusable design styles for agents

A **playbook** is a reusable design system (one per visual style). You author
**structure + roles**; the style applies the look deterministically. Styling
never touches the isotopo DSL — `apply` emits plain isotopo YAML.

## Flow A — use a style
1. `isotopo playbook list` · `isotopo playbook search "<intent / vendor / mood>"` → pick a name.
   `search` returns each style's `roles`, `preview`, and ready `apply` command.
2. Author structure with `role:` on every node (no palette/effects). Universal roles:
   `hero · surface · source · sink · store · gateway · group · accent`.
3. `isotopo playbook apply <style> structure.yaml -o styled.yaml && isotopo render styled.yaml out/`
   No match? use `default`. Escape hatch: an explicit per-node `style:` still wins.

## Flow B — distil a style from a reference image
`isotopo playbook distill <name> --source <image> [--iters N] [--target 75]`
extract (vision) → synthesize → render → judge → refine (inverse rendering: the
renderer is the forward model, a vision judge is the loss). Output is
`trust: auto` (vs hand-vetted `blessed`), confidence = final score.

Catalogue: `samples/playbook/INDEX.json` (regenerate: `isotopo playbook index`).
Role ontology: `samples/playbook/_roles.yaml`. Plan: `docs/design/playbook-flywheel-plan.md`.
