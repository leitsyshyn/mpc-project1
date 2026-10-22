#!/usr/bin/env bash
# Runs the two control experiments of the V3 analysis and appends their
# timings to results.csv:
#   v0-notop20, v3-notop20   V0 and V3 on a copy of the corpus without the 20
#                            most frequent words, for every P in $THREADS
#   v3-padded, v3-unpadded   V3 at P=8 with stripes padded to a cache line and
#                            without padding, for every K in $KS
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

bin/invindex -in "$dir" -out "$tmp/v0.tsv" -warmup 0 -repeat 1 2>/dev/null
top20=$(sort -t$'\t' -k2,2nr "$tmp/v0.tsv" | awk -F'\t' 'NR <= 20 { print $1 }' | paste -sd'|' -)
mkdir "$tmp/corpus"
for f in "$dir"/*.txt; do
  perl -pe "s/\\b($top20)\\b//gi" "$f" >"$tmp/corpus/$(basename "$f")"
done

run() { "$@" -out "$tmp/index.tsv" 2>&1 | grep median; }
append() {
  [[ -s results.csv ]] || head -1 "$1" >results.csv
  tail -n +2 "$1" | awk -F, -v OFS=, -v label="$2" '{ $2 = label; print }' >>results.csv
}

[[ -f results.csv ]] && sed -i '' -e '/,v[03]-notop20,/d' -e '/,v3-padded,/d' -e '/,v3-unpadded,/d' results.csv

run bin/invindex -in "$tmp/corpus" -v v0 -csv "$tmp/v0.csv"
append "$tmp/v0.csv" v0-notop20
for p in $threads; do
  run bin/invindex -in "$tmp/corpus" -v v3 -p "$p" -csv "$tmp/notop20.csv"
done
append "$tmp/notop20.csv" v3-notop20
for k in $ks; do
  run bin/invindex -in "$dir" -v v3 -p 8 -k "$k" -csv "$tmp/padded.csv"
  run "$tmp/unpadded" -in "$dir" -v v3 -p 8 -k "$k" -csv "$tmp/unpadded.csv"
done
append "$tmp/padded.csv" v3-padded
append "$tmp/unpadded.csv" v3-unpadded
