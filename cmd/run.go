package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/cybozu-go/contour-plus/controllers"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

	opts := controllers.ReconcilerOptions{
		Prefix:            viper.GetString("name-prefix"),
		DefaultIssuerName: viper.GetString("default-issuer-name"),
	}

	crds := viper.GetStringSlice("crds")
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

	serviceName := viper.GetString("service-name")
	nsname := strings.Split(serviceName, "/")
	if len(nsname) != 2 || nsname[0] == "" || nsname[1] == "" {
		return errors.New("service-name should be valid string as namespaced-name")
	}
	opts.ServiceKey = client.ObjectKey{
		Namespace: nsname[0],
		Name:      nsname[1],
	}

	defaultIssuerKind := viper.GetString("default-issuer-kind")
	switch defaultIssuerKind {
	case controllers.IssuerKind, controllers.ClusterIssuerKind:
	default:
		return errors.New("unsupported Issuer kind: " + defaultIssuerKind)
	}
	opts.DefaultIssuerKind = defaultIssuerKind

	opts.IngressClassName = viper.GetString("ingress-class-name")

	opts.CSRRevisionLimit = viper.GetUint("csr-revision-limit")

	opts.PropagatedAnnotations = viper.GetStringSlice("propagated-annotations")
	opts.PropagatedLabels = viper.GetStringSlice("propagated-labels")

	opts.DefaultDelegatedDomain = viper.GetString("default-delegated-domain")
	opts.AllowCustomDelegations = viper.GetBool("allow-custom-delegations")
	opts.AllowedDelegatedDomains = viper.GetStringSlice("allowed-delegated-domains")

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
