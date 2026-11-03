#!/usr/bin/env bash
set -euo pipefail
project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { printf 'Use make package on your native supported platform\n' >&2; exit 1; }
exec python3 "$project_dir/scripts/release.py" package --project "$project_dir" --version "${1:-0.5.3}"
