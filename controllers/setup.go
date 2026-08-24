package controllers

import (
	"errors"
	"strings"
	"time"

	cmapiv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	// +kubebuilder:scaffold:imports
)

// ReconcilerOptions is a set of options for reconcilers
type ReconcilerOptions struct {
	ServiceKey                     client.ObjectKey `mapstructure:"-"`
	DefaultDelegatedDomain         string           `mapstructure:"default-delegated-domain"`
	DefaultIssuerKind              string           `mapstructure:"default-issuer-kind"`
	DefaultIssuerName              string           `mapstructure:"default-issuer-name"`
	IngressClassName               string           `mapstructure:"ingress-class-name"`
	Prefix                         string           `mapstructure:"name-prefix"`
	ServiceName                    string           `mapstructure:"service-name"`
	AllowedDelegatedDomains        []string         `mapstructure:"allowed-delegated-domains"`
	AllowedDNSNamespaces           []string         `mapstructure:"allowed-dns-namespaces"`
	AllowedIssuerNamespaces        []string         `mapstructure:"allowed-issuer-namespaces"`
	CRDs                           []string         `mapstructure:"crds"`
	PropagatedAnnotations          []string         `mapstructure:"propagated-annotations"`
	PropagatedLabels               []string         `mapstructure:"propagated-labels"`
	CSRRevisionLimit               uint             `mapstructure:"csr-revision-limit"`
	CertificateApplyLimit          float64          `mapstructure:"certificate-apply-limit"`
	CertificateApplyRetryBaseDelay time.Duration    `mapstructure:"certificate-apply-retry-base-delay"`
	CertificateApplyRetryMaxDelay  time.Duration    `mapstructure:"certificate-apply-retry-max-delay"`
	AllowCustomDelegations         bool             `mapstructure:"allow-custom-delegations"`
	CreateDNSEndpoint              bool             `mapstructure:"-"`
	CreateCertificate              bool             `mapstructure:"-"`
	DefaultDNSTTL                  int32            `mapstructure:"default-dns-ttl"`
	DefaultDNSDelegationTTL        int32            `mapstructure:"default-dns-delegation-ttl"`
}

// BindFlags binds command line flags for ReconcilerOptions.
func BindFlags(fs *pflag.FlagSet) {
	fs.StringSlice("crds", []string{DNSEndpointKind, CertificateKind}, "List of CRD names to be created")
	fs.String("name-prefix", "", "Prefix of CRD names to be created")
	fs.String("service-name", "", "NamespacedName of the Contour LoadBalancer Service")
	fs.String("default-issuer-name", "", "Issuer name used by default")
	fs.String("default-issuer-kind", ClusterIssuerKind, "Issuer kind used by default")
	fs.String("default-delegated-domain", "", "Delegated domain used by default")
	fs.StringSlice("allowed-delegated-domains", []string{}, "List of allowed delegated domains")
	fs.Bool("allow-custom-delegations", false, "Allow custom delegated domains via annotations")
	fs.Uint("csr-revision-limit", 0, "Maximum number of CertificateRequest revisions to keep")
	fs.String("ingress-class-name", "", "Ingress class name that watched by Contour Plus. If not specified, then all classes are watched")
	fs.StringSlice("propagated-annotations", []string{}, "List of annotation keys to be propagated from HTTPProxy to generated resources")
	fs.StringSlice("propagated-labels", []string{}, "List of label keys to be propagated from HTTPProxy to generated resources")
	fs.StringSlice("allowed-dns-namespaces", []string{}, "List of namespaces where DNSEndpoint resources can be created. If empty, no namespaces are allowed")
	fs.StringSlice("allowed-issuer-namespaces", []string{}, "List of namespaces where Certificate resources can be created. If empty, no namespaces are allowed")
	fs.Float64("certificate-apply-limit", 0, "Maximum number of certificate apply operations allowed per second (0 disables rate limiting)")
	fs.Duration("certificate-apply-retry-base-delay", DefaultRetryBaseDelay, "Base delay for certificate apply exponential backoff retry")
	fs.Duration("certificate-apply-retry-max-delay", DefaultRetryMaxDelay, "Maximum delay for certificate apply exponential backoff retry")
	fs.Int32("default-dns-ttl", DefaultDNSTTL, "Default TTL value for DNSEndpoint A records")
	fs.Int32("default-dns-delegation-ttl", DefaultDNSDelegationTTL, "Default TTL value for DNSEndpoint CNAME delgation records")
}

