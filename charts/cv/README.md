# cv

Helm chart for `cvd`, the service serving Simon Ostendorf's curriculum vitae over HTTP
inside the cluster.

## Prerequisites

- Kubernetes 1.26 or newer.
- The Gateway API CRDs installed and a `Gateway` already running, only if `httpRoute.enabled`
  is set to `true`.

## Values

| Key | Description | Default |
| --- | --- | --- |
| `replicaCount` | Number of pod replicas. | `1` |
| `image.repository` | Container image repository. | `ghcr.io/ostendorf-cloud/cv` |
| `image.tag` | Image tag. Defaults to the chart's `appVersion`. | `""` |
| `image.pullPolicy` | Image pull policy. | `IfNotPresent` |
| `serviceAccount.create` | Whether to create a ServiceAccount. | `true` |
| `service.port` | Public listener's Service port. | `8080` |
| `service.adminPort` | govue's admin listener's Service port: health, readiness, metrics. | `8081` |
| `ingress.enabled` | Expose the service through an Ingress. Mutually exclusive with `httpRoute.enabled`. | `false` |
| `httpRoute.enabled` | Expose through a Gateway API HTTPRoute. Mutually exclusive with `ingress.enabled`. | `false` |
| `httpRoute.parentRefs.name` | Name of the Gateway to attach to. Required when `httpRoute.enabled` is `true`. | `""` |
| `httpRoute.parentRefs.namespace` | Namespace the Gateway lives in. Required when `httpRoute.enabled` is `true`. | `""` |
| `config.addr` | The public listener's address, passed as `ADDR`. | `:8080` |
| `config.adminAddr` | govue's admin listener's address, passed as `ADMIN_ADDR`. | `:8081` |
| `config.logLevel` | `LOG_LEVEL`. | `info` |
| `config.dataDir` | Absolute path of the CV data directory, passed as `DATA_DIR`. | `/data` |
| `data.existingConfigMap` | Name of a ConfigMap with the keys `cv.de.yaml` and `cv.en.yaml`, mounted read only at `config.dataDir`. Empty uses the data embedded in the image. | empty |

## Upgrade notes

No breaking change yet; this is the chart's first version.

## Uninstall

`helm uninstall` removes every object this chart creates. Nothing is left behind: the
chart creates no PersistentVolumeClaim and no Secret.
