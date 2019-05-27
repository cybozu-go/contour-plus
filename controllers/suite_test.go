package controllers

import (
	"context"
	"math/rand"
	"path/filepath"
	"testing"

	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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

	serviceKey = client.ObjectKey{Namespace: "test-ns", Name: "test-svc"}
)

const (
	testNamespacePrefix = "test-ns-"
	dummyLoadBalancerIP = "10.0.0.0"
)

type reconcilerOptions struct {
	prefix            string
	defaultIssuerName string
	defaultIssuerKind string
	createDNSEndpoint bool
	createCertificate bool
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

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
	Expect(setupScheme(scheme)).ShouldNot(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("creating IngressRoute loadbalancer service")
	svc := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Namespace: serviceKey.Namespace,
			Name:      serviceKey.Name,
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
	Context("contour-plus", testReconcile)
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
	return
}

func setupReconciler(mgr manager.Manager, scheme *runtime.Scheme, opts reconcilerOptions) error {
	reconciler := &IngressRouteReconciler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("IngressRoute"),
		Scheme:            scheme,
		ServiceKey:        serviceKey,
		Prefix:            opts.prefix,
		DefaultIssuerName: opts.defaultIssuerName,
		DefaultIssuerKind: opts.defaultIssuerKind,
		CreateDNSEndpoint: opts.createDNSEndpoint,
		CreateCertificate: opts.createCertificate,
	}
	return reconciler.SetupWithManager(mgr)
}

func setupScheme(scm *runtime.Scheme) error {
	if err := contourv1beta1.AddToScheme(scm); err != nil {
		return err
	}

	groupVersion := ctrl.GroupVersion{
		Group:   "externaldns.k8s.io",
		Version: "v1alpha1",
	}
	scm.AddKnownTypes(groupVersion,
		&endpoint.DNSEndpoint{},
		&endpoint.DNSEndpointList{},
	)
	metav1.AddToGroupVersion(scm, groupVersion)

	if err := certmanagerv1alpha1.AddToScheme(scm); err != nil {
		return err
	}

	return corev1.AddToScheme(scm)
}

func setupManager() (*runtime.Scheme, manager.Manager) {
	scm := scheme.Scheme
	Expect(setupScheme(scm)).ShouldNot(HaveOccurred())
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
