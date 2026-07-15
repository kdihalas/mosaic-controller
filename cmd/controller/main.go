package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	artifactconfig "github.com/fluxcd/pkg/artifact/config"
	artifactdigest "github.com/fluxcd/pkg/artifact/digest"
	artifactstorage "github.com/fluxcd/pkg/artifact/storage"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	mosaicv1 "github.com/kdihalas/mosaic-controller/api/v1alpha1"
	artifactserver "github.com/kdihalas/mosaic-controller/internal/artifact"
	mosaiccompiler "github.com/kdihalas/mosaic-controller/internal/compiler"
	mosaiccontroller "github.com/kdihalas/mosaic-controller/internal/controller"
	artifactsource "github.com/kdihalas/mosaic-controller/internal/source"
	"github.com/spf13/pflag"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var version = "devel"

func main() {
	var metricsAddress, probeAddress, logLevel, logEncoding string
	var leaderElect, noCrossNamespace bool
	var concurrent int
	var dependencyRequeue time.Duration
	var limits mosaiccontroller.Limits
	storageOptions := &artifactconfig.Options{}
	storageOptions.BindFlags(pflag.CommandLine)
	pflag.StringVar(&metricsAddress, "metrics-bind-address", ":8080", "Metrics endpoint bind address.")
	pflag.StringVar(&probeAddress, "health-probe-bind-address", ":9440", "Health endpoint bind address.")
	pflag.BoolVar(&leaderElect, "leader-elect", true, "Enable leader election.")
	pflag.IntVar(&concurrent, "concurrent", 2, "Maximum concurrent reconciliations.")
	pflag.DurationVar(&dependencyRequeue, "requeue-dependency", 30*time.Second, "Requeue delay for sources that are not ready.")
	pflag.BoolVar(&noCrossNamespace, "no-cross-namespace-refs", true, "Disallow cross-namespace source references.")
	pflag.Int64Var(&limits.MaxDownloadBytes, "max-download-bytes", 100<<20, "Maximum compressed source bytes.")
	pflag.Int64Var(&limits.MaxExtractedBytes, "max-extracted-bytes", 500<<20, "Maximum extracted source bytes.")
	pflag.IntVar(&limits.MaxFiles, "max-source-files", 10000, "Maximum source file count.")
	pflag.Int64Var(&limits.MaxFileBytes, "max-source-file-bytes", 50<<20, "Maximum individual source file bytes.")
	pflag.DurationVar(&limits.MaxBuildDuration, "max-build-duration", 2*time.Minute, "Maximum build duration.")
	pflag.IntVar(&limits.MaxDiagnostics, "max-compiler-diagnostics", 100, "Maximum compiler diagnostics.")
	pflag.IntVar(&limits.MaxGraphResources, "max-graph-resources", 10000, "Maximum graph resources.")
	pflag.IntVar(&limits.MaxTransformOps, "max-transform-ops", 100000, "Maximum transform operations.")
	pflag.IntVar(&limits.MaxPolicyEvaluations, "max-policy-evaluations", 100000, "Maximum policy evaluations.")
	pflag.StringVar(&logLevel, "log-level", "info", "Log level: debug or info.")
	pflag.StringVar(&logEncoding, "log-encoding", "json", "Log encoding: json or console.")
	pflag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(logEncoding == "console" || logLevel == "debug")))
	setupLog := ctrl.Log.WithName("setup").WithValues("version", version)
	if concurrent <= 0 {
		setupLog.Error(fmt.Errorf("must be positive"), "invalid --concurrent")
		os.Exit(2)
	}
	digestAlgorithm, err := artifactdigest.AlgorithmForName(storageOptions.ArtifactDigestAlgo)
	if err != nil {
		setupLog.Error(err, "invalid artifact digest algorithm")
		os.Exit(2)
	}
	artifactdigest.Canonical = digestAlgorithm
	if err := os.MkdirAll(storageOptions.StoragePath, 0o750); err != nil {
		setupLog.Error(err, "create storage directory")
		os.Exit(1)
	}
	storage, err := artifactstorage.New(storageOptions)
	if err != nil {
		setupLog.Error(err, "initialize artifact storage")
		os.Exit(1)
	}

	scheme := clientgoscheme.Scheme
	if err := mosaicv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "register Mosaic API")
		os.Exit(1)
	}
	if err := sourcev1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "register Flux source API")
		os.Exit(1)
	}
	grace := 30 * time.Second
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: metricsAddress, TLSOpts: []func(*tls.Config){}}, HealthProbeBindAddress: probeAddress, LeaderElection: leaderElect, LeaderElectionID: "mosaic-controller.mosaic.toolkit.fluxcd.io", LeaderElectionReleaseOnCancel: true, GracefulShutdownTimeout: &grace})
	if err != nil {
		setupLog.Error(err, "create manager")
		os.Exit(1)
	}
	server := &artifactserver.Server{Address: storageOptions.StorageAddress, Root: storageOptions.StoragePath}
	if err := mgr.Add(server); err != nil {
		setupLog.Error(err, "add artifact server")
		os.Exit(1)
	}
	reconciler := &mosaiccontroller.MosaicReleaseReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Recorder: mgr.GetEventRecorderFor("mosaic-controller"), Storage: storage, Downloader: artifactsource.NewDownloader(2*time.Minute, limits.MaxDownloadBytes), Compiler: mosaiccompiler.Adapter{}, NoCrossNamespaceRefs: noCrossNamespace, DependencyRequeue: dependencyRequeue, Limits: limits}
	if err := reconciler.SetupWithManager(mgr, concurrent); err != nil {
		setupLog.Error(err, "setup controller")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "add health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("storage-server", server.Ready); err != nil {
		setupLog.Error(err, "add readiness check")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager stopped")
		os.Exit(1)
	}
}
