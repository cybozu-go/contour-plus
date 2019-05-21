/*
.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
)

// IngressRouteReconciler reconciles a IngressRoute object
type IngressRouteReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=contour.heptio.com,resources=ingressroutes/status,verbs=get;update;patch

func (r *IngressRouteReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("ingressroute", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *IngressRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&contourv1beta1.IngressRoute{}).
		Complete(r)
}
