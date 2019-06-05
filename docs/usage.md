Usage
=====

contour-plus is an add-on controller for [Contour][]'s [IngressRoute][].

It helps integration of Contour with [external-dns][] and [cert-manager][].

Command-line flags and environment variables
--------------------------------------------

contour-plus takes following command-line flags or environment variables.
If both is specified, command-line flags take precedence.

| Flag                  | Envvar                   | Default                   | Description                                        |
| --------------------- | ------------------------ | ------------------------- | -------------------------------------------------- |
| `metrics-addr`        | `CP_METRICS_ADDR`        | :8180                     | Bind address for the metrics endpoint              |
| `crds`                | `CP_CRDS`                | `DNSEndpoint,Certificate` | Comma-separated list of CRDs to be created         |
| `name-prefix`         | `CP_NAME_PREFIX`         | ""                        | Prefix of CRD names to be created                  |
| `service-name`        | `CP_SERVICE_NAME`        | ""                        | NamespacedName of the Contour LoadBalancer Service |
| `default-issuer-name` | `CP_DEFAULT_ISSUER_NAME` | ""                        | Issuer name used by default                        |
| `default-issuer-kind` | `CP_DEFAULT_ISSUER_KIND` | `ClusterIssuer`           | Issuer kind used by default                        |
| `leader-election`     | `CP_LEADER_ELECTION`     | `true`                    | Enable / disable leader election                   |

By default, contour-plus creates [DNSEndpoint][] when `spec.virtualhost.fqdn` of an IngressRoute is not empty,
and creates [Certificate][] when `spec.virtualhost.tls.secretName` is not empty and not namespaced.

To disable CRD creation, specify `crds` command-line flag or `CP_CRDS` environment variable.

`service-name` is a required flag/envvar that must be the namespaced name of Service for Contour.
In a normal setup, Contour has a `type=LoadBalancer` Service to expose its Envoy pods to Internet.
By specifying `service-name`, contour-plus can identify the global IP address for FQDNs in IngressRoute.

How it works
------------

contour-plus monitors events for [IngressRoute][] and creates / updates / deletes
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
```

### Supported annotations

contour-plus interprets following annotations for IngressRoute.

- `contour-plus.cybozu.com/exclude: "true"` - With this, contour-plus ignores this IngressRoute. 
- `certmanager.k8s.io/issuer` - The name of an  [Issuer][] to acquire the certificate required for this Ingressroute from. The Issuer must be in the same namespace as the IngressRoute.
- `certmanager.k8s.io/cluster-issuer` - The name of a [ClusterIssuer][Issuer] to acquire the certificate required for this ingress from. It does not matter which namespace your Ingress resides, as ClusterIssuers are non-namespaced resources.
- `kubernetes.io/tls-acme: "true"` - With this, contour-plus generates Certificate automatically from IngressRoute.

If both of `certmanager.k8s.io/issuer` and `certmanager.k8s.io/cluster-issuer` exist, `cluster-issuer` takes precedence.

[Contour]: https://github.com/heptio/contour
[IngressRoute]: https://github.com/heptio/contour/blob/master/docs/ingressroute.md
[DNSEndpoint]: https://github.com/kubernetes-incubator/external-dns/blob/master/docs/contributing/crd-source.md
[external-dns]: https://github.com/kubernetes-incubator/external-dns
[Certificate]: http://docs.cert-manager.io/en/latest/reference/certificates.html
[cert-manager]: http://docs.cert-manager.io/en/latest/index.html
[Issuer]: https://docs.cert-manager.io/en/latest/reference/issuers.html
