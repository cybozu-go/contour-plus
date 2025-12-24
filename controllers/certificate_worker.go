package controllers

import (
	"context"
	"fmt"
	"math"
	"sync"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Applier[T client.Object] interface {
	Apply(ctx context.Context, obj T) error
}

var _ Applier[*cmv1.Certificate] = &CertificateApplier{}

// CertificateApplier implements ApplyWorker[cmv1.Certificate] without a workqueue.
// Any objects applied with Apply method will be applied without going through a queue.
// Start method returns error unconditionally, as no worker should be started.
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

type ApplyWorker[T client.Object] interface {
	Applier[T]
	// manager.Runnable defines signature for Start
	manager.Runnable
}

var _ ApplyWorker[*cmv1.Certificate] = &CertificateApplyWorker{}

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
}

func NewCertificateApplyWorker(client client.Client, opt ReconcilerOptions) *CertificateApplyWorker {
	limiter := rate.NewLimiter(rate.Limit(opt.CertificateApplyLimit), max(int(math.Ceil(opt.CertificateApplyLimit)), 1))
	global := &workqueue.TypedBucketRateLimiter[types.NamespacedName]{Limiter: limiter}
	workqueue := workqueue.NewTypedRateLimitingQueueWithConfig(
		global,
		workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{
			Name: "certificate-apply",
		},
	)
	return &CertificateApplyWorker{
		ReconcilerOptions: opt,
		client:            client,
		workqueue:         workqueue,
		manifests:         make(map[types.NamespacedName]*cmv1.Certificate),
		limiter:           limiter,
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

		func() {
			defer w.workqueue.Done(objKey)

			w.mu.Lock()
			defer w.mu.Unlock()
			obj, ok := w.manifests[objKey]
			if !ok {
				log.Error(fmt.Errorf("cannot find certificate manifest for %s", objKey.String()), "cert apply failed", "key", objKey.String())
				return
			}

			if err := applyCertificate(ctx, w.client, obj); err != nil {
				log.Error(err, "cert apply failed", "key", objKey.String())
				w.workqueue.AddRateLimited(objKey)
				return
			}

			log.Info("cert applied from queue", "key", objKey.String())
			w.workqueue.Forget(objKey)
			delete(w.manifests, objKey)
		}()
	}
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
	if equality.Semantic.DeepEqual(obj.Spec, current.Spec) {
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

// applyCertificate applies provided certificate object with provided context and apiserver client
func applyCertificate(ctx context.Context, k8sClient client.Client, obj *cmv1.Certificate) error {
	if err := k8sClient.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: "contour-plus",
	}); err != nil {
		return err
	}
	return nil
}
