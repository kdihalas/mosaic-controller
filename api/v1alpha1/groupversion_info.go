// Package v1alpha1 contains API Schema definitions for the mosaic v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=mosaic.toolkit.fluxcd.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "mosaic.toolkit.fluxcd.io", Version: "v1alpha1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)
