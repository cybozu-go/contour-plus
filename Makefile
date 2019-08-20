
# Image URL to use all building/pushing image targets
IMG ?= quay.io/cybozu/contour-plus:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

GO111MODULE = on
GOFLAGS     = -mod=vendor
export GO111MODULE GOFLAGS

GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
SUDO = sudo
KUBEBUILDER_VERSION = 2.0.0-rc.0
CTRLTOOLS_VERSION = 0.2.0-rc.0

all: bin/contour-plus

# Run tests
test: vet manifests
	test -z "$$(gofmt -s -l . | grep -v '^vendor' | tee /dev/stderr)"
	test -z "$$(golint $$(go list ./... | grep -v /vendor/) | tee /dev/stderr)"
	ineffassign .
	go test -v -count 1 ./controllers/... -coverprofile cover.out

# Build contour-plus binary
bin/contour-plus: main.go cmd/root.go controllers/ingressroute_controller.go
	CGO_ENABLED=0 go build -o $@ .

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=contour-plus paths="./..."

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./apis/contour/...

# Build the docker image
docker-build: bin/contour-plus
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	cd $(shell mktemp -d) && curl -sSLfO https://github.com/kubernetes-sigs/controller-tools/archive/v$(CTRLTOOLS_VERSION).tar.gz && tar -x -z -f v$(CTRLTOOLS_VERSION).tar.gz && cd controller-tools-$(CTRLTOOLS_VERSION) && GOFLAGS= go install ./cmd/controller-gen
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

clean:
	rm -f bin/contour-plus $(CONTROLLER_GEN)

setup:
	curl -sL https://go.kubebuilder.io/dl/$(KUBEBUILDER_VERSION)/$(GOOS)/$(GOARCH) | tar -xz -C /tmp/
	$(SUDO) mv /tmp/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH) /usr/local/kubebuilder
	$(SUDO) curl -o /usr/local/kubebuilder/bin/kustomize -sL https://go.kubebuilder.io/kustomize/$(GOOS)/$(GOARCH)
	$(SUDO) chmod a+x /usr/local/kubebuilder/bin/kustomize
	go install github.com/jstemmer/go-junit-report

mod:
	go mod tidy
	go mod vendor
	git add -f vendor
	git add go.mod

.PHONY: all test manifests vet generate docker-build docker-push setup mod
