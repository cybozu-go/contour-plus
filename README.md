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
  - 1.32
- Contour
  - 1.30
- ExternalDNS
  - 0.15
- cert-manager
  - 1.17

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
[cert-manager]: https://github.com/cert-manager/cert-manager
[HTTPProxy]: https://projectcontour.io/docs/v1.11.0/config/api/#projectcontour.io/v1.HTTPProxy
[DNSEndpoint]: https://github.com/kubernetes-sigs/external-dns/blob/master/docs/contributing/crd-source.md
[Certificate]: https://cert-manager.io/docs/reference/api-docs/#cert-manager.io/v1.Certificate

Docker images
-------------

Docker images are available on [ghcr.io](https://github.com/cybozu-go/contour-plus/pkgs/container/contour-plus)
