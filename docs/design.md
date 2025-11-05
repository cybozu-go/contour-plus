Design notes
============

Background
----------

Contour controlls CRDs called [`HTTProxy`][HTTPProxy].  However, [ExternalDNS][]
and [cert-manager][] does not recognize it unlike the standard `Ingress`.

Fortunately, ExternalDNS can watch arbitrary CRD resources and manages external
DNS service such as AWS Route53 according to the CRD contents.  An example of
such a CRD is [`DNSEndpoint`][DNSEndpoint].

Similarly, cert-manager watches [`Certificate`][Certificate] CRD and issues
TLS certificates.

Goals
-----

- Automatic DNS record management for `HTTPProxy`
- Automatic TLS certificate issuance for `HTTPProxy`

How
---

Create a custom controller / operator called `contour-plus` that watches `HTTPProxy`
and IP address of the load balancer (`Service`) for Contour.

When a new `HTTPProxy` wants a FQDN to be routed, `contour-plus` creates
`DNSEndpoint` for ExternalDNS.  If a new `HTTPProxy` wants a TLS certificate,
`contour-plus` creates `Certificate` for cert-manager.

When an existing `HTTPProxy` is updated or removed, `contour-plus` updates or
deletes corresponding `DNSEndpoint` and/or `Certificate`.

This way, DNS records can be managed and TLS certificates can be issued automatically.

### Access CRDs

Contour provides Go types and API to manage `HTTPProxy` resource:

- [`HTTPProxy`][HTTPProxy]


cert-manager provides Go types and API to manage `Certificate` resource:

- [`Certificate`](https://pkg.go.dev/github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1#Certificate)

ExternalDNS provides Go types for `DNSEndpoint`, but does not provide strictly-typed
API client.  Therefore, `contour-plus` uses [kubebuilder][] to generate strictly-typed
API client for itself.

- [`DNSEndpoint`][DNSEndpoint]

[HTTPProxy]: https://pkg.go.dev/github.com/projectcontour/contour/apis/projectcontour/v1#HTTPProxy
[ExternalDNS]: https://github.com/kubernetes-sigs/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
[Certificate]: https://cert-manager.io/docs/usage/certificate/
[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
[DNSEndpoint]: https://pkg.go.dev/github.com/kubernetes-sigs/external-dns/endpoint#DNSEndpoint
