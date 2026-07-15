package controller

import (
	"testing"
	"time"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	release := &mosaicv1.MosaicRelease{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team"}, Spec: mosaicv1.MosaicReleaseSpec{Interval: metav1.Duration{Duration: time.Minute}, Environment: "prod", Path: "./app", SourceRef: mosaicv1.SourceReference{Kind: "OCIRepository", Name: "app"}}}
	if err := Validate(release, true); err != nil {
		t.Fatal(err)
	}
	release.Spec.Path = "../escape"
	if err := Validate(release, true); err == nil {
		t.Fatal("expected traversal rejection")
	}
	release.Spec.Path = "."
	release.Spec.SourceRef.Namespace = "other"
	if err := Validate(release, true); err == nil {
		t.Fatal("expected cross-namespace rejection")
	}
}

func TestDefaultsPreserveVariantSelection(t *testing.T) {
	release := &mosaicv1.MosaicRelease{ObjectMeta: metav1.ObjectMeta{Name: "app"}, Spec: mosaicv1.MosaicReleaseSpec{Interval: metav1.Duration{Duration: 10 * time.Minute}, Variants: []string{"production"}}}
	got := Defaults(release)
	if len(got.Variants) != 1 || got.Variants[0] != "production" {
		t.Fatalf("variants lost: %#v", got.Variants)
	}
	if got.Path != "./" || got.InputKind != mosaicv1.InputKindAuto || got.Policy.FailureMode != mosaicv1.PolicyFailureModeFail {
		t.Fatalf("defaults missing: %#v", got)
	}
}
