# Security

Sources are untrusted. Downloads use bounded streaming, explicit timeouts, digest verification, limited redirects, and TLS verification. Extraction rejects absolute/traversing/Windows paths, links, devices, pipes, sockets, duplicate or case-fold-colliding names, and bounded-size/count/depth violations.

Cross-namespace references are denied by default. The controller reads no Secrets or ConfigMaps. The deployment is non-root, drops all capabilities, uses RuntimeDefault seccomp and a read-only root filesystem, and exposes writable storage and temporary volumes only.

The restricted multitenant overlay permits DNS, Flux artifact services, and RFC1918 Kubernetes API endpoints only. Adapt its API CIDR and DNS labels to the target cluster before rollout; it intentionally has no general internet egress.
