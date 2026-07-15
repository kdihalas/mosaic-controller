package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	meta "github.com/fluxcd/pkg/apis/meta"
	artifactstorage "github.com/fluxcd/pkg/artifact/storage"
	"github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	artifactlayout "github.com/kdihalas/mosaic-controller/internal/artifact"
	mosaiccompiler "github.com/kdihalas/mosaic-controller/internal/compiler"
	"github.com/kdihalas/mosaic-controller/internal/extraction"
	controllermetrics "github.com/kdihalas/mosaic-controller/internal/metrics"
	stageerr "github.com/kdihalas/mosaic-controller/internal/reconcile"
	"github.com/kdihalas/mosaic-controller/internal/revision"
	artifactsource "github.com/kdihalas/mosaic-controller/internal/source"
	"github.com/kdihalas/mosaic/pkg/compiler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	Finalizer                  = "finalizers.mosaic.toolkit.fluxcd.io"
	SourceIndex                = "mosaic.sourceRef"
	ReconcileRequestAnnotation = "reconcile.fluxcd.io/requestedAt"
	CompilerVersion            = "d5fdeb1698eb55f0b73b6a03a9349d71af788b9b"
)

type Limits struct {
	MaxDownloadBytes     int64
	MaxExtractedBytes    int64
	MaxFiles             int
	MaxFileBytes         int64
	MaxBuildDuration     time.Duration
	MaxDiagnostics       int
	MaxGraphResources    int
	MaxTransformOps      int
	MaxPolicyEvaluations int
}

type MosaicReleaseReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	Storage              *artifactstorage.Storage
	Downloader           *artifactsource.Downloader
	Compiler             mosaiccompiler.MosaicCompiler
	NoCrossNamespaceRefs bool
	DependencyRequeue    time.Duration
	Limits               Limits
}

