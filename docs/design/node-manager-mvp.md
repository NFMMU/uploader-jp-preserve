# Archive Node Manager MVP

This is an initial local-development scaffold. It is not yet a participant-ready release: the Agent image is expected to exist locally, and automatic updates, signed releases, rollback, editable settings, and Windows-specific socket validation are not implemented.

## Local smoke test

From the repository root, copy these new files into the repository, then run:

```sh
./scripts/run-node-manager-local.sh
```

Open `http://localhost:8780`. Enter the central API URL, the request-specific one-time enrollment token issued after an administrator approves the public participation request, node name, and capacity. The name, node type, and capacity must match the approved request. For local development, the central API must be reachable from a container; `http://host.docker.internal:8080` may be appropriate when the API runs on the Docker Desktop host. Do not use `localhost` for a service running outside the Manager container. The local Compose file enables `ALLOW_INSECURE_HTTP` for development only. Before a participant release, remove this override and require HTTPS for non-loopback API hosts.

The Manager stores its setup in the Docker volume `archive-node-manager-config`. Agent credentials and archive data are stored in the separate Docker volume `archive-node-data`. Do not delete either volume as part of routine upgrades or troubleshooting.

## Security limitations

The Manager mounts the Docker Engine socket and can control the Docker daemon. This is effectively host-level administrative access; mounting the socket read-only would not make Docker API operations read-only. The UI port is bound to loopback only. Do not publish it on a LAN or the internet, and only run Manager images you trust.

The one-time enrollment token is stored in the Manager's persistent configuration with restrictive file permissions. The initial UI does not display it after saving. It is consumed on successful registration and cannot be reused if the node credential volume is lost; request a fresh token from the operator in that case. A production release must additionally include signed image verification, a published immutable image reference, update rollback, and a reviewed Windows Docker Desktop installation flow.
