# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.2.6] - 2019-06-12

### Fixed

- Update controller-runtime to v0.2.0-beta.2 (#12).

## [0.2.5] - 2019-06-06

### Fixed

- Enable leader election by default (#11).
- Tidy up and fix bugs (#10).

## [0.2.4] - 2019-06-03

### Fixed

- Fixed resource name of Certificate in RBAC (#9).

## [0.2.3] - 2019-06-03

### Fixed

- Fixed null pointer exception bugs (#10).

## [0.2.2] - 2019-05-30

### Fixed

- Changed the default port of metrics server to :8180 (#8).
- Do not mandate --service-name as it can be passed via CP_SERVICE_NAME envvar too (#8).

## [0.2.1] - 2019-05-29

### Fixed

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

[Unreleased]: https://github.com/cybozu-go/contour-plus/compare/v0.2.6...HEAD
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
