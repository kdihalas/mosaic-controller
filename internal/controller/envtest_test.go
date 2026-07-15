//go:build envtest

package controller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestMosaicReleaseCRDDefaultsAndValidation(t *testing.T) {
	environment := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
	config, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := mosaicv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "envtest"}}
	if err := c.Create(ctx, namespace); err != nil {
		t.Fatal(err)
	}
	release := &mosaicv1.MosaicRelease{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace.Name}, Spec: mosaicv1.MosaicReleaseSpec{Interval: metav1.Duration{Duration: time.Minute}, Environment: "prod", SourceRef: mosaicv1.SourceReference{Kind: "OCIRepository", Name: "source"}}}
	if err := c.Create(ctx, release); err != nil {
		t.Fatal(err)
	}
	var stored mosaicv1.MosaicRelease
	if err := c.Get(ctx, client.ObjectKeyFromObject(release), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Spec.InputKind != mosaicv1.InputKindAuto || stored.Spec.Path != "./" || stored.Spec.Policy.FailureMode != mosaicv1.PolicyFailureModeFail {
		t.Fatalf("CRD defaults not applied: %#v", stored.Spec)
	}
	invalid := release.DeepCopy()
	invalid.ResourceVersion = ""
	invalid.UID = ""
	invalid.Name = "invalid"
	invalid.Spec.Path = "../escape"
	if err := c.Create(ctx, invalid); err == nil || !apierrors.IsInvalid(err) {
		t.Fatalf("expected Invalid for traversal, got %v", err)
	}
}
