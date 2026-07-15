# Sources

Supported sources are `source.toolkit.fluxcd.io/v1` `OCIRepository` and `ExternalArtifact`. The source must have `Ready=True` and a non-empty revision, digest, and URL. The reported bytes are streamed with a compressed-size limit and verified before extraction.

Flux source-controller owns OCI authentication, tag selection, polling, and signature verification. The controller has no registry client.
