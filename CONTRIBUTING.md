# Contributing to goRDFlib

## Development Setup

```bash
git clone --recurse-submodules https://github.com/tggo/goRDFlib.git
cd goRDFlib
go test ./...
```

Requires Go 1.25+.

## Running Tests

```bash
go test ./...                          # all tests
go test ./turtle/ -run TestW3C -v      # W3C conformance for a specific parser
go test ./tests/integration/ -v        # integration tests + examples
go test -race ./...                    # race detector
go test -cover ./...                   # coverage
```

## Benchmarks

```bash
go test ./benchmarks/ -bench=. -benchmem
python3 benchmarks/bench_python.py     # Python comparison
```

## Adding a New Format

1. Create a package under the root (e.g. `trig/`)
2. Implement `Parse(g *graph.Graph, r io.Reader, opts ...Option) error`
3. Implement `Serialize(g *graph.Graph, w io.Writer, opts ...Option) error`
4. Register in `plugin/plugin.go`
5. Add W3C conformance tests if available
6. Add a `doc.go` with package description

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use functional options for configurable APIs
- Return errors, don't panic in library code
- Add godoc comments on all exported symbols
- Keep dependencies minimal

## Pull Requests

- One logical change per PR
- Include tests for new functionality
- Run `go test ./...` before submitting
- Update golden files if example output changes:
  ```bash
  go run ./examples/<name>/ > examples/<name>/output.golden
  ```
