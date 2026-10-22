#!/usr/bin/env bash
# Runs all measurements of the report from scratch (about 25 minutes on an
# M1 Max) and writes the timings to results.csv. Quit other applications first
# and keep the Mac on power with the lid open; caffeinate keeps it awake.
#
# Usage: scripts/final.sh        (log: results/final.log)
# Env:   DIR (corpus, default: data); ALLOW_BATTERY=1 skips the power check (for test runs only)
set -euo pipefail
cd "$(dirname "$0")/.."
[[ -n ${CAFFEINATED:-} ]] || exec env CAFFEINATED=1 caffeinate -i "$0" "$@"

export DIR=${DIR:-data} LC_ALL=C
[[ -n ${ALLOW_BATTERY:-} ]] || pmset -g batt | grep -q "AC Power" || { echo "connect the charger first" >&2; exit 1; }
mkdir -p results
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

rm -f results.csv results/v1-lost-updates.csv
go build -trimpath -o bin/ ./cmd/...
conditions start

echo "== index versions"
scripts/bench.sh v0 v2 v3 v4seq v4tree cas syncmap k std

echo "== false sharing"
bin/falseshare -csv results.csv
bin/falseshare -n 200000000 -stride 8,32,40,64,96,128,256 -csv "$tmp/falseshare.csv"
tail -n +2 "$tmp/falseshare.csv" | awk -F, -v OFS=, '{ $2 = $2 "-200M"; print }' >>results.csv

echo "== V1 lost updates"
scripts/v1-lost-updates.sh 2 4 8 10 >results/v1-lost-updates.csv

echo "== control experiments"
scripts/supplementary.sh
conditions end
