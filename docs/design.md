Design notes
============

Background
----------

Contour controlls CRDs called [`IngressRoute`][IngressRoute] and [`HTTProxy`][HTTPProxy].  However, [ExternalDNS][]
and [cert-manager][] does not recognize it unlike the standard `Ingress`.

Fortunately, ExternalDNS can watch arbitrary CRD resources and manages external
DNS service such as AWS Route53 according to the CRD contents.  An example of
such a CRD is [`DNSEndpoint`](https://github.com/kubernetes-incubator/external-dns/blob/master/docs/contributing/crd-source/crd-manifest.yaml).

Similarly, cert-manager watches [`Certificate`][Certificate] CRD and issues
TLS certificates.

Goals
-----

- Automatic DNS record management for `IngressRoute` and `HTTPProxy`
- Automatic TLS certificate issuance for `IngressRoute` and `HTTPProxy`

How
---

Create a custom controller / operator called `contour-plus` that watches `IngressRoute`/`HTTPProxy`
and IP address of the load balancer (`Service`) for Contour.

When a new `IngressRoute`/`HTTPProxy` wants a FQDN to be routed, `contour-plus` creates
`DNSEndpoint` for ExternalDNS.  If a new `IngressRoute`/`HTTPProxy` wants a TLS certificate,
`contour-plus` creates `Certificate` for cert-manager.

When an existing `IngressRoute`/`HTTPProxy` is updated or removed, `contour-plus` updates or
deletes corresponding `DNSEndpoint` and/or `Certificate`.

This way, DNS records can be managed and TLS certificates can be issued automatically.

### Access CRDs

Contour provides Go types and API to manage `IngressRoute`/`HTTPProxy` resource:

- [`contourv1Client`](https://github.com/projectcontour/contour/blob/81f2c011656304973d2bd00fa6034d2b1ea6e60f/apis/generated/clientset/versioned/typed/contour/v1beta1/contour_client.go#L70)
- [`IngressRoute`](https://github.com/heptio/contour/blob/03dcee7fedf52ba28852d75ff7752ec7ec0ae36c/apis/contour/v1/ingressroute.go#L164)
- [`projectcontourv1Client`](https://github.com/projectcontour/contour/blob/81f2c011656304973d2bd00fa6034d2b1ea6e60f/apis/generated/clientset/versioned/typed/projectcontour/v1/projectcontour_client.go#L70)
- [`HTTPProxy`](https://github.com/projectcontour/contour/blob/81f2c011656304973d2bd00fa6034d2b1ea6e60f/apis/projectcontour/v1/httpproxy.go#L268)


cert-manager provides Go types and API to manage `Certificate` resource:

- [`Certificate`](https://github.com/jetstack/cert-manager/blob/0aba30b25123e729d9dc8602cdcc4a5cc4b73bef/pkg/apis/certmanager/v1alpha2/types_certificate.go#L37)
- [`certmanagerv1alpha2Client`](https://github.com/jetstack/cert-manager/blob/0aba30b25123e729d9dc8602cdcc4a5cc4b73bef/pkg/client/clientset/versioned/typed/certmanager/v1alpha2/certmanager_client.go#L80)

ExternalDNS provides Go types for `DNSEndpoint`, but does not provide strictly-typed
API client.  Therefore, `contour-plus` uses [kubebuilder][] to generate strictly-typed
API client for itself.

- [`DNSEndpoint`](https://github.com/kubernetes-incubator/external-dns/blob/d1bc8fe147f0ffd7cc4be3e9c6f693186b0aa0bf/endpoint/endpoint.go#L191)

[IngressRoute]: https://github.com/heptio/contour/blob/master/docs/ingressroute.md
[HTTPProxy]: https://github.com/projectcontour/contour/blob/master/site/docs/master/httpproxy.md
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
[Certificate]: https://docs.cert-manager.io/en/latest/reference/certificates.html
[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
