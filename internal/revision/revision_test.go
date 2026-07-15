package revision

import (
	"testing"
	"time"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCalculateDeterministicAndVariantSensitive(t *testing.T) {
	base := Input{Spec: Spec(mosaicv1.MosaicReleaseSpec{Interval: metav1.Duration{Duration: time.Minute}, Environment: "prod", Variants: []string{"restricted"}}), Source: Source{Revision: "v1", Digest: "sha256:abc"}, CompilerVersion: "test"}
	a, err := Calculate("prod", base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Calculate("prod", base)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("revision is not deterministic: %q != %q", a, b)
	}
	base.Spec.Variants = []string{"baseline"}
	c, err := Calculate("prod", base)
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("variant selection did not affect revision")
	}
}

func TestOperationalFieldsDoNotAffectRevision(t *testing.T) {
	a := mosaicv1.MosaicReleaseSpec{Interval: metav1.Duration{Duration: time.Minute}, Environment: "prod"}
	b := a
	b.Interval.Duration = time.Hour
	b.Suspend = true
	left, _ := Calculate("prod", Input{Spec: Spec(a), Source: Source{Digest: "sha256:a"}})
	right, _ := Calculate("prod", Input{Spec: Spec(b), Source: Source{Digest: "sha256:a"}})
	if left != right {
		t.Fatalf("operational fields changed revision: %s != %s", left, right)
	}
}
