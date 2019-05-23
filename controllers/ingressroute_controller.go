package controllers

import (
	"context"
	"errors"
	"net"

	"github.com/go-logr/logr"
	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/kubernetes-incubator/external-dns/endpoint"
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
	issuerNameAnnotation        = "certmanager.k8s.io/issuer"
	clusterIssuerNameAnnotation = "certmanager.k8s.io/cluster-issuer"
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
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=certificate,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get

// Reconcile creates/updates CRDs from given IngressRoute
func (r *IngressRouteReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingressroute", req.NamespacedName)

	// Get IngressRoute
	ir := new(contourv1beta1.IngressRoute)
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

func (r *IngressRouteReconciler) reconcileDNSEndpoint(ctx context.Context, ir *contourv1beta1.IngressRoute, log logr.Logger) error {
	if !r.CreateDNSEndpoint {
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
		return errors.New("no IP address for service " + r.ServiceKey.String())
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

func (r *IngressRouteReconciler) reconcileCertificate(ctx context.Context, ir *contourv1beta1.IngressRoute, log logr.Logger) error {
	if !r.CreateCertificate {
		return nil
	}
	if ir.Annotations[testACMETLSAnnotation] != "true" {
		log.Info(`skip to reconcile certificate, because "kubernetes.io/tls-acme" is not "true"`)
		return nil
	}

	crt := &certmanagerv1alpha1.Certificate{}
	crt.SetNamespace(ir.Namespace)
	crt.SetName(r.Prefix + ir.Name)

	issuerName, issuerKind := func() (string, string) {
		name := ""
		kind := ""
		annotations := ir.Annotations
		if ir.Annotations == nil {
			annotations = map[string]string{}
		}
		if issuerName, ok := annotations[issuerNameAnnotation]; ok {
			name = issuerName
			kind = certmanagerv1alpha1.IssuerKind
		}
		if issuerName, ok := annotations[clusterIssuerNameAnnotation]; ok {
			name = issuerName
			kind = certmanagerv1alpha1.ClusterIssuerKind
		}
		return name, kind
	}()

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, crt, func() error {
		crt.Spec.DNSNames = []string{ir.Spec.VirtualHost.Fqdn}
		crt.Spec.SecretName = ir.Spec.VirtualHost.TLS.SecretName
		crt.Spec.CommonName = ir.Spec.VirtualHost.Fqdn
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
			var irList contourv1beta1.IngressRouteList
			err := r.List(ctx, &irList)
			if err != nil {
				r.Log.Error(err, "listing IngressRoute failed")
				return nil
			}

			requests := make([]reconcile.Request, len(irList.Items))
			for i, ir := range irList.Items {
				requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      ir.GetObjectMeta().GetName(),
					Namespace: ir.GetObjectMeta().GetNamespace(),
				}}
			}
			return requests
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&contourv1beta1.IngressRoute{}).
		Owns(&endpoint.DNSEndpoint{}).
		Owns(&certmanagerv1alpha1.Certificate{}).
		Watches(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: listIRs}).
		Watches(&source.Kind{Type: &certmanagerv1alpha1.Issuer{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: listIRs}).
		Complete(r)
}
