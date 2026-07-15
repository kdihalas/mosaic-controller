package v1alpha1

const (
	ReadyCondition             = "Ready"
	ReconcilingCondition       = "Reconciling"
	StalledCondition           = "Stalled"
	SourceReadyCondition       = "SourceReady"
	BuildReadyCondition        = "BuildReady"
	PolicyReadyCondition       = "PolicyReady"
	ArtifactInStorageCondition = "ArtifactInStorage"
)

const (
	ProgressingReason                = "Progressing"
	ProgressingWithRetryReason       = "ProgressingWithRetry"
	SucceededReason                  = "Succeeded"
	SuspendedReason                  = "Suspended"
	SourceNotFoundReason             = "SourceNotFound"
	SourceNotReadyReason             = "SourceNotReady"
	DependencyNotReadyReason         = "DependencyNotReady"
	AccessDeniedReason               = "AccessDenied"
	ArtifactDownloadFailedReason     = "ArtifactDownloadFailed"
	ArtifactVerificationFailedReason = "ArtifactVerificationFailed"
	ExtractionFailedReason           = "ExtractionFailed"
	InvalidSpecReason                = "InvalidSpec"
	BuildFailedReason                = "BuildFailed"
	PolicyFailedReason               = "PolicyFailed"
	StorageOperationFailedReason     = "StorageOperationFailed"
	ArtifactPublicationFailedReason  = "ArtifactPublicationFailed"
	GarbageCollectionFailedReason    = "GarbageCollectionFailed"
)
