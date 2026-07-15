# Flux integration

Flux `kustomize-controller` must run with `--feature-gates=ExternalArtifact=true`. Verify it with:

```bash
kubectl -n flux-system get deploy kustomize-controller \
  -o jsonpath='{.spec.template.spec.containers[0].args}'
```

Create a `Kustomization` whose source is the generated `ExternalArtifact`, with `path: ./deploy`. The controller does not create or manage this object.
