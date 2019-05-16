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

Details
-------

cert-manager provides Go types and API to manage `Certificate` resource.

ExternalDNS provides Go types for `DNSEndpoint`, but does not provide strictly-typed
API client.  Therefore, `contour-plus` uses [kubebuilder][] to generate strictly-typed
API client for itself.

[IngressRoute]: https://github.com/heptio/contour/blob/master/docs/ingressroute.md
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
[Certificate]: https://docs.cert-manager.io/en/latest/reference/certificates.html
[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
