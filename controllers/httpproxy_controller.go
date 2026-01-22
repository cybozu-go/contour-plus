package controllers

import (
	"context"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	excludeAnnotation                 = "contour-plus.cybozu.com/exclude"
	testACMETLSAnnotation             = "kubernetes.io/tls-acme"
	issuerNameAnnotation              = "cert-manager.io/issuer"
	clusterIssuerNameAnnotation       = "cert-manager.io/cluster-issuer"
	revisionHistoryLimitAnnotation    = "cert-manager.io/revision-history-limit"
	privateKeyAlgorithmAnnotation     = "cert-manager.io/private-key-algorithm"
	privateKeySizeAnnotation          = "cert-manager.io/private-key-size"
	ingressClassNameAnnotation        = "kubernetes.io/ingress.class"
	contourIngressClassNameAnnotation = "projectcontour.io/ingress.class"
	delegatedDomainAnnotation         = "contour-plus.cybozu.com/delegated-domain"
	dnsNamespaceAnnotation            = "contour-plus.cybozu.com/dns-namespace"
	issuerNamespaceAnnotation         = "contour-plus.cybozu.com/issuer-namespace"
	ownerAnnotation                   = "contour-plus.cybozu.com/owned-by"
	finalizerName                     = "contour-plus.cybozu.com/finalizer"
)

// HTTPProxyReconciler reconciles a HTTPProxy object
type HTTPProxyReconciler struct {
	client.Client
	ReconcilerOptions
	Log    logr.Logger
	Scheme *runtime.Scheme

	CertApplier Applier[*cmv1.Certificate]
}

// +kubebuilder:rbac:groups=projectcontour.io,resources=httpproxies,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=projectcontour.io,resources=httpproxies/status,verbs=get
// +kubebuilder:rbac:groups=projectcontour.io.resources=tlscertificatedelegations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get

// Reconcile creates/updates CRDs from given HTTPProxy
func (r *HTTPProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	// Get HTTPProxy
	hp := new(projectcontourv1.HTTPProxy)
	objKey := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	err := r.Get(ctx, objKey, hp)
	if k8serrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "unable to get HTTPProxy resources")
		return ctrl.Result{}, err
	}

	if hp.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(hp, finalizerName) {
			return ctrl.Result{}, nil
		}
		// Clean up owned resources in other namespaces
		return ctrl.Result{}, r.cleanupCrossNamespaceResources(ctx, hp, log)
	}

	if hp.Annotations[excludeAnnotation] == "true" {
		return ctrl.Result{}, nil
	}

	if r.IngressClassName != "" {
		if !r.isClassNameMatched(hp) {
			return ctrl.Result{}, nil
		}
	}

	if err := r.reconcileDNSEndpoint(ctx, hp, log); err != nil {
		log.Error(err, "unable to reconcile DNSEndpoint")
		return ctrl.Result{}, err
	}

	if err := r.reconcileDelegationDNSEndpoint(ctx, hp, log); err != nil {
		log.Error(err, "unable to reconcile delegation DNSEndpoint")
		return ctrl.Result{}, err
	}

	if err := r.reconcileCertificate(ctx, hp, log); err != nil {
		log.Error(err, "unable to reconcile Certificate")
		return ctrl.Result{}, err
	}

	if err := r.reconcileTLSCertificateDelegation(ctx, hp, log); err != nil {
		log.Error(err, "unable to reconcile TLSCertificateDelegation")
		return ctrl.Result{}, err
	}

	if err := r.reconcileSecretName(ctx, hp, log); err != nil {
		log.Error(err, "unable to reconcile HTTPProxy SecretName")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HTTPProxyReconciler) isClassNameMatched(hp *projectcontourv1.HTTPProxy) bool {
	ingressClassName := hp.Annotations[ingressClassNameAnnotation]
	if ingressClassName != "" {
		if ingressClassName != r.IngressClassName {
			return false
		}
	}

	contourIngressClassName := hp.Annotations[contourIngressClassNameAnnotation]
	if contourIngressClassName != "" {
		if contourIngressClassName != r.IngressClassName {
			return false
		}
	}

	specIngressClassName := hp.Spec.IngressClassName
	if specIngressClassName != "" {
		if specIngressClassName != r.IngressClassName {
			return false
		}
	}

	if contourIngressClassName == "" && ingressClassName == "" && specIngressClassName == "" {
		return false
	}

	return true
}

