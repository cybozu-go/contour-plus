# Maintenance procedure

1. Update Contour version in `go.mod`.
   It also updates reference to Kubernetes in `go.mod`.
   The Kubernetes version is the one used by Contour, but the latest patch version.
   ```console
   $ make update-contour
   ```
2. Update `go.mod` for the other dependencies.
3. Update Go & Ubuntu versions if needed.
4. Update `CONTROLLER_TOOLS_VERSION` in `Makefile`.  
5. Check for new software versions using `make version`. You may be prompted to login to github.com.
   ```console
   $ make version
   ```
6. Check `Makefile.versions` and revert some changes that you don't want now.
7. Update software versions using `make maintenance`.
   ```console
   $ make maintenance
   ```
8. Update e2e dependencies.
   ```console
   $ cd e2e
   $ make update-dependencies
   ```
9. Follow [RELEASE.md](/RELEASE.md) to update software version.
