# Changelog

## [Unreleased]

## [0.2.0] - 2026-11-07

### Breaking

- See repository diff for API changes in this release (secure defaults, renamed packages where applicable, context-aware constructors).

### Security

- Hardened defaults: TLS, timeouts, body limits, PII redaction where logging applies.

### Fixed

- Bug fixes from the v0.2.0 hardening pass.

### Changed

- CI uses go-version-file; added race, staticcheck, govulncheck, golangci-lint (gosec).



### Changed

- **Breaking:** `NewClient` and `NewClientWithTransport` take a `context.Context` first.
- **Breaking:** `NewIndex` returns `(IndexAPI[T], error)` and fails with `ErrUnsupportedClient` instead of returning a broken handle.
- `Config.Timeout` is now enforced on every HTTP attempt (and the default transport dial/TLS handshake).
- `hits.total` is parsed as `{value, relation}` (bare numbers accepted); `PaginatedResult.TotalRelation` added.
- Response bodies are read through `io.LimitReader` and read errors are surfaced.
- `net.Error.Temporary()` is no longer consulted for retries.
- Retry helper uses `time.NewTimer` + `Stop`, `math/rand/v2`, and honours ctx.

## [0.1.0] - 2026-11-07

### Added

- `NewClient` and `NewClientWithTransport` with HTTP retry
- Generic `Index[T]` for index, delete, keyword search, and hybrid search (bool + kNN + RRF)
- Query helpers (`BoolQuery`, `MultiMatch`, `Term`, `DefaultRRF`)
- English README, examples, and package documentation