func (r *HTTPProxyReconciler) reconcileDNSEndpoint(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !r.CreateDNSEndpoint {
		return nil
	}

	if hp.Spec.VirtualHost == nil {
		return nil
	}
	fqdn := hp.Spec.VirtualHost.Fqdn
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

	dnsEndpointName := getDNSEndpointName(r, hp)
	targetNamespace := hp.Namespace
	if ns, ok := hp.Annotations[dnsNamespaceAnnotation]; ok && slices.Contains(r.AllowedDNSNamespaces, ns) {
		targetNamespace = ns
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(externalDNSGroupVersion.WithKind(DNSEndpointKind))
	obj.SetName(dnsEndpointName)
	obj.SetNamespace(targetNamespace)
	obj.SetAnnotations(r.generateObjectAnnotations(hp))
	obj.SetLabels(r.generateObjectLabels(hp))
	obj.UnstructuredContent()["spec"] = map[string]interface{}{
		"endpoints": makeEndpoints(fqdn, serviceIPs),
	}
	err = r.trackResourceOwnership(hp, obj)
	if err != nil {
		return err
	}
	err = r.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: "contour-plus",
	})
	if err != nil {
		return err
	}

	log.Info("DNSEndpoint successfully reconciled")
	return nil
}

func (r *HTTPProxyReconciler) reconcileDelegationDNSEndpoint(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !r.CreateDNSEndpoint {
		return nil
	}

	delegatedDomain := r.DefaultDelegatedDomain
	userDelegatedDomain := hp.Annotations[delegatedDomainAnnotation]
	if userDelegatedDomain != "" && r.AllowCustomDelegations && slices.Contains(r.AllowedDelegatedDomains, userDelegatedDomain) {
		delegatedDomain = userDelegatedDomain
	}

	if delegatedDomain == "" {
		return nil
	}

	if hp.Spec.VirtualHost == nil {
		return nil
	}
	fqdn := hp.Spec.VirtualHost.Fqdn
	if len(fqdn) == 0 {
		return nil
	}

	dnsEndpointName := getDNSEndpointName(r, hp)
	targetNamespace := hp.Namespace
	if ns, ok := hp.Annotations[dnsNamespaceAnnotation]; ok && slices.Contains(r.AllowedDNSNamespaces, ns) {
		targetNamespace = ns
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(externalDNSGroupVersion.WithKind(DNSEndpointKind))
	obj.SetName(dnsEndpointName + "-delegation")
	obj.SetNamespace(targetNamespace)
	obj.SetAnnotations(r.generateObjectAnnotations(hp))
	obj.SetLabels(r.generateObjectLabels(hp))
	obj.UnstructuredContent()["spec"] = map[string]interface{}{
		"endpoints": makeDelegationEndpoint(fqdn, delegatedDomain),
	}

	if err := r.trackResourceOwnership(hp, obj); err != nil {
		return err
	}

	if err := r.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: "contour-plus",
	}); err != nil {
		return err
	}

	log.Info("Delegation DNSEndpoint successfully reconciled")
	return nil
}

