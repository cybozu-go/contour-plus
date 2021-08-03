[![GitHub release](https://img.shields.io/github/release/cybozu-go/contour-plus.svg?maxAge=60)][releases]
[![CI](https://github.com/cybozu-go/contour-plus/workflows/main/badge.svg)](https://github.com/cybozu-go/contour-plus/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/cybozu-go/contour-plus.svg)](https://pkg.go.dev/github.com/cybozu-go/contour-plus)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/contour-plus)](https://goreportcard.com/report/github.com/cybozu-go/contour-plus)

Contour Plus
============

Contour Plus enhances [Contour][] for [ExternalDNS][] and [cert-manager][].

**Project Status**: Testing for GA

Supported environments
----------------------

- Kubernetes
  - 1.21
- Contour
  - 1.18
- ExternalDNS
  - 0.7
- cert-manager
  - 1.3

Other versions may or may not work.

Features
--------

- Create/update/delete [DNSEndpoint][] for ExternalDNS according to FQDN in [HTTPProxy][].
- Create/update/delete [Certificate][] for cert-manager when [HTTPProxy][] is annotated with `kubernetes.io/tls-acme: true`.

Other features are described in [docs/usage.md](docs/usage.md).

Documentation
-------------

[docs](docs/) directory contains documents about designs and specifications.

[releases]: https://github.com/cybozu-go/contour-plus/releases
[godoc]: https://pkg.go.dev/github.com/cybozu-go/contour-plus
[Contour]: https://github.com/projectcontour/contour
[ExternalDNS]: https://github.com/kubernetes-sigs/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
[HTTPProxy]: https://projectcontour.io/docs/v1.11.0/config/api/#projectcontour.io/v1.HTTPProxy
[DNSEndpoint]: https://github.com/kubernetes-sigs/external-dns/blob/master/docs/contributing/crd-source.md
[Certificate]: http://docs.cert-manager.io/en/latest/reference/certificates.html

Docker images
-------------

Docker images are available on [Quay.io](https://quay.io/repository/cybozu/contour-plus)