func (r *MosaicReleaseReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, reconcileErr error) {
	var release mosaicv1.MosaicRelease
	if err := r.Get(ctx, request.NamespacedName, &release); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log := ctrl.LoggerFrom(ctx).WithValues("controller", "mosaicrelease", "namespace", release.Namespace, "name", release.Name, "generation", release.Generation, "reconcileID", strconv.FormatInt(time.Now().UnixNano(), 36))

	if !release.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &release)
	}
	if !containsString(release.Finalizers, Finalizer) {
		base := release.DeepCopy()
		release.Finalizers = append(release.Finalizers, Finalizer)
		if err := r.Patch(ctx, &release, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	base := release.DeepCopy()
	defer func() {
		release.Status.ObservedGeneration = release.Generation
		if err := r.Status().Patch(ctx, &release, client.MergeFrom(base)); err != nil && reconcileErr == nil {
			reconcileErr = err
		}
	}()
	spec := Defaults(&release)
	requestedAt := release.Annotations[ReconcileRequestAnnotation]
	if requestedAt != "" {
		release.Status.LastHandledReconcileAt = requestedAt
	}
	if spec.Suspend {
		conditions.Delete(&release, mosaicv1.ReconcilingCondition)
		conditions.Delete(&release, mosaicv1.StalledCondition)
		conditions.MarkTrue(&release, mosaicv1.ReadyCondition, mosaicv1.SuspendedReason, "Reconciliation is suspended")
		return ctrl.Result{}, nil
	}
	conditions.MarkReconciling(&release, mosaicv1.ProgressingReason, "Building Mosaic release")
	conditions.MarkUnknown(&release, mosaicv1.ReadyCondition, mosaicv1.ProgressingReason, "Building Mosaic release")

	if err := Validate(&release, r.NoCrossNamespaceRefs); err != nil {
		reason := mosaicv1.InvalidSpecReason
		if strings.Contains(err.Error(), "cross-namespace") {
			reason = mosaicv1.AccessDeniedReason
		}
		return r.fail(&release, spec, stageerr.Stall(reason, err))
	}
	resolved, err := artifactsource.Resolve(ctx, r.Client, release.Namespace, spec.SourceRef)
	if err != nil {
		conditions.MarkFalse(&release, mosaicv1.SourceReadyCondition, mosaicv1.SourceNotReadyReason, "%s", err)
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.SourceNotReadyReason, err))
	}
	conditions.MarkTrue(&release, mosaicv1.SourceReadyCondition, mosaicv1.SucceededReason, "Source artifact is ready")
	release.Status.ObservedSourceRevision = resolved.Artifact.Revision
	desiredRevision, err := revision.Calculate(spec.Environment, revision.Input{Spec: revision.Spec(spec), Source: revision.Source{Revision: resolved.Artifact.Revision, Digest: resolved.Artifact.Digest}, CompilerVersion: compiler.Version + "+" + CompilerVersion, LanguageVersion: compiler.LanguageVersion, ArtifactFormatVersion: revision.ArtifactFormatVersion})
	if err != nil {
		return r.fail(&release, spec, stageerr.Stall(mosaicv1.InvalidSpecReason, err))
	}
	release.Status.LastAttemptedRevision = desiredRevision
	forced := requestedAt != "" && requestedAt != base.Status.LastHandledReconcileAt
	if !forced && desiredRevision == release.Status.LastSuccessfulRevision && release.Status.Artifact != nil && r.Storage.ArtifactExist(*release.Status.Artifact) && r.Storage.VerifyArtifact(*release.Status.Artifact) == nil {
		if ok, err := r.externalArtifactCurrent(ctx, &release, spec.Output.ExternalArtifactName); err == nil && ok {
			controllermetrics.MarkActive(string(release.UID))
			r.markSuccess(&release)
			return ctrl.Result{RequeueAfter: jitter(spec.Interval.Duration, release.UID)}, nil
		}
	}

	buildCtx, cancel := context.WithTimeout(ctx, minDuration(spec.Timeout.Duration, r.Limits.MaxBuildDuration))
	defer cancel()
	work, err := os.MkdirTemp("", "mosaic-release-")
	if err != nil {
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.StorageOperationFailedReason, err))
	}
	defer func() {
		if err := os.RemoveAll(work); err != nil {
			log.Error(err, "remove reconciliation work directory")
		}
	}()
	archivePath := filepath.Join(work, "source.tar.gz")
	downloadStarted := time.Now()
	downloadBytes, downloadErr := r.Downloader.Download(buildCtx, resolved.Artifact.URL, resolved.Artifact.Digest, archivePath)
	controllermetrics.SourceDownloadDuration.Observe(time.Since(downloadStarted).Seconds())
	if downloadBytes > 0 {
		controllermetrics.SourceDownloadBytes.Add(float64(downloadBytes))
	}
	if downloadErr != nil {
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.ArtifactDownloadFailedReason, downloadErr))
	}
	extracted := filepath.Join(work, "source")
	if err := os.Mkdir(extracted, 0o750); err != nil {
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.StorageOperationFailedReason, err))
	}
	if err := extraction.ExtractTarGzip(archivePath, extracted, extraction.Limits{MaxExtractedBytes: r.Limits.MaxExtractedBytes, MaxFiles: r.Limits.MaxFiles, MaxFileBytes: r.Limits.MaxFileBytes, MaxPathBytes: 4096, MaxDepth: 64}); err != nil {
		return r.fail(&release, spec, stageerr.Stall(mosaicv1.ExtractionFailedReason, err))
	}
	root, err := sourceRoot(extracted, spec.Path)
	if err != nil {
		return r.fail(&release, spec, stageerr.Stall(mosaicv1.InvalidSpecReason, err))
	}
	buildStarted := time.Now()
	compiled, diagnostics := r.Compiler.Compile(buildCtx, mosaiccompiler.Input{RootPath: root, InputKind: spec.InputKind, Environment: spec.Environment, Variants: spec.Variants, Policy: spec.Policy, Limits: compiler.Limits{MaxDiagnostics: r.Limits.MaxDiagnostics, MaxResources: r.Limits.MaxGraphResources, MaxTransformOps: r.Limits.MaxTransformOps, MaxPolicyEvaluations: r.Limits.MaxPolicyEvaluations}})
	if diagnostics.HasErrors() || compiled == nil {
		reason := mosaicv1.BuildFailedReason
		conditions.MarkFalse(&release, mosaicv1.BuildReadyCondition, reason, "%s", mosaiccompiler.Summary(diagnostics, 512))
		policySummary := &mosaicv1.PolicySummary{}
		for _, diagnostic := range diagnostics {
			log.Error(errors.New(diagnostic.Message), "Mosaic diagnostic", "diagnosticCode", diagnostic.Code, "sourceFile", diagnostic.Span.SourceName, "line", diagnostic.Span.Start.Line, "column", diagnostic.Span.Start.Column)
			if strings.HasPrefix(diagnostic.Code, "POL") {
				reason = mosaicv1.PolicyFailedReason
				policySummary.Evaluated++
				if diagnostic.Severity == "warning" {
					policySummary.Warnings++
				} else {
					policySummary.Violations++
				}
				conditions.MarkFalse(&release, mosaicv1.PolicyReadyCondition, reason, "%s", mosaiccompiler.Summary(diagnostics, 512))
			}
		}
		if policySummary.Evaluated > 0 {
			release.Status.PolicySummary = policySummary
			controllermetrics.PolicyViolations.WithLabelValues("failure").Add(float64(policySummary.Violations))
		}
		controllermetrics.BuildTotal.WithLabelValues("failure", reason, string(spec.InputKind)).Inc()
		controllermetrics.BuildFailures.WithLabelValues(reason, string(spec.InputKind)).Inc()
		controllermetrics.BuildDuration.WithLabelValues("failure", string(spec.InputKind)).Observe(time.Since(buildStarted).Seconds())
		if r.Recorder != nil && desiredRevision != base.Status.LastAttemptedRevision {
			r.Recorder.Event(&release, corev1.EventTypeWarning, reason, mosaiccompiler.Summary(diagnostics, 512))
		}
		return r.fail(&release, spec, stageerr.Stall(reason, errors.New(mosaiccompiler.Summary(diagnostics, 512))))
	}
	controllermetrics.BuildTotal.WithLabelValues("success", mosaicv1.SucceededReason, string(compiled.InputKind)).Inc()
	controllermetrics.BuildDuration.WithLabelValues("success", string(compiled.InputKind)).Observe(time.Since(buildStarted).Seconds())
	conditions.MarkTrue(&release, mosaicv1.BuildReadyCondition, mosaicv1.SucceededReason, "Mosaic compilation succeeded")
	conditions.MarkTrue(&release, mosaicv1.PolicyReadyCondition, mosaicv1.SucceededReason, "Mosaic policies passed")
	release.Status.ResourceSummary = &mosaicv1.ResourceSummary{Kinds: map[string]int32{}}
	if compiled.Bundle != nil && compiled.Bundle.Graph != nil {
		for _, resource := range compiled.Bundle.Graph.List() {
			release.Status.ResourceSummary.Total++
			kind := string(resource.Type)
			if _, exists := release.Status.ResourceSummary.Kinds[kind]; !exists && len(release.Status.ResourceSummary.Kinds) >= 64 {
				kind = "Other"
			}
			release.Status.ResourceSummary.Kinds[kind]++
		}
	}
	release.Status.PolicySummary = &mosaicv1.PolicySummary{}
	if compiled.Compilation != nil {
		for _, result := range compiled.Compilation.PolicyReport.Results {
			release.Status.PolicySummary.Evaluated++
			switch result.Severity {
			case "warning":
				release.Status.PolicySummary.Warnings++
			case "error":
				release.Status.PolicySummary.Violations++
			}
		}
	}
	staging := filepath.Join(work, "artifact")
	if err := os.Mkdir(staging, 0o750); err != nil {
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.StorageOperationFailedReason, err))
	}
	if err := artifactlayout.Stage(staging, compiled, spec.Output); err != nil {
		return r.fail(&release, spec, stageerr.Stall(mosaicv1.BuildFailedReason, err))
	}
	name := strings.TrimPrefix(strings.Split(desiredRevision, "sha256:")[1], "sha256:") + ".tar.gz"
	generated := r.Storage.NewArtifactFor("MosaicRelease", &release, desiredRevision, name)
	if err := os.MkdirAll(filepath.Dir(r.Storage.LocalPath(generated)), 0o750); err != nil {
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.StorageOperationFailedReason, err))
	}
	unlock, err := r.Storage.Lock(generated)
	if err != nil {
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.StorageOperationFailedReason, err))
	}
	defer unlock()
	if err := r.Storage.Archive(&generated, staging, nil); err != nil {
		conditions.MarkFalse(&release, mosaicv1.ArtifactInStorageCondition, mosaicv1.StorageOperationFailedReason, "%s", err)
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.StorageOperationFailedReason, err))
	}
	if err := r.Storage.VerifyArtifact(generated); err != nil {
		conditions.MarkFalse(&release, mosaicv1.ArtifactInStorageCondition, mosaicv1.ArtifactVerificationFailedReason, "%s", err)
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.ArtifactVerificationFailedReason, err))
	}
	conditions.MarkTrue(&release, mosaicv1.ArtifactInStorageCondition, mosaicv1.SucceededReason, "Generated artifact is verified in storage")
	if err := r.reconcileExternalArtifact(ctx, &release, spec.Output.ExternalArtifactName, &generated); err != nil {
		controllermetrics.ArtifactBuildTotal.WithLabelValues("failure").Inc()
		return r.fail(&release, spec, stageerr.Retry(mosaicv1.ArtifactPublicationFailedReason, err))
	}
	controllermetrics.ArtifactBuildTotal.WithLabelValues("success").Inc()
	if generated.Size != nil {
		controllermetrics.ArtifactSize.WithLabelValues(string(compiled.InputKind)).Set(float64(*generated.Size))
	}
	controllermetrics.MarkActive(string(release.UID))
	release.Status.Artifact = generated.DeepCopy()
	release.Status.LastSuccessfulRevision = desiredRevision
	release.Status.ExternalArtifactRef = &corev1.LocalObjectReference{Name: spec.Output.ExternalArtifactName}
	r.markSuccess(&release)
	if r.Recorder != nil && desiredRevision != base.Status.LastSuccessfulRevision {
		r.Recorder.Event(&release, corev1.EventTypeNormal, "ArtifactPublished", "Published Mosaic artifact "+desiredRevision)
	}
	_, _ = r.Storage.RemoveAllButCurrent(generated)
	return ctrl.Result{RequeueAfter: jitter(spec.Interval.Duration, release.UID)}, nil
}

