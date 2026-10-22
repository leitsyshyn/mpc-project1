# Parallel inverted index

Course project 1 for "Methods of Parallel Computing" (KNU, FCSC): building an
inverted index with different synchronization strategies and measuring their
effect on performance. The report is [`report/report.pdf`](report/report.pdf);
all timings are in [`results.csv`](results.csv).

## Requirements

- Go 1.26+ (no third-party modules)
- `curl`, `unzip`, `iconv` to download the corpus
- macOS for `scripts/final.sh` (uses `caffeinate`, `pmset`, `sysctl`)

## Layout

| Path             | Contents                                                          |
| ---------------- | ----------------------------------------------------------------- |
| `cmd/invindex`   | index builder: loads the corpus, times the build, writes the index |
| `cmd/falseshare` | false sharing experiment                                          |
| `internal/index` | tokenizer, corpus loader, index, output format, all versions      |
| `internal/bench` | measurement: warmup, repeats, median, `results.csv` rows          |
| `scripts/`       | corpus download and measurement scripts                           |

## Corpus

The corpus is not part of the submission. Download it with:

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
bin/invindex -v v3 -p 8 -k 1024 -in data -out index.tsv -csv results.csv
```

| Flag          | Default     | Meaning                                                    |
| ------------- | ----------- | ---------------------------------------------------------- |
| `-v`          | `v0`        | version: `v0` sequential, `v1` unsynchronized, `v2` one mutex, `v3` K mutexes, `v4seq` / `v4tree` local indexes merged sequentially / as a tree, `cas` lock-free (compare-and-swap), `syncmap` `sync.Map` |
| `-p`          | `1`         | number of goroutines (ignored by `v0`)                     |
| `-k`          | `1024`      | number of locks K (`v3` only)                              |
| `-tokenizer`  | `swar`      | `swar` (`FastWords`) or `std` (`Words`, standard library)  |
| `-in`         | `data`      | corpus directory                                           |
| `-out`        | `index.tsv` | output file                                                |
| `-warmup`     | `1`         | untimed runs before measuring                              |
| `-repeat`     | `5`         | timed runs                                                 |
| `-csv`        | none        | append per-run timings to this CSV file                    |
| `-cpuprofile` | none        | write a CPU profile of the runs (`go tool pprof`)          |

The corpus is read into memory before timing and the index is written after
it; only the build is timed. The tool prints the median, min and max time, the
V4 merge time and the total number of occurrences (how V1's lost updates are
counted).

Output format, one line per word, sorted by UTF-8 bytes; postings by descending
count, then ascending document number:

```
word<TAB>total<TAB>doc17:12,doc3:5,doc41:1
```

```sh
bin/falseshare -p 1,2,4,8 -stride 8,64,128 -n 10000000 -csv results.csv
```

| Flag      | Default                 | Meaning                                                  |
| --------- | ----------------------- | -------------------------------------------------------- |
| `-p`      | `1,2,4,8`               | numbers of goroutines                                    |
| `-stride` | `8,16,32,40,64,128,256` | distances between counters, bytes; 8 is implementation A, 64 is B |
| `-n`      | `10000000`              | increments per goroutine                                 |
| `-warmup`, `-repeat`, `-csv` | `1`, `5`, none | as for `invindex`                           |

## Measurements

All measurements of the report, from scratch, in one command (about 25 minutes
on an M1 Max). Quit other applications, connect the charger and keep the lid
open; the script refuses to run on battery and keeps the Mac awake:

```sh
scripts/final.sh            # log in results/final.log
```

It runs:

```sh
scripts/bench.sh v0 v2 v3 v4seq v4tree cas syncmap k std   # scaling over P, V3 over K, Words vs FastWords
bin/falseshare -csv results.csv                           # false sharing, 10 million increments
bin/falseshare -n 200000000 ...                           # false sharing, 200 million increments
scripts/v1-lost-updates.sh 2 4 8 10                       # 10 V1 runs per thread count vs V0
scripts/supplementary.sh                                  # V3 control experiments
```

`bench.sh` compares every output with V0 byte for byte and discards the
timings of a mismatching run.

`results.csv` has the columns `program, version, threads, k, run, seconds`.
Besides the version names, it uses these labels:

| Label                    | Measurement                                              |
| ------------------------ | -------------------------------------------------------- |
| `v0-std`, `v4tree-std`   | with the standard-library tokenizer `Words`              |
| `v4seq-merge`, `v4tree-merge` | V4 merge phase alone                                |
| `stride-N`               | false sharing, counters N bytes apart, 10 million increments |
| `stride-N-200M`          | the same, 200 million increments                         |
| `v0-notop20`, `v3-notop20` | corpus without the 20 most frequent words              |
| `v3-padded`, `v3-unpadded` | V3 at P=8 with and without cache-line padding          |

## Tests

```sh
go test ./...                                          # unit and end-to-end tests
go test -race ./...                                    # with the race detector (skips V1's race tests)
go test ./internal/index -run '^$' -fuzz FuzzWords     # fuzz both tokenizers against a reference
go vet ./... && gofmt -l .
```
