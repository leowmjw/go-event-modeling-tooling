# go-event-modeling-tooling

Idiomatic Go port of Event Modeling tooling with:

- EVML parser that builds an AST for Event Modeling constructs
- SVG renderer for `.evml` models
- `evml` CLI with `svg` output

## Usage

```bash
go run ./cmd/evml svg /absolute/path/to/model.evml
```

To choose a destination directory:

```bash
go run ./cmd/evml svg /absolute/path/to/model.evml -d out
```
