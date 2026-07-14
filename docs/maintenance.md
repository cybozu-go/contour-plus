# Maintenance procedure

1. Update Contour version in `go.mod`.
   It also updates reference to Kubernetes in `go.mod`.
   The Kubernetes version is the one used by Contour, but the latest patch version.
   ```console
   $ make update-contour
   ```
2. Update `go.mod` for the other dependencies.
3. Update Go & Ubuntu versions if needed.
4. Check for new software versions using `make version`. You may be prompted to login to github.com.
   ```console
   $ make version
   ```
   This also updates the tools managed by [aqua](https://aquaproj.github.io/) in `aqua.yaml`
   (`gh`, `yq`, `kubectl`, `kustomize`, `helm`, `kind`, `controller-gen`, `setup-envtest`, `staticcheck`, `goimports`),
   bumps the registry `ref` to the latest [aqua-registry release](https://github.com/aquaproj/aqua-registry/releases),
   and regenerates `aqua-checksums.json` accordingly. Note two exceptions that aren't bumped to their own latest release:
   - `kustomize` is pinned to the version bundled in the Argo CD image, since that's what renders our manifests for deployment.
   - `kubectl` is pinned to the same version as `ENVTEST_K8S_VERSION`, to match the Kubernetes version used by envtest.
5. Check `Makefile.versions` and `aqua.yaml` and revert some changes that you don't want now.
   If you revert an `aqua.yaml` change, re-run `aqua update-checksum --prune` to keep `aqua-checksums.json` in sync.
   Then run `make check-generate lint test` to confirm the tool versions still work.
6. Update software versions using `make maintenance`.
   ```console
   $ make maintenance
   ```
7. Update e2e test dependencies.
   ```console
   $ cd e2e
   $ make update-dependencies
   ```
8. Follow [RELEASE.md](/RELEASE.md) to update software version.
