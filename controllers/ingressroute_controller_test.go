package controllers

import (
	"context"
	"time"

	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	contourv1beta1 "github.com/projectcontour/contour/apis/contour/v1beta1"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dnsName        = "test.example.com"
	testSecretName = "test-secret"
)

func testIngressRouteReconcile() {
	It("should create DNSEndpoint and Certificate", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		scm, mgr := setupManager()

		prefix := "test-"
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			prefix:            prefix,
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createDNSEndpoint: true,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyIngressRoute(irKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := &endpoint.DNSEndpoint{}
		objKey := client.ObjectKey{
			Name:      prefix + irKey.Name,
			Namespace: irKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		Expect(de.Spec.Endpoints[0].Targets).Should(Equal(endpoint.Targets{"10.0.0.0"}))
		Expect(de.Spec.Endpoints[0].DNSName).Should(Equal(dnsName))

		By("getting Certificate with prefixed name")
		crt := &certmanagerv1alpha2.Certificate{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())
	})

	It("should not crash with null fields", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		scm, mgr := setupManager()

		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createDNSEndpoint: true,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute with null virtualHost")
		ir := &contourv1beta1.IngressRoute{}
		ir.Namespace = ns
		ir.Name = "foo"
		ir.Spec.Routes = []contourv1beta1.Route{}
		Expect(k8sClient.Create(context.Background(), ir)).ShouldNot(HaveOccurred())

		By("creating IngressRoute with null TLS")
		ir = &contourv1beta1.IngressRoute{}
		ir.Namespace = ns
		ir.Name = "foo2"
		ir.Annotations = map[string]string{
			testACMETLSAnnotation: "true",
		}
		ir.Spec.VirtualHost = &projectcontourv1.VirtualHost{
			Fqdn: "foo2.example.com",
		}
		ir.Spec.Routes = []contourv1beta1.Route{}
		Expect(k8sClient.Create(context.Background(), ir)).ShouldNot(HaveOccurred())
	})

	It(`should not create DNSEndpoint and Certificate if "contour-plus.cybozu.com/exclude"" is "true"`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		scm, mgr := setupManager()

		prefix := "test-"
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			prefix:            prefix,
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createDNSEndpoint: true,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute having the annotation to exclude from contour-plus's targets")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		ingressRoute := newDummyIngressRoute(irKey)
		ingressRoute.Annotations[excludeAnnotation] = "true"
		Expect(k8sClient.Create(context.Background(), ingressRoute)).ShouldNot(HaveOccurred())

		By("confirming that DNSEndpoint and Certificate do not exist")
		time.Sleep(time.Second)
		endpointList := &endpoint.DNSEndpointList{}
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())

		crtList := &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})

	It("should create Certificate with specified IssuerKind", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("setup manager with ClusterIssuer")
		scm, mgr := setupManager()
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.ClusterIssuerKind,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyIngressRoute(irKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: irKey.Name, Namespace: irKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, certificate)
		}, 5*time.Second).Should(Succeed())
		Expect(certificate.Spec.IssuerRef.Kind).Should(Equal(certmanagerv1alpha2.ClusterIssuerKind))
		Expect(certificate.Spec.IssuerRef.Name).Should(Equal("test-issuer"))
	})

	It(`should create Certificate with Issuer specified in "cert-manager.io/issuer"`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("setup manager")
		scm, mgr := setupManager()
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute with annotations")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		ingressRoute := newDummyIngressRoute(irKey)
		ingressRoute.Annotations[issuerNameAnnotation] = "custom-issuer"
		Expect(k8sClient.Create(context.Background(), ingressRoute)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: irKey.Name, Namespace: irKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, certificate)
		}, 5*time.Second).Should(Succeed())

		By("confirming that specified issuer used")
		Expect(certificate.Spec.IssuerRef.Kind).Should(Equal(certmanagerv1alpha2.IssuerKind))
		Expect(certificate.Spec.IssuerRef.Name).Should(Equal("custom-issuer"))

	})

	It(`should create Certificate with Issuer specified in "cert-manager.io/cluster-issuer"`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})
		By("setup manager")
		scm, mgr := setupManager()
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("updating IngressRoute with annotations, both of issuer and cluster-issuer are specified")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		ingressRoute := newDummyIngressRoute(irKey)
		ingressRoute.Annotations[issuerNameAnnotation] = "custom-issuer"
		ingressRoute.Annotations[clusterIssuerNameAnnotation] = "custom-cluster-issuer"
		Expect(k8sClient.Create(context.Background(), ingressRoute)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: irKey.Name, Namespace: irKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, certificate)
		}, 5*time.Second).Should(Succeed())

		By("confirming that specified issuer used, cluster-issuer is precedence over issuer")
		Expect(certificate.Spec.IssuerRef.Kind).Should(Equal(certmanagerv1alpha2.ClusterIssuerKind))
		Expect(certificate.Spec.IssuerRef.Name).Should(Equal("custom-cluster-issuer"))

	})

	It("should create DNSEndpoint, but should not create Certificate, if `createCertificate` is false", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("disabling the feature to create Certificate")
		scm, mgr := setupManager()

		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createDNSEndpoint: true,
			createCertificate: false,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyIngressRoute(irKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint")
		de := &endpoint.DNSEndpoint{}
		objKey := client.ObjectKey{
			Name:      irKey.Name,
			Namespace: irKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		Expect(de.Spec.Endpoints[0].Targets).Should(Equal(endpoint.Targets{dummyLoadBalancerIP}))
		Expect(de.Spec.Endpoints[0].DNSName).Should(Equal(dnsName))

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})

	It("should create Certificate, but should not create DNSEndpoint, if `createDNSEndpoint` is false", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("disabling the feature to create DNSEndpoint")
		scm, mgr := setupManager()
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createDNSEndpoint: false,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyIngressRoute(irKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: irKey.Name, Namespace: irKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, certificate)
		}, 5*time.Second).Should(Succeed())
		Expect(certificate.Spec.SecretName).Should(Equal(testSecretName))

		By("confirming that DNSEndpoint does not exist")
		time.Sleep(time.Second)
		endpointList := &endpoint.DNSEndpointList{}
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())
	})

	It(`should not create Certificate, if "kubernetes.io/tls-acme" is not "true"`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("disabling the feature to create Certificate")
		scm, mgr := setupManager()

		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerName: "test-issuer",
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createDNSEndpoint: true,
			createCertificate: false,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		ingressRoute := newDummyIngressRoute(irKey)
		ingressRoute.Annotations[testACMETLSAnnotation] = "aaa"
		Expect(k8sClient.Create(context.Background(), ingressRoute)).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint")
		de := &endpoint.DNSEndpoint{}
		objKey := client.ObjectKey{
			Name:      irKey.Name,
			Namespace: irKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		Expect(de.Spec.Endpoints[0].Targets).Should(Equal(endpoint.Targets{dummyLoadBalancerIP}))
		Expect(de.Spec.Endpoints[0].DNSName).Should(Equal(dnsName))

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})

	It("should create Certificate, if `defaultIssuerName` is empty but 'issuer-name' annotation is not empty", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("setup reconciler with empty defaultIssuerName")
		scm, mgr := setupManager()
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		ingressRoute := newDummyIngressRoute(irKey)
		ingressRoute.Annotations[issuerNameAnnotation] = "custom-issuer"
		Expect(k8sClient.Create(context.Background(), ingressRoute)).ShouldNot(HaveOccurred())

		By("getting Certificate with specified name")
		crt := &certmanagerv1alpha2.Certificate{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: irKey.Name}, crt)
		}).Should(Succeed())
		Expect(crt.Spec.IssuerRef.Name).Should(Equal("custom-issuer"))
		Expect(crt.Spec.IssuerRef.Kind).Should(Equal(certmanagerv1alpha2.IssuerKind))
	})

	It("should not create Certificate, if `defaultIssuerName` and 'issuer-name' annotation are empty", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())
		defer k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})

		By("setup reconciler with empty defaultIssuerName")
		scm, mgr := setupManager()
		Expect(setupReconciler(mgr, scm, reconcilerOptions{
			defaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			createCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating IngressRoute")
		irKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyIngressRoute(irKey))).ShouldNot(HaveOccurred())

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})
}

func newDummyIngressRoute(irKey client.ObjectKey) *contourv1beta1.IngressRoute {
	return &contourv1beta1.IngressRoute{
		ObjectMeta: v1.ObjectMeta{
			Namespace: irKey.Namespace,
			Name:      irKey.Name,
			Annotations: map[string]string{
				testACMETLSAnnotation: "true",
			},
		},
		Spec: contourv1beta1.IngressRouteSpec{
			VirtualHost: &projectcontourv1.VirtualHost{
				Fqdn: dnsName,
				TLS:  &projectcontourv1.TLS{SecretName: testSecretName},
			},
			Routes: []contourv1beta1.Route{},
		},
	}
}