func (r *HTTPProxyReconciler) reconcileCertificate(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !r.CreateCertificate {
		return nil
	}
	if hp.Annotations[testACMETLSAnnotation] != "true" {
		return nil
	}

	vh := hp.Spec.VirtualHost
	switch {
	case vh == nil:
		return nil
	case vh.Fqdn == "":
		return nil
	}
	secretName := getCertificateSecretName(r, hp)
	if secretName == "" {
		return nil
	}

	issuerName := r.DefaultIssuerName
	issuerKind := r.DefaultIssuerKind
	if name, ok := hp.Annotations[issuerNameAnnotation]; ok {
		issuerName = name
		issuerKind = IssuerKind
	}
	if name, ok := hp.Annotations[clusterIssuerNameAnnotation]; ok {
		issuerName = name
		issuerKind = ClusterIssuerKind
	}

	if issuerName == "" {
		log.Info("no issuer name")
		return nil
	}

	certificateSpec := cmv1.CertificateSpec{
		DNSNames:   []string{vh.Fqdn},
		SecretName: secretName,
		CommonName: vh.Fqdn,
		IssuerRef: cmmeta.IssuerReference{
			Kind: issuerKind,
			Name: issuerName,
		},
		Usages: []cmv1.KeyUsage{
			cmv1.UsageDigitalSignature,
			cmv1.UsageKeyEncipherment,
			cmv1.UsageServerAuth,
		},
	}

	if r.CSRRevisionLimit > 0 {
		certificateSpec.RevisionHistoryLimit = ptr.To(int32(r.CSRRevisionLimit))
	}
	if value, ok := hp.Annotations[revisionHistoryLimitAnnotation]; ok {
		limit, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			log.Error(err, "invalid revisionHistoryLimit", "value", value)
			return nil
		}
		certificateSpec.RevisionHistoryLimit = ptr.To(int32(limit))
	}
	secretTemplate := &cmv1.CertificateSecretTemplate{}
	annotations := r.generateObjectAnnotations(hp)
	if annotations != nil {
		secretTemplate.Annotations = annotations
	}
	labels := r.generateObjectLabels(hp)
	if labels != nil {
		secretTemplate.Labels = labels
	}
	if secretTemplate.Annotations != nil || secretTemplate.Labels != nil {
		certificateSpec.SecretTemplate = secretTemplate
	}

	if algorithm, ok := hp.Annotations[privateKeyAlgorithmAnnotation]; ok {
		privateKeySpec := &cmv1.CertificatePrivateKey{
			Algorithm: cmv1.PrivateKeyAlgorithm(algorithm),
		}
		if value, ok := hp.Annotations[privateKeySizeAnnotation]; ok {
			size, err := strconv.ParseUint(value, 10, 32)
			if err == nil {
				privateKeySpec.Size = int(size)
			} else {
				log.Error(err, "invalid privateKey size", "value", value)
			}
		}
		certificateSpec.PrivateKey = privateKeySpec
	}

	certificateName := getCertificateName(r, hp)
	targetNamespace := hp.Namespace
	if ns, ok := hp.Annotations[issuerNamespaceAnnotation]; ok && slices.Contains(r.AllowedIssuerNamespaces, ns) {
		targetNamespace = ns
	}

	obj := &cmv1.Certificate{}
	obj.SetGroupVersionKind(certManagerGroupVersion.WithKind(CertificateKind))
	obj.SetName(certificateName)
	obj.SetNamespace(targetNamespace)
	obj.Spec = certificateSpec

	obj.SetAnnotations(annotations)
	obj.SetLabels(labels)

	err := r.trackResourceOwnership(hp, obj)
	if err != nil {
		return err
	}
	return r.CertApplier.Apply(ctx, obj)
}

// generateObjectAnnotations creates a map that contains annotations that should be propagated to child resources from HTTPProxy.
// The map can be used to set annotations on unstructured.Unstructured.
// Returns uninitizalied map (nil) when the map is empty to avoid SSA patching with empty map.
func (r *HTTPProxyReconciler) generateObjectAnnotations(hp *projectcontourv1.HTTPProxy) map[string]string {
	annotations := map[string]string{}
	for _, key := range r.PropagatedAnnotations {
		if annotation, ok := hp.Annotations[key]; ok {
			annotations[key] = annotation
		}
	}
	if len(annotations) == 0 {
		return nil
	}
	return annotations
}

