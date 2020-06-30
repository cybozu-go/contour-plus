[![GitHub release](https://img.shields.io/github/release/cybozu-go/contour-plus.svg?maxAge=60)][releases]
[![CircleCI](https://circleci.com/gh/cybozu-go/contour-plus.svg?style=svg)](https://circleci.com/gh/cybozu-go/contour-plus)
[![GoDoc](https://godoc.org/github.com/cybozu-go/contour-plus?status.svg)][godoc]
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/contour-plus)](https://goreportcard.com/report/github.com/cybozu-go/contour-plus)

Contour Plus
============

Contour Plus enhances [Contour][] for [ExternalDNS][] and [cert-manager][].

**Project Status**: Testing for GA

Supported environments
----------------------

- Kubernetes
  - 1.17
- Contour
  - 1.6
- ExternalDNS
  - 0.7
- cert-manager
  - 0.15

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
[godoc]: https://godoc.org/github.com/cybozu-go/contour-plus
[Contour]: https://github.com/heptio/contour
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
[HTTPProxy]: https://github.com/projectcontour/contour/blob/master/site/docs/master/httpproxy.md
[DNSEndpoint]: https://github.com/kubernetes-incubator/external-dns/blob/master/docs/contributing/crd-source.md
[Certificate]: http://docs.cert-manager.io/en/latest/reference/certificates.html

Docker images
-------------

Docker images are available on [Quay.io](https://quay.io/repository/cybozu/contour-plus)
