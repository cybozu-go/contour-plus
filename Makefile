
# Image URL to use all building/pushing image targets
IMG ?= quay.io/cybozu/contour-plus:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

GO111MODULE = on
GOFLAGS     = -mod=vendor
export GO111MODULE GOFLAGS

all: bin/contour-plus

# Run tests
test: vet manifests
	test -z "$$(gofmt -s -l . | grep -v '^vendor' | tee /dev/stderr)"
	test -z "$$(golint $$(go list ./... | grep -v /vendor/) | tee /dev/stderr)"
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
	go install sigs.k8s.io/controller-tools/cmd/controller-gen
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

clean:
	rm -f bin/contour-plus $(CONTROLLER_GEN)

.PHONY: all test manifests vet generate docker-build docker-push
