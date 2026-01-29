# Repository Guidelines

## Project Structure & Module Organization
- `cmd/gtm/`: CLI entrypoint for gastown-micro.
- `README.md`: usage and behavior overview.
- `Makefile`: build shortcut.

## Build, Test, and Development Commands
- `make build`: builds the `gtm` binary.
- `go build ./cmd/gtm`: direct build without Makefile.
- `gofmt -w cmd/gtm/main.go`: format the CLI source.

## Coding Style & Naming Conventions
- Go standard formatting and idioms (gofmt, minimal abstractions).
- Keep CLI flags explicit and stable; prefer small helper functions.

## Testing Guidelines
- No automated tests yet; add small unit tests if logic grows (e.g., hook merge).

## Commit & PR Guidelines
- Keep commits small and descriptive.
- Note any behavior changes to hook merging or tmux handling.

