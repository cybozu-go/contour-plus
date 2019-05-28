module github.com/cybozu-go/contour-plus

go 1.12

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go => k8s.io/client-go v11.0.0+incompatible
	launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
)

require (
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/heptio/contour v0.12.1
	github.com/jetstack/cert-manager v0.8.0
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/kubernetes-incubator/external-dns v0.5.14
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/spf13/cobra v0.0.4
	github.com/spf13/viper v1.3.2
	k8s.io/api v0.0.0-20190413052509-3cc1b3fb6d0f
	k8s.io/apimachinery v0.0.0-20190413052414-40a3f73b0fa2
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0-beta.1
	sigs.k8s.io/controller-tools v0.2.0-beta.1
)
