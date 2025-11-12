package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dnsName            = "test.example.com"
	testSecretName     = "test-secret"
	testDelegationName = "acme.example.com"
)

func certificate() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(certManagerGroupVersion.WithKind(CertificateKind))
	return obj
}

func certificateList() *unstructured.UnstructuredList {
	obj := &unstructured.UnstructuredList{}
	obj.SetGroupVersionKind(certManagerGroupVersion.WithKind(CertificateListKind))
	return obj
}

func dnsEndpoint() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(externalDNSGroupVersion.WithKind(DNSEndpointKind))
	return obj
}

func dnsEndpointList() *unstructured.UnstructuredList {
	obj := &unstructured.UnstructuredList{}
	obj.SetGroupVersionKind(externalDNSGroupVersion.WithKind(DNSEndpointListKind))
	return obj
}

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
			DefaultIssuerKind: IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{"10.0.0.0"}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("ensuring additional DNSEndpoint does not exist")
		dde := dnsEndpoint()
		dObjKey := client.ObjectKey{
			Name:      prefix + hpKey.Name + "-delegation",
			Namespace: hpKey.Namespace,
		}
		Consistently(func() error {
			return k8sClient.Get(context.Background(), dObjKey, dde)
		}, 5*time.Second).ShouldNot(Succeed())

		By("getting Certificate with prefixed name")
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec["dnsNames"]).Should(Equal([]interface{}{dnsName}))
		Expect(crtSpec["secretName"]).Should(Equal(testSecretName))
		Expect(crtSpec["commonName"]).Should(Equal(dnsName))
		Expect(crtSpec["usages"]).Should(Equal([]interface{}{
			usageDigitalSignature,
			usageKeyEncipherment,
			usageServerAuth,
			usageClientAuth,
		}))
		Expect(crtSpec["revisionHistoryLimit"]).Should(BeNil())
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
			DefaultIssuerKind: IssuerKind,
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
		endpointList := dnsEndpointList()
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())

		crtList := certificateList()
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())
	})

	It("should create delegation DNSEndpoint if requested", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		prefix := "test-"
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:             testServiceKey,
			Prefix:                 prefix,
			DefaultIssuerName:      "test-issuer",
			DefaultIssuerKind:      IssuerKind,
			DefaultDelegatedDomain: testDelegationName,
			CreateDNSEndpoint:      true,
			CreateCertificate:      true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{"10.0.0.0"}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("ensuring additional DNSEndpoint has been created")
		dde := dnsEndpoint()
		dObjKey := client.ObjectKey{
			Name:      prefix + hpKey.Name + "-delegation",
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), dObjKey, dde)
		}, 5*time.Second).Should(Succeed())
		ddeSpec := dde.UnstructuredContent()["spec"].(map[string]interface{})
		dEndPoints := ddeSpec["endpoints"].([]interface{})
		dEndPoint := dEndPoints[0].(map[string]interface{})
		Expect(dEndPoint["targets"]).Should(Equal([]interface{}{"_acme-challenge." + dnsName + "." + testDelegationName}))
		Expect(dEndPoint["dnsName"]).Should(Equal("_acme-challenge." + dnsName))
		Expect(dEndPoint["recordType"]).Should(Equal("CNAME"))
	})

	It("should create delegation DNSEndpoint if requested via annotation", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		prefix := "test-"
		customDelegationName := "test." + testDelegationName
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:              testServiceKey,
			Prefix:                  prefix,
			DefaultIssuerName:       "test-issuer",
			DefaultIssuerKind:       IssuerKind,
			DefaultDelegatedDomain:  testDelegationName,
			AllowedDelegatedDomains: []string{customDelegationName},
			AllowCustomDelegations:  true,
			CreateDNSEndpoint:       true,
			CreateCertificate:       true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[delegatedDomainAnnotation] = customDelegationName
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{"10.0.0.0"}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("ensuring additional DNSEndpoint has been created")
		dde := dnsEndpoint()
		dObjKey := client.ObjectKey{
			Name:      prefix + hpKey.Name + "-delegation",
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), dObjKey, dde)
		}, 5*time.Second).Should(Succeed())
		ddeSpec := dde.UnstructuredContent()["spec"].(map[string]interface{})
		dEndPoints := ddeSpec["endpoints"].([]interface{})
		dEndPoint := dEndPoints[0].(map[string]interface{})
		Expect(dEndPoint["targets"]).Should(Equal([]interface{}{"_acme-challenge." + dnsName + "." + customDelegationName}))
		Expect(dEndPoint["dnsName"]).Should(Equal("_acme-challenge." + dnsName))
		Expect(dEndPoint["recordType"]).Should(Equal("CNAME"))
	})

	It("should ignore custom delegated domain if not permitted", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		prefix := "test-"
		customDelegationName := "test." + testDelegationName
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:             testServiceKey,
			Prefix:                 prefix,
			DefaultIssuerName:      "test-issuer",
			DefaultIssuerKind:      IssuerKind,
			DefaultDelegatedDomain: testDelegationName,
			AllowCustomDelegations: true,
			CreateDNSEndpoint:      true,
			CreateCertificate:      true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[delegatedDomainAnnotation] = customDelegationName
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{"10.0.0.0"}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("ensuring additional DNSEndpoint has been created")
		dde := dnsEndpoint()
		dObjKey := client.ObjectKey{
			Name:      prefix + hpKey.Name + "-delegation",
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), dObjKey, dde)
		}, 5*time.Second).Should(Succeed())
		ddeSpec := dde.UnstructuredContent()["spec"].(map[string]interface{})
		dEndPoints := ddeSpec["endpoints"].([]interface{})
		dEndPoint := dEndPoints[0].(map[string]interface{})
		Expect(dEndPoint["targets"]).Should(Equal([]interface{}{"_acme-challenge." + dnsName + "." + testDelegationName}))
		Expect(dEndPoint["dnsName"]).Should(Equal("_acme-challenge." + dnsName))
		Expect(dEndPoint["recordType"]).Should(Equal("CNAME"))
	})

	It("should ignore custom delegated domain if not whitelisted", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		prefix := "test-"
		customDelegationName := "test." + testDelegationName
		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:             testServiceKey,
			Prefix:                 prefix,
			DefaultIssuerName:      "test-issuer",
			DefaultIssuerKind:      IssuerKind,
			DefaultDelegatedDomain: testDelegationName,
			CreateDNSEndpoint:      true,
			CreateCertificate:      true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[delegatedDomainAnnotation] = customDelegationName
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint with prefixed name")
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      prefix + hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{"10.0.0.0"}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("ensuring additional DNSEndpoint has been created")
		dde := dnsEndpoint()
		dObjKey := client.ObjectKey{
			Name:      prefix + hpKey.Name + "-delegation",
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), dObjKey, dde)
		}, 5*time.Second).Should(Succeed())
		ddeSpec := dde.UnstructuredContent()["spec"].(map[string]interface{})
		dEndPoints := ddeSpec["endpoints"].([]interface{})
		dEndPoint := dEndPoints[0].(map[string]interface{})
		Expect(dEndPoint["targets"]).Should(Equal([]interface{}{"_acme-challenge." + dnsName + "." + testDelegationName}))
		Expect(dEndPoint["dnsName"]).Should(Equal("_acme-challenge." + dnsName))
		Expect(dEndPoint["recordType"]).Should(Equal("CNAME"))
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
			DefaultIssuerKind: ClusterIssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}, 5*time.Second).Should(Succeed())

		By("confirming that specified issuer used")
		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		issuerRef := crtSpec["issuerRef"].(map[string]interface{})
		Expect(issuerRef["kind"]).Should(Equal(ClusterIssuerKind))
		Expect(issuerRef["name"]).Should(Equal("test-issuer"))
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
			DefaultIssuerKind: IssuerKind,
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
		crt := certificate()
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}, 5*time.Second).Should(Succeed())

		By("confirming that specified issuer used")
		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		issuerRef := crtSpec["issuerRef"].(map[string]interface{})
		Expect(issuerRef["kind"]).Should(Equal(IssuerKind))
		Expect(issuerRef["name"]).Should(Equal("custom-issuer"))

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
			DefaultIssuerKind: IssuerKind,
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
		crt := certificate()
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}, 5*time.Second).Should(Succeed())

		By("confirming that specified issuer used, cluster-issuer is precedence over issuer")
		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		issuerRef := crtSpec["issuerRef"].(map[string]interface{})
		Expect(issuerRef["kind"]).Should(Equal(ClusterIssuerKind))
		Expect(issuerRef["name"]).Should(Equal("custom-cluster-issuer"))
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
			DefaultIssuerKind: IssuerKind,
			CreateDNSEndpoint: true,
			CreateCertificate: false,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint")
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())

		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{dummyLoadBalancerIP}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := certificateList()
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
			DefaultIssuerKind: IssuerKind,
			CreateDNSEndpoint: false,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}, 5*time.Second).Should(Succeed())
		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec["secretName"]).Should(Equal(testSecretName))

		By("confirming that DNSEndpoint does not exist")
		time.Sleep(time.Second)
		endpointList := dnsEndpointList()
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
			DefaultIssuerKind: IssuerKind,
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
		de := dnsEndpoint()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}, 5*time.Second).Should(Succeed())
		deSpec := de.UnstructuredContent()["spec"].(map[string]interface{})
		endPoints := deSpec["endpoints"].([]interface{})
		endPoint := endPoints[0].(map[string]interface{})
		Expect(endPoint["targets"]).Should(Equal([]interface{}{dummyLoadBalancerIP}))
		Expect(endPoint["dnsName"]).Should(Equal(dnsName))

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := certificateList()
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
			DefaultIssuerKind: IssuerKind,
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
		crt := certificate()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: hpKey.Name}, crt)
		}).Should(Succeed())
		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		issuerRef := crtSpec["issuerRef"].(map[string]interface{})
		Expect(issuerRef["name"]).Should(Equal("custom-issuer"))
		Expect(issuerRef["kind"]).Should(Equal(IssuerKind))
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
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("confirming that Certificate does not exist")
		time.Sleep(time.Second)
		crtList := certificateList()
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
			DefaultIssuerKind: IssuerKind,
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
		endpointList := dnsEndpointList()
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())

		crtList := certificateList()
		Expect(k8sClient.List(context.Background(), crtList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(crtList.Items).Should(BeEmpty())

		By("creating HTTPProxy without the annotation")
		hpKey = client.ObjectKey{Name: "bar", Namespace: ns}
		hp = newDummyHTTPProxy(hpKey)
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("confirming that DNSEndpoint and Certificate do not exist")
		time.Sleep(time.Second)
		endpointList = dnsEndpointList()
		Expect(k8sClient.List(context.Background(), endpointList, client.InNamespace(ns))).ShouldNot(HaveOccurred())
		Expect(endpointList.Items).Should(BeEmpty())

		crtList = certificateList()
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
			DefaultIssuerKind: IssuerKind,
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
		crt := certificate()
		objKey := client.ObjectKey{Name: hpKey.Name, Namespace: hpKey.Namespace}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}, 5*time.Second).Should(Succeed())
	})

	It(`should create Certificate with revisionHistoryLimit set if specified`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
			CSRRevisionLimit:  1,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		Expect(k8sClient.Create(context.Background(), newDummyHTTPProxy(hpKey))).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec["dnsNames"]).Should(Equal([]interface{}{dnsName}))
		Expect(crtSpec["secretName"]).Should(Equal(testSecretName))
		Expect(crtSpec["commonName"]).Should(Equal(dnsName))
		Expect(crtSpec["usages"]).Should(Equal([]interface{}{
			usageDigitalSignature,
			usageKeyEncipherment,
			usageServerAuth,
			usageClientAuth,
		}))
		Expect(crtSpec["revisionHistoryLimit"]).Should(Equal(int64(1)))
	})

	It(`should create Certificate with revisionHistoryLimit set if cert-manager.io/revision-history-limit is specified`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[revisionHistoryLimitAnnotation] = "2"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec["dnsNames"]).Should(Equal([]interface{}{dnsName}))
		Expect(crtSpec["secretName"]).Should(Equal(testSecretName))
		Expect(crtSpec["commonName"]).Should(Equal(dnsName))
		Expect(crtSpec["usages"]).Should(Equal([]interface{}{
			usageDigitalSignature,
			usageKeyEncipherment,
			usageServerAuth,
			usageClientAuth,
		}))
		Expect(crtSpec["revisionHistoryLimit"]).Should(Equal(int64(2)))
	})

	It(`should prioritize revisionHistoryLimit set by cert-manager.io/revision-history-limit is specified`, func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
			CSRRevisionLimit:  1,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[revisionHistoryLimitAnnotation] = "2"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec["dnsNames"]).Should(Equal([]interface{}{dnsName}))
		Expect(crtSpec["secretName"]).Should(Equal(testSecretName))
		Expect(crtSpec["commonName"]).Should(Equal(dnsName))
		Expect(crtSpec["usages"]).Should(Equal([]interface{}{
			usageDigitalSignature,
			usageKeyEncipherment,
			usageServerAuth,
			usageClientAuth,
		}))
		Expect(crtSpec["revisionHistoryLimit"]).Should(Equal(int64(2)))
	})

	It("should create a Certificate with the specified key algorithm and default size", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with key algorithm annotation")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[privateKeyAlgorithmAnnotation] = "ECDSA"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		keySpec := crtSpec["privateKey"].(map[string]interface{})
		Expect(keySpec["algorithm"]).Should(Equal("ECDSA"))
		Expect(keySpec["size"]).Should(BeNil())
	})

	It("should create a Certificate with the specified key algorithm and size", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with key algorithm annotation")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[privateKeyAlgorithmAnnotation] = "ECDSA"
		hp.Annotations[privateKeySizeAnnotation] = "384"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		keySpec := crtSpec["privateKey"].(map[string]interface{})
		Expect(keySpec["algorithm"]).Should(Equal("ECDSA"))
		Expect(keySpec["size"]).Should(Equal(int64(384)))
	})

	It("should create a Certificate with the specified key algorithm and fallback to default size", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with key algorithm annotation")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations[privateKeyAlgorithmAnnotation] = "RSA"
		hp.Annotations[privateKeySizeAnnotation] = "four thousand ninty six"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		keySpec := crtSpec["privateKey"].(map[string]interface{})
		Expect(keySpec["algorithm"]).Should(Equal("RSA"))
		Expect(keySpec["size"]).Should(BeNil())
	})

	It("should propagate annotations to the generated resources", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
			CreateDNSEndpoint: true,
			PropagatedAnnotations: []string{
				"example.com/propagate-me",
			},
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with annotations")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Annotations["example.com/propagate-me"] = "yes"
		hp.Annotations["example.com/do-not-propagate-me"] = "yes"
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtAnnotations := crt.GetAnnotations()
		Expect(crtAnnotations).ToNot(BeNil())
		Expect(crtAnnotations["example.com/propagate-me"]).To(Equal("yes"))
		Expect(crtAnnotations).ToNot(HaveKey("example.com/do-not-propagate-me"))

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec).ToNot(BeNil())
		secretTemplate := crtSpec["secretTemplate"].(map[string]interface{})
		Expect(secretTemplate).ToNot(BeNil())
		secretAnnotations := secretTemplate["annotations"].(map[string]interface{})
		Expect(secretAnnotations).ToNot(BeNil())
		Expect(secretAnnotations["example.com/propagate-me"]).To(Equal("yes"))
		Expect(secretAnnotations).ToNot(HaveKey("example.com/do-not-propagate-me"))

		By("getting DNSEndpoint")
		de := dnsEndpoint()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}).Should(Succeed())

		deAnnotations := de.GetAnnotations()
		Expect(deAnnotations).ToNot(BeNil())
		Expect(deAnnotations["example.com/propagate-me"]).To(Equal("yes"))
		Expect(deAnnotations).ToNot(HaveKey("example.com/do-not-propagate-me"))
	})

	It("should progatate labels to the generated resources", func() {
		ns := testNamespacePrefix + randomString(10)
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: ns},
		})).ShouldNot(HaveOccurred())

		scm, mgr := setupManager()

		Expect(SetupReconciler(mgr, scm, ReconcilerOptions{
			ServiceKey:        testServiceKey,
			DefaultIssuerName: "test-issuer",
			DefaultIssuerKind: IssuerKind,
			CreateCertificate: true,
			CreateDNSEndpoint: true,
			PropagatedLabels: []string{
				"example.com/propagate-me",
			},
		})).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer stopMgr()

		By("creating HTTPProxy with labels")
		hpKey := client.ObjectKey{Name: "foo", Namespace: ns}
		hp := newDummyHTTPProxy(hpKey)
		hp.Labels = map[string]string{
			"example.com/propagate-me":     "yes",
			"example.com/do-not-propagate": "yes",
		}
		Expect(k8sClient.Create(context.Background(), hp)).ShouldNot(HaveOccurred())

		By("getting Certificate")
		crt := certificate()
		objKey := client.ObjectKey{
			Name:      hpKey.Name,
			Namespace: hpKey.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		crtLabels := crt.GetLabels()
		Expect(crtLabels).ToNot(BeNil())
		Expect(crtLabels["example.com/propagate-me"]).To(Equal("yes"))
		Expect(crtLabels).ToNot(HaveKey("example.com/do-not-propagate"))

		crtSpec := crt.UnstructuredContent()["spec"].(map[string]interface{})
		Expect(crtSpec).ToNot(BeNil())
		secretTemplate := crtSpec["secretTemplate"].(map[string]interface{})
		Expect(secretTemplate).ToNot(BeNil())
		secretLabels := secretTemplate["labels"].(map[string]interface{})
		Expect(secretLabels).ToNot(BeNil())
		Expect(secretLabels["example.com/propagate-me"]).To(Equal("yes"))
		Expect(secretLabels).ToNot(HaveKey("example.com/do-not-propagate"))

		By("getting DNSEndpoint")
		de := dnsEndpoint()
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, de)
		}).Should(Succeed())

		deLabels := de.GetLabels()
		Expect(deLabels).ToNot(BeNil())
		Expect(deLabels["example.com/propagate-me"]).To(Equal("yes"))
		Expect(deLabels).ToNot(HaveKey("example.com/do-not-propagate"))
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
			name:             "Spec is not set",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: projectcontourv1.HTTPProxySpec{},
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
			name:             "Both annotation and spec are set",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						ingressClassNameAnnotation:        "class-name",
						contourIngressClassNameAnnotation: "class-name",
					},
				},
				Spec: projectcontourv1.HTTPProxySpec{
					IngressClassName: "class-name",
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
			name:             "Both annotation and spec are set but not matched",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						ingressClassNameAnnotation:        "class-name",
						contourIngressClassNameAnnotation: "wrong",
					},
				},
				Spec: projectcontourv1.HTTPProxySpec{
					IngressClassName: "class-name",
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
		{
			name:             "Spec is set",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: projectcontourv1.HTTPProxySpec{
					IngressClassName: "class-name",
				},
			},
			want: true,
		},
		{
			name:             "Spec is set but not matched",
			ingressClassName: "class-name",
			hp: &projectcontourv1.HTTPProxy{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: projectcontourv1.HTTPProxySpec{
					IngressClassName: "wrong",
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

func TestMakeDelegationEndpoint(t *testing.T) {
	tests := []struct {
		name            string
		hostname        string
		delegatedDomain string
		expectDNSName   string
		expectTarget    string
	}{
		{
			name:            "Hostname without trailing dot",
			hostname:        "example.com",
			delegatedDomain: "delegated.com",
			expectDNSName:   "_acme-challenge.example.com",
			expectTarget:    "_acme-challenge.example.com.delegated.com",
		},
		{
			name:            "Fully-qualified domain name",
			hostname:        "example.com.",
			delegatedDomain: "delegated.com",
			expectDNSName:   "_acme-challenge.example.com",
			expectTarget:    "_acme-challenge.example.com.delegated.com",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actuals := makeDelegationEndpoint(tc.hostname, tc.delegatedDomain)
			if len(actuals) != 1 {
				t.Errorf("HTTPProxyReconciler.makeDelegationEndpoint() = %v, want 1 item", len(actuals))
			}
			actual := actuals[0]
			if actual["dnsName"] != tc.expectDNSName {
				t.Errorf("HTTPProxyReconciler.makeDelegationEndpoint() dnsName = %v, want %v", actual["dnsName"], tc.expectDNSName)
			}
			actualTargets := actual["targets"].([]string)
			if len(actualTargets) != 1 {
				t.Errorf("HTTPProxyReconciler.makeDelegationEndpoint() targets = %v, want 1 item", len(actualTargets))
			}
			if actualTargets[0] != tc.expectTarget {
				t.Errorf("HTTPProxyReconciler.makeDelegationEndpoint() target = %v, want %v", actualTargets[0], tc.expectTarget)
			}
		})
	}
}
