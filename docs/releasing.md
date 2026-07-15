# Releasing

Release images are built with `ko` for linux/amd64 and linux/arm64. Release automation must publish immutable digests, generate an SBOM, scan, sign with Cosign using GitHub OIDC, and attach provenance and versioned installation manifests. Credentials must never be printed.