// generateObjectLabels creates a map that contains labels that should be propagated to child resources from HTTPProxy.
// The map can be used to set labels on unstructured.Unstructured.
// Returns uninitizalied map (nil) when the map is empty to avoid SSA patching with empty map.
func (r *HTTPProxyReconciler) generateObjectLabels(hp *projectcontourv1.HTTPProxy) map[string]string {
	labels := map[string]string{}
	for _, key := range r.PropagatedLabels {
		if label, ok := hp.Labels[key]; ok {
			labels[key] = label
		}
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func (r *HTTPProxyReconciler) reconcileTLSCertificateDelegation(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	namespace, ok := hp.Annotations[issuerNamespaceAnnotation]
	if !ok || !slices.Contains(r.AllowedIssuerNamespaces, namespace) {
		return nil
	}
	certificateName := getCertificateName(r, hp)
	delegationSpec := map[string]interface{}{
		"delegations": []map[string]interface{}{
			{
				"secretName": certificateName,
				"targetNamespaces": []string{
					hp.Namespace,
				},
			},
		},
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(contourGroupVersion.WithKind(TLSCertificateDelegationKind))
	obj.SetName(certificateName)
	obj.SetNamespace(namespace)
	obj.SetAnnotations(r.generateObjectAnnotations(hp))
	obj.SetLabels(r.generateObjectLabels(hp))
	obj.UnstructuredContent()["spec"] = delegationSpec
	err := r.trackResourceOwnership(hp, obj)
	if err != nil {
		return err
	}
	err = r.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: "contour-plus",
	})
	if err != nil {
		return err
	}

	log.Info("TLSCertificateDelegation successfully reconciled")
	return nil
}

func (r *HTTPProxyReconciler) reconcileSecretName(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	certNamespace, ok := hp.Annotations[issuerNamespaceAnnotation]
	if !ok || !slices.Contains(r.AllowedIssuerNamespaces, certNamespace) {
		return nil
	}
	certificateName := getCertificateName(r, hp)
	if hp.Spec.VirtualHost.TLS == nil {
		hp.Spec.VirtualHost.TLS = &projectcontourv1.TLS{}
	}
	hp.Spec.VirtualHost.TLS.SecretName = certNamespace + "/" + certificateName

	err := r.Patch(ctx, hp, client.Merge)
	if err != nil {
		return err
	}

	log.Info("HTTPProxy SecretName successfully reconciled")
	return nil
}

func (r *HTTPProxyReconciler) trackResourceOwnership(hp *projectcontourv1.HTTPProxy, obj client.Object) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[ownerAnnotation] = hp.Namespace + "/" + hp.Name
	obj.SetAnnotations(annotations)

	if obj.GetNamespace() == hp.Namespace {
		return ctrl.SetControllerReference(hp, obj, r.Scheme)
	}

	if !controllerutil.ContainsFinalizer(hp, finalizerName) {
		controllerutil.AddFinalizer(hp, finalizerName)
		err := r.Update(context.Background(), hp)
		return err
	}
	return nil
}

func (r *HTTPProxyReconciler) cleanupCrossNamespaceResources(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !controllerutil.ContainsFinalizer(hp, finalizerName) {
		return nil
	}

	if err := r.cleanupCrossNamespaceDNSEndpoints(ctx, hp, log); err != nil {
		return err
	}

	if err := r.cleanupCrossNamespaceCertificates(ctx, hp, log); err != nil {
		return err
	}

	if err := r.cleanupCrossNamespaceTLSCertificateDelegations(ctx, hp, log); err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(hp, finalizerName)
	return r.Update(ctx, hp)
}

func (r *HTTPProxyReconciler) cleanupCrossNamespaceDNSEndpoints(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !r.CreateDNSEndpoint {
		return nil
	}

	deNs, ok := hp.Annotations[dnsNamespaceAnnotation]
	if !ok || !slices.Contains(r.AllowedDNSNamespaces, deNs) {
		return nil
	}

	del := &unstructured.UnstructuredList{}
	del.SetGroupVersionKind(externalDNSGroupVersion.WithKind(DNSEndpointKind))
	err := r.List(ctx, del, &client.ListOptions{Namespace: deNs})
	if err != nil {
		return err
	}

	for _, de := range del.Items {
		annotations := de.GetAnnotations()
		owner, ok := annotations[ownerAnnotation]
		if !ok || owner != hp.Namespace+"/"+hp.Name {
			continue
		}

		err := r.Delete(ctx, &de)
		if err != nil && !k8serrors.IsNotFound(err) {
			log.Error(err, "failed to delete cross-namespace DNSEndpoint", "name", de.GetName(), "namespace", de.GetNamespace())
			return err
		}
		log.Info("deleted cross-namespace DNSEndpoint", "name", de.GetName(), "namespace", de.GetNamespace())
	}
	return nil
}

func (r *HTTPProxyReconciler) cleanupCrossNamespaceCertificates(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !r.CreateCertificate {
		return nil
	}

	issuerNs, ok := hp.Annotations[issuerNamespaceAnnotation]
	if !ok || !slices.Contains(r.AllowedIssuerNamespaces, issuerNs) {
		return nil
	}

	certList := &unstructured.UnstructuredList{}
	certList.SetGroupVersionKind(certManagerGroupVersion.WithKind(CertificateKind))
	err := r.List(ctx, certList, &client.ListOptions{Namespace: issuerNs})
	if err != nil {
		return err
	}

	for _, cert := range certList.Items {
		annotations := cert.GetAnnotations()
		owner, ok := annotations[ownerAnnotation]
		if !ok || owner != hp.Namespace+"/"+hp.Name {
			continue
		}

		err := r.Delete(ctx, &cert)
		if err != nil && !k8serrors.IsNotFound(err) {
			log.Error(err, "failed to delete cross-namespace Certificate", "name", cert.GetName(), "namespace", cert.GetNamespace())
			return err
		}
		log.Info("deleted cross-namespace Certificate", "name", cert.GetName(), "namespace", cert.GetNamespace())
	}
	return nil
}

