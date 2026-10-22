#!/usr/bin/env bash
# Runs every measurement of the report from scratch (about 25-30 minutes) and
# regenerates the graphs and tables. Quit other applications first and keep the
# Mac on power with the lid open; caffeinate keeps it awake while this runs.
#
# Usage: scripts/final.sh        (log: results/final.log)
# Env:   DIR (corpus, default: data); ALLOW_BATTERY=1 skips the power check (for test runs only)
set -euo pipefail
cd "$(dirname "$0")/.."
[[ -n ${CAFFEINATED:-} ]] || exec env CAFFEINATED=1 caffeinate -i "$0" "$@"

export DIR=${DIR:-data}
[[ -n ${ALLOW_BATTERY:-} ]] || pmset -g batt | grep -q "AC Power" || { echo "connect the charger first" >&2; exit 1; }
mkdir -p results/profiles
exec > >(tee results/final.log) 2>&1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

conditions() { echo "== $1 $(date '+%F %T'); load average $(sysctl -n vm.loadavg); $(pmset -g batt | head -1)"; }

{
  sw_vers
  sysctl machdep.cpu.brand_string hw.physicalcpu hw.logicalcpu hw.perflevel0.physicalcpu hw.perflevel1.physicalcpu \
    hw.cachelinesize hw.perflevel0.l1dcachesize hw.perflevel0.l2cachesize hw.perflevel1.l2cachesize hw.memsize
  go version
} >results/system.txt

rm -f results.csv results/falseshare-200M.csv results/v1-lost-updates.csv results/profiles/*
go build -trimpath -o bin/ ./cmd/...
conditions start

echo "== index versions"
scripts/bench.sh v0 v2 v3 v4seq v4tree cas syncmap k std

echo "== false sharing"
bin/falseshare -csv results.csv
bin/falseshare -n 200000000 -stride 8,32,40,64,96,128,256 -csv results/falseshare-200M.csv

echo "== V1 lost updates"
scripts/v1-lost-updates.sh 2 4 8 10 >results/v1-lost-updates.csv

echo "== profiles"
for cfg in v0:1 v2:8 v3:8 v4tree:8 cas:8 syncmap:8; do
  v=${cfg%:*} p=${cfg#*:}
  bin/invindex -in "$DIR" -v "$v" -p "$p" -warmup 0 -repeat 3 -out "$tmp/index.tsv" -cpuprofile "results/profiles/$v-p$p.pprof" 2>&1 | grep median
  go tool pprof -top -nodecount=25 "results/profiles/$v-p$p.pprof" >"results/profiles/$v-p$p.txt" 2>/dev/null
done

echo "== word frequencies and merge cost"
bin/invindex -in "$DIR" -out "$tmp/v0.tsv" -warmup 0 -repeat 1 2>/dev/null
scripts/zipf.py "$tmp/v0.tsv" >results/zipf.md
scripts/merge_cost.py "$tmp/v0.tsv" 8 >results/merge-cost.md

echo "== graphs and tables"
uv run scripts/analyze.py
conditions end
