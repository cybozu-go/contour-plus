/*
.
*/

package controllers

import (
	"context"
	"errors"
	"net"

	"github.com/go-logr/logr"
	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/test/unit/gen"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const excludeKey = "contour-plus.cybozu.com/exclude"

// IngressRouteReconciler reconciles a IngressRoute object
type IngressRouteReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	ServiceKey        client.ObjectKey
	Prefix            string
	CreateDNSEndpoint bool
	CreateCertificate bool
}

// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes/status,verbs=get
// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=certificate,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get

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

	if ir.Annotations[excludeKey] == "true" {
		return ctrl.Result{}, nil
	}

	err = r.reconcileDNSEndpoint(ctx, ir, log)
	if err != nil {
		log.Error(err, "unable to create/update DNSEndpoint")
		return ctrl.Result{}, err
	}

	err = r.reconcileCertificate(ctx, ir, log)
	if err != nil {
		log.Error(err, "unable to create/update Certificate")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *IngressRouteReconciler) reconcileDNSEndpoint(ctx context.Context, ir *contourv1beta1.IngressRoute, log logr.Logger) error {
	if !r.CreateDNSEndpoint {
		return nil
	}

	// Get IP list of loadbalancer Service
	var serviceIPs []net.IP
	var svc corev1.Service
	err := r.Get(ctx, r.ServiceKey, &svc)
	if k8serrors.IsNotFound(err) {
		log.Info("service is not found")
		return nil
	} else if err != nil {
		log.Error(err, "unable to get services")
		return err
	}
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if len(ing.IP) == 0 {
			continue
		}
		serviceIPs = append(serviceIPs, net.ParseIP(ing.IP))
	}
	if len(serviceIPs) == 0 {
		log.Info("no IP address", "service", r.ServiceKey)
		return errors.New("no IP address for service " + r.ServiceKey.String())
	}

	// Create DNSEndpoint from IngressRoute if do not exist
	objKey := client.ObjectKey{
		Namespace: ir.Namespace,
		Name:      r.Prefix + ir.Name,
	}
	var de endpoint.DNSEndpoint
	err = r.Get(ctx, objKey, &de)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		de := newDNSEndpoint(objKey, ir.Spec.VirtualHost.Fqdn, serviceIPs)
		err = ctrl.SetControllerReference(ir, de, r.Scheme)
		if err != nil {
			return err
		}
		err = r.Create(ctx, de)
		if err != nil {
			return err
		}
	}

	// Update DNSEndpoint if service IPs are changed
	ipv4Targets, ipv6Targets := ipsToTargets(serviceIPs)
	for _, endpoint := range de.Spec.Endpoints {
		if !endpoint.Targets.Same(ipv4Targets) && !endpoint.Targets.Same(ipv6Targets) {
			de := newDNSEndpoint(objKey, ir.Spec.VirtualHost.Fqdn, serviceIPs)
			err = r.Update(ctx, de)
			if err != nil {
				log.Error(err, "unable to update DNSEndpoint")
				return err
			}
			break
		}
	}
	return nil
}

func (r *IngressRouteReconciler) reconcileCertificate(ctx context.Context, ir *contourv1beta1.IngressRoute, log logr.Logger) error {
	if !r.CreateCertificate {
		return nil
	}

	// Create Certificate from IngressRoute if do not exist
	objKey := client.ObjectKey{
		Namespace: ir.Namespace,
		Name:      r.Prefix + ir.Name,
	}
	var crt certmanagerv1alpha1.Certificate
	err := r.Get(ctx, objKey, &crt)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}

		certificate := newCertificate(objKey)
		err = ctrl.SetControllerReference(ir, certificate, r.Scheme)
		if err != nil {
			return err
		}
		err = r.Create(ctx, certificate)
		if err != nil {
			return err
		}
	}
	return nil
}

func newCertificate(objKey client.ObjectKey) *certmanagerv1alpha1.Certificate {
	// TODO: set certificate's field
	crt := gen.Certificate(objKey.Name)

	crt.SetNamespace(objKey.Namespace)
	return crt
}

func newDNSEndpoint(objKey client.ObjectKey, hostname string, ips []net.IP) *endpoint.DNSEndpoint {
	ipv4Targets, ipv6Targets := ipsToTargets(ips)
	var endpoints []*endpoint.Endpoint
	if len(ipv4Targets) != 0 {
		endpoints = append(endpoints, &endpoint.Endpoint{
			DNSName:    hostname,
			Targets:    ipv4Targets,
			RecordType: endpoint.RecordTypeA,
			RecordTTL:  180,
		})
	}
	if len(ipv6Targets) != 0 {
		endpoints = append(endpoints, &endpoint.Endpoint{
			DNSName:    hostname,
			Targets:    ipv6Targets,
			RecordType: "AAAA",
			RecordTTL:  180,
		})
	}

	return &endpoint.DNSEndpoint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1alpha1",
			Kind:       "DNSEndpoint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       objKey.Name,
			Namespace:  objKey.Namespace,
			Generation: 1,
		},
		Spec: endpoint.DNSEndpointSpec{
			Endpoints: endpoints,
		},
	}
}

func ipsToTargets(ips []net.IP) (endpoint.Targets, endpoint.Targets) {
	ipv4Targets := endpoint.Targets{}
	ipv6Targets := endpoint.Targets{}
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4Targets = append(ipv4Targets, ip.String())
		} else if ip.To16() != nil {
			ipv6Targets = append(ipv6Targets, ip.String())
		}
	}
	return ipv4Targets, ipv6Targets
}

func (r *IngressRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	listIRs := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			ctx := context.Background()
			var irList contourv1beta1.IngressRouteList
			_ = r.List(ctx, &irList)
			var requests []reconcile.Request
			for _, ir := range irList.Items {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      ir.GetObjectMeta().GetName(),
					Namespace: ir.GetObjectMeta().GetNamespace(),
				}})
			}
			return requests
		})

	svc := &unstructured.Unstructured{}
	svc.SetNamespace(r.ServiceKey.Namespace)
	svc.SetName(r.ServiceKey.Name)
	svc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	})
	inf, err := mgr.GetCache().GetInformer(svc)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&contourv1beta1.IngressRoute{}).
		Owns(&endpoint.DNSEndpoint{}).
		Owns(&certmanagerv1alpha1.Certificate{}).
		Watches(&source.Informer{Informer: inf}, &handler.EnqueueRequestsFromMapFunc{ToRequests: listIRs}).
		Complete(r)
}
