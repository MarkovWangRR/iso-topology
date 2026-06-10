# Agent skills

Installable skills that teach coding agents how to drive iso-topology
well — the distilled version of the agent docs, in the format agent
harnesses load natively.

| Skill | What it does |
|---|---|
| [`draw-iso-diagram`](draw-iso-diagram/SKILL.md) | end-to-end workflow: discover capabilities → imitate a sample → author coordinate-free YAML → validate loop → render & verify, plus the visual-quality rules the showcase gallery follows |

## Install

**Claude Code** — copy into your skills directory (project-local or
global):

```bash
# project-local (checked into your repo, shared with your team)
mkdir -p .claude/skills
cp -r skills/draw-iso-diagram .claude/skills/

# or global (all your sessions)
cp -r skills/draw-iso-diagram ~/.claude/skills/
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

Skills restate the DSL contract, so they can drift. The source of
truth is always `isotopo capabilities` /
[CAPABILITIES.md](../docs/agent/CAPABILITIES.md) (generated from
code); the skill instructs the agent to read it at session start, so
a stale skill degrades gracefully rather than teaching wrong DSL.
