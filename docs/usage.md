Usage
=====

contour-plus is an add-on controller for [Contour][]'s [IngressRoute][].

It helps integration of Contour with [external-dns][] and [cert-manager][].

Command-line flags and environment variables
--------------------------------------------

contour-plus takes following command-line flags or environment variables.
If both is specified, command-line flags take precedence.

| Flag           | Envvar            | Default                   | Description                                                   |
| -------------- | ----------------- | ------------------------- | ------------------------------------------------------------- |
| `metrics-addr` | `CP_METRICS_ADDR` | :8080                     | Bind address for the metrics endpoint                         |
| `crds`         | `CP_CRDS`         | `DNSEndpoint,Certificate` | Comma-separated list of CRDs to be created                    |
| `name-prefix`  | `CP_NAME_PREFIX`  | ""                        | Prefix of CRD names to be created                             |
| `service-name` | `CP_SERVICE_NAME` | ""                        | NamespacedName of the Contour LoadBalancer Service (required) |

By default, contour-plus creates [DNSEndpoint][] when `spec.virtualhost.fqdn` of an IngressRoute is not empty,
and creates [Certificate][] when `spec.virtualhost.tls.secretName` is not empty and not namespaced.

To disable CRD creation, specify `crds` command-line flag or `CP_CRDS` environment variable.

`service-name` is a required flag/envvar that must be the namespaced name of Service for Contour.
In a normal setup, Contour has a `type=LoadBalancer` Service to expose its Envoy pods to Internet.
By specifying `service-name`, contour-plus can identify the global IP address for FQDNs in IngressRoute.

How it works
------------

contour-plus should be deployed with Deployment.  It monitors events for [IngressRoute][] and
creates / updates / deletes [DNSEndpoint][] and/or [Certificate][].

### Excluding IngressRoute from contour-plus targets

You can exclude an IngressRoute from contour-plus targets by adding the following annotation.

- `contour-plus.cybozu.com/exclude`: `true`

[Contour]: https://github.com/heptio/contour
[IngressRoute]: https://github.com/heptio/contour/blob/master/docs/ingressroute.md
[DNSEndpoint]: https://github.com/kubernetes-incubator/external-dns/blob/master/docs/contributing/crd-source.md
[external-dns]: https://github.com/kubernetes-incubator/external-dns
[Certificate]: http://docs.cert-manager.io/en/latest/reference/certificates.html
[cert-manager]: http://docs.cert-manager.io/en/latest/index.html
