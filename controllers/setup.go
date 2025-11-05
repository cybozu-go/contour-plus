package controllers

import (
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
}

// SetupScheme initializes a schema
func SetupScheme(scm *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(scm))
	utilruntime.Must(projectcontourv1.AddToScheme(scm))

	// +kubebuilder:scaffold:scheme
}

// SetupReconciler initializes reconcilers
func SetupReconciler(mgr manager.Manager, scheme *runtime.Scheme, opts ReconcilerOptions) error {
	httpProxyReconciler := &HTTPProxyReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("controllers").WithName("HTTPProxy"),
		Scheme:                  scheme,
		ServiceKey:              opts.ServiceKey,
		Prefix:                  opts.Prefix,
		DefaultIssuerName:       opts.DefaultIssuerName,
		DefaultIssuerKind:       opts.DefaultIssuerKind,
		DefaultDelegatedDomain:  opts.DefaultDelegatedDomain,
		AllowedDelegatedDomains: opts.AllowedDelegatedDomains,
		AllowCustomDelegations:  opts.AllowCustomDelegations,
		CSRRevisionLimit:        opts.CSRRevisionLimit,
		CreateDNSEndpoint:       opts.CreateDNSEndpoint,
		CreateCertificate:       opts.CreateCertificate,
		IngressClassName:        opts.IngressClassName,
		PropagatedAnnotations:   opts.PropagatedAnnotations,
		PropagatedLabels:        opts.PropagatedLabels,
	}
	err := httpProxyReconciler.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	// +kubebuilder:scaffold:builder
	return nil
}
