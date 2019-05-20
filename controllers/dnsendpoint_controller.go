/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	externaldnsv1alpha1 "github.com/cybozu-go/contour-plus/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/heptio/contour/apis/contour/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DNSEndpointReconciler reconciles a DNSEndpoint object
type DNSEndpointReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=externaldns.contour-plus.cybozu.com,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=externaldns.contour-plus.cybozu.com,resources=dnsendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=contour.heptio.com,resources=dnsendpoints,verbs=get;list
// +kubebuilder:rbac:groups=contour.heptio.com,resources=dnsendpoints/status,verbs=get
// +kubebuilder:rbac:groups=,resources=services,verbs=get;list

func (r *DNSEndpointReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("dnsendpoint", req.NamespacedName)

	var deList externaldnsv1alpha1.DNSEndpointList
	if err := r.List(ctx, &deList, client.InNamespace(req.Name)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var irList v1beta1.IngressRouteList
	if err := r.List(ctx, &irList, client.InNamespace(req.Name)); err != nil {
		log.Error(err, "unable to fetch IngressRouteList")
		return ctrl.Result{}, err
	}

	for _, ir := range irList.Items {
		if deList.Find(ir.Spec.VirtualHost.Fqdn) == nil {
			endpoint := externaldnsv1alpha1.NewEndpoint(ir.Spec.VirtualHost.Fqdn, "A", "0.0.0.0")
			dnsEndpoint := externaldnsv1alpha1.DNSEndpoint{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1alpha1",
					Kind:       "DNSEndpoint",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ir.Name,
					Namespace: ir.Namespace,
				},
				Spec: externaldnsv1alpha1.DNSEndpointSpec{
					Endpoints: []*externaldnsv1alpha1.Endpoint{endpoint},
				},
			}
			err := r.Create(ctx, &dnsEndpoint)
			if err != nil {
				log.Error(err, "unable to create DNSEndpoint for IngressRoute", "DNSEndpoint", dnsEndpoint)
				return ctrl.Result{}, err
			}
			log.V(1).Info("created DNSEndpoint for IngressRoute", "DNSEndpoint", dnsEndpoint)
		}

	}
	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *DNSEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.IngressRoute{}).
		Owns(&externaldnsv1alpha1.DNSEndpoint{}).
		Complete(r)
}
