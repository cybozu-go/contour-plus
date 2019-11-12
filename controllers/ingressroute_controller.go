package controllers

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	excludeAnnotation           = "contour-plus.cybozu.com/exclude"
	testACMETLSAnnotation       = "kubernetes.io/tls-acme"
	issuerNameAnnotation        = "cert-manager.io/issuer"
	clusterIssuerNameAnnotation = "cert-manager.io/cluster-issuer"
)

// IngressRouteReconciler reconciles a IngressRoute object
type IngressRouteReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	ServiceKey        client.ObjectKey
	IssuerKey         client.ObjectKey
	Prefix            string
	DefaultIssuerName string
	DefaultIssuerKind string
	CreateDNSEndpoint bool
	CreateCertificate bool
}

// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes/status,verbs=get
// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get

// Reconcile creates/updates CRDs from given IngressRoute
func (r *IngressRouteReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingressroute", req.NamespacedName)

	// Get IngressRoute
	ir := new(contourv1.IngressRoute)
	objKey := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	err := r.Get(ctx, objKey, ir)
	if k8serrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "unable to get IngressRoute resources")
		return ctrl.Result{}, err
	}

	if ir.Annotations[excludeAnnotation] == "true" {
		return ctrl.Result{}, nil
	}

	err = r.reconcileDNSEndpoint(ctx, ir, log)
	if err != nil {
		log.Error(err, "unable to reconcile DNSEndpoint")
		return ctrl.Result{}, err
	}

	err = r.reconcileCertificate(ctx, ir, log)
	if err != nil {
		log.Error(err, "unable to reconcile Certificate")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *IngressRouteReconciler) reconcileDNSEndpoint(ctx context.Context, ir *contourv1.IngressRoute, log logr.Logger) error {
	if !r.CreateDNSEndpoint {
		return nil
	}

	if ir.Spec.VirtualHost == nil {
		return nil
	}
	fqdn := ir.Spec.VirtualHost.Fqdn
	if len(fqdn) == 0 {
		return nil
	}

	// Get IP list of loadbalancer Service
	var serviceIPs []net.IP
	var svc corev1.Service
	err := r.Get(ctx, r.ServiceKey, &svc)
	if err != nil {
		return err
	}

	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if len(ing.IP) == 0 {
			continue
		}
		serviceIPs = append(serviceIPs, net.ParseIP(ing.IP))
	}
	if len(serviceIPs) == 0 {
		log.Info("no IP address for service " + r.ServiceKey.String())
		// we can return nil here because the controller will be notified
		// as soon as a new IP address is assigned to the service.
		return nil
	}

	de := &endpoint.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ir.Namespace,
			Name:      r.Prefix + ir.Name,
		},
	}
	op, err := ctrl.CreateOrUpdate(ctx, r.Client, de, func() error {
		de.Spec.Endpoints = makeEndpoints(fqdn, serviceIPs)
		return ctrl.SetControllerReference(ir, de, r.Scheme)
	})
	if err != nil {
		return err
	}

	log.Info("DNSEndpoint successfully reconciled", "operation", op)
	return nil
}

func makeEndpoints(hostname string, ips []net.IP) []*endpoint.Endpoint {
	ipv4Targets, ipv6Targets := ipsToTargets(ips)
	var endpoints []*endpoint.Endpoint
	if len(ipv4Targets) != 0 {
		endpoints = append(endpoints, &endpoint.Endpoint{
			DNSName:    hostname,
			Targets:    ipv4Targets,
			RecordType: endpoint.RecordTypeA,
			RecordTTL:  3600,
		})
	}
	if len(ipv6Targets) != 0 {
		endpoints = append(endpoints, &endpoint.Endpoint{
			DNSName:    hostname,
			Targets:    ipv6Targets,
			RecordType: "AAAA",
			RecordTTL:  3600,
		})
	}
	return endpoints
}

func ipsToTargets(ips []net.IP) (endpoint.Targets, endpoint.Targets) {
	ipv4Targets := endpoint.Targets{}
	ipv6Targets := endpoint.Targets{}
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4Targets = append(ipv4Targets, ip.String())
			continue
		}
		ipv6Targets = append(ipv6Targets, ip.String())
	}
	return ipv4Targets, ipv6Targets
}

func (r *IngressRouteReconciler) reconcileCertificate(ctx context.Context, ir *contourv1.IngressRoute, log logr.Logger) error {
	if !r.CreateCertificate {
		return nil
	}
	if ir.Annotations[testACMETLSAnnotation] != "true" {
		return nil
	}

	vh := ir.Spec.VirtualHost
	switch {
	case vh == nil:
		return nil
	case vh.Fqdn == "":
		return nil
	case vh.TLS == nil:
		return nil
	case vh.TLS.SecretName == "":
		return nil
	}

	issuerName := r.DefaultIssuerName
	issuerKind := r.DefaultIssuerKind
	if name, ok := ir.Annotations[issuerNameAnnotation]; ok {
		issuerName = name
		issuerKind = certmanagerv1alpha2.IssuerKind
	}
	if name, ok := ir.Annotations[clusterIssuerNameAnnotation]; ok {
		issuerName = name
		issuerKind = certmanagerv1alpha2.ClusterIssuerKind
	}

	if issuerName == "" {
		log.Info("no issuer name")
		return nil
	}

	crt := &certmanagerv1alpha2.Certificate{}
	crt.SetNamespace(ir.Namespace)
	crt.SetName(r.Prefix + ir.Name)
	op, err := ctrl.CreateOrUpdate(ctx, r.Client, crt, func() error {
		crt.Spec.DNSNames = []string{vh.Fqdn}
		crt.Spec.SecretName = vh.TLS.SecretName
		crt.Spec.CommonName = vh.Fqdn
		crt.Spec.IssuerRef.Name = issuerName
		crt.Spec.IssuerRef.Kind = issuerKind
		return ctrl.SetControllerReference(ir, crt, r.Scheme)
	})
	if err != nil {
		return err
	}

	log.Info("Certificate successfully reconciled", "operation", op)
	return nil
}

// SetupWithManager setup controller manager
func (r *IngressRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	listIRs := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			if a.Meta.GetNamespace() != r.ServiceKey.Namespace {
				return nil
			}
			if a.Meta.GetName() != r.ServiceKey.Name {
				return nil
			}

			ctx := context.Background()
			var irList contourv1.IngressRouteList
			err := r.List(ctx, &irList)
			if err != nil {
				r.Log.Error(err, "listing IngressRoute failed")
				return nil
			}

			requests := make([]reconcile.Request, len(irList.Items))
			for i, ir := range irList.Items {
				requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      ir.Name,
					Namespace: ir.Namespace,
				}}
			}
			return requests
		})

	b := ctrl.NewControllerManagedBy(mgr).
		For(&contourv1.IngressRoute{}).
		Watches(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: listIRs})
	if r.CreateDNSEndpoint {
		b = b.Owns(&endpoint.DNSEndpoint{})
	}
	if r.CreateCertificate {
		b = b.Owns(&certmanagerv1alpha2.Certificate{})
	}
	return b.Complete(r)
}
