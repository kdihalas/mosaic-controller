package compiler

import (
	"context"
	"fmt"

	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	"github.com/kdihalas/mosaic/pkg/build"
	mosaiccompiler "github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/policy"
)

type Input struct {
	RootPath    string
	InputKind   mosaicv1.InputKind
	Environment string
	Variants    []string
	Policy      mosaicv1.PolicySpec
	Limits      mosaiccompiler.Limits
}

type MosaicCompiler interface {
	Compile(context.Context, Input) (*build.Result, diagnostics.List)
}

type Adapter struct{}

func (Adapter) Compile(ctx context.Context, in Input) (*build.Result, diagnostics.List) {
	failure := policy.FailureMode(in.Policy.FailureMode)
	if failure == "" {
		failure = policy.FailureModeFail
	}
	return build.Run(ctx, build.Input{
		RootPath: in.RootPath, InputKind: build.InputKind(in.InputKind), Environment: in.Environment,
		Variants: append([]string(nil), in.Variants...),
		Policy:   policy.Options{FailureMode: failure, Include: append([]string(nil), in.Policy.Include...), Exclude: append([]string(nil), in.Policy.Exclude...)},
		Offline:  true, Locked: true, Limits: in.Limits,
	})
}

func Summary(ds diagnostics.List, max int) string {
	errors := 0
	for _, d := range ds {
		if d.Severity == diagnostics.SeverityError {
			errors++
		}
	}
	if errors == 0 {
		return ""
	}
	message := fmt.Sprintf("Mosaic compilation failed with %d error(s).", errors)
	for _, d := range ds {
		if d.Severity == diagnostics.SeverityError {
			message += " First error: " + d.Message
			break
		}
	}
	if max > 0 && len(message) > max {
		message = message[:max-1] + "…"
	}
	return message
}
