package controllers

// unit test for CertificateApplyWorker
// It should test the following
// 1. Apply metheod with different conditions such as
//   - entirely new object
//   - metadata update
//   - spec update
//   and inspect the queue content to see if the apply was queued or direct.
// 2. applying objects from Start method's loop. test both happy and unhappy path.
//
// Other integration tests should be in httpproxy_controller_test.go

import (
	"context"
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// client wrapper for avoiding SSA Patch since fake client does not support SSA.
// this should be okay since we are not using it to test SSA specific behavior.
type applyAsUpdateClient struct {
	client.Client
}

func (c *applyAsUpdateClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	// Emulate "apply" as upsert for tests:
	// - if the object exists, Update
	// - if not, Create
	existing := obj.DeepCopyObject().(client.Object)
	err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), existing)
	if err == nil {
		return c.Client.Update(ctx, obj)
	}
	if k8serrors.IsNotFound(err) {
		return c.Client.Create(ctx, obj)
	}
	return err
}

// client wrapper that always fails Patch to simulate apply failure
type failingPatchClient struct {
	client.Client
}

func (f *failingPatchClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return fmt.Errorf("simulated patch failure")
}

func testCertificateApplyWorker() {

	ctx := context.Background()

	scheme := runtime.NewScheme()
	Expect(cmv1.AddToScheme(scheme)).To(Succeed())
	Expect(projectcontourv1.AddToScheme(scheme)).To(Succeed())

	newFakeClient := func(objs ...client.Object) client.Client {
		return crfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			Build()
	}

	Describe("Apply + RequiresQueue integration", func() {
		// This test will not call Start so that items put into the queue does not get dequeued.
		// This allows the inspection of the queue content withouot adding a wrapper.
		key := types.NamespacedName{
			Namespace: "default",
			Name:      "test-cert",
		}

		It("queues a completely new object (NotFound path)", func() {
			baseClient := newFakeClient() // no existing Certificate
			cl := &applyAsUpdateClient{Client: baseClient}
			worker := NewCertificateApplyWorker(cl, ReconcilerOptions{
				CertificateApplyLimit: 10, // avoid rate.Limit(0) semantics
			})

			reg := prometheus.NewRegistry()
			Expect(worker.RegisterMetrics(reg)).To(Succeed())

			desired := &cmv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
				Spec: cmv1.CertificateSpec{
					SecretName: "test-secret",
					DNSNames:   []string{"example.com"},
				},
			}

			// Call Apply, which internally calls RequiresQueue and should enqueue
			Expect(worker.Apply(ctx, desired)).To(Succeed())

			// New object -> should NOT be created in the API yet (only queued)
			got := &cmv1.Certificate{}
			err := cl.Get(ctx, key, got)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "new object should not be applied immediately, but queued")

			// And the worker should have the key in its internal queue
			Expect(worker.workqueue.Len()).To(Equal(1))

			queuedKey, shutdown := worker.workqueue.Get()
			Expect(shutdown).To(BeFalse())
			Expect(queuedKey).To(Equal(key))
			worker.workqueue.Done(queuedKey)

			// assert metrics: 0 for all metrics
			// queued apply is not recorded since we are not running Start
			assertMetricsCombinations(worker.certificatesAppliedTotal, expectedValsForMetrics{
				viaQueueSuccess: 0,
				viaQueueError:   0,
				directSuccess:   0,
				directError:     0,
			})
		})

		It("applies directly when only metadata (annotations) change", func() {
			// current object in cluster
			current := &cmv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				Spec: cmv1.CertificateSpec{
					SecretName: "test-secret",
					DNSNames:   []string{"example.com"},
				},
			}

			baseClient := newFakeClient(current) // init with a certificate
			cl := &applyAsUpdateClient{Client: baseClient}
			worker := NewCertificateApplyWorker(cl, ReconcilerOptions{
				CertificateApplyLimit: 10,
			})

			reg := prometheus.NewRegistry()
			Expect(worker.RegisterMetrics(reg)).To(Succeed())

			// desired has same Spec but different annotation
			desired := current.DeepCopy()
			if desired.Annotations == nil {
				desired.Annotations = map[string]string{}
			}
			desired.Annotations["foo"] = "baz" // metadata-only change

			// This should go down the "no queue" path and call applyCertificate (Patch)
			Expect(worker.Apply(ctx, desired)).To(Succeed())

			// Should NOT be queued
			Expect(worker.workqueue.Len()).To(Equal(0))

			// And the object in the fake client should be updated
			got := &cmv1.Certificate{}
			Expect(cl.Get(ctx, key, got)).To(Succeed())
			Expect(got.Annotations).To(HaveKeyWithValue("foo", "baz"))
			// Spec should still match
			Expect(got.Spec.SecretName).To(Equal("test-secret"))
			Expect(got.Spec.DNSNames).To(ConsistOf("example.com"))

			// assert metrics: 1 successful direct apply
			assertMetricsCombinations(worker.certificatesAppliedTotal, expectedValsForMetrics{
				viaQueueSuccess: 0,
				viaQueueError:   0,
				directSuccess:   1,
				directError:     0,
			})
		})

		It("queues when Spec changes (re-issuance required)", func() {
			current := &cmv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
				Spec: cmv1.CertificateSpec{
					SecretName: "test-secret",
					DNSNames:   []string{"example.com"},
				},
			}

			baseClient := newFakeClient(current) // init with a certificate
			cl := &applyAsUpdateClient{Client: baseClient}
			worker := NewCertificateApplyWorker(cl, ReconcilerOptions{
				CertificateApplyLimit: 10,
			})

			reg := prometheus.NewRegistry()
			Expect(worker.RegisterMetrics(reg)).To(Succeed())

			// desired has different Spec (extra DNS name)
			desired := current.DeepCopy()
			desired.Spec.DNSNames = append(desired.Spec.DNSNames, "extra.example.com")

			// Apply should decide this needs queueing
			Expect(worker.Apply(ctx, desired)).To(Succeed())

			// Should be queued, not applied directly
			Expect(worker.workqueue.Len()).To(Equal(1))

			// The object in the fake client should still be the old spec (no extra DNS yet),
			// because the queued apply hasn’t run Start() / processed the queue.
			got := &cmv1.Certificate{}
			Expect(cl.Get(ctx, key, got)).To(Succeed())
			Expect(got.Spec.DNSNames).To(ConsistOf("example.com"))

			queuedKey, shutdown := worker.workqueue.Get()
			Expect(shutdown).To(BeFalse())
			Expect(queuedKey).To(Equal(key))
			worker.workqueue.Done(queuedKey)

			// assert metrics: 0 for all metrics
			// queued apply is not recorded since we are not running Start
			assertMetricsCombinations(worker.certificatesAppliedTotal, expectedValsForMetrics{
				viaQueueSuccess: 0,
				viaQueueError:   0,
				directSuccess:   0,
				directError:     0,
			})
		})
	})

	Describe("Start & retry channel", func() {
		It("dequeues and applies a certificate from the queue", func() {
			baseClient := newFakeClient() // no existing certificate
			cl := &applyAsUpdateClient{Client: baseClient}
			worker := NewCertificateApplyWorker(cl, ReconcilerOptions{
				CertificateApplyLimit: 10, // avoid rate.Limit(0) oddness
			})

			reg := prometheus.NewRegistry()
			Expect(worker.RegisterMetrics(reg)).To(Succeed())

			key := types.NamespacedName{
				Namespace: "default",
				Name:      "cert-from-queue",
			}

			cert := &cmv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
				Spec: cmv1.CertificateSpec{
					SecretName: "test-secret",
					DNSNames:   []string{"example.com"},
				},
			}

			// Run Start in the background
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- worker.Start(ctx)
			}()

			// Apply should enqueue the certificate (RequiresQueue => true for new object)
			Expect(worker.Apply(ctx, cert)).To(Succeed())

			// Wait until the worker has dequeued and applied the certificate.
			// Using Eventually + Succeed() matcher for func() error.
			Eventually(func() error {
				got := &cmv1.Certificate{}
				return cl.Get(ctx, key, got)
			}, 2*time.Second, 100*time.Millisecond).Should(Succeed(), "certificate should eventually be applied from queue")

			// Double-check the applied spec
			got := &cmv1.Certificate{}
			Expect(cl.Get(ctx, key, got)).To(Succeed())
			Expect(got.Spec.SecretName).To(Equal("test-secret"))
			Expect(got.Spec.DNSNames).To(ConsistOf("example.com"))

			// assert metrics: 1 successful apply via queue
			assertMetricsCombinations(worker.certificatesAppliedTotal, expectedValsForMetrics{
				viaQueueSuccess: 1,
				viaQueueError:   0,
				directSuccess:   0,
				directError:     0,
			})

			// Queue should be drained
			Expect(worker.workqueue.Len()).To(Equal(0))

			// Shut down the worker loop cleanly
			cancel()
			Eventually(errCh, time.Second).Should(Receive(BeNil()))
		})

		It("sends an HTTPProxy event on retry channel when certificate apply fails", func() {
			baseClient := newFakeClient()
			cl := &failingPatchClient{Client: baseClient}

			worker := NewCertificateApplyWorker(cl, ReconcilerOptions{
				CertificateApplyLimit: 10,
			})

			reg := prometheus.NewRegistry()
			Expect(worker.RegisterMetrics(reg)).To(Succeed())

			// Certificate annotated with ownerAnnotation so enqueueHTTPProxy can find the HTTPProxy key
			cert := &cmv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "cert-with-owner",
					Annotations: map[string]string{
						ownerAnnotation: "default/test-httpproxy",
					},
				},
				Spec: cmv1.CertificateSpec{
					SecretName: "test-secret",
				},
			}

			// Run Start in the background
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- worker.Start(ctx)
			}()

			// Apply should enqueue the certificate (RequiresQueue returns true for new object)
			Expect(worker.Apply(ctx, cert)).To(Succeed())

			// We expect a retry event on the channel because Patch always fails
			retryCh := worker.GetRetryChannel()

			var evt event.TypedGenericEvent[*projectcontourv1.HTTPProxy]
			Eventually(retryCh, 2*time.Second, 100*time.Millisecond).Should(
				Receive(&evt),
				"expected an HTTPProxy event to be sent on retry channel after apply failure",
			)

			Expect(evt.Object).NotTo(BeNil())
			Expect(evt.Object.Namespace).To(Equal("default"))
			Expect(evt.Object.Name).To(Equal("test-httpproxy"))

			// assert metrics: 1 apply error via queue
			assertMetricsCombinations(worker.certificatesAppliedTotal, expectedValsForMetrics{
				viaQueueSuccess: 0,
				viaQueueError:   1,
				directSuccess:   0,
				directError:     0,
			})

			// shut down the worker loop
			cancel()
			Eventually(errCh, time.Second).Should(Receive(BeNil()))
		})
	})
}

type expectedValsForMetrics struct {
	viaQueueSuccess float64
	viaQueueError   float64
	directSuccess   float64
	directError     float64
}

type seriesKey struct {
	viaQueue viaQueueValue
	result   applyResultValue
}

// Convert the nice struct to a map keyed by label combinations.
func (e expectedValsForMetrics) asMap() map[seriesKey]float64 {
	return map[seriesKey]float64{
		{viaQueue: viaQueueYes, result: applyResultSuccess}: e.viaQueueSuccess,
		{viaQueue: viaQueueYes, result: applyResultError}:   e.viaQueueError,
		{viaQueue: viaQueueNo, result: applyResultSuccess}:  e.directSuccess,
		{viaQueue: viaQueueNo, result: applyResultError}:    e.directError,
	}
}

func assertMetricsCombinations(counterVec *prometheus.CounterVec, expectedVals expectedValsForMetrics) {
	for key, expected := range expectedVals.asMap() {
		val := testutil.ToFloat64(counterVec.WithLabelValues(certificateApplierName, string(key.viaQueue), string(key.result)))
		Expect(val).To(Equal(expected), "viaQueue=%s result=%s", key.viaQueue, key.result)
	}
}
