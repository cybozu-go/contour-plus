# Maintenance procedure

1. Update `go.mod` for dependencies other than Contour and cert-manager.
2. Update GitHub Actions used in `.github/workflows/` to their newest trusted version and commit hash.
3. Update Go & Ubuntu versions if needed.
4. Check for new software versions using `make version`. You may be prompted to login to github.com.
   ```console
   $ make version
   ```
   This also updates the tools managed by [aqua](https://aquaproj.github.io/) in `aqua.yaml`
   (`gh`, `yq`, `kubectl`, `kustomize`, `helm`, `kind`, `controller-gen`, `setup-envtest`, `golangci-lint`),
   bumps the registry `ref` to the latest [aqua-registry release](https://github.com/aquaproj/aqua-registry/releases),
   and regenerates `aqua-checksums.json` accordingly. Note two exceptions that aren't bumped to their own latest release:
   - `kustomize` is pinned to the version bundled in the Argo CD image, since that's what renders our manifests for deployment.
   - `kubectl` is pinned to the same version as `ENVTEST_K8S_VERSION`, to match the Kubernetes version used by envtest.
5. Check `Makefile.versions` and `aqua.yaml` and revert some changes that you don't want now.
   If you revert an `aqua.yaml` change, re-run `aqua update-checksum --prune` to keep `aqua-checksums.json` in sync.
   Then run `make check-generate lint test` to confirm the tool versions still work.
6. Update commit hashes and file checksums for the pinned dependency versions.
   ```console
   $ make update-hashes
   ```
   This resolves `EXTERNAL_DNS_COMMIT` and `CONTOUR_COMMIT` to the commit each pinned version's tag points to, and
   recomputes `CERTMANAGER_CRD_SHA256`/`CERTMANAGER_MANIFEST_SHA256` (cert-manager's manifests are GitHub Release
   assets with no git-tree equivalent, so they're pinned by checksum instead of commit hash).
7. Update Contour and cert-manager versions in `go.mod` to the `CONTOUR_VERSION` and `CERT_MANAGER_VERSION` pinned in the previous steps.
   `update-contour` also bumps `k8s.io/api`, `k8s.io/apimachinery`, and `k8s.io/client-go` to the latest patch of the Kubernetes minor version used by Contour.
   ```console
   $ make update-contour
   $ make update-cert-manager
   ```
8. Update software versions using `make maintenance`.
   ```console
   $ make maintenance
   ```
9. Update e2e test dependencies.
   ```console
   $ cd e2e
   $ make update-dependencies
   ```
10. Follow [RELEASE.md](/RELEASE.md) to update software version.
