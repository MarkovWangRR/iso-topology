#!/usr/bin/env bash
# Install the draw-iso-diagram skill so an agent harness can load it at runtime.
#
# A harness loads the INSTALLED copy under a skills directory — NOT this repo's
# skills/ source. So after editing skills/draw-iso-diagram/SKILL.md you must
# re-run this (copy mode) for the change to take effect — unless you used
# --link, in which case the install is a symlink back to the repo and edits are
# live immediately.
#
# Usage:
#   scripts/install-skill.sh                 # copy into ~/.claude/skills (re-run after edits)
#   scripts/install-skill.sh --link          # symlink ~/.claude/skills -> repo (live, dev)
#   scripts/install-skill.sh --dest DIR      # install into DIR/draw-iso-diagram instead
#   scripts/install-skill.sh --link --dest .claude/skills   # project-local live link
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$repo_root/skills/draw-iso-diagram"
dest_root="${HOME}/.claude/skills"
mode="copy"

while [ $# -gt 0 ]; do
  case "$1" in
    --link) mode="link" ;;
    --dest) shift; dest_root="$1" ;;
    -h|--help) sed -n '2,16p' "$0"; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

[ -f "$src/SKILL.md" ] || { echo "source skill not found at $src" >&2; exit 1; }
mkdir -p "$dest_root"
target="$dest_root/draw-iso-diagram"
rm -rf "$target"

if [ "$mode" = "link" ]; then
  ln -s "$src" "$target"
  echo "linked  $target -> $src  (edits are live; no reinstall needed)"
else
  cp -r "$src" "$target"
  echo "copied  $src -> $target  (re-run this script after editing the source)"
fi
