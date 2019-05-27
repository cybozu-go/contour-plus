package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cybozu-go/contour-plus/controllers"
	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	if err := contourv1beta1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// ExternalDNS does not implement AddToScheme
	groupVersion := ctrl.GroupVersion{
		Group:   "externaldns.k8s.io",
		Version: "v1alpha1",
	}
	scheme.AddKnownTypes(groupVersion,
		&endpoint.DNSEndpoint{},
		&endpoint.DNSEndpointList{},
	)
	metav1.AddToGroupVersion(scheme, groupVersion)

	if err := certmanagerv1alpha1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// for corev1.Service
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme

	fs := rootCmd.Flags()
	fs.String("metrics-addr", ":8080", "Bind address for the metrics endpoint")
	fs.StringSlice("crds", []string{dnsEndpointKind, certmanagerv1alpha1.CertificateKind}, "List of CRD names to be created")
	fs.String("name-prefix", "", "Prefix of CRD names to be created")
	fs.String("service-name", "", "NamespacedName of the Contour LoadBalancer Service")
	fs.String("default-issuer-name", "", "Issuer name used by default")
	fs.String("default-issuer-kind", certmanagerv1alpha1.ClusterIssuerKind, "Issuer kind used by default")
	if err := viper.BindPFlags(fs); err != nil {
		panic(err)
	}
	if err := cobra.MarkFlagRequired(fs, "service-name"); err != nil {
		panic(err)
	}
	viper.SetEnvPrefix("cp")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

var rootCmd = &cobra.Command{
	Use:   "contour-plus",
	Short: "contour-plus is a custom controller for Contour IngressRoute",
	Long: `contour-plus is a custom controller for Contour IngressRoute.
	
In addition to flags, the following environment variables are read:

	CP_METRICS_ADDR          Bind address for the metrics endpoint
	CP_CRDS                  Comma-separated list of CRD names
	CP_NAME_PREFIX           Prefix of CRD names to be created
	CP_SERVICE_NAME          NamespacedName of the Contour LoadBalancer Service
	CP_DEFAULT_ISSUER_NAME   Issuer name used by default
	CP_DEFAULT_ISSUER_KIND   Issuer kind used by default`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	ctrl.SetLogger(zap.Logger(true))

	crds := viper.GetStringSlice("crds")
	if len(crds) == 0 {
		return errors.New("at least one service need to be enabled")
	}
	var createDNSEndpoint, createCertificate bool
	for _, crd := range crds {
		switch crd {
		case dnsEndpointKind:
			createDNSEndpoint = true
		case certmanagerv1alpha1.CertificateKind:
			createCertificate = true
		default:
			return errors.New("unsupported CRD: " + crd)
		}
	}

	serviceName := viper.GetString("service-name")
	nsname := strings.Split(serviceName, "/")
	if len(nsname) != 2 || nsname[0] == "" || nsname[1] == "" {
		return errors.New("service-name should be valid string as namespaced-name")
	}
	serviceKey := client.ObjectKey{
		Namespace: nsname[0],
		Name:      nsname[1],
	}

	defaultIssuerKind := viper.GetString("default-issuer-kind")
	switch defaultIssuerKind {
	case certmanagerv1alpha1.IssuerKind, certmanagerv1alpha1.ClusterIssuerKind:
	default:
		return errors.New("unsupported Issuer kind: " + defaultIssuerKind)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme, MetricsBindAddress: viper.GetString("metrics-addr")})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	err = (&controllers.IngressRouteReconciler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("IngressRoute"),
		Scheme:            mgr.GetScheme(),
		ServiceKey:        serviceKey,
		Prefix:            viper.GetString("name-prefix"),
		DefaultIssuerName: viper.GetString("default-issuer-name"),
		DefaultIssuerKind: defaultIssuerKind,
		CreateDNSEndpoint: createDNSEndpoint,
		CreateCertificate: createCertificate,
	}).SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IngressRoute")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
	return nil
}
