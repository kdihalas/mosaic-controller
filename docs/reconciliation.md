# Reconciliation

The flow is validate → resolve source → calculate revision → verify no-op state → download → verify digest → extract safely → load/compile/policy/render → stage → archive/verify → reconcile `ExternalArtifact` → garbage collect.

`reconcile.fluxcd.io/requestedAt` forces a complete state check. A failed new revision updates `lastAttemptedRevision` and `Ready=False` while retaining `status.artifact`, `lastSuccessfulRevision`, and the child artifact at the last valid output. Terminal input failures reconcile on source/spec events and the normal interval; temporary failures use `retryInterval`.
