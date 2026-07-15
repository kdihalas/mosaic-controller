# API

`MosaicRelease` is namespaced and uses `mosaic.toolkit.fluxcd.io/v1alpha1`. Its desired state includes interval, retry interval, timeout, one Flux source reference, a relative source path, input kind, required environment, baked-in variant names, baked-in policy selection, metadata output controls, and suspension.

There are intentionally no external values, `valuesFrom`, or imports. `spec.variants` is ordered and participates in the input revision.

Status is bounded: conditions, generated artifact metadata, child reference, attempted and successful revisions, handled reconcile annotation, observed source revision, and summaries. It never stores source, manifests, graphs, or compiler diagnostic lists.
