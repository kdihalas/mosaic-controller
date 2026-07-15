# Development

Go and tools are pinned in `go.mod` and `Makefile`. Run `make generate manifests test vet test-race`. The controller imports Mosaic directly and must never add an `os/exec` path for the CLI or a network package resolver.
