Usage
=====

contour-plus is an add-on controller for [Contour][]'s [HTTPProxy][].

It helps integration of Contour with [external-dns][] and [cert-manager][].

Command-line flags and environment variables
--------------------------------------------

contour-plus takes following command-line flags or environment variables.
If both is specified, command-line flags take precedence.

| Flag                  | Envvar                   | Default                   | Description                                        |
| --------------------- | ------------------------ | ------------------------- | -------------------------------------------------- |
| `metrics-addr`        | `CP_METRICS_ADDR`        | :8180                     | Bind address for the metrics endpoint              |
| `crds`                | `CP_CRDS`                | `DNSEndpoint,Certificate` | Comma-separated list of CRDs to be created.        |
| `name-prefix`         | `CP_NAME_PREFIX`         | ""                        | Prefix of CRD names to be created                  |
| `service-name`        | `CP_SERVICE_NAME`        | ""                        | NamespacedName of the Contour LoadBalancer Service |
| `default-issuer-name` | `CP_DEFAULT_ISSUER_NAME` | ""                        | Issuer name used by default                        |
| `default-issuer-kind` | `CP_DEFAULT_ISSUER_KIND` | `ClusterIssuer`           | Issuer kind used by default                        |
| `default-delegated-domain` | `CP_DEFAULT_DELEGATED_DOMAIN` | ""            | Domain to which DNS-01 validation is delegated to   |
| `allowed-delegated-domains` | `CP_ALLOWED_DELEGATED_DOMAINS` | []            | Comma-separated list of allowed delegated domains |
| `allow-custom-delegations` | `CP_ALLOW_CUSTOM_DELEGATIONS` | `false`       | Allow users to specify a custom delegated domain |
| `csr-revision-limit`  | `CP_CSR_REVISION_LIMIT`  | 0                         | Maximum number of CertificateRequests to be kept for a Certificate. By default, all CertificateRequests are kept             |
| `leader-election`     | `CP_LEADER_ELECTION`     | `true`                    | Enable / disable leader election                   |
| `ingress-class-name`  | `CP_INGRESS_CLASS_NAME`  | ""                        | Ingress class name that watched by Contour Plus. If not specified, then all classes are watched    |
| `propagated-annotations`  | `CP_PROPAGATED_ANNOTATIONS`  | ""                | Comma-separated list of annotation keys that should be propagated to the resources contour-plus generates |
| `propagated-labels     `  | `CP_PROPAGATED_LABELS`       | ""                | Comma-separated list of label keys that should be propagated to the resources contour-plus generates      |

By default, contour-plus creates [DNSEndpoint][] when `spec.virtualhost.fqdn` of an HTTPProxy is not empty,
and creates [Certificate][] when `spec.virtualhost.tls.secretName` is not empty and not namespaced.

When a delegated domain is specified, either via `default-delegated-domain` or the `contour-plus.cybozu.com/delegated-domain` annotation, contour-plus creates an additional [DNSEndpoint][] delegating DNS-01 validation to the given delegation domain. The delegation record will not be created if the DNSEndpoint for `spec.virtualhost.fqdn` cannot be created. If `allow-custom-delegations` is enabled, users will be able to specify a custom domain for delegation via the `contour-plus.cybozu.com/delegated-domain` annotation. To prevent users from being able to specify any arbitrary delegation domains, `allowed-delegated-domains` can be used to specify a list of permitted domains.

To disable CRD creation, specify `crds` command-line flag or `CP_CRDS` environment variable.

`service-name` is a required flag/envvar that must be the namespaced name of Service for Contour.
In a normal setup, Contour has a `type=LoadBalancer` Service to expose its Envoy pods to Internet.
By specifying `service-name`, contour-plus can identify the global IP address for FQDNs in HTTPProxy.

If `ingress-class-name` is specified, contour-plus watches only HTTPProxy annotated by `kubernetes.io/ingress.class=<ingress-class-name>`, `projectcontour.io/ingress.class=<ingress-class-name>` or with the `HTTPProxy.Spec.IngressClassName` field that matches the given `ingress-class-name`.
**If `kubernetes.io/ingress.class=<ingress-class-name>` , `projectcontour.io/ingress.class=<ingress-class-name>` and `HTTPProxy.Spec.IngressClassName` are all specified and those values are different from the given `ingress-class-name`, then contour-plus doesn't watch the resource.**

How it works
------------

contour-plus monitors events for [HTTPProxy][] and creates / updates / deletes
[DNSEndpoint][] and/or [Certificate][].

The container of contour-plus should be deployed as a sidecar of Contour/Envoy Pod.

### Leader election

Unless  `--leader-election` is set to `false`, contour-plus does leader election using
ConfigMap in the same namespace where its Pod exists.  In addition, it creates
Event to log leader election activity.

Therefore, the service account for Contour need to be bound to a Role like this:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: contour-plus
  namespace: ingress
rules:
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - ""
    resources:
      - configmaps/status
    verbs:
      - get
      - update
      - patch
  - apiGroups:
    - ""
    resources:
    - events
    verbs:
    - create
  - apiGroups:
      - coordination.k8s.io
      resources:
      - leases
      verbs:
      - '*'
```

### Certificate RBAC

The following permissions are needed to create/update `Certificates`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: contour-plus
  namespace: ingress
rules:
  - apiGroups:
    - cert-manager.io
    resources:
    - certificates
    verbs:
    - get
    - list
    - watch
    - patch
    - create
```

### Supported annotations

contour-plus interprets following annotations for HTTPProxy.

- `contour-plus.cybozu.com/exclude: "true"` - With this, contour-plus ignores this HTTPProxy.
- `cert-manager.io/issuer` - The name of an  [Issuer][] to acquire the certificate required for this HTTPProxy from. The Issuer must be in the same namespace as the HTTPProxy.
- `cert-manager.io/cluster-issuer` - The name of a [ClusterIssuer][Issuer] to acquire the certificate required for this ingress from. It does not matter which namespace your Ingress resides, as ClusterIssuers are non-namespaced resources.
- `cert-manager.io/revision-history-limit` - The maximum number of CertificateRequests to keep for a given Certificate.
- `cert-manager.io/private-key-algorithm` - The algorithm for the private key generation for a Certificate.
- `cert-manager.io/private-key-size` - If `cert-manager.io/private-key-algorithm` is set, this annotation allows the specification of the size of the private key.
- `kubernetes.io/tls-acme: "true"` - With this, contour-plus generates Certificate automatically from HTTPProxy.
- `contour-plus.cybozu.com/delegated-domain: "acme.example.com"` - With this, contour-plus generates a [DNSEndpoint][] to create a CNAME record pointing to the delegation domain for use when performing DNS-01 DCV during the Certificate creation.

If both of `cert-manager.io/issuer` and `cert-manager.io/cluster-issuer` exist, `cluster-issuer` takes precedence.

If `cert-manager.io/revision-history-limit` is present, it takes precedence over the value globally specified via the `--csr-revision-limit` command-line flag.

[Contour]: https://github.com/projectcontour/contour
[HTTPProxy]: https://projectcontour.io/docs/main/config/fundamentals/
[DNSEndpoint]: https://pkg.go.dev/github.com/kubernetes-sigs/external-dns/endpoint#DNSEndpoint
[external-dns]: https://github.com/kubernetes-sigs/external-dns
[Certificate]: https://cert-manager.io/docs/usage/certificate/
[cert-manager]: https://cert-manager.io/docs/
[Issuer]: https://cert-manager.io/docs/configuration/issuers/
