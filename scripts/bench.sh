#!/usr/bin/env bash
# Runs the report's measurements and appends them to results.csv. Every
# output is compared byte for byte with V0 before its timing is accepted.
#
# Usage: scripts/bench.sh [experiment...]
#   v0           sequential baseline
#   v2 v3 ...    scaling: the version for every P in $THREADS (V3 with K=$K)
#   k            V3 for every K in $KS with P=$KP
# Env: DIR (data), CSV (results.csv), THREADS (1 2 4 6 8 10), K (1024),
#      KS (1 4 16 64 256 1024 4096), KP (8)
set -euo pipefail

dir=${DIR:-data}
csv=${CSV:-results.csv}
threads=${THREADS:-1 2 4 6 8 10}
k=${K:-1024}
ks=${KS:-1 4 16 64 256 1024 4096}
kp=${KP:-8}
bin=bin/invindex
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

go build -trimpath -o bin/ ./cmd/...

measure() {
  "$bin" -in "$dir" -out "$tmp/index.tsv" -csv "$tmp/run.csv" "$@" 2>&1 | grep -E '^v[0-9]|merge:'
  if ! cmp -s "$tmp/index.tsv" "$tmp/v0.tsv"; then
    echo "  output differs from v0, timings discarded" >&2
    rm -f "$tmp/run.csv"
    return 1
  fi
  if [[ -s $csv ]]; then # the header goes only into a new results file
    tail -n +2 "$tmp/run.csv" >>"$csv"
  else
    cat "$tmp/run.csv" >>"$csv"
  fi
  rm -f "$tmp/run.csv"
}

# The V0 reference output; it is checked, not timed.
"$bin" -v v0 -in "$dir" -out "$tmp/v0.tsv" -warmup 0 -repeat 1 2>/dev/null

exps=("$@")
((${#exps[@]})) || exps=(v2 v3 k)

for exp in "${exps[@]}"; do
  case $exp in
    v0) measure -v v0 ;;
    k) for kk in $ks; do measure -v v3 -p "$kp" -k "$kk"; done ;;
    *) for p in $threads; do measure -v "$exp" -p "$p" -k "$k"; done ;;
  esac
done