func (r *MosaicReleaseReconciler) fail(release *mosaicv1.MosaicRelease, spec mosaicv1.MosaicReleaseSpec, err error) (ctrl.Result, error) {
	class, reason := stageerr.Classify(err)
	message := err.Error()
	conditions.MarkFalse(release, mosaicv1.ReadyCondition, reason, "%s", message)
	if class == stageerr.Stalling {
		if release.Status.Artifact != nil {
			message += "; previous successful artifact remains available"
		}
		conditions.MarkStalled(release, reason, "%s", message)
		return ctrl.Result{RequeueAfter: spec.Interval.Duration}, nil
	}
	conditions.MarkReconciling(release, mosaicv1.ProgressingWithRetryReason, "%s", message)
	if reason == mosaicv1.SourceNotReadyReason && r.DependencyRequeue > 0 {
		return ctrl.Result{RequeueAfter: r.DependencyRequeue}, nil
	}
	return ctrl.Result{RequeueAfter: spec.RetryInterval.Duration}, nil
}

func (r *MosaicReleaseReconciler) markSuccess(release *mosaicv1.MosaicRelease) {
	conditions.Delete(release, mosaicv1.ReconcilingCondition)
	conditions.Delete(release, mosaicv1.StalledCondition)
	conditions.MarkTrue(release, mosaicv1.ReadyCondition, mosaicv1.SucceededReason, "Artifact is ready")
}

