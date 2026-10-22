#!/usr/bin/env bash
# Downloads a natural-language corpus from Project Gutenberg via its robot
# harvest endpoint (the officially sanctioned way to fetch books in bulk).
#
# Usage: scripts/corpus.sh [dir] [min_mb] [min_files]
# Env:   LANG_CODE (default: en), MIRROR (default: the PGLAF mirror; the
#        harvest links point to aleph.gutenberg.org, whose TLS certificate is invalid)
set -euo pipefail

dir=${1:-data}
min_mb=${2:-220}
min_files=${3:-200}
lang=${LANG_CODE:-en}
mirror=${MIRROR:-https://gutenberg.pglaf.org}
harvest="https://www.gutenberg.org/robot/harvest?filetypes[]=txt&langs[]=$lang"

mkdir -p "$dir"

# fetch URL DIR: downloads one zipped book and keeps it if it is valid, non-empty UTF-8.
fetch() {
  local url=$1 dir=$2 tmp
  local out="$dir/$(basename "$(dirname "$url")").txt"
  [[ -e $out ]] && return
  tmp=$(mktemp -d)
  if curl -fsS -o "$tmp/book.zip" "$url" &&
    unzip -p "$tmp/book.zip" '*.txt' >"$tmp/book.txt" 2>/dev/null &&
    [[ -s $tmp/book.txt ]] && iconv -f UTF-8 -t UTF-8 "$tmp/book.txt" >/dev/null 2>&1; then
    mv "$tmp/book.txt" "$out"
  fi
  rm -rf "$tmp"
}
export -f fetch

enough() {
  (($(du -sm "$dir" | cut -f1) >= min_mb && $(find "$dir" -type f -name '*.txt' | wc -l) >= min_files))
}

offset=0
until enough; do
  page=$(curl -fsS "$harvest&offset=$offset")
  # One file per book: prefer UTF-8 (-0), fall back to plain ASCII; skip
  # Latin-1 (-8) since the assignment requires UTF-8 input.
  grep -oE 'https://[^"]+/[0-9]+(-0)?\.zip' <<<"$page" |
    awk -F/ '{ id=$(NF-1); if (!(id in u) || $NF ~ /-0\.zip$/) u[id]=$0 } END { for (id in u) print u[id] }' |
    sed "s#^https://[^/]*#$mirror#" |
    xargs -P 8 -I{} bash -c 'fetch "$1" "$2"' _ {} "$dir"

  offset=$(grep -oE 'offset=[0-9]+' <<<"$page" | tail -1 | cut -d= -f2)
  [[ -z $offset ]] && { echo "harvest exhausted" >&2; break; }
  sleep 2 # be polite to the harvest endpoint
done

echo "$(find "$dir" -type f -name '*.txt' | wc -l | tr -d ' ') files, $(du -sm "$dir" | cut -f1) MB in $dir"
