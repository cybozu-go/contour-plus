# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/cybozu-go/contour-plus/compare/v0.6.3...HEAD
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
