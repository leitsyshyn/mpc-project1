#!/usr/bin/env bash
# Runs the two control experiments of the V3 analysis and writes their
# timings to results/:
#   v3-notop20.csv   V0 and V3 on a copy of the corpus without the 20 most
#                    frequent words, for every P in $THREADS
#   v3-unpadded.csv  V3 at P=8 with stripes padded to a cache line and without
#                    padding, for every K in $KS
#
# Usage: scripts/supplementary.sh
# Env:   DIR (corpus, default: data), THREADS (1 2 4 6 8 10), KS (16 256 1024)
set -euo pipefail
cd "$(dirname "$0")/.."
export LC_ALL=C

dir=${DIR:-data}
threads=${THREADS:-1 2 4 6 8 10}
ks=${KS:-16 256 1024}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

go build -trimpath -o bin/ ./cmd/invindex
cp -R go.mod cmd internal "$tmp"
sed -i '' -e '/_  \[cacheLine - unsafe.Sizeof/d' -e '/^	"unsafe"$/d' "$tmp/internal/index/build.go"
(cd "$tmp" && go build -trimpath -o unpadded ./cmd/invindex)

top20=$(awk '/^\| [0-9]+ \|/ && n++ < 20 { print $4 }' results/zipf.md | paste -sd'|' -)
mkdir "$tmp/corpus"
for f in "$dir"/*.txt; do
  perl -pe "s/\\b($top20)\\b//gi" "$f" >"$tmp/corpus/$(basename "$f")"
done

rm -f results/v3-notop20.csv results/v3-unpadded.csv
run() { "$@" -out "$tmp/index.tsv" 2>&1 | grep median; }

run bin/invindex -in "$tmp/corpus" -v v0 -csv results/v3-notop20.csv
for p in $threads; do
  run bin/invindex -in "$tmp/corpus" -v v3 -p "$p" -csv results/v3-notop20.csv
done
for k in $ks; do
  run bin/invindex -in "$dir" -v v3 -p 8 -k "$k" -csv results/v3-unpadded.csv
  run "$tmp/unpadded" -in "$dir" -v v3 -p 8 -k "$k" -csv "$tmp/unpadded.csv"
done
tail -n +2 "$tmp/unpadded.csv" | awk -F, -v OFS=, '{ $2 = "v3-unpadded"; print }' >>results/v3-unpadded.csv