func (r *HTTPProxyReconciler) cleanupCrossNamespaceTLSCertificateDelegations(ctx context.Context, hp *projectcontourv1.HTTPProxy, log logr.Logger) error {
	if !r.CreateCertificate {
		return nil
	}

	issuerNs, ok := hp.Annotations[issuerNamespaceAnnotation]
	if !ok || !slices.Contains(r.AllowedIssuerNamespaces, issuerNs) {
		return nil
	}

	tcdList := &unstructured.UnstructuredList{}
	tcdList.SetGroupVersionKind(contourGroupVersion.WithKind(TLSCertificateDelegationListKind))
	err := r.List(ctx, tcdList, &client.ListOptions{Namespace: issuerNs})
	if err != nil {
		return err
	}

	for _, tcd := range tcdList.Items {
		annotations := tcd.GetAnnotations()
		owner, ok := annotations[ownerAnnotation]
		if !ok || owner != hp.Namespace+"/"+hp.Name {
			continue
		}

		err := r.Delete(ctx, &tcd)
		if err != nil && !k8serrors.IsNotFound(err) {
			log.Error(err, "failed to delete cross-namespace TLSCertificateDelegation", "name", tcd.GetName(), "namespace", tcd.GetNamespace())
			return err
		}
		log.Info("deleted cross-namespace TLSCertificateDelegation", "name", tcd.GetName(), "namespace", tcd.GetNamespace())
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HTTPProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// start worker if CertApplier requires one
	if certWorker, ok := r.CertApplier.(ApplyWorker[*cmv1.Certificate]); ok {
		if err := mgr.Add(certWorker); err != nil {
			return err
		}
	}
	listHPs := func(ctx context.Context, a client.Object) []reconcile.Request {
		if a.GetNamespace() != r.ServiceKey.Namespace {
			return nil
		}
		if a.GetName() != r.ServiceKey.Name {
			return nil
		}

		var hpList projectcontourv1.HTTPProxyList
		err := r.List(ctx, &hpList)
		if err != nil {
			r.Log.Error(err, "listing HTTPProxy failed")
			return nil
		}

		requests := make([]reconcile.Request, len(hpList.Items))
		for i, hp := range hpList.Items {
			requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      hp.Name,
				Namespace: hp.Namespace,
			}}
		}
		return requests
	}

	// Spec OR metadata changed => reconcile.
	// Status-only (or no-op) update => ignore.
	specOrMetadataChanged := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld
			newObj := e.ObjectNew

			// Check for spec changes
			if oldObj.GetGeneration() != newObj.GetGeneration() {
				return true
			}

			// Must check for Labels and Annotations changes
			if !reflect.DeepEqual(oldObj.GetLabels(), newObj.GetLabels()) {
				return true
			}
			if !reflect.DeepEqual(oldObj.GetAnnotations(), newObj.GetAnnotations()) {
				return true
			}

			// Must be status-only or no-op update
			return false
		},
	}

	// should only be used in .Owns
	ignoreChildCreateEvent := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
	}

	ignoreInitialCreateEvent := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return !e.IsInInitialList
		},
	}

	// specOrMetadataChanged predicate is added so that only spec and metadata changes result in a workqueue event.
	// ignoreInitialCreateEvent is added to guarantee that only one workqueue event is queued for each HTTPProxy at controller startup.
	// This may not be necessary most of the time since the events will be coalesced in the workqueue while waiting for the controller to start.
	// That being said, this is added to avoid any race condition between Service watch and HTTPProxy watch causing event coalescence to fail in the workqueue.
	// retryCh is used by CertApplier to requeue HTTPProxy when the apply for Certificate fails
	b := ctrl.NewControllerManagedBy(mgr).
		For(&projectcontourv1.HTTPProxy{}, builder.WithPredicates(specOrMetadataChanged)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(listHPs), builder.WithPredicates(ignoreInitialCreateEvent))

	// add retry logic for cert worker.
	// this allows requeing HTTPProxy back into the main workqueue when applying Certificate resouce from cert worker fails
	if certWorker, ok := r.CertApplier.(ApplyWorker[*cmv1.Certificate]); ok {
		b = b.WatchesRawSource(source.Channel(certWorker.GetRetryChannel(), &handler.TypedEnqueueRequestForObject[*projectcontourv1.HTTPProxy]{}))
	}

	// DNSEndpoint, Certificate, & TLSCertificateDelegation resource should emit HTTPProxy workqueue event only when their specs have changed.
	// predicate.GenerationChangedPredicate ignores any status or metadata updates made by other controllers and
	// ignoreChildCreateEvent ignores events emitted when contour-plus creates these resources.
	// This leaves only spec update and object delete events capable of emitting HTTPProxy workqueue event.
	// WARNING: spec change mady by contour-plus will still emit HTTPPRoxy event.
	// If the new release of contour-plus is expected to update the spec of child component, bear in mind that it will trigger another reconciliation for each resource with spec update.
	if r.CreateDNSEndpoint {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(externalDNSGroupVersion.WithKind(DNSEndpointKind))
		b = b.Owns(obj, builder.WithPredicates(ignoreChildCreateEvent, predicate.GenerationChangedPredicate{}))
	}
	if r.CreateCertificate {
		b = b.Owns(&cmv1.Certificate{}, builder.WithPredicates(ignoreChildCreateEvent, predicate.GenerationChangedPredicate{}))
		tcdObj := &unstructured.Unstructured{}
		tcdObj.SetGroupVersionKind(contourGroupVersion.WithKind(TLSCertificateDelegationKind))
		b = b.Owns(tcdObj, builder.WithPredicates(ignoreChildCreateEvent, predicate.GenerationChangedPredicate{}))
	}
	return b.Complete(r)
}

