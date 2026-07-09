# Maintenance procedure

1. Update Contour version in `go.mod`.
   It also updates reference to Kubernetes in `go.mod`.
   The Kubernetes version is the one used by Contour, but the latest patch version.
   ```console
   $ make update-contour
   ```
2. Update `go.mod` for the other dependencies.
3. Update Go & Ubuntu versions if needed.
4. Update tool versions managed by [aqua](https://aquaproj.github.io/) (`gh`, `yq`, `kubectl`, `kustomize`, `helm`, `kind`, `controller-gen`, `setup-envtest`, `staticcheck`, `goimports`).
   Edit the `packages[].name` entries in `aqua.yaml`, and bump the registry `ref` if a newer [aqua-registry release](https://github.com/aquaproj/aqua-registry/releases) is available.
   Then regenerate `aqua-checksums.json`, pruning entries for versions that are no longer referenced.
   ```console
   $ aqua update-checksum --prune
   ```
   Run `make check-generate lint test` to confirm the new tool versions still work.
5. Check for new software versions using `make version`. You may be prompted to login to github.com.
   ```console
   $ make version
   ```
6. Check `Makefile.versions` and revert some changes that you don't want now.
7. Update software versions using `make maintenance`.
   ```console
   $ make maintenance
   ```
8. Update e2e test dependencies.
   ```console
   $ cd e2e
   $ make update-dependencies
   ```
9. Follow [RELEASE.md](/RELEASE.md) to update software version.
