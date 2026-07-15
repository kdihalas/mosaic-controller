# Operations

Run one replica with leader election. Use a PVC for artifacts that should survive pod replacement; otherwise reconciliation rebuilds missing files from Flux sources. Multi-replica artifact serving and distributed storage are outside the MVP.

Normal reconciles use interval jitter; transient dependencies use `retryInterval`. `/healthz` checks the manager process and `/readyz` requires the leader-owned artifact server to be serving.
