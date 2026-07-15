# Multitenancy

`--no-cross-namespace-refs=true` is the secure default. The two supported Flux source kinds do not currently expose a compatible per-object `AccessFrom` ACL, so cross-namespace references remain denied even when the flag is disabled. This fail-closed behavior can be relaxed only after an explicit, auditable ACL is available. Secret access is impossible because external values do not exist.
