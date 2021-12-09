package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cybozu-go/contour-plus/controllers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var zapOpts zap.Options

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	controllers.SetupScheme(scheme)

	fs := rootCmd.Flags()
	fs.String("metrics-addr", ":8180", "Bind address for the metrics endpoint")
	fs.StringSlice("crds", []string{controllers.DNSEndpointKind, controllers.CertificateKind}, "List of CRD names to be created")
	fs.String("name-prefix", "", "Prefix of CRD names to be created")
	fs.String("service-name", "", "NamespacedName of the Contour LoadBalancer Service")
	fs.String("default-issuer-name", "", "Issuer name used by default")
	fs.String("default-issuer-kind", controllers.ClusterIssuerKind, "Issuer kind used by default")
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
	zapOpts.BindFlags(goflags)

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
		return run()
	},
}
