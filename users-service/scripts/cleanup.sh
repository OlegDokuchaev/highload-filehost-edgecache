#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

remove_target() {
  local target="$1"
  if [[ -e "${target}" ]]; then
    if [[ "${DRY_RUN}" == "true" ]]; then
      echo "[dry-run] would remove: ${target}"
    else
      rm -rf "${target}"
      echo "removed: ${target}"
    fi
  fi
}

paths_to_remove=(
  ".pytest_cache"
  ".mypy_cache"
  ".ruff_cache"
  ".coverage"
  "htmlcov"
  "build"
  "dist"
)

for path in "${paths_to_remove[@]}"; do
  remove_target "${path}"
done

while IFS= read -r -d '' dir; do
  remove_target "${dir}"
done < <(find . -type d \( -name "__pycache__" -o -name "*.egg-info" \) -print0)

echo "cleanup done"
