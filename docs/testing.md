# Testing

Unit tests cover path validation, defaults, variant-sensitive canonical revisions, digest verification, secure extraction, and artifact layout. `make test-race` enables the race detector. The Kind E2E entrypoint checks its pinned prerequisites before creating a local-registry Flux environment.
