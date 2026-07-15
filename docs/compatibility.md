# Compatibility

| Component | Pinned version |
|---|---|
| mosaic-controller | pre-alpha |
| Mosaic compiler | `d5fdeb1698eb55f0b73b6a03a9349d71af788b9b` |
| Flux CLI/controllers | 2.9.x / source API 1.9.3 |
| Kubernetes libraries | 0.36.2 |
| Kubernetes clusters | 1.33–1.36 target |
| Go | 1.26.5 |
| controller-runtime | 0.24.1 |
| Flux Artifact SDK | 0.20.0 |
| Flux runtime SDK | 0.111.0 |

The Flux `ExternalArtifact` consumer feature gate is required. API and compiler compatibility must be reviewed before every dependency update.
