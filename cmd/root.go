package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cybozu-go/contour-plus/controllers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	fs.String("default-delegated-domain", "", "Delegated domain used by default")
	fs.StringSlice("allowed-delegated-domains", []string{}, "List of allowed delegated domains")
	fs.Bool("allow-custom-delegations", false, "Allow custom delegated domains via annotations")
	fs.Uint("csr-revision-limit", 0, "Maximum number of CertificateRequest revisions to keep")
	fs.String("ingress-class-name", "", "Ingress class name that watched by Contour Plus. If not specified, then all classes are watched")
	fs.Bool("leader-election", true, "Enable/disable leader election")
	fs.StringSlice("propagated-annotations", []string{}, "List of annotation keys to be propagated from HTTPProxy to generated resources")
	fs.StringSlice("propagated-labels", []string{}, "List of label keys to be propagated from HTTPProxy to generated resources")
	fs.StringSlice("allowed-dns-namespaces", []string{}, "List of namespaces where DNSEndpoint resources can be created. If empty, no namespaces are allowed")
	fs.StringSlice("allowed-issuer-namespaces", []string{}, "List of namespaces where Certificate resources can be created. If empty, no namespaces are allowed")
	if err := viper.BindPFlags(fs); err != nil {
		panic(err)
	}
	envKeyReplacer := strings.NewReplacer("-", "_")
	viper.SetEnvPrefix("cp")
	viper.SetEnvKeyReplacer(envKeyReplacer)
	viper.AutomaticEnv()

	// Because k8s.io/klog uses Go flag package, we need to add flags for klog to fs.
	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	zapOpts.BindFlags(goflags)

	fs.AddGoFlagSet(goflags)
	rootCmd.Long = rootCmd.Short + "\n\n" + generateEnvDoc(fs, envKeyReplacer)
}

func generateEnvDoc(fs *pflag.FlagSet, replacer *strings.Replacer) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 4, ' ', 0)
	_, _ = w.Write([]byte("In addition to flags, the following environment variables are read:\n\n"))
	fs.VisitAll(func(f *pflag.Flag) {
		envName := "CP_" + strings.ToUpper(replacer.Replace(f.Name))
		fmt.Fprintf(w, "\t\t%s\t%s\n", envName, f.Usage)
	})
	w.Flush()
	return buf.String()
}

var rootCmd = &cobra.Command{
	Use:   "contour-plus",
	Short: "contour-plus is a custom controller for Contour HTTPProxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return run()
	},
}
