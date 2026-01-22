package controllers

import (
	cmapiv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
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
	ServiceKey              client.ObjectKey
	Prefix                  string
	DefaultIssuerName       string
	DefaultIssuerKind       string
	DefaultDelegatedDomain  string
	AllowedDelegatedDomains []string
	AllowCustomDelegations  bool
	CSRRevisionLimit        uint
	CreateDNSEndpoint       bool
	CreateCertificate       bool
	IngressClassName        string
	PropagatedAnnotations   []string
	PropagatedLabels        []string
	AllowedDNSNamespaces    []string
	AllowedIssuerNamespaces []string
	CertificateApplyLimit   float64
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