// Finalize finalizes the ReconcilerOptions by setting derived fields.
func (o *ReconcilerOptions) Finalize() error {
	for _, crd := range o.CRDs {
		switch crd {
		case DNSEndpointKind:
			o.CreateDNSEndpoint = true
		case CertificateKind:
			o.CreateCertificate = true
		default:
			return errors.New("unsupported CRD: " + crd)
		}
	}
	nsName := strings.Split(o.ServiceName, "/")
	if len(nsName) != 2 || nsName[0] == "" || nsName[1] == "" {
		return errors.New("service-name should be valid string as namespaced-name")
	}
	o.ServiceKey = client.ObjectKey{
		Namespace: nsName[0],
		Name:      nsName[1],
	}
	return nil
}

// Validate validates the ReconcilerOptions.
func (o *ReconcilerOptions) Validate() error {
	if len(o.CRDs) == 0 {
		return errors.New("at least one service need to be enabled")
	}
	switch o.DefaultIssuerKind {
	case IssuerKind, ClusterIssuerKind:
	default:
		return errors.New("unsupported Issuer kind: " + o.DefaultIssuerKind)
	}
	if o.CertificateApplyLimit < 0 {
		return errors.New("certificate-apply-limit must be greater than or equal to 0")
	}
	if o.CertificateApplyRetryBaseDelay <= 0 {
		return errors.New("certificate-apply-retry-base-delay must be greater than 0")
	}
	if o.CertificateApplyRetryMaxDelay <= 0 {
		return errors.New("certificate-apply-retry-max-delay must be greater than 0")
	}
	if o.CertificateApplyRetryMaxDelay < o.CertificateApplyRetryBaseDelay {
		return errors.New("certificate-apply-retry-max-delay must be greater than or equal to certificate-apply-retry-base-delay")
	}
	if o.DefaultDNSTTL < 0 {
		return errors.New("default-dns-ttl must be a positive integer")
	}
	if o.DefaultDNSDelegationTTL < 0 {
		return errors.New("default-dns-delegation-ttl must be a positive integer")
	}
	return nil
}

// SetupScheme initializes a schema
func SetupScheme(scm *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(scm))
	utilruntime.Must(projectcontourv1.AddToScheme(scm))
	utilruntime.Must(cmapiv1.AddToScheme(scm))

	// +kubebuilder:scaffold:scheme
}

// SetupReconciler initializes reconcilers
func SetupReconciler(mgr manager.Manager, scheme *runtime.Scheme, opts ReconcilerOptions) error {
	var certWorker Applier[*cmapiv1.Certificate]
	if opts.CertificateApplyLimit > 0 {
		certWorker = NewCertificateApplyWorker(mgr.GetClient(), opts)
	} else {
		certWorker = NewCertificateApplier(mgr.GetClient())
	}
	_, err := SetupAndGetReconciler(mgr, scheme, opts, certWorker)

	// +kubebuilder:scaffold:builder
	return err
}

// SetupAndGetReconciler initializes reconcilers and return the reconciler struct
func SetupAndGetReconciler(mgr manager.Manager, scheme *runtime.Scheme, opts ReconcilerOptions, certWorker Applier[*cmapiv1.Certificate]) (*HTTPProxyReconciler, error) {
	httpProxyReconciler := &HTTPProxyReconciler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("HTTPProxy"),
		Scheme:            scheme,
		ReconcilerOptions: opts,
		CertApplier:       certWorker,
	}

	err := httpProxyReconciler.SetupWithManager(mgr)
	if err != nil {
		return nil, err
	}

	return httpProxyReconciler, nil
}
