package controllers

import (
	"context"
	"sync"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ ApplyWorker[*cmv1.Certificate] = &WrappedCertificateApplyWorker{}

type WrappedCertificateApplyWorker struct {
	wrapped *CertificateApplyWorker

	mu sync.Mutex
	// tracked globally since the rate limit is global
	queuedCounter uint64

	// how many times Apply was called for this Certificate
	// tracked per key for the ease of use
	applyCounts map[types.NamespacedName]uint64

	// how many initial attempts should fail before
	// we pass through to the wrapped worker.
	// tracked per key for the ease of use
	failFirst map[types.NamespacedName]uint64
}

func (w *WrappedCertificateApplyWorker) Apply(ctx context.Context, obj *cmv1.Certificate) error {
	objKey := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
	requiresQueue, err := w.wrapped.RequiresQueue(ctx, objKey, obj)
	if err != nil {
		return err
	}

	newObj := func() *cmv1.Certificate {
		w.mu.Lock()
		defer w.mu.Unlock()
		if requiresQueue {
			w.queuedCounter += 1
		}

		if _, ok := w.applyCounts[objKey]; !ok {
			w.applyCounts[objKey] = 0
		}
		w.applyCounts[objKey] += 1

		if _, ok := w.failFirst[objKey]; !ok {
			return obj
		}

		if w.applyCounts[objKey] <= w.failFirst[objKey] {
			// fail by manipulating obj's spec
			newObj := obj.DeepCopy()
			// set invalid algorithm that should be rejected by the apiserver
			// see https://github.com/cert-manager/cert-manager/blob/master/pkg/apis/certmanager/v1/types_certificate.go#L371-L377
			newObj.Spec.PrivateKey = &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.PrivateKeyAlgorithm("SUPERSECURE"),
			}
			return newObj
		}
		return obj
	}()
	return w.wrapped.Apply(ctx, newObj)
}

func (w *WrappedCertificateApplyWorker) Start(ctx context.Context) error {
	return w.wrapped.Start(ctx)
}

func (w *WrappedCertificateApplyWorker) GetRetryChannel() <-chan event.TypedGenericEvent[*projectcontourv1.HTTPProxy] {
	return w.wrapped.GetRetryChannel()
}

func (w *WrappedCertificateApplyWorker) GetQueuedCounter() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.queuedCounter
}

func (w *WrappedCertificateApplyWorker) GetApplyCounts(objKey types.NamespacedName) uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.applyCounts[objKey]
}

func (w *WrappedCertificateApplyWorker) SetFailFirstNForKey(objKey types.NamespacedName, n uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failFirst[objKey] = n
}

func NewWrappedCertificateApplyWorker(wrapped *CertificateApplyWorker) *WrappedCertificateApplyWorker {
	return &WrappedCertificateApplyWorker{
		wrapped:     wrapped,
		applyCounts: make(map[types.NamespacedName]uint64),
		failFirst:   make(map[types.NamespacedName]uint64),
	}
}

