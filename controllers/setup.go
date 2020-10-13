package controllers

import (
	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/external-dns/endpoint"
	// +kubebuilder:scaffold:imports
)

// ReconcilerOptions is a set of options for reconcilers
type ReconcilerOptions struct {
	ServiceKey        client.ObjectKey
	Prefix            string
	DefaultIssuerName string
	DefaultIssuerKind string
	CreateDNSEndpoint bool
	CreateCertificate bool
	IngressClassName  string
}

// SetupScheme initializes a schema
func SetupScheme(scm *runtime.Scheme) error {
	projectcontourv1.AddKnownTypes(scm)

	// ExternalDNS does not implement AddToScheme
	groupVersion := ctrl.GroupVersion{
		Group:   "externaldns.k8s.io",
		Version: "v1alpha1",
	}
	scm.AddKnownTypes(groupVersion,
		&endpoint.DNSEndpoint{},
		&endpoint.DNSEndpointList{},
	)
	metav1.AddToGroupVersion(scm, groupVersion)

	if err := certmanagerv1alpha2.AddToScheme(scm); err != nil {
		return err
	}

	// for corev1.Service
	if err := corev1.AddToScheme(scm); err != nil {
		return err
	}

	// +kubebuilder:scaffold:scheme
	return nil
}

// SetupReconciler initializes reconcilers
func SetupReconciler(mgr manager.Manager, scheme *runtime.Scheme, opts ReconcilerOptions) error {
	httpProxyReconciler := &HTTPProxyReconciler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("HTTPProxy"),
		Scheme:            scheme,
		ServiceKey:        opts.ServiceKey,
		Prefix:            opts.Prefix,
		DefaultIssuerName: opts.DefaultIssuerName,
		DefaultIssuerKind: opts.DefaultIssuerKind,
		CreateDNSEndpoint: opts.CreateDNSEndpoint,
		CreateCertificate: opts.CreateCertificate,
		IngressClassName:  opts.IngressClassName,
	}
	err := httpProxyReconciler.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	// +kubebuilder:scaffold:builder
	return nil
}
