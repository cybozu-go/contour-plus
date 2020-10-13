package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
)

const (
	dnsName        = "test.example.com"
	testSecretName = "test-secret"
)

func testHTTPProxyReconcile() {
	It("should create DNSEndpoint and Certificate", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		prefix := "test-"
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			Prefix:            prefix,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := &endpoint.DNSEndpoint{}
		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
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
		Expect(crt.Spec.DNSNames).Should(Equal([]string{dnsName}))
		Expect(crt.Spec.SecretName).Should(Equal(testSecretName))
		Expect(crt.Spec.CommonName).Should(Equal(dnsName))
		Expect(crt.Spec.Usages).Should(Equal([]certmanagerv1alpha2.KeyUsage{
			certmanagerv1alpha2.UsageDigitalSignature,
			certmanagerv1alpha2.UsageKeyEncipherment,
			certmanagerv1alpha2.UsageServerAuth,
			certmanagerv1alpha2.UsageClientAuth,
		}))
	})

	It(`should not create DNSEndpoint and Certificate if "contour-plus.cybozu.com/exclude"" is "true"`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		prefix := "test-"
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			Prefix:            prefix,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy having the annotation to exclude from contour-plus's targets")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[excludeAnnotation] = "true"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

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

		By("setup manager with ClusterIssuer")
		scm, mgr := setupManager()
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.ClusterIssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
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

		By("setup manager")
		scm, mgr := setupManager()
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with annotations")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[issuerNameAnnotation] = "custom-issuer"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
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

		By("setup manager")
		scm, mgr := setupManager()
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("updating HTTPProxy with annotations, both of issuer and cluster-issuer are specified")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[issuerNameAnnotation] = "custom-issuer"
		hp.Annotations[clusterIssuerNameAnnotation] = "custom-cluster-issuer"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
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

		By("disabling the feature to create Certificate")
		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: false,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint")
		de := &endpoint.DNSEndpoint{}
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
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

	It("should create Certificate, but should not create DNSEndpoint, if `CreateDNSEndpoint` is false", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		By("disabling the feature to create DNSEndpoint")
		scm, mgr := setupManager()
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateDNSEndpoint: false,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
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

		By("disabling the feature to create Certificate")
		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: false,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[testACMETLSAnnotation] = "aaa"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint")
		de := &endpoint.DNSEndpoint{}
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
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

	It("should create Certificate, if `DefaultIssuerName` is empty but 'issuer-name' annotation is not empty", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		By("setup reconciler with empty DefaultIssuerName")
		scm, mgr := setupManager()
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[issuerNameAnnotation] = "custom-issuer"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate with specified name")
		crt := &certmanagerv1alpha2.Certificate{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: hpKey.Name}, crt)
		}).Should(Succeed())
		Expect(crt.Spec.IssuerRef.Name).Should(Equal("custom-issuer"))
		Expect(crt.Spec.IssuerRef.Kind).Should(Equal(certmanagerv1alpha2.IssuerKind))
	})

	It("should not create Certificate, if `DefaultIssuerName` and 'issuer-name' annotation are empty", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		By("setup reconciler with empty DefaultIssuerName")
		scm, mgr := setupManager()
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})

	It(`should not create DNSEndpoint and Certificate if the class name is not the target`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: true,
			IngressClassName:  "class-name",
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy having the annotation")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[ingressClassNameAnnotation] = "wrong"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("confirming that DNSEndpoint and Certificate do not exist")
		time.Sleep(time.Second)
		endpointList := &endpoint.DNSEndpointList{}
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())

		crtList := &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())

		By("creating HTTPProxy without the annotation")
		hpKey = client.ObjectKey{Name: "bar", Namespace: ns}
		hp = newDummyHTTPProxy(hpKey)
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("confirming that DNSEndpoint and Certificate do not exist")
		time.Sleep(time.Second)
		endpointList = &endpoint.DNSEndpointList{}
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())

		crtList = &certmanagerv1alpha2.CertificateList{}
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})

	It(`should create Certificate if the class name equals to the target`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: certmanagerv1alpha2.IssuerKind,
			CreateCertificate: true,
			IngressClassName:  "class-name",
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy having the annotation")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[ingressClassNameAnnotation] = "class-name"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		certificate := &certmanagerv1alpha2.Certificate{}
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, certificate)
		}, 5*time.Second).Should(Succeed())
	})
}

func newDummyHTTPProxy(hpKey client.ObjectKey) *projectcontourv1.HTTPProxy {
	return &projectcontourv1.HTTPProxy{
		ObjectMeta: v1.ObjectMeta{
			Namespace: hpKey.Namespace,
			Name:      hpKey.Name,
			Annotations: map[string]string{
				testACMETLSAnnotation: "true",
			},
		},
		Spec: projectcontourv1.HTTPProxySpec{
			VirtualHost: &projectcontourv1.VirtualHost{
				Fqdn: dnsName,
				TLS:  &projectcontourv1.TLS{SecretName: testSecretName},
			},
			Routes: []projectcontourv1.Route{},
		},
	}
}

func TestIsClassNameMatched(t *testing.T) {
	tests := []struct {
		name             string
		ingressClassName string
		hp               *projectcontourv1.HTTPProxy
		want             bool
	}{
		{
			name:             "Annotation is not set",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			want: false,
		},
		{
			name:             "Both annotation are set",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						ingressClassNameAnnotation:        "class-name",
						contourIngressClassNameAnnotation: "class-name",
					},
				},
			},
			want: true,
		},
		{
			name:             "Both annotation are set but not matched",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						ingressClassNameAnnotation:        "class-name",
						contourIngressClassNameAnnotation: "wrong",
					},
				},
			},
			want: false,
		},
		{
			name:             fmt.Sprintf("Annotation %s is set", ingressClassNameAnnotation),
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						ingressClassNameAnnotation: "class-name",
					},
				},
			},
			want: true,
		},
		{
			name:             fmt.Sprintf("Annotation %s is set but not matched", ingressClassNameAnnotation),
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						ingressClassNameAnnotation: "wrong",
					},
				},
			},
			want: false,
		},
		{
			name:             fmt.Sprintf("Annotation %s is set", contourIngressClassNameAnnotation),
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						contourIngressClassNameAnnotation: "class-name",
					},
				},
			},
			want: true,
		},
		{
			name:             fmt.Sprintf("Annotation %s is set but not matched", contourIngressClassNameAnnotation),
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						contourIngressClassNameAnnotation: "wrong",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HTTPProxyReconciler{
				IngressClassName: tt.ingressClassName,
			}
			if got := r.isClassNameMatched(tt.hp); got != tt.want {
				t.Errorf("HTTPProxyReconciler.isClassNameMatched() = %v, want %v", got, tt.want)
			}
		})
	}
}