func testCertificateApplyWorkerApply() {
	var ns string
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		// Let apiserver generate a unique name to avoid collisions.
		n := &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{
				GenerateName: testNamespacePrefix,
			},
		}
		Expect(k8sClient.Create(ctx, n)).To(Succeed())
		ns = n.Name
	})

	AfterEach(func() {
		// delete any resource created by the previous spec
		Expect(k8sClient.DeleteAllOf(ctx, &projectcontourv1.HTTPProxy{}, client.InNamespace(ns))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, certificate(), client.InNamespace(ns))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, dnsEndpoint(), client.InNamespace(ns))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, tlsCertificateDelegation(), client.InNamespace(ns))).To(Succeed())

		n := &corev1.Namespace{ObjectMeta: ctrl.ObjectMeta{Name: ns}}
		_ = k8sClient.Delete(ctx, n) // this actually does not remove the namespace, it just puts it into terminating state
	})

	It("should create Certificate via workqueue", func() {
		scm, mgr := setupManager()

		prefix := "test-"
		opts := ReconcilerOptions{
			ServiceKey:            testServiceKey,
			Prefix:                prefix,
			DefaultIssuerName:     "test-issuer",
			DefaultIssuerKind:     IssuerKind,
			CreateDNSEndpoint:     true,
			CreateCertificate:     true,
			CertificateApplyLimit: 1,
		}

		applyWorker := NewWrappedCertificateApplyWorker(NewCertificateApplyWorker(mgr.GetClient(), opts))

		r, err := SetupAndGetReconciler(mgr, scm, opts, applyWorker)
		Expect(err).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		By("getting Certificate with prefixed name")
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).WithTimeout(5 * time.Second).Should(Succeed())
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(1)))

		By("creating second HTTPProxy")
		hpKey = client.ObjectKey{Name: "bar", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		objKey = client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		By("getting second Certificate with prefixed name")
		crt = certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).WithTimeout(5 * time.Second).Should(Succeed()) // use longer timeout so that it can wait for the rate limit
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(2))) // should go through queue each create
	})

	It("should update Certificate without workqueue", func() {
		scm, mgr := setupManager()

		prefix := "test-"
		opts := ReconcilerOptions{
			ServiceKey:            testServiceKey,
			Prefix:                prefix,
			DefaultIssuerName:     "test-issuer",
			DefaultIssuerKind:     IssuerKind,
			CreateDNSEndpoint:     true,
			CreateCertificate:     true,
			PropagatedAnnotations: []string{"foo"}, // must propagate an annotation to update cert metadata
			CertificateApplyLimit: 1,
		}

		applyWorker := NewWrappedCertificateApplyWorker(NewCertificateApplyWorker(mgr.GetClient(), opts))

		r, err := SetupAndGetReconciler(mgr, scm, opts, applyWorker)
		Expect(err).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		By("getting Certificate with prefixed name")
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).WithTimeout(5 * time.Second).Should(Succeed())
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(1)))

		By("updating HTTPProxy")

		latest := &projectcontourv1.HTTPProxy{}
		Expect(k8sClient.Get(context.Background(), hpKey, latest)).To(Succeed())
		base := latest.DeepCopy()
		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations["foo"] = "bar"

		Expect(k8sClient.Patch(context.Background(), latest, client.MergeFrom(base))).To(Succeed())

		By("getting Certificate with prefixed name again")
		crt = certificate()
		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), objKey, crt); err != nil {
				return false
			}
			_, ok := crt.GetAnnotations()["foo"]
			return ok
		}).WithTimeout(10 * time.Second).Should(BeTrue())
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(1))) // should go through queue once (create)
	})

	It("should update Certificate via workqueue when spec is updated", func() {
		scm, mgr := setupManager()

		prefix := "test-"
		opts := ReconcilerOptions{
			ServiceKey:            testServiceKey,
			Prefix:                prefix,
			DefaultIssuerName:     "test-issuer",
			DefaultIssuerKind:     IssuerKind,
			CreateDNSEndpoint:     true,
			CreateCertificate:     true,
			PropagatedAnnotations: []string{"foo"}, // must propagate an annotation to update cert metadata
			CertificateApplyLimit: 1,
		}

		applyWorker := NewWrappedCertificateApplyWorker(NewCertificateApplyWorker(mgr.GetClient(), opts))

		r, err := SetupAndGetReconciler(mgr, scm, opts, applyWorker)
		Expect(err).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		By("getting Certificate with prefixed name")
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).WithTimeout(5 * time.Second).Should(Succeed())
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(1)))

		By("updating HTTPProxy")
		latest := &projectcontourv1.HTTPProxy{}
		Expect(k8sClient.Get(context.Background(), hpKey, latest)).To(Succeed())
		base := latest.DeepCopy()
		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations[privateKeyAlgorithmAnnotation] = "ECDSA" // changing privateKeyAlgorithm should trigger reissuance of the Certificate

		Expect(k8sClient.Patch(context.Background(), latest, client.MergeFrom(base))).To(Succeed())

		time.Sleep(5 * time.Second) // wait for the certificate changes to be applied via rate limited queue
		By("getting Certificate with prefixed name again")
		crt = certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).WithTimeout(5 * time.Second).Should(Succeed())
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(2))) // should go through queue twice (create & update)
	})

	It("should create Certificate via workqueue after a retry", func() {
		scm, mgr := setupManager()

		prefix := "test-"
		opts := ReconcilerOptions{
			ServiceKey:            testServiceKey,
			Prefix:                prefix,
			DefaultIssuerName:     "test-issuer",
			DefaultIssuerKind:     IssuerKind,
			CreateDNSEndpoint:     true,
			CreateCertificate:     true,
			CertificateApplyLimit: 1,
		}

		applyWorker := NewWrappedCertificateApplyWorker(NewCertificateApplyWorker(mgr.GetClient(), opts))

		r, err := SetupAndGetReconciler(mgr, scm, opts, applyWorker)
		Expect(err).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		certObjKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		r.CertApplier.(*WrappedCertificateApplyWorker).SetFailFirstNForKey(certObjKey, 1) // CertApply should fail once
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting Certificate with prefixed name")
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), certObjKey, crt)
		}).WithTimeout(5 * time.Second).Should(Succeed())
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(2)))
		Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetApplyCounts(certObjKey)).Should(Equal(uint64(2)))
	})

	It("should create Certificate in the specified namespace via a workqueue after a retry", func() {
		certNsObj := &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{GenerateName: testNamespacePrefix},
		}
		Expect(k8sClient.Create(context.Background(), certNsObj)).ShouldNot(HaveOccurred())
		certNs := certNsObj.Name
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, certNsObj)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: certNs}, &corev1.Namespace{})
				return client.IgnoreNotFound(err) == nil
			}, 10*time.Second).Should(BeTrue())
		})

		scm, mgr := setupManager()

		opts := ReconcilerOptions{
			ServiceKey:              testServiceKey,
			CreateCertificate:       true,
			DefaultIssuerKind:       IssuerKind,
			DefaultIssuerName:       "test-issuer",
			AllowedIssuerNamespaces: []string{certNs},
			CertificateApplyLimit:   1,
		}

		applyWorker := NewWrappedCertificateApplyWorker(NewCertificateApplyWorker(mgr.GetClient(), opts))

		r, err := SetupAndGetReconciler(mgr, scm, opts, applyWorker)
		Expect(err).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with Certificate namespace annotation")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Spec.VirtualHost.TLS = nil
		hp.Annotations[issuerNamespaceAnnotation] = certNs

		certName := hpKey.Namespace + "-" + hpKey.Name
		certObjKey := client.ObjectKey{
			Name:      certName,
			Namespace: certNs,
		}
		r.CertApplier.(*WrappedCertificateApplyWorker).SetFailFirstNForKey(certObjKey, 1) // CertApply should fail once

		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate in the specified namespace")
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), certObjKey, crt)
		}, 5*time.Second).Should(Succeed())
		// NOTE: We are not checking the number of times the item went through the queue here
		// since creating child resources in another namespace involves updating HTTPProxy, causing additional reconciliation.
		// The timing of this reconciliation can vary and cause flakiness in this test case.
		// should apply 2~3 times:
		// 1. first apply
		// 2. triggered by patch of HTTPProxy in `reconcileSecretName`
		// 3. triggered by requeue
		/* Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetQueuedCounter()).Should(Equal(uint64(3))) */
		/* Expect(r.CertApplier.(*WrappedCertificateApplyWorker).GetApplyCounts(certObjKey)).Should(Equal(uint64(3))) */

		By("ensuring HTTPProxy deletion deletes the Certificate")
		Expect(k8sClient.Delete(context.Background(), hp)).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Get(context.Background(), certObjKey, crt)
		}, 5*time.Second).ShouldNot(Succeed())
	})
}
