# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.11.0] - 2024-01-10

### Breaking Changes

#### Migrate image registry

We migrated the image repository of contour-plus to `ghcr.io`.
From contour-plus v0.11.0, please use the following image.

- https://github.com/cybozu-go/contour-plus/pkgs/container/contour-plus

The [quay.io/cybozu/contour-plus](https://quay.io/repository/cybozu/contour-plus) will not be updated in the future.

### Changed

- Migrate to ghcr.io (#96)

## [0.10.0] - 2023-11-13

### Changed

- Update contour to 1.27.0 and Kubernetes to 1.28 (#94)
  - Kubernetes: 1.28
  - Contour: 1.27
  - ExternalDNS: 0.13
  - cert-manager: 1.13

## [0.9.0] - 2023-04-27

### Changed

- Support k8s 1.26 and update dependencies (#90)
  - Kubernetes: 1.26
  - Contour: 1.24
  - ExternalDNS: 0.13
  - cert-manager: 1.10

## [0.8.1] - 2022-11-02

### Changed

- Update dependencies (#86)
  - Kubernetes: 1.25
  - Contour: 1.23


## [0.8.0] - 2022-09-30

### Changed

- Update dependencies (#84)
  - Kubernetes: 1.24
  - Contour: 1.22
  - ExternalDNS: 0.12
  - cert-manager: 1.9
  - depended go packages and Actions


## [0.7.0] - 2022-04-12

### Changed

- Update supported Kubernetes version to 1.23 (#78)
  - Kubernetes: 1.23
  - Contour: 1.20
  - ExternalDNS: 0.11
  - cert-manager: 1.7
- Specify k8s version instead of using latest one during envtest (#77)
- Update Makefile and Actions (#79)

## [0.6.6] - 2021-12-10

### Changed

- update supported k8s version to 1.22 (#75)
  - kubernetes 1.22.1
  - contour 1.19.1
  - cert-manager 1.6.1
  - external-dns 0.10.1
- Change LICENSE from MIT to Apache 2.0 (#73)

## [0.6.5] - 2021-09-17

## Changed
- follow golang 1.17 and dependent software updates (#71)
  - golang 1.17
  - contour 1.18.1
  - cert-manager 1.5.3
  - external-dns 0.9.0

## [0.6.4] - 2021-08-03

### Changed
- Update contour to 1.18.0 (#68)
  - Add support for the newly added HTTPProxy.Spec.IngressClassName

## [0.6.3] - 2021-07-27

### Changed
- Update contour to 1.17.1 (#65)
- Update controller-runtime to 0.9.3 (#65)

## [0.6.2] - 2021-04-20

### Changed

- Fix the issue that options cannot be specified with environment variables. (#61)

## [0.6.1] - 2021-04-19

### Changed

- Update contour to 1.14.1 (#59)
- Update controller-runtime to 0.8.3 (#59)

## [0.6.0] - 2021-02-01

### Changed

- Update contour to 1.11.0 (#53)
- Update controller-runtime to 0.7.2 (#53)

## [0.5.2] - 2020-10-20

### Changed

- Update contour to 1.9.0 (#48).
- Use cert-manager v1 API Endpoint (#48).
- Remove compile dependency on cert-manager and external-dns (#48).
- Stop vendoring dependencies (#50).

## [0.5.1] - 2020-10-02

### Changed

- Update controller-runtime to 0.6.3 (#46).

### Fixed

- Do not reconcile being-deleted object (#46).

## [0.5.0] - 2020-06-30

### Changed

- Update contour to 1.6.0, controller-runtime to 0.6.0 (#44).

### Removed

- Support for IngressRoute has been discontinued (#44).

## [0.4.3] - 2020-05-08

### Changed

- Add key usages to certificate resources to generate (#40).

## [0.4.2] - 2020-04-21

### Changed

- Update contour to 1.3.0 and cert-manager to 0.14.1 (#35).
- Update dependent packages for k8s v1.17.5 (#37).

## [0.4.1] - 2020-03-27

### Changed

- Update dependent packages for k8s v1.17 (#33).

## [0.4.0] - 2019-12-13

### Added

- Class-name filter (#28).

## [0.3.1] - 2019-12-04

### Changed

- Fix missing initialize options of HTTPProxy controller (#25).

## [0.3.0] - 2019-11-13

### Changed

- Update contour to 1.0.0, controller-runtime to 0.3.0, and cert-manager to 0.11.0 (#20).
- Change the API version of Certificate resource from certmanger.k8s.io/v1alpha1 to cert-manger.io/v1alpha2 (#20).
- Update controller-tools to 0.2.2 and kubebuilder to 2.1.0 (#21).

### Added

- Support HTTPProxy resource (#20).

## [0.2.7] - 2019-08-26

### Changed

- Add "list" verb for Services to the RBAC manifest (#18).
- Update kubebuilder to 2.0.0 (#18).
- Update controller-runtime and controller-tools to 0.2.0 (#18).

## [0.2.6] - 2019-06-12

### Changed

- Update controller-runtime to v0.2.0-beta.2 (#12).

## [0.2.5] - 2019-06-06

### Changed

- Enable leader election by default (#11).
- Tidy up and fix bugs (#10).

## [0.2.4] - 2019-06-03

### Changed

- Fixed resource name of Certificate in RBAC (#9).

## [0.2.3] - 2019-06-03

### Changed

- Fixed null pointer exception bugs (#10).

## [0.2.2] - 2019-05-30

### Changed

- Changed the default port of metrics server to :8180 (#8).
- Do not mandate --service-name as it can be passed via CP_SERVICE_NAME envvar too (#8).

## [0.2.1] - 2019-05-29

### Changed

- Initialize klog flags (#7).

## [0.2.0] - 2019-05-28

### Added

- Leader election (#5).

## [0.1.0] - 2019-05-28

### Added

- Implement `contour-plus`
    - with [kubebuilder][] v2.0.0-alpha.2
    - with [controller-runtime][] v0.2.0-beta.1
    - for [Contour][] v0.12.1
    - for [ExternalDNS][] v0.5.14
    - for [cert-manager][] v0.8.0

[Unreleased]: https://github.com/cybozu-go/contour-plus/compare/v0.11.0...HEAD
[0.11.0]: https://github.com/cybozu-go/contour-plus/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/cybozu-go/contour-plus/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/cybozu-go/contour-plus/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/cybozu-go/contour-plus/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/cybozu-go/contour-plus/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/cybozu-go/contour-plus/compare/v0.6.6...v0.7.0
[0.6.6]: https://github.com/cybozu-go/contour-plus/compare/v0.6.5...v0.6.6
[0.6.5]: https://github.com/cybozu-go/contour-plus/compare/v0.6.4...v0.6.5
[0.6.4]: https://github.com/cybozu-go/contour-plus/compare/v0.6.3...v0.6.4
[0.6.3]: https://github.com/cybozu-go/contour-plus/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/cybozu-go/contour-plus/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/cybozu-go/contour-plus/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/cybozu-go/contour-plus/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/cybozu-go/contour-plus/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/cybozu-go/contour-plus/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/cybozu-go/contour-plus/compare/v0.4.3...v0.5.0
[0.4.3]: https://github.com/cybozu-go/contour-plus/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/cybozu-go/contour-plus/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/cybozu-go/contour-plus/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/cybozu-go/contour-plus/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/cybozu-go/contour-plus/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/cybozu-go/contour-plus/compare/v0.2.7...v0.3.0
[0.2.7]: https://github.com/cybozu-go/contour-plus/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/cybozu-go/contour-plus/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/cybozu-go/contour-plus/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/cybozu-go/contour-plus/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/cybozu-go/contour-plus/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/cybozu-go/contour-plus/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/cybozu-go/contour-plus/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cybozu-go/contour-plus/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cybozu-go/contour-plus/compare/e51fdf92f56eaf3e9eb4b3cce6527dc6d97626e3...v0.1.0
[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[Contour]: https://github.com/heptio/contour
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager/tree/v0.8.0