func (r *MosaicReleaseReconciler) finalize(ctx context.Context, release *mosaicv1.MosaicRelease) (ctrl.Result, error) {
	if !containsString(release.Finalizers, Finalizer) {
		return ctrl.Result{}, nil
	}
	cleanup := r.Storage.NewArtifactFor("MosaicRelease", release, "cleanup", "cleanup.tar.gz")
	if _, err := r.Storage.RemoveAll(cleanup); err != nil && !os.IsNotExist(err) {
		return ctrl.Result{}, err
	}
	controllermetrics.MarkInactive(string(release.UID))
	base := release.DeepCopy()
	release.Finalizers = removeString(release.Finalizers, Finalizer)
	return ctrl.Result{}, r.Patch(ctx, release, client.MergeFrom(base))
}

func sourceRoot(root, relative string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source path escapes artifact")
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source path escapes artifact")
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("source path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source path is not a directory")
	}
	return target, nil
}

func jitter(interval time.Duration, uid types.UID) time.Duration {
	if interval <= 0 {
		return 0
	}
	span := interval / 10
	offset := time.Duration(0)
	if len(uid) > 0 {
		offset = span / 256 * time.Duration(uid[0])
	}
	return interval - interval/20 + offset
}
func containsString(xs []string, x string) bool {
	for _, item := range xs {
		if item == x {
			return true
		}
	}
	return false
}
func removeString(xs []string, x string) []string {
	out := xs[:0]
	for _, item := range xs {
		if item != x {
			out = append(out, item)
		}
	}
	return out
}

func (r *MosaicReleaseReconciler) externalArtifactCurrent(ctx context.Context, release *mosaicv1.MosaicRelease, name string) (bool, error) {
	var obj sourcev1.ExternalArtifact
	if err := r.Get(ctx, types.NamespacedName{Namespace: release.Namespace, Name: name}, &obj); err != nil {
		return false, err
	}
	return obj.Status.Artifact != nil && release.Status.Artifact != nil && obj.Status.Artifact.Digest == release.Status.Artifact.Digest && obj.Status.Artifact.Revision == release.Status.Artifact.Revision, nil
}

