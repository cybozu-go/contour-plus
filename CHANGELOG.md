# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.2.2] - 2019-05-30

### Fixed
- Changed the default port of metrics server to :8180 (#8)
- Do not mandate --service-name as it can be passed via CP_SERVICE_NAME envvar too (#8)

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

[Unreleased]: https://github.com/cybozu-go/contour-plus/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/cybozu-go/contour-plus/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/cybozu-go/contour-plus/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cybozu-go/contour-plus/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cybozu-go/contour-plus/compare/e51fdf92f56eaf3e9eb4b3cce6527dc6d97626e3...v0.1.0
[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[Contour]: https://github.com/heptio/contour
[ExternalDNS]: https://github.com/kubernetes-incubator/external-dns
[cert-manager]: https://github.com/jetstack/cert-manager/tree/v0.8.0