func makeEndpoints(hostname string, ips []net.IP) []map[string]interface{} {
	ipv4Targets, ipv6Targets := ipsToTargets(ips)
	var endpoints []map[string]interface{}
	if len(ipv4Targets) != 0 {
		endpoints = append(endpoints, map[string]interface{}{
			"dnsName":    hostname,
			"targets":    ipv4Targets,
			"recordType": "A",
			"recordTTL":  3600,
		})
	}
	if len(ipv6Targets) != 0 {
		endpoints = append(endpoints, map[string]interface{}{
			"dnsName":    hostname,
			"targets":    ipv6Targets,
			"recordType": "AAAA",
			"recordTTL":  3600,
		})
	}
	return endpoints
}

func ipsToTargets(ips []net.IP) ([]string, []string) {
	var ipv4Targets []string
	var ipv6Targets []string
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4Targets = append(ipv4Targets, ip.String())
			continue
		}
		ipv6Targets = append(ipv6Targets, ip.String())
	}
	return ipv4Targets, ipv6Targets
}

func makeDelegationEndpoint(hostname, delegatedDomain string) []map[string]interface{} {
	fqdn := strings.Trim(hostname, ".")
	return []map[string]interface{}{
		{
			"dnsName":    "_acme-challenge." + fqdn,
			"targets":    []string{"_acme-challenge." + fqdn + "." + delegatedDomain},
			"recordType": "CNAME",
			"recordTTL":  3600,
		},
	}
}

func getCertificateName(r *HTTPProxyReconciler, hp *projectcontourv1.HTTPProxy) string {
	certNamespace, ok := hp.Annotations[issuerNamespaceAnnotation]
	if !ok || certNamespace == "" || certNamespace == hp.Namespace {
		return r.Prefix + hp.Name
	}
	return r.Prefix + hp.Namespace + "-" + hp.Name
}

func getCertificateSecretName(r *HTTPProxyReconciler, hp *projectcontourv1.HTTPProxy) string {
	certNamespace, ok := hp.Annotations[issuerNamespaceAnnotation]
	if !ok || certNamespace == "" || certNamespace == hp.Namespace {
		if hp.Spec.VirtualHost.TLS == nil {
			return ""
		}
		return hp.Spec.VirtualHost.TLS.SecretName
	}
	return r.Prefix + hp.Namespace + "-" + hp.Name
}

func getDNSEndpointName(r *HTTPProxyReconciler, hp *projectcontourv1.HTTPProxy) string {
	deNamespace, ok := hp.Annotations[dnsNamespaceAnnotation]
	if !ok || deNamespace == "" || deNamespace == hp.Namespace {
		return r.Prefix + hp.Name
	}
	return r.Prefix + hp.Namespace + "-" + hp.Name
}