func (r *MosaicReleaseReconciler) reconcileExternalArtifact(ctx context.Context, release *mosaicv1.MosaicRelease, name string, generated *meta.Artifact) error {
	key := types.NamespacedName{Namespace: release.Namespace, Name: name}
	var obj sourcev1.ExternalArtifact
	err := r.Get(ctx, key, &obj)
	if apierrors.IsNotFound(err) {
		obj = sourcev1.ExternalArtifact{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: release.Namespace}, Spec: sourcev1.ExternalArtifactSpec{SourceRef: &meta.NamespacedObjectKindReference{APIVersion: mosaicv1.GroupVersion.String(), Kind: "MosaicRelease", Name: release.Name}}}
		if err := ctrl.SetControllerReference(release, &obj, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, &obj); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !metav1.IsControlledBy(&obj, release) {
		return fmt.Errorf("ExternalArtifact %s/%s is controlled by another owner", obj.Namespace, obj.Name)
	}
	desiredRef := &meta.NamespacedObjectKindReference{APIVersion: mosaicv1.GroupVersion.String(), Kind: "MosaicRelease", Name: release.Name}
	if obj.Spec.SourceRef == nil || *obj.Spec.SourceRef != *desiredRef {
		base := obj.DeepCopy()
		obj.Spec.SourceRef = desiredRef
		if err := r.Patch(ctx, &obj, client.MergeFrom(base)); err != nil {
			return err
		}
	}
	base := obj.DeepCopy()
	obj.Status.Artifact = generated.DeepCopy()
	conditions.MarkTrue(&obj, "Ready", mosaicv1.SucceededReason, "Artifact produced by MosaicRelease %s", release.Name)
	return r.Status().Patch(ctx, &obj, client.MergeFrom(base))
}

func (r *MosaicReleaseReconciler) SetupWithManager(mgr ctrl.Manager, concurrent int) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mosaicv1.MosaicRelease{}, SourceIndex, func(obj client.Object) []string {
		release := obj.(*mosaicv1.MosaicRelease)
		return []string{artifactsource.IndexKey(release.Namespace, release.Spec.SourceRef)}
	}); err != nil {
		return err
	}
	mapSource := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		if kind == "" {
			switch obj.(type) {
			case *sourcev1.OCIRepository:
				kind = "OCIRepository"
			case *sourcev1.ExternalArtifact:
				kind = "ExternalArtifact"
			}
		}
		key := kind + ":" + obj.GetNamespace() + "/" + obj.GetName()
		var list mosaicv1.MosaicReleaseList
		if err := r.List(ctx, &list, client.MatchingFields{SourceIndex: key}); err != nil {
			return nil
		}
		out := make([]ctrl.Request, 0, len(list.Items))
		for i := range list.Items {
			out = append(out, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
		}
		return out
	})
	releaseEvents := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRelease, oldOK := e.ObjectOld.(*mosaicv1.MosaicRelease)
			newRelease, newOK := e.ObjectNew.(*mosaicv1.MosaicRelease)
			if !oldOK || !newOK {
				return true
			}
			return oldRelease.Generation != newRelease.Generation || !oldRelease.DeletionTimestamp.Equal(newRelease.DeletionTimestamp) || oldRelease.Annotations[ReconcileRequestAnnotation] != newRelease.Annotations[ReconcileRequestAnnotation]
		},
	}
	sourceEvents := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return sourceState(e.ObjectOld) != sourceState(e.ObjectNew) },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&mosaicv1.MosaicRelease{}, builder.WithPredicates(releaseEvents)).
		Watches(&sourcev1.OCIRepository{}, mapSource, builder.WithPredicates(sourceEvents)).
		Watches(&sourcev1.ExternalArtifact{}, mapSource, builder.WithPredicates(sourceEvents)).
		WithOptions(controllerpkg.Options{MaxConcurrentReconciles: concurrent}).Complete(r)
}

func sourceState(obj client.Object) string {
	ready, revision, digest := false, "", ""
	var artifact *meta.Artifact
	var sourceConditions []metav1.Condition
	switch typed := obj.(type) {
	case *sourcev1.OCIRepository:
		artifact, sourceConditions = typed.Status.Artifact, typed.Status.Conditions
	case *sourcev1.ExternalArtifact:
		artifact, sourceConditions = typed.Status.Artifact, typed.Status.Conditions
	default:
		return "unknown"
	}
	for _, condition := range sourceConditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			ready = true
			break
		}
	}
	if artifact != nil {
		revision, digest = artifact.Revision, artifact.Digest
	}
	return fmt.Sprintf("%t|%s|%s", ready, revision, digest)
}
