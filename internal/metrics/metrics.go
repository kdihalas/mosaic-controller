package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sync"
)

var active = struct {
	sync.Mutex
	releases map[string]struct{}
}{releases: map[string]struct{}{}}

var (
	BuildTotal             = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mosaic_controller_build_total", Help: "Mosaic builds by outcome."}, []string{"result", "reason", "input_kind"})
	BuildDuration          = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "mosaic_controller_build_duration_seconds", Help: "Mosaic build duration.", Buckets: prometheus.DefBuckets}, []string{"result", "input_kind"})
	BuildFailures          = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mosaic_controller_build_failures_total", Help: "Mosaic build failures."}, []string{"reason", "input_kind"})
	PolicyViolations       = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mosaic_controller_policy_violations_total", Help: "Policy violations by result."}, []string{"result"})
	ArtifactSize           = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "mosaic_controller_artifact_size_bytes", Help: "Latest generated artifact size."}, []string{"input_kind"})
	ArtifactBuildTotal     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mosaic_controller_artifact_build_total", Help: "Artifact publication outcomes."}, []string{"result"})
	SourceDownloadDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "mosaic_controller_source_download_duration_seconds", Help: "Source artifact download duration.", Buckets: prometheus.DefBuckets})
	SourceDownloadBytes    = prometheus.NewCounter(prometheus.CounterOpts{Name: "mosaic_controller_source_download_bytes", Help: "Downloaded source artifact bytes."})
	ActiveArtifacts        = prometheus.NewGauge(prometheus.GaugeOpts{Name: "mosaic_controller_active_artifacts", Help: "Artifacts successfully published by this process."})
)

func init() {
	ctrlmetrics.Registry.MustRegister(BuildTotal, BuildDuration, BuildFailures, PolicyViolations, ArtifactSize, ArtifactBuildTotal, SourceDownloadDuration, SourceDownloadBytes, ActiveArtifacts)
}

func MarkActive(uid string) {
	active.Lock()
	defer active.Unlock()
	active.releases[uid] = struct{}{}
	ActiveArtifacts.Set(float64(len(active.releases)))
}

func MarkInactive(uid string) {
	active.Lock()
	defer active.Unlock()
	delete(active.releases, uid)
	ActiveArtifacts.Set(float64(len(active.releases)))
}
