package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	"github.com/kdihalas/mosaic/pkg/build"
)

const kustomization = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - resources.yaml\n"

func Stage(root string, result *build.Result, output mosaicv1.OutputSpec) error {
	if result == nil || result.Rendered == nil {
		return fmt.Errorf("build produced no Kubernetes rendering")
	}
	resources, ok := result.Rendered.Files["kubernetes.yaml"]
	if !ok {
		return fmt.Errorf("build did not produce kubernetes.yaml")
	}
	deploy := filepath.Join(root, "deploy")
	if err := os.MkdirAll(deploy, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(deploy, "kustomization.yaml"), []byte(kustomization), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(deploy, "resources.yaml"), resources, 0o600); err != nil {
		return err
	}
	if result.Bundle == nil {
		return nil
	}
	metadata := filepath.Join(root, "metadata")
	selected := map[string]bool{"bundle.json": output.IncludeBundleMetadata, "policy-report.json": output.IncludePolicyReport, "graph.json": output.IncludeGraph, "provenance.json": output.IncludeProvenance}
	names := make([]string, 0, len(selected))
	for name, include := range selected {
		if include {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	if err := os.MkdirAll(metadata, 0o750); err != nil {
		return err
	}
	for _, name := range names {
		data, ok := result.Bundle.Files[name]
		if !ok {
			continue
		}
		if err := os.WriteFile(filepath.Join(metadata, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func Kustomization() []byte { return []byte(kustomization) }
