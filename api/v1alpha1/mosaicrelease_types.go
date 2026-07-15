package v1alpha1

import (
	meta "github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InputKind identifies the Mosaic source layout.
// +kubebuilder:validation:Enum=Auto;Project;Package;Bundle
type InputKind string

const (
	InputKindAuto    InputKind = "Auto"
	InputKindProject InputKind = "Project"
	InputKindPackage InputKind = "Package"
	InputKindBundle  InputKind = "Bundle"
)

// PolicyFailureMode controls handling of downgradeable policy violations.
// +kubebuilder:validation:Enum=Fail;Warn
type PolicyFailureMode string

const (
	PolicyFailureModeFail PolicyFailureMode = "Fail"
	PolicyFailureModeWarn PolicyFailureMode = "Warn"
)

// SourceReference identifies a Flux source artifact.
type SourceReference struct {
	// +kubebuilder:validation:Enum=source.toolkit.fluxcd.io/v1
	APIVersion string `json:"apiVersion,omitempty"`
	// +kubebuilder:validation:Enum=OCIRepository;ExternalArtifact
	Kind string `json:"kind"`
	// +kubebuilder:validation:MinLength=1
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// PolicySpec selects Mosaic policies and their permitted failure behavior.
type PolicySpec struct {
	// +kubebuilder:default=Fail
	FailureMode PolicyFailureMode `json:"failureMode,omitempty"`
	// +listType=set
	// +kubebuilder:validation:MaxItems=128
	Include []string `json:"include,omitempty"`
	// +listType=set
	// +kubebuilder:validation:MaxItems=128
	Exclude []string `json:"exclude,omitempty"`
}

// OutputSpec controls files exposed in the generated artifact.
type OutputSpec struct {
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	ExternalArtifactName string `json:"externalArtifactName,omitempty"`
	// +kubebuilder:default=true
	IncludeBundleMetadata bool `json:"includeBundleMetadata,omitempty"`
	IncludeGraph          bool `json:"includeGraph,omitempty"`
	IncludeProvenance     bool `json:"includeProvenance,omitempty"`
	// +kubebuilder:default=true
	IncludePolicyReport bool `json:"includePolicyReport,omitempty"`
}

// MosaicReleaseSpec describes a deterministic compiled Mosaic artifact.
type MosaicReleaseSpec struct {
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')",message="interval must be positive"
	Interval metav1.Duration `json:"interval"`
	// +optional
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')",message="retryInterval must be positive"
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`
	// +optional
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')",message="timeout must be positive"
	Timeout   *metav1.Duration `json:"timeout,omitempty"`
	SourceRef SourceReference  `json:"sourceRef"`
	// +kubebuilder:default="./"
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('/') && !self.contains('\\\\') && !self.matches('^[A-Za-z]:.*') && self.split('/').all(p, p != '..')",message="path must be a portable relative path without parent traversal"
	Path string `json:"path,omitempty"`
	// +kubebuilder:default=Auto
	InputKind InputKind `json:"inputKind,omitempty"`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Environment string `json:"environment"`
	// Variants selects variants already present in the source artifact. External
	// values and imported variant sources are intentionally unsupported.
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	Variants []string   `json:"variants,omitempty"`
	Policy   PolicySpec `json:"policy,omitempty"`
	Output   OutputSpec `json:"output,omitempty"`
	Suspend  bool       `json:"suspend,omitempty"`
}

type ResourceSummary struct {
	Total int32            `json:"total,omitempty"`
	Kinds map[string]int32 `json:"kinds,omitempty"`
}

type PolicySummary struct {
	Evaluated  int32 `json:"evaluated,omitempty"`
	Warnings   int32 `json:"warnings,omitempty"`
	Violations int32 `json:"violations,omitempty"`
}

// MosaicReleaseStatus records bounded reconciliation state.
type MosaicReleaseStatus struct {
	ObservedGeneration     int64                        `json:"observedGeneration,omitempty"`
	Conditions             []metav1.Condition           `json:"conditions,omitempty"`
	Artifact               *meta.Artifact               `json:"artifact,omitempty"`
	ExternalArtifactRef    *corev1.LocalObjectReference `json:"externalArtifactRef,omitempty"`
	LastAttemptedRevision  string                       `json:"lastAttemptedRevision,omitempty"`
	LastSuccessfulRevision string                       `json:"lastSuccessfulRevision,omitempty"`
	LastHandledReconcileAt string                       `json:"lastHandledReconcileAt,omitempty"`
	ObservedSourceRevision string                       `json:"observedSourceRevision,omitempty"`
	ResourceSummary        *ResourceSummary             `json:"resourceSummary,omitempty"`
	PolicySummary          *PolicySummary               `json:"policySummary,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mr,categories=flux
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`
// +kubebuilder:printcolumn:name="Environment",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceRef.name`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.lastSuccessfulRevision`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type MosaicRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MosaicReleaseSpec   `json:"spec,omitempty"`
	Status            MosaicReleaseStatus `json:"status,omitempty"`
}

func (in *MosaicRelease) GetConditions() []metav1.Condition  { return in.Status.Conditions }
func (in *MosaicRelease) SetConditions(c []metav1.Condition) { in.Status.Conditions = c }
func (in *MosaicRelease) GetRequeueAfter() metav1.Duration   { return in.Spec.Interval }

// +kubebuilder:object:root=true
type MosaicReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MosaicRelease `json:"items"`
}

func init() { SchemeBuilder.Register(&MosaicRelease{}, &MosaicReleaseList{}) }
