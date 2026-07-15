# Architecture

The controller is an artifact producer, not a second Kubernetes deployment engine. Reconciliation validates and authorizes the source, resolves its ready immutable artifact, calculates a canonical input revision, downloads and verifies it, safely extracts it, invokes Mosaic's public offline/locked build API, stages deterministic Kubernetes output, archives it through the Flux Artifact SDK, verifies storage, and publishes a child `ExternalArtifact`.

Each generated archive contains `deploy/kustomization.yaml`, `deploy/resources.yaml`, and enabled files under `metadata/`. Archive content has stable names and ordering and contains no wall-clock or temporary-path inputs.

The Mosaic compiler dependency is pinned to commit `d5fdeb1698eb55f0b73b6a03a9349d71af788b9b`.
