package revision

import (
	"testing"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
)

func FuzzCalculateDeterministic(f *testing.F) {
	f.Add("prod", "variant", "sha256:abc")
	f.Fuzz(func(t *testing.T, environment, variant, digest string) {
		in := Input{Spec: Spec(mosaicv1.MosaicReleaseSpec{Environment: environment, Variants: []string{variant}}), Source: Source{Digest: digest}, CompilerVersion: "test"}
		a, errA := Calculate(environment, in)
		b, errB := Calculate(environment, in)
		if (errA == nil) != (errB == nil) || a != b {
			t.Fatalf("non-deterministic: %q/%v != %q/%v", a, errA, b, errB)
		}
	})
}
