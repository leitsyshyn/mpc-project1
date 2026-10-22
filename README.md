# Parallel inverted index

Course project 1 for "Methods of Parallel Computing" (KNU, FCSC): building an
inverted index with different synchronization strategies and measuring their
effect on performance. The report is in [`report/report.md`](report/report.md).

## Requirements

- Go 1.26+ (no third-party modules)
- `curl`, `unzip`, `iconv` to download the corpus

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
| `-v`      | `v0`        | version: `v0` sequential, `v1` unsynchronized   |
| `-p`      | `1`         | number of worker goroutines (ignored by `v0`)   |
| `-in`     | `data`      | corpus directory                                |
| `-out`    | `index.tsv` | output file                                     |
| `-warmup` | `1`         | untimed runs before measuring                   |
| `-repeat` | `5`         | timed runs                                      |
| `-csv`    | none        | append per-run timings to this CSV file         |

The tool also prints the total number of occurrences, which is how V1's lost
updates are counted.

```sh
scripts/v1-lost-updates.sh 2 4 8 10 > results/v1-lost-updates.csv   # 10 V1 runs per thread count vs V0
```

The corpus is read into memory before timing, and the index is written after
it. Only the build is timed. The tool prints the median, min and max to stderr.

Output format, one line per word, sorted by UTF-8 bytes; postings by descending
count, then ascending document number:

```
word<TAB>total<TAB>doc17:12,doc3:5,doc41:1
```

## Tests

```sh
go test ./...                                          # unit and end-to-end tests
go test -race ./...                                    # with the race detector (skips V1's race tests)
go test ./internal/index -run '^$' -fuzz FuzzWords     # fuzz the tokenizer
go vet ./... && gofmt -l .
```
