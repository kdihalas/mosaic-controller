package controller

import (
	"fmt"
	"path"
	"strings"
	"time"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Defaults(in *mosaicv1.MosaicRelease) mosaicv1.MosaicReleaseSpec {
	s := *in.Spec.DeepCopy()
	if s.Path == "" {
		s.Path = "./"
	}
	if s.InputKind == "" {
		s.InputKind = mosaicv1.InputKindAuto
	}
	if s.Policy.FailureMode == "" {
		s.Policy.FailureMode = mosaicv1.PolicyFailureModeFail
	}
	if s.SourceRef.APIVersion == "" {
		s.SourceRef.APIVersion = "source.toolkit.fluxcd.io/v1"
	}
	if s.SourceRef.Namespace == "" {
		s.SourceRef.Namespace = in.Namespace
	}
	if s.RetryInterval == nil {
		d := metav1.Duration{Duration: minDuration(s.Interval.Duration/10, time.Minute)}
		if d.Duration < 5*time.Second {
			d.Duration = 5 * time.Second
		}
		s.RetryInterval = &d
	}
	if s.Timeout == nil {
		d := metav1.Duration{Duration: 2 * time.Minute}
		s.Timeout = &d
	}
	if s.Output.ExternalArtifactName == "" {
		s.Output.ExternalArtifactName = in.Name
	}
	return s
}

func Validate(release *mosaicv1.MosaicRelease, noCrossNamespace bool) error {
	s := Defaults(release)
	if s.Interval.Duration <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if s.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	if s.SourceRef.Kind != "OCIRepository" && s.SourceRef.Kind != "ExternalArtifact" {
		return fmt.Errorf("unsupported source kind %q", s.SourceRef.Kind)
	}
	if s.SourceRef.APIVersion != "source.toolkit.fluxcd.io/v1" {
		return fmt.Errorf("unsupported source API %q", s.SourceRef.APIVersion)
	}
	normalized := strings.ReplaceAll(s.Path, "\\", "/")
	if path.IsAbs(normalized) || (len(normalized) >= 2 && normalized[1] == ':') {
		return fmt.Errorf("path must be relative")
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return fmt.Errorf("path must not contain parent traversal")
		}
	}
	ns := s.SourceRef.Namespace
	if ns == "" {
		ns = release.Namespace
	}
	if ns != release.Namespace {
		if noCrossNamespace {
			return fmt.Errorf("cross-namespace source references are disabled")
		}
		return fmt.Errorf("cross-namespace source has no supported Flux ACL and is denied")
	}
	if s.SourceRef.Kind == "ExternalArtifact" && s.SourceRef.Name == s.Output.ExternalArtifactName {
		return fmt.Errorf("source ExternalArtifact must not be this release's output artifact")
	}
	return nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
