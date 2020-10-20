package controllers

import (
	"context"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment

	testServiceKey = client.ObjectKey{Namespace: "test-ns", Name: "test-svc"}
)

const (
	testNamespacePrefix = "test-ns-"
	dummyLoadBalancerIP = "10.0.0.0"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "third")},
	}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("setting up scheme")
	scheme := runtime.NewScheme()
	Expect(SetupScheme(scheme)).ShouldNot(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("creating namespace")
	namespace := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: testServiceKey.Namespace,
		},
	}
	Expect(k8sClient.Create(context.Background(), namespace)).ShouldNot(HaveOccurred())

	By("creating httpproy loadbalancer service")
	svc := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Namespace: testServiceKey.Namespace,
			Name:      testServiceKey.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports:          []corev1.ServicePort{{Port: 8080}},
			LoadBalancerIP: dummyLoadBalancerIP,
			Type:           corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{
					IP: dummyLoadBalancerIP,
				}},
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), svc)).ShouldNot(HaveOccurred())
	Expect(k8sClient.Status().Update(context.Background(), svc)).ShouldNot(HaveOccurred())
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Test contour-plus", func() {
	Context("httpproxy", testHTTPProxyReconcile)
})

func startTestManager(mgr manager.Manager) (stop func()) {
	ch := make(chan struct{})
	waitCh := make(chan struct{})
	stop = func() {
		close(ch)
		<-waitCh
	}
	go func() {
		err := mgr.Start(ch)
		if err != nil {
			panic(err)
		}
		close(waitCh)
	}()
	time.Sleep(100 * time.Millisecond)
	return
}

func setupManager() (*runtime.Scheme, manager.Manager) {
	scm := runtime.NewScheme()
	scheme.AddToScheme(scm)
	Expect(SetupScheme(scm)).ShouldNot(HaveOccurred())
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scm})
	Expect(err).ShouldNot(HaveOccurred())
	return scm, mgr
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}
