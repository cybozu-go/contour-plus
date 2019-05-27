package controllers

import (
	"context"
	"path/filepath"
	"testing"

	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

const prefix = "test-"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
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
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	By("creating IngressRoute loadbalancer service")
	svc := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Namespace: serviceKey.Namespace,
			Name:      serviceKey.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports:          []corev1.ServicePort{{Port: 8080}},
			LoadBalancerIP: "10.0.0.0",
			Type:           corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{
					IP: "10.0.0.0",
				}},
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), svc)).ShouldNot(HaveOccurred())
	Expect(k8sClient.Status().Update(context.Background(), svc)).ShouldNot(HaveOccurred())
	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
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

func setupReconciler(mgr *manager.Manager, scheme *runtime.Scheme, client client.Client) error {
	reconciler := &IngressRouteReconciler{
		Client:            client,
		Log:               ctrl.Log.WithName("controllers").WithName("IngressRoute"),
		Scheme:            scheme,
		ServiceKey:        serviceKey,
		Prefix:            prefix,
		DefaultIssuerName: "test-issuer",
		DefaultIssuerKind: certmanagerv1alpha1.ClusterIssuerKind,
		CreateDNSEndpoint: true,
		CreateCertificate: true,
	}
	return reconciler.SetupWithManager(*mgr)
}

func setupScheme(scm *runtime.Scheme) error {
	err := contourv1beta1.AddToScheme(scm)
	if err != nil {
		return err
	}
	err = apiextensions.AddToScheme(scm)
	if err != nil {
		return err
	}
	err = certmanagerv1alpha1.AddToScheme(scm)
	if err != nil {
		return err
	}
	err = corev1.AddToScheme(scm)
	if err != nil {
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
	return nil
}
