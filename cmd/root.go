package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cybozu-go/contour-plus/controllers"
	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

const (
	dnsEndpointKind = "DNSEndpoint"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	if err := controllers.SetupScheme(scheme); err != nil {
		panic(err)
	}

	fs := rootCmd.Flags()
	fs.String("metrics-addr", ":8180", "Bind address for the metrics endpoint")
	fs.StringSlice("crds", []string{dnsEndpointKind, certmanagerv1alpha2.CertificateKind}, "List of CRD names to be created")
	fs.String("name-prefix", "", "Prefix of CRD names to be created")
	fs.String("service-name", "", "NamespacedName of the Contour LoadBalancer Service")
	fs.String("default-issuer-name", "", "Issuer name used by default")
	fs.String("default-issuer-kind", certmanagerv1alpha2.ClusterIssuerKind, "Issuer kind used by default")
	fs.String("ingress-class-name", "", "Ingress class name that watched by Contour Plus. If not specified, then all classes are watched")
	fs.Bool("leader-election", true, "Enable/disable leader election")
	if err := viper.BindPFlags(fs); err != nil {
		panic(err)
	}
	viper.SetEnvPrefix("cp")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Because k8s.io/klog uses Go flag package, we need to add flags for klog to fs.
	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}

var rootCmd = &cobra.Command{
	Use:   "contour-plus",
	Short: "contour-plus is a custom controller for Contour HTTPProxy",
	Long: `contour-plus is a custom controller for Contour HTTPProxy.
	
In addition to flags, the following environment variables are read:

	CP_METRICS_ADDR          Bind address for the metrics endpoint
	CP_CRDS                  Comma-separated list of CRD names
	CP_NAME_PREFIX           Prefix of CRD names to be created
	CP_SERVICE_NAME          NamespacedName of the Contour LoadBalancer Service
	CP_DEFAULT_ISSUER_NAME   Issuer name used by default
	CP_DEFAULT_ISSUER_KIND   Issuer kind used by default
	CP_LEADER_ELECTION       Disable leader election if set to "false"
	CP_INGRESS_CLASS_NAME    Ingress class name that watched by Contour Plus. If not specified, then all classes are watched`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	ctrl.SetLogger(zap.Logger(false))

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
		case dnsEndpointKind:
			opts.CreateDNSEndpoint = true
		case certmanagerv1alpha2.CertificateKind:
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
	case certmanagerv1alpha2.IssuerKind, certmanagerv1alpha2.ClusterIssuerKind:
	default:
		return errors.New("unsupported Issuer kind: " + defaultIssuerKind)
	}
	opts.DefaultIssuerKind = defaultIssuerKind

	opts.IngressClassName = viper.GetString("ingress-class-name")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: viper.GetString("metrics-addr"),
		LeaderElection:     viper.GetBool("leader-election"),
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
