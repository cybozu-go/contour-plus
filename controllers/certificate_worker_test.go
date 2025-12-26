package controllers

import (
	"context"
	"sync/atomic"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WrappedCertificateApplyWorker struct {
	wrapped       *CertificateApplyWorker
	queuedCounter atomic.Uint64
}

func (w *WrappedCertificateApplyWorker) Apply(ctx context.Context, obj *cmv1.Certificate) error {
	objKey := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
	requiresQueue, err := w.wrapped.RequiresQueue(ctx, objKey, obj)
	if err != nil {
		return err
	}
	if requiresQueue {
		w.queuedCounter.Add(1)
	}
	return w.wrapped.Apply(ctx, obj)
}

func (w *WrappedCertificateApplyWorker) Start(ctx context.Context) error {
	return w.wrapped.Start(ctx)
}

func (w *WrappedCertificateApplyWorker) GetQueuedCounter() uint64 {
	return w.queuedCounter.Load()
}

func testCertificateApplyWorkerApply() {
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
		applyWorker := &WrappedCertificateApplyWorker{
			wrapped: NewCertificateApplyWorker(mgr.GetClient(), opts),
		}

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
		applyWorker := &WrappedCertificateApplyWorker{
			wrapped: NewCertificateApplyWorker(mgr.GetClient(), opts),
		}

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
		applyWorker := &WrappedCertificateApplyWorker{
			wrapped: NewCertificateApplyWorker(mgr.GetClient(), opts),
		}

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
}
