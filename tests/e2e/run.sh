#!/usr/bin/env bash
set -euo pipefail

required=(kind kubectl flux docker)
for tool in "${required[@]}"; do
  command -v "${tool}" >/dev/null || { echo "missing E2E prerequisite: ${tool}" >&2; exit 2; }
done

echo "E2E harness prerequisites found. Set MOSAIC_E2E_FIXTURE to a deployable locked Mosaic project archive to run the cluster workflow."
test -n "${MOSAIC_E2E_FIXTURE:-}" || exit 2
echo "The full cluster workflow is intentionally gated on an explicit local fixture; no public registry is used."
