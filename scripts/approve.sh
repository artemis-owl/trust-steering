#!/usr/bin/env bash
set -euo pipefail

# Human side of the approval gate: list/grant/deny pending work orders.
# See tools/approve for usage.

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export APPROVALS_DIR="${APPROVALS_DIR:-${root_dir}/out/approvals}"
cd "${root_dir}/tools/approve" && exec go run . "$@"
