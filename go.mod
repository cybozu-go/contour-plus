module github.com/cybozu-go/contour-plus

go 1.12

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190805141119-fdd30b57c827
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190805141520-2fe0317bcee0
	launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
)

require (
	github.com/go-logr/logr v0.1.0
	github.com/heptio/contour v0.12.1
	github.com/jetstack/cert-manager v0.8.0
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/kubernetes-incubator/external-dns v0.5.12
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/spf13/cobra v0.0.4
	github.com/spf13/viper v1.3.2
	k8s.io/api v0.0.0-20190805141119-fdd30b57c827
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v0.3.1
	k8s.io/kube-openapi v0.0.0-20190401085232-94e1e7b7574c // indirect
	sigs.k8s.io/controller-runtime v0.2.0
)
