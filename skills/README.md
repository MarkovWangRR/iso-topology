# Agent skills

Installable skills that teach coding agents how to drive iso-topology
well — the distilled version of the agent docs, in the format agent
harnesses load natively.

| Skill | What it does |
|---|---|
| [`draw-iso-diagram`](draw-iso-diagram/SKILL.md) | end-to-end workflow: discover capabilities → imitate a sample → author coordinate-free YAML → validate loop → render & verify, plus the visual-quality rules the showcase gallery follows |

This repo ships **exactly one skill**, `draw-iso-diagram`. There is no skill
nested inside another — `skills/` is just the folder that holds skill
directories, one level deep.

## Source vs. installed — read this first

A harness (Claude Code, etc.) loads the **installed** copy from a skills
directory (`~/.claude/skills/draw-iso-diagram/` globally, or
`.claude/skills/...` project-local). The `skills/draw-iso-diagram/` folder **in
this repo is the source** — it is **inert at runtime** until you install it.

Consequence: **editing the repo source changes nothing at runtime until you
re-install** (and editing the installed copy directly never reaches git). Keep
the two in sync with one of the two modes below, or they silently drift.

## Install

**Claude Code** — use the install script (`--link` for development, plain copy
for distribution):

```bash
# DEV: symlink the install back to this repo — edits to the source are LIVE,
# no reinstall ever needed (one file, zero drift). Local-machine only.
scripts/install-skill.sh --link

# DISTRIBUTE: copy into ~/.claude/skills — portable, but you must RE-RUN this
# after every edit to skills/draw-iso-diagram/SKILL.md for it to take effect.
scripts/install-skill.sh

# project-local install (checked into a consuming repo) — copy or link:
scripts/install-skill.sh --dest .claude/skills          # copy
scripts/install-skill.sh --link --dest .claude/skills   # live link
```

Claude Code discovers the skill automatically; it triggers when you
ask for an architecture diagram / isometric visual, or invoke it
explicitly with `/draw-iso-diagram`.

**Other harnesses (Cursor, custom agents)** — `SKILL.md` is plain
markdown with YAML frontmatter; paste its body into your system
prompt or rules file, or point your agent at the raw URL:

```
https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/skills/draw-iso-diagram/SKILL.md
```

**MCP instead of CLI** — if your agent speaks MCP, prefer
[`isotopo-mcp`](../docs/agent/MCP.md): the same loop without shelling
out.

## Keeping skills honest

Two kinds of drift to watch:

1. **Skill vs. DSL contract.** Skills restate the DSL, so they can fall behind
   the code. The source of truth is always `isotopo capabilities` /
   [CAPABILITIES.md](../docs/agent/CAPABILITIES.md) (generated from code); the
   skill tells the agent to read it at session start, so a stale skill degrades
   gracefully rather than teaching wrong DSL.

2. **Source vs. installed copy.** If you `cp`-install and later edit the repo
   source without re-installing (or edit the installed copy without committing
   back), the running skill diverges from git — the agent then follows
   instructions you can no longer see. Prefer `scripts/install-skill.sh --link`
   on a dev machine so there is only one file. To check for drift:

   ```bash
   diff skills/draw-iso-diagram/SKILL.md ~/.claude/skills/draw-iso-diagram/SKILL.md
   ```
