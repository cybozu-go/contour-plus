package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/cybozu-go/contour-plus/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	controllers.SetupScheme(scheme)
}

func run() error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	opts := controllers.ReconcilerOptions{
		Prefix:            config.namePrefix,
		DefaultIssuerName: config.defaultIssuerName,
	}

	crds := config.crds
	if len(crds) == 0 {
		return errors.New("at least one service need to be enabled")
	}
	for _, crd := range crds {
		switch crd {
		case controllers.DNSEndpointKind:
			opts.CreateDNSEndpoint = true
		case controllers.CertificateKind:
			opts.CreateCertificate = true
		default:
			return errors.New("unsupported CRD: " + crd)
		}
	}

	serviceName := config.serviceName
	nsname := strings.Split(serviceName, "/")
	if len(nsname) != 2 || nsname[0] == "" || nsname[1] == "" {
		return errors.New("service-name should be valid string as namespaced-name")
	}
	opts.ServiceKey = client.ObjectKey{
		Namespace: nsname[0],
		Name:      nsname[1],
	}

	defaultIssuerKind := config.defaultIssuerKind
	switch defaultIssuerKind {
	case controllers.IssuerKind, controllers.ClusterIssuerKind:
	default:
		return errors.New("unsupported Issuer kind: " + defaultIssuerKind)
	}
	opts.DefaultIssuerKind = defaultIssuerKind

	opts.IngressClassName = config.ingressClassName

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		LeaderElection:     config.leaderElection,
		LeaderElectionID:   "contour-plus-leader",
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
