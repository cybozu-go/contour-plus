module github.com/cybozu-go/contour-plus

go 1.13

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
	launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
)

require (
	github.com/go-logr/logr v0.1.0
	github.com/jetstack/cert-manager v0.11.0
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/kubernetes-incubator/external-dns v0.5.12
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/projectcontour/contour v1.0.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.3.2
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v0.4.0
	sigs.k8s.io/controller-runtime v0.3.0
)
