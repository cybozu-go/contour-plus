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

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	externaldnsv1alpha1 "github.com/cybozu-go/contour-plus/api/v1alpha1"
)

// DNSEndpointReconciler reconciles a DNSEndpoint object
type DNSEndpointReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=externaldns.contour-plus.cybozu.com,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=externaldns.contour-plus.cybozu.com,resources=dnsendpoints/status,verbs=get;update;patch

func (r *DNSEndpointReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("dnsendpoint", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *DNSEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&externaldnsv1alpha1.DNSEndpoint{}).
		Complete(r)
}
