#!/usr/bin/env bash
# Demonstrates lost updates in V1: runs it repeatedly on the same corpus with
# the same number of threads and compares the total number of occurrences with
# the V0 reference. Prints CSV.
#
# Usage: scripts/v1-lost-updates.sh [threads...]   (default: 8)
# Env:   DIR (corpus, default: data), RUNS (default: 10), BIN (default: bin/invindex)
set -euo pipefail
export LC_ALL=C

dir=${DIR:-data}
runs=${RUNS:-10}
bin=${BIN:-bin/invindex}
threads=${*:-8}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

occurrences() {
  "$bin" -in "$dir" -out "$tmp/index.tsv" -warmup 0 -repeat 1 "$@" 2>&1 | awk '/^occurrences:/ { print $2 }'
}

ref=$(occurrences -v v0)
echo "threads,run,occurrences,reference,lost,lost_pct"
for p in $threads; do
  for run in $(seq "$runs"); do
    n=$(occurrences -v v1 -p "$p")
    awk -v p="$p" -v r="$run" -v n="$n" -v ref="$ref" \
      'BEGIN { printf "%d,%d,%d,%d,%d,%.3f\n", p, r, n, ref, ref - n, 100 * (ref - n) / ref }'
  done
done
