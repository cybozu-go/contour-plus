package controllers

import (
	"context"
	"fmt"
	"math"
	"sync"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Applier[T client.Object] interface {
	Apply(ctx context.Context, obj T) error
}

var _ Applier[*cmv1.Certificate] = &CertificateApplier{}

// CertificateApplier implements Applier[cmv1.Certificate] without a workqueue.
// Any objects applied with Apply method will be applied without going through a queue.
type CertificateApplier struct {
	client client.Client
}

func (w *CertificateApplier) Apply(ctx context.Context, obj *cmv1.Certificate) error {
	return applyCertificate(ctx, w.client, obj)
}

func NewCertificateApplier(client client.Client) *CertificateApplier {
	return &CertificateApplier{
		client: client,
	}
}

// ApplyWorker is Applier with Start method so that it can be used directly by manager.Manager.Add
// Imlement ApplyWorker if the Applier requires a worker that runs in a background and start it via controller manager.
type ApplyWorker[T client.Object] interface {
	Applier[T]
	// manager.Runnable defines signature for Start
	manager.Runnable
	// GetRetryChannel should return a receive only channel that can be used by the main reconciliation loop.
	// For e.g. by WatchesRawSource and source.Channel in SetupWithManager to add a retry path.
	// NOTE: could use second generics type instead of HTTPProxy if we want to use this somewhere else.
	GetRetryChannel() <-chan event.TypedGenericEvent[*projectcontourv1.HTTPProxy]
}

var _ ApplyWorker[*cmv1.Certificate] = &CertificateApplyWorker{}

// CertificateApplyWorker implements Applier and ApplyWorker.
type CertificateApplyWorker struct {
	mu sync.Mutex
	ReconcilerOptions
	client client.Client
	// internal workqueue for rate limiting Certificate changes
	workqueue workqueue.TypedRateLimitingInterface[types.NamespacedName]
	// manifests contains full client.Object that should be applied for the key
	manifests map[types.NamespacedName]*cmv1.Certificate
	// limiter is the underlying rate limiter used by the workqueue
	limiter *rate.Limiter
	// channel for queueing HTTPProxy back into main reconcile loop for a retry
	retryCh chan event.TypedGenericEvent[*projectcontourv1.HTTPProxy]
}

func NewCertificateApplyWorker(client client.Client, opt ReconcilerOptions) *CertificateApplyWorker {
	limit := opt.CertificateApplyLimit

	var limiter *rate.Limiter
	if limit <= 0 {
		// Unlimited: allow everything, burst doesn’t really matter in this case
		limiter = rate.NewLimiter(rate.Inf, 1)
	} else {
		burst := max(int(math.Ceil(limit)), 1)
		limiter = rate.NewLimiter(rate.Limit(limit), burst)
	}

	global := &workqueue.TypedBucketRateLimiter[types.NamespacedName]{Limiter: limiter}
	workqueue := workqueue.NewTypedRateLimitingQueueWithConfig(
		global,
		workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{
			Name: "certificate-apply",
		},
	)
	retryCh := make(chan event.TypedGenericEvent[*projectcontourv1.HTTPProxy], 10)
	return &CertificateApplyWorker{
		ReconcilerOptions: opt,
		client:            client,
		workqueue:         workqueue,
		manifests:         make(map[types.NamespacedName]*cmv1.Certificate),
		limiter:           limiter,
		retryCh:           retryCh,
	}
}

func (w *CertificateApplyWorker) Apply(ctx context.Context, obj *cmv1.Certificate) error {
	log := crlog.FromContext(ctx)
	objKey := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
	requiresQueue, err := w.RequiresQueue(ctx, objKey, obj)
	if err != nil {
		return err
	}
	if requiresQueue {
		log.Info("cert queued for apply", "key", objKey.String())
		w.enqueueCertificate(objKey, obj)
		return nil
	}
	log.Info("cert applied without queueing", "key", objKey.String())
	return applyCertificate(ctx, w.client, obj)
}

func (w *CertificateApplyWorker) Start(ctx context.Context) error {
	log := crlog.FromContext(ctx)
	go func() {
		<-ctx.Done()
		log.Info("context.Done received. Shutting down certificate apply worker.")
		w.workqueue.ShutDown()
	}()

	for {
		objKey, shutdown := w.workqueue.Get()
		if shutdown {
			return nil
		}
		log.Info("processing cert queue item", "key", objKey.String())

		func() {
			// there is no need to call .Forget since we are only using BucketRateLimiter
			defer w.workqueue.Done(objKey)

			w.mu.Lock()
			obj, ok := w.manifests[objKey]
			delete(w.manifests, objKey)
			w.mu.Unlock()

			if !ok {
				log.Error(fmt.Errorf("cannot find certificate manifest for %s", objKey.String()), "cert apply failed", "key", objKey.String())
				return
			}

			if ctx.Err() != nil {
				log.Info("context cancelled, skipping cert apply", "key", objKey.String())
				return
			}

			if err := applyCertificate(ctx, w.client, obj); err != nil {
				log.Error(err, "cert apply failed", "key", objKey.String())
				w.enqueueHTTPProxy(ctx, obj)
				return
			}

			log.Info("cert applied from queue", "key", objKey.String())
		}()
	}
}

func (w *CertificateApplyWorker) GetRetryChannel() <-chan event.TypedGenericEvent[*projectcontourv1.HTTPProxy] {
	return w.retryCh
}

// RequiresQueue indicates whether the object should be queued or not.
func (w *CertificateApplyWorker) RequiresQueue(ctx context.Context, key types.NamespacedName, obj *cmv1.Certificate) (bool, error) {
	log := crlog.FromContext(ctx)

	current := new(cmv1.Certificate)
	err := w.client.Get(ctx, key, current)
	if k8serrors.IsNotFound(err) {
		// MUST be queued with rate limit
		return true, nil
	}
	if err != nil {
		log.Error(err, "unable to get Certificate resource")
		return false, err
	}
	// MUST COMPARE specs of desired and current to see if there will be re-issuance of the Certificate
	// can safely Ignore spec.secretTemplate changes as they are only secret metadata change and does not trigger re-issuance
	objCopy := obj.DeepCopy()
	objCopy.Spec.SecretTemplate = nil
	current.Spec.SecretTemplate = nil
	if equality.Semantic.DeepEqual(objCopy.Spec, current.Spec) {
		// no-reissuance, safe to patch without rate limit
		return false, nil
	}
	return true, nil
}

func (w *CertificateApplyWorker) enqueueCertificate(objKey types.NamespacedName, obj *cmv1.Certificate) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.manifests[objKey] = obj
	w.workqueue.AddRateLimited(objKey)
}

