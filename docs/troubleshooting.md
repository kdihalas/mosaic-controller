# Troubleshooting

Inspect `MosaicRelease` conditions and Events first. `SourceNotReady` means the Flux source has no ready immutable artifact. `ExtractionFailed` indicates unsafe or over-limit archive content. `BuildFailed` and `PolicyFailed` require a new source or spec change; the last valid child artifact remains published. `ArtifactVerificationFailed` causes regeneration rather than publication of corrupt bytes.
