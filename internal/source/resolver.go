package source

import (
	"context"
	"fmt"

	meta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resolved struct {
	Artifact              meta.Artifact
	Namespace, Name, Kind string
}

func Resolve(ctx context.Context, c client.Client, ownerNamespace string, ref mosaicv1.SourceReference) (*Resolved, error) {
	ns := ref.Namespace
	if ns == "" {
		ns = ownerNamespace
	}
	key := types.NamespacedName{Namespace: ns, Name: ref.Name}
	var artifact *meta.Artifact
	var conditions []metav1.Condition
	switch ref.Kind {
	case "OCIRepository":
		var obj sourcev1.OCIRepository
		if err := c.Get(ctx, key, &obj); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("source %s/%s not found", ns, ref.Name)
			}
			return nil, err
		}
		artifact, conditions = obj.Status.Artifact, obj.Status.Conditions
	case "ExternalArtifact":
		var obj sourcev1.ExternalArtifact
		if err := c.Get(ctx, key, &obj); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("source %s/%s not found", ns, ref.Name)
			}
			return nil, err
		}
		artifact, conditions = obj.Status.Artifact, obj.Status.Conditions
	default:
		return nil, fmt.Errorf("unsupported source kind %q", ref.Kind)
	}
	ready := false
	for _, condition := range conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			ready = true
			break
		}
	}
	if !ready || artifact == nil || artifact.URL == "" || artifact.Digest == "" || artifact.Revision == "" {
		return nil, fmt.Errorf("source %s/%s is not ready", ns, ref.Name)
	}
	return &Resolved{Artifact: *artifact.DeepCopy(), Namespace: ns, Name: ref.Name, Kind: ref.Kind}, nil
}

func IndexKey(namespace string, ref mosaicv1.SourceReference) string {
	ns := ref.Namespace
	if ns == "" {
		ns = namespace
	}
	return ref.Kind + ":" + ns + "/" + ref.Name
}
