Design notes
============

Background
----------

Contour uses a CRD called [`IngressRoute`][IngressRoute].  However, [ExternalDNS][]
and [cert-manager][] does not recognize it unlike the standard `Ingress`.

Fortunately, ExternalDNS can watch arbitrary CRD resources and manages external
DNS service such as AWS Route53 according to the CRD contents.  An example of
such a CRD is [`DNSEndpoint`](https://github.com/kubernetes-incubator/external-dns/blob/master/docs/contributing/crd-source/crd-manifest.yaml).

Similarly, cert-manager watches [`Certificate`][Certificate] CRD and issues
TLS certificates.

Goals
-----

- Automatic DNS record management for `IngressRoute`
- Automatic TLS certificate issuance for `IngressRoute`

How
---

Create a custom controller / operator called `contour-plus` that watches `IngressRoute`
and IP address of the load balancer (`Service`) for Contour.

When a new `IngressRoute` wants a FQDN to be routed, `contour-plus` creates
`DNSEndpoint` for ExternalDNS.  If a new `IngressRoute` wants a TLS certificate,
`contour-plus` creates `Certificate` for cert-manager.

When an existing `IngressRoute` is updated or removed, `contour-plus` updates or
deletes corresponding `DNSEndpoint` and/or `Certificate`.

This way, DNS records can be managed and TLS certificates can be issued automatically.

### Access CRDs

Contour provides Go types and API to manage `IngressRoute` resource:

- [`ContourV1beta1Client`](https://github.com/heptio/contour/blob/master/apis/generated/clientset/versioned/typed/contour/v1beta1/contour_client.go)
- [`IngressRoute`](https://github.com/heptio/contour/blob/03dcee7fedf52ba28852d75ff7752ec7ec0ae36c/apis/contour/v1beta1/ingressroute.go#L164)

cert-manager provides Go types and API to manage `Certificate` resource:

- [`Certificate`](https://github.com/jetstack/cert-manager/blob/3201d126d0441298805b8ff6165afebae4ce1550/pkg/apis/certmanager/v1alpha1/types_certificate.go#L32)
- [`CertmanagerV1alpha1Client`](https://github.com/jetstack/cert-manager/blob/8752770769d6d641c5ef6703b0a9b0bf11c2cf01/pkg/client/clientset/versioned/typed/certmanager/v1alpha1/certmanager_client.go#L86)

ExternalDNS provides Go types for `DNSEndpoint`, but does not provide strictly-typed
API client.  Therefore, `contour-plus` uses [kubebuilder][] to generate strictly-typed
API client for itself.

- [`DNSEndpoint`](https://github.com/kubernetes-incubator/external-dns/blob/d1bc8fe147f0ffd7cc4be3e9c6f693186b0aa0bf/endpoint/endpoint.go#L191)

[IngressRoute]: https://github.com/heptio/contour/blob/master/docs/ingressroute.md
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
[Certificate]: https://docs.cert-manager.io/en/latest/reference/certificates.html
[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
