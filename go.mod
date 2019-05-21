module github.com/cybozu-go/contour-plus

go 1.12

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

replace k8s.io/client-go => k8s.io/client-go v11.0.0+incompatible

require (
	cloud.google.com/go v0.38.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/heptio/contour v0.12.0
	github.com/jetstack/cert-manager v0.7.2
	github.com/kubernetes-incubator/external-dns v0.5.14
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/spf13/viper v1.3.2
	golang.org/x/crypto v0.0.0-20190426145343-a29dc8fdc734 // indirect
	golang.org/x/net v0.0.0-20190502183928-7f726cade0ab // indirect
	golang.org/x/sys v0.0.0-20190502175342-a43fa875dd82 // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0-beta.1
)
