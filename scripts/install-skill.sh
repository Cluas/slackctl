#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${ROOT}/skills/slackctl"

if [[ ! -d "${SRC}" ]]; then
  echo "Skill source not found: ${SRC}" >&2
  exit 1
fi

install_one() {
  local dest_root="$1"
  local dest="${dest_root}/slackctl"

  mkdir -p "${dest_root}"

  if command -v rsync >/dev/null 2>&1; then
    mkdir -p "${dest}"
    rsync -a --delete "${SRC}/" "${dest}/"
  else
    rm -rf "${dest}"
    cp -R "${SRC}" "${dest}"
  fi

  echo "Installed skill to: ${dest}"
}

install_one "${HOME}/.agents/skills"   # Codex, Gemini CLI, OpenCode
install_one "${HOME}/.claude/skills"   # Claude Code

echo ""
echo "Done. Skill installed for Claude Code and other agents."
