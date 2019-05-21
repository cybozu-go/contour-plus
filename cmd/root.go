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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

const (
	dnsEndpointCRD = "DNSEndpoint"
	certificateCRD = "Certificate"
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
	err := contourv1beta1.AddToScheme(scheme)
	if err != nil {
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

	err = certmanagerv1alpha1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme

	fs := rootCmd.Flags()
	fs.String("metrics-addr", ":8080", "Bind address for the metrics endpoint")
	fs.StringSlice("crds", []string{dnsEndpointCRD, certificateCRD}, "List of CRD names to be created")
	fs.String("name-prefix", "", "Prefix of CRD names to be created")
	fs.String("service-name", "", "NamespacedName of the Contour LoadBalancer Service")
	err = viper.BindPFlags(fs)
	if err != nil {
		panic(err)
	}
	err = cobra.MarkFlagRequired(fs, "service-name")
	if err != nil {
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

	CP_METRICS_ADDR      Bind address for the metrics endpoint
	CP_CRDS              Comma-separated list of CRD names
	CP_NAME_PREFIX       Prefix of CRD names to be created
	CP_SERVICE_NAME      NamespacedName of the Contour LoadBalancer Service`,
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
	for _, crd := range crds {
		switch crd {
		case dnsEndpointCRD:
		case certificateCRD:
		default:
			return errors.New("unsupported CRD: " + crd)
		}
	}

	serviceName := viper.GetString("service-name")
	if !strings.Contains(serviceName[1:len(serviceName)-2], "/") {
		return errors.New("service-name should be valid string as namespaced-name")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme, MetricsBindAddress: viper.GetString("metrics-addr")})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	err = (&controllers.IngressRouteReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("IngressRoute"),
		Scheme: mgr.GetScheme(),
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