// enqueueHTTPProxy enqueues HTTPProxy in to the workqueue used by the main Reconcile function.
// It requires the Controller to be setup with .WatchesRawSource using the same channel as w.retryCh
func (w *CertificateApplyWorker) enqueueHTTPProxy(ctx context.Context, obj *cmv1.Certificate) {
	log := crlog.FromContext(ctx)
	if w.retryCh == nil {
		return
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		log.Error(fmt.Errorf("annotation does not exist"), "skipping HTTPProxy enqueue", "certificateName", obj.Name, "certificateNamespace", obj.Namespace)
		return
	}
	owner, ok := annotations[ownerAnnotation]
	if !ok {
		log.Error(fmt.Errorf("annotation not found for %s", ownerAnnotation), "skipping HTTPProxy enqueue", "certificateName", obj.Name, "certificateNamespace", obj.Namespace)
		return
	}

	ns, name, err := cache.SplitMetaNamespaceKey(owner)
	if err != nil {
		log.Error(err, "skipping HTTPProxy enqueue", "certificateName", obj.Name, "certificateNamespace", obj.Namespace)
		return
	}
	h := projectcontourv1.HTTPProxy{}
	h.SetNamespace(ns)
	h.SetName(name)
	log.Info("re-queueing HTTPProxy", "namespace", ns, "name", name)
	w.retryCh <- event.TypedGenericEvent[*projectcontourv1.HTTPProxy]{
		Object: &h,
	}
}

// applyCertificate applies provided certificate object with provided context and apiserver client
func applyCertificate(ctx context.Context, k8sClient client.Client, obj *cmv1.Certificate) error {
	return k8sClient.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: "contour-plus",
	})
}
