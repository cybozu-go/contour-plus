[![GitHub release](https://img.shields.io/github/release/cybozu-go/contour-plus.svg?maxAge=60)][releases]
[![CircleCI](https://circleci.com/gh/cybozu-go/contour-plus.svg?style=svg)](https://circleci.com/gh/cybozu-go/contour-plus)
[![GoDoc](https://godoc.org/github.com/cybozu-go/contour-plus?status.svg)][godoc]
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/contour-plus)](https://goreportcard.com/report/github.com/cybozu-go/contour-plus)
[![Docker Repository on Quay](https://quay.io/repository/cybozu/contour-plus/status "Docker Repository on Quay")](https://quay.io/repository/cybozu/contour-plus)

Contour Plus
============

Contour Plus enhances [Contour][] for external-dns and cert-manager.

**Project Status**: Initial Development

Supported environments
----------------------

- Kubernetes
  - 1.14+
- Contour
  - 0.12.0+

Features
--------

**TBD**

Programs
--------

This repository contains these programs:

- `contour-plus`: Kubernetes controller to operate with [ExternalDNS][] and [cert-manager][]

Documentation
-------------

[docs](docs/) directory contains documents about designs and specifications.

[releases]: https://github.com/cybozu-go/contour-plus/releases
[godoc]: https://godoc.org/github.com/cybozu-go/contour-plus
[Contour]: https://github.com/heptio/contour
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager
