package controllers

import (
	"context"
	"fmt"
	"time"

	contourv1beta1 "github.com/heptio/contour/apis/contour/v1beta1"
	certmanagerv1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/kubernetes-incubator/external-dns/endpoint"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testReconcile() {
	It("should create DNSEndpoint and Certificate", func() {
		scm := scheme.Scheme
		Expect(setupScheme(scm)).ShouldNot(HaveOccurred())

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scm})
		Expect(err).ShouldNot(HaveOccurred())

		Expect(setupReconciler(&mgr, scm, mgr.GetClient())).ShouldNot(HaveOccurred())

		stopMgr := startTestManager(mgr)
		defer func() {
			stopMgr()
		}()

		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{Name: "test-ns"},
		}))
		By("creating IngressRoute")
		irKey := client.ObjectKey{
			Name:      "foo",
			Namespace: "test-ns",
		}
		dnsName := "test.example.com"
		Expect(k8sClient.Create(context.Background(), &contourv1beta1.IngressRoute{
			ObjectMeta: v1.ObjectMeta{
				Namespace: irKey.Namespace,
				Name:      irKey.Name,
				Annotations: map[string]string{
					testACMETLSAnnotation: "true",
				},
			},
			Spec: contourv1beta1.IngressRouteSpec{
				VirtualHost: &contourv1beta1.VirtualHost{
					Fqdn: dnsName,
					TLS:  &contourv1beta1.TLS{SecretName: "test-secret"},
				},
				Routes: []contourv1beta1.Route{},
			},
		})).ShouldNot(HaveOccurred())

		By("getting DNSEndpoint")
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

		By("getting Certificate")
		crt := &certmanagerv1alpha1.Certificate{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), objKey, crt)
		}).Should(Succeed())

		fmt.Println("aaaaaaaaaaaaaaaaaa")
		fmt.Println(crt.Spec.SecretName)
		fmt.Println("aaaaaaaaaaaaaaaaaa")
	})
}
