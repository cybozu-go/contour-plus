package cmd

import (
	"os"

	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/cybozu-go/contour-plus/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	controllers.SetupScheme(scheme)
}

func run() error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	var opts controllers.ReconcilerOptions
	if err := viper.Unmarshal(&opts); err != nil {
		return err
	}

	if err := opts.Finalize(); err != nil {
		return err
	}

	if err := opts.Validate(); err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: viper.GetString("metrics-addr"),
		},
		LeaderElection:   viper.GetBool("leader-election"),
		LeaderElectionID: "contour-plus-leader",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	err = controllers.SetupReconciler(mgr, mgr.GetScheme(), opts)
	if err != nil {
		setupLog.Error(err, "unable to create controllers")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
	return nil
}
