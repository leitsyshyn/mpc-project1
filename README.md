# Parallel inverted index

Course project 1 for "Methods of Parallel Computing" (KNU, FCSC): building an
inverted index with different synchronization strategies and measuring their
effect on performance. The report is in [`report/report.md`](report/report.md).

## Requirements

- Go 1.26+ (no third-party modules)
- `curl`, `unzip`, `iconv` to download the corpus
- Python 3 for `scripts/zipf.py` and `scripts/merge_cost.py`; [`uv`](https://docs.astral.sh/uv/) for `scripts/analyze.py` (fetches matplotlib)

## Layout

| Path              | Contents                                                     |
| ----------------- | ------------------------------------------------------------ |
| `cmd/invindex`    | index builder CLI: loads the corpus, times the build, writes the index |
| `cmd/falseshare`  | standalone false sharing experiment                          |
| `internal/index`  | tokenizer, corpus loader, index, output format, versions     |
| `internal/bench`  | measurement methodology: warmup, repeats, median, `results.csv` |
| `scripts/`        | corpus download and experiment scripts                       |
| `results/`        | measurement results                                          |
| `report/`         | report                                                       |

## Corpus

```sh
scripts/corpus.sh data 220 200   # ≥220 MB and ≥200 English UTF-8 books from Project Gutenberg
```

Documents are numbered from 0 in lexical path order (`docN` is the N-th file
under the corpus directory; hidden files are skipped).

## Build

Go optimizes and inlines by default, so a plain build is the optimized
("release") build. Measurements must not use `-race` or `-gcflags=-N -l`.

```sh
go build -trimpath -o bin/ ./cmd/...
```

## Run

```sh
bin/invindex -v v0 -in data -out v0.tsv -csv results.csv
```

| Flag      | Default     | Meaning                                         |
| --------- | ----------- | ----------------------------------------------- |
| `-v`      | `v0`        | version: `v0` sequential, `v1` unsynchronized, `v2` one mutex, `v3` K mutexes, `v4seq` / `v4tree` local indexes merged sequentially / as a tree, `cas` lock-free (compare-and-swap), `syncmap` standard `sync.Map` |
| `-p`      | `1`         | number of worker goroutines (ignored by `v0`)   |
| `-k`      | `1024`      | number of locks K (`v3` only)                   |
| `-in`     | `data`      | corpus directory                                |
| `-out`    | `index.tsv` | output file                                     |
| `-warmup` | `1`         | untimed runs before measuring                   |
| `-repeat` | `5`         | timed runs                                      |
| `-csv`    | none        | append per-run timings to this CSV file         |
| `-cpuprofile` | none    | write a CPU profile of the timed runs (`go tool pprof`) |
| `-tokenizer` | `swar` | `swar` (`FastWords`, SWAR-optimized) or `std` (`Words`, standard library); a non-default tokenizer is recorded as e.g. `v0-std` |

V4 also reports its merge phase, recorded in the CSV as `v4seq-merge` /
`v4tree-merge`. The tool also prints the total number of occurrences, which is how V1's lost
updates are counted.

## Experiments

All measurements of the report, from scratch, in one command (about 25–30
minutes). Quit other applications, connect the charger and keep the lid open;
the script refuses to run on battery and keeps the Mac awake with `caffeinate`:

```sh
scripts/final.sh            # log in results/final.log
```

It runs the following steps, which can also be run one by one:

```sh
scripts/bench.sh v0 v2 v3 v4seq v4tree cas syncmap k std    # scaling over P, V3 over K, Words vs FastWords; appended to results.csv
scripts/v1-lost-updates.sh 2 4 8 10 > results/v1-lost-updates.csv   # 10 V1 runs per thread count vs V0
bin/invindex -out v0.tsv && scripts/zipf.py v0.tsv                  # top-20 words' share of updates, per-K lock load
scripts/merge_cost.py v0.tsv 8              # work per step of V4's two merge schemes
bin/falseshare -csv results.csv            # false sharing: strides 8..256 B, P = 1, 2, 4, 8
bin/falseshare -n 200000000 -stride 8,32,40,64,96,128,256 -csv results/falseshare-200M.csv
uv run scripts/analyze.py                   # graphs into report/, tables into results/tables.md
```

`bench.sh` compares every output with V0 byte for byte and discards the
timings of a mismatching run. Close other applications before measuring.

The corpus is read into memory before timing, and the index is written after
it. Only the build is timed. The tool prints the median, min and max to stderr.

Output format, one line per word, sorted by UTF-8 bytes; postings by descending
count, then ascending document number:

```
word<TAB>total<TAB>doc17:12,doc3:5,doc41:1
```

### `falseshare` flags

| Flag      | Default                 | Meaning                                       |
| --------- | ----------------------- | --------------------------------------------- |
| `-p`      | `1,2,4,8`               | numbers of goroutines                         |
| `-stride` | `8,16,32,40,64,128,256` | distances between counters, bytes; 8 is implementation A, 64 is B |
| `-n`      | `10000000`              | increments per goroutine                      |
| `-warmup`, `-repeat`, `-csv` | `1`, `5`, none | as for `invindex`                 |

## Tests

```sh
go test ./...                                          # unit and end-to-end tests
go test -race ./...                                    # with the race detector (skips V1's race tests)
go test ./internal/index -run '^$' -fuzz FuzzWords     # fuzz both tokenizers against a reference
go test ./internal/index -run '^$' -bench Words        # tokenizer throughput, also on data/ if present
go vet ./... && gofmt -l .
```
