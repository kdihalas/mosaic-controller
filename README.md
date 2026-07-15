# mosaic-controller

`mosaic-controller` compiles deployable Mosaic projects, packages, and composable bundles into deterministic Flux `ExternalArtifact` resources. Flux `kustomize-controller` remains the deployment engine and owns apply, inventory, pruning, drift correction, health assessment, retries, and deletion policy.

```text
OCIRepository or ExternalArtifact
              ↓
        MosaicRelease
              ↓
  verified deterministic archive
              ↓
       ExternalArtifact
              ↓
     Flux Kustomization
```

The controller calls `github.com/kdihalas/mosaic/pkg/build` directly. It never executes the Mosaic CLI, accesses an OCI registry, resolves packages over the network, or directly applies Kubernetes objects. Builds are always offline and locked.

## API

The API is `mosaic.toolkit.fluxcd.io/v1alpha1`, kind `MosaicRelease`. It supports Flux v1 `OCIRepository` and `ExternalArtifact` sources and Mosaic `Auto`, `Project`, `Package`, and `Bundle` input detection. `spec.variants` selects variants already baked into the source package.

External build values and imported sources are deliberately unsupported. Mosaic artifacts are the complete build input; the controller has no ConfigMap or Secret data access.

## Quick start

Enable the Flux consumer feature gate first:

```bash
kubectl -n flux-system patch deployment kustomize-controller --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--feature-gates=ExternalArtifact=true"}]'
make deploy
kubectl apply -f config/samples/mosaic_v1alpha1_mosaicrelease.yaml
```

Users create the consuming Flux `Kustomization` explicitly with `spec.sourceRef.kind: ExternalArtifact` and normally `spec.path: ./deploy`.

Local filesystem storage is suitable for the single-replica MVP. The default `emptyDir` is not durable; use the PVC overlay for production. Missing or corrupt generated archives are rebuilt from the verified Flux source artifact.

See [architecture](docs/architecture.md), [API](docs/api.md), [Flux integration](docs/flux-integration.md), and [security](docs/security.md).

## MVP limitations

The controller is single-replica for artifact serving, uses local filesystem storage, supports only `OCIRepository` and `ExternalArtifact`, and does not manage a `Kustomization`. External values and imports are intentionally absent. See the [roadmap](docs/roadmap.md).
