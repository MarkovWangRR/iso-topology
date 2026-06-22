# Node-Style Playbook Index

The 26 references are grouped by **node visual grammar** (not topology) into **8 style
families**. Each family = a set of node visual dimensions + style semantics + prompt +
canonical DSL template, detailed in `styles/<slug>.md`. Extraction method in `HOW_TO_EXTRACT.md`.

| slug | name | one-line semantics | node signature (the key stroke) | merged references |
|---|---|---|---|---|
| `wireframe-lineart` | Wireframe / line-art | blueprint feel, restrained, structure-first | hollow faces (`wireframe`), thin stroke only | black-line / pure-black-line / green-bg-line |
| `handdrawn-sketch` | Hand-drawn / sketch | warm, human, low-fi draft | thin ink line + `grain`, paper background | hand-drawn / green hand-drawn / sketch+tech / wireframe-handdrawn |
| `ascii-dot` | ASCII / dot-matrix | data feel, retro terminal, print halftone | `pattern` dots/hatch on faces, monochrome | bw-ascii-line |
| `solid-flat` | Solid structuralism | clean, assertive, flat illustration | solid two-tone face + same-hue-darker thin border, small radius | structuralism / solid-colour tech |
| `gradient-face` | Gradient face | modern, dynamic, warm/cool tension | per-face linear gradient (light->dark), faint/no border | red-gradient / dark frosted gradient |
| `frosted-glass` | Frosted glass / breathing | premium, light, translucent | translucent faces (opacity ~0.6) + soft glow, almost no border | borderless frosted glass / transparent frosted |
| `neon-glow` | Neon tech | futuristic, high-energy, glows in the dark | dark bg + border brighter than face + strong `backglow` | tech-blue breathing / dark-purple tech / blue-bg tech |
| `soft-rounded-card` | Soft rounded card | gentle, product UI, layered | large radius + soft shadow + faceSplit lighting | rounded-layers (the `lustre` preset is the mature example) |

## How an agent uses this
1. Read the reference -> identify the family with the 7-dimension checklist in `HOW_TO_EXTRACT.md` (or the decision tree).
2. Open `styles/<slug>.md`, copy its "canonical DSL template" into `theme.presets` or a node `style`.
3. **Only change colours / background** to fit the specific reference; keep the structural
   dimensions (border relationship / corner radius / form / light) consistent with the family.
4. Use one family preset for all nodes in a diagram for cohesion; the hero may be intensified
   within the same family (larger radius / stronger glow).

## Dimension overview (8 families)
| slug | fill | border vs face | radius | face | light | form (h:w) | texture |
|---|---|---|---|---|---|---|---|
| wireframe-lineart | hollow | thin line = the body | 0~small | none | none | cubic | optional fine dots |
| handdrawn-sketch | hollow / light wash | thin ink | small | paper-white / pale | none | cubic | grain |
| ascii-dot | light fill | thin dark line | 0~small | light grey / white | none | flat~cubic | dots/hatch |
| solid-flat | solid | same-hue darker, 1~1.2 | small (2~6) | two-tone solid | none | cubic / mid-tall | none |
| gradient-face | solid | faint / none | small~mid (6~10) | linear gradient | optional soft shadow | mid~tall | none |
| frosted-glass | translucent | none / very faint | mid (8~12) | cool gradient, opacity 0.6 | soft backglow | cubic | optional blur |
| neon-glow | solid / gradient | **brighter neon**, 1.2~1.5 | small~mid | saturated dark | strong backglow | mid | optional glow lines |
| soft-rounded-card | solid / subtle gradient | light, 1 | **large (16~24)** | light / subtle gradient | soft dropShadow | flat~mid | none |
