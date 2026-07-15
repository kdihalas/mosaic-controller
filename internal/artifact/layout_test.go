package artifact

import (
	"os"
	"path/filepath"
	"testing"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	"github.com/kdihalas/mosaic/pkg/build"
	"github.com/kdihalas/mosaic/pkg/bundle"
	"github.com/kdihalas/mosaic/pkg/renderer"
)

func TestStageStableLayout(t *testing.T) {
	root := t.TempDir()
	result := &build.Result{Rendered: &renderer.ArtifactSet{Files: map[string][]byte{"kubernetes.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n")}}, Bundle: &bundle.Bundle{Files: map[string][]byte{"bundle.json": []byte("{}\n"), "graph.json": []byte("secret\n"), "policy-report.json": []byte("{\"results\":[]}\n")}}}
	if err := Stage(root, result, mosaicv1.OutputSpec{IncludeBundleMetadata: true, IncludePolicyReport: true}); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"deploy/kustomization.yaml", "deploy/resources.yaml", "metadata/bundle.json", "metadata/policy-report.json"} {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Errorf("missing %s: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "metadata/graph.json")); !os.IsNotExist(err) {
		t.Fatal("graph must be omitted by default")
	}
	got, err := os.ReadFile(filepath.Join(root, "deploy/kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(Kustomization()) {
		t.Fatalf("unexpected kustomization:\n%s", got)
	}
}
