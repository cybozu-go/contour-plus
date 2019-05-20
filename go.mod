module github.com/cybozu-go/contour-plus

go 1.12

require (
	github.com/go-logr/logr v0.1.0
	github.com/heptio/contour v0.12.0 // indirect
	github.com/kubernetes-incubator/external-dns v0.5.14 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0-beta.1
)
