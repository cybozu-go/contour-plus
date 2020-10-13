module github.com/cybozu-go/contour-plus

go 1.13

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

replace github.com/projectcontour/contour/apis/contour/v1beta1 => github.com/projectcontour/contour v1.6.0

require (
	github.com/go-logr/logr v0.2.1-0.20200730175230-ee2de8da5be6
	github.com/jetstack/cert-manager v1.0.3
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/projectcontour/contour v1.9.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.4.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/external-dns v0.7.4
)
