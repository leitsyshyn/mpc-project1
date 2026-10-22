# Parallel inverted index

Course project for "Methods of Parallel Computing". Assignment: `Навчальний проєкт 1 — Паралельна побудова зворотного індексу.pdf`.

## Layout

- `cmd/invindex` — inverted index builder (versions V0–V4)
- `cmd/falseshare` — standalone false sharing experiment

## Usage

Requires Go 1.26+.

```sh
go run ./cmd/invindex -v v0 -in data -out v0.tsv
go run ./cmd/invindex -v v3 -p 8 -k 256 -in data -out v3.tsv
cmp v0.tsv v3.tsv

go run ./cmd/falseshare -p 8
```

Build optimized binaries for measurements (Go optimizes by default):

```sh
go build -o bin/ ./cmd/...
```

## Checks

```sh
gofmt -l .
go vet ./...
go test -race ./...
```
