package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
)

const ArtifactFormatVersion = "v1alpha1"

type Source struct {
	Revision string `json:"revision"`
	Digest   string `json:"digest"`
}

type Input struct {
	Spec                  BuildSpec `json:"spec"`
	Source                Source    `json:"source"`
	CompilerVersion       string    `json:"compilerVersion"`
	LanguageVersion       string    `json:"languageVersion,omitempty"`
	ArtifactFormatVersion string    `json:"artifactFormatVersion"`
}

// BuildSpec contains only fields that can affect artifact bytes or build meaning.
// Operational scheduling and suspension fields deliberately do not affect revisions.
type BuildSpec struct {
	SourceRef   mosaicv1.SourceReference `json:"sourceRef"`
	Path        string                   `json:"path"`
	InputKind   mosaicv1.InputKind       `json:"inputKind"`
	Environment string                   `json:"environment"`
	Variants    []string                 `json:"variants,omitempty"`
	Policy      mosaicv1.PolicySpec      `json:"policy"`
	Output      mosaicv1.OutputSpec      `json:"output"`
}

func Spec(in mosaicv1.MosaicReleaseSpec) BuildSpec {
	return BuildSpec{SourceRef: in.SourceRef, Path: in.Path, InputKind: in.InputKind, Environment: in.Environment, Variants: append([]string(nil), in.Variants...), Policy: in.Policy, Output: in.Output}
}

func Calculate(environment string, in Input) (string, error) {
	if in.ArtifactFormatVersion == "" {
		in.ArtifactFormatVersion = ArtifactFormatVersion
	}
	b, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("canonicalize revision input: %w", err)
	}
	sum := sha256.Sum256(b)
	return environment + "@sha256:" + hex.EncodeToString(sum[:]), nil
}
