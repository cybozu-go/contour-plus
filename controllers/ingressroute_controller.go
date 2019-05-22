/*
.
*/

package controllers

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/test/unit/gen"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// IngressRouteReconciler reconciles a IngressRoute object
type IngressRouteReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	ServiceKey client.ObjectKey
}

// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=certificate,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;watch

func (r *IngressRouteReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingressroute", req.NamespacedName)
	var serviceIPs []net.IP

	// Get IP list of loadbalancer Service
	var svc corev1.Service
	err := client.IgnoreNotFound(r.Get(ctx, r.ServiceKey, &svc))
	if err != nil {
		log.Error(err, "unable to get services")
		return ctrl.Result{}, err
	}
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if len(ing.IP) == 0 {
			continue
		}
		serviceIPs = append(serviceIPs, net.ParseIP(ing.IP))
	}

	// Get IngressRoute
	var ir contourv1beta1.IngressRoute
	objKey := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	err = client.IgnoreNotFound(r.Get(ctx, objKey, &ir))
	if err != nil {
		log.Error(err, "unable to list IngressRoute resources")
		return ctrl.Result{}, err
	}

	// Create DNSEndpoint from IngressRoute if do not exist
	var de endpoint.DNSEndpoint
	err = r.Get(ctx, objKey, &de)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to get a DNSEndpoint")
			return ctrl.Result{}, err
		}
		de := newDNSEndpoint(req, ir.Spec.VirtualHost.Fqdn, serviceIPs)
		err = ctrl.SetControllerReference(&ir, de, r.Scheme)
		if err != nil {
			log.Error(err, "unable to set owner reference for DNSEndpoint")
			return ctrl.Result{}, err
		}
		err = r.Create(ctx, de)
		if err != nil {
			log.Error(err, "unable to create a DNSEndpoint")
			return ctrl.Result{}, err
		}
	}

	// Create Certificate from IngressRoute if do not exist
	var crt certmanagerv1alpha1.Certificate
	err = r.Get(ctx, objKey, &crt)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to get a Certificate")
			return ctrl.Result{}, err
		}

		certificate := newCertificate(req)
		err = ctrl.SetControllerReference(&ir, certificate, r.Scheme)
		if err != nil {
			log.Error(err, "unable to set owner reference for Certificate")
			return ctrl.Result{}, err
		}
		err = r.Create(ctx, certificate)
		if err != nil {
			log.Error(err, "unable to create a Certificate")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func newCertificate(req ctrl.Request) *certmanagerv1alpha1.Certificate {
	// TODO: set certificate's field
	crt := gen.Certificate(req.Name)

	crt.SetNamespace(req.Namespace)
	return crt
}

func newDNSEndpoint(req ctrl.Request, hostname string, ips []net.IP) *endpoint.DNSEndpoint {
	ipv4Targets := endpoint.Targets{}
	ipv6Targets := endpoint.Targets{}
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4Targets = append(ipv4Targets, ip.String())
		} else if ip.To16() != nil {
			ipv6Targets = append(ipv6Targets, ip.String())
		}
	}
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
			RecordType: "AAA",
			RecordTTL:  180,
		})
	}

	return &endpoint.DNSEndpoint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1alpha1",
			Kind:       "DNSEndpoint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       req.Name,
			Namespace:  req.Namespace,
			Generation: 1,
		},
		Spec: endpoint.DNSEndpointSpec{
			Endpoints: endpoints,
		},
	}
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

	return ctrl.NewControllerManagedBy(mgr).
		For(&contourv1beta1.IngressRoute{}).
		Watches(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: listIRs}).
		Complete(r)
}
