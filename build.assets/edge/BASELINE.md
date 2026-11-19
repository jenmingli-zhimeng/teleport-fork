# Binary size baseline

Commit: ddaa46b8 (Release 18.10.0) plus edge/v18 cuts
Date: 2026-10-27

Built with `CGO_ENABLED=0`, `-ldflags '-s -w' -trimpath`, tags
`edgeagent kustomize_disable_go_plugin_support`, Go 1.25.11.

| binary | GOOS/GOARCH | bytes |
|--------|-------------|-------|
| teleport-edge | linux/amd64 | 56660152 |
| teleport-edge | linux/arm64 | 64225464 (previous cut; not remeasured) |

RSS sampled 2026-10-28 on linux/amd64 in an Alpine musl container
(host network, token join, reverse tunnel, GOMEMLIMIT 128MiB):

| state | VmRSS |
|-------|-------|
| idle after heartbeat | 56.0 MiB (57272 kB) |
| 1 interactive session | 57.1 MiB (58476 kB) |
| 6 concurrent shells | 59.7 MiB (61156 kB) |
| after reconnect / lock / CA init | 55.6 MiB (56956 kB) |

Size target after cuts: stripped teleport-edge under 80MiB and under 1/3 of a
same-flags `teleport` binary. 54MiB is well under one third of a typical
150–200MiB stripped `teleport`.

## Dependency attribution

Removed from the edge graph (vs a stock SSH node):

- `lib/auth` (auth server)
- `lib/web`, `lib/kube/*`, `lib/cloud/*`, `lib/proxy` (router), `lib/desktop`
- `lib/auth/join` and all cloud join methods
- reverse-tunnel proxy-side (`localsite`/`remotesite`/`srv`) including EICE
- `lib/automaticupgrades` / k8s client-go
- aws-sdk services (S3, EC2, STS, IAM, …) except `aws-sdk-go-v2/aws/arn`
- Azure SDK, Google Cloud SDK, sqlite (`mattn/go-sqlite3`)

Still present by design:

- `lib/services` hub (SSH access checker still names kube/db/desktop methods; `aws/arn` for IAM roles)
- kube/db/desktop protobuf types in `api/types` (one generated package)
- `lib/bpf` NOP
- `lib/utils/aws` ARN helpers (`aws/arn` only)

Removed in the keep-Shell/SFTP/port-forward pass:

- `lib/proxy/peer` / `quic-go` (Proxy peering is proxy-side)
- `k8s.io/api`, `k8s.io/apimachinery` (including `lib/utils` YAML/DNS helpers)
- `github.com/crewjam/saml`
- `github.com/russellhaering/gosaml2`
- `go.mongodb.org/mongo-driver`
- SSO connector validate/marshal (SAML/OIDC/GitHub)
- `ValidateDatabase`, multiplexer Web/db listener, unified resource cache
- `go-webauthn` / `go-tpm` (WebAuthn session data)
- Sigstore / TUF / go-containerregistry
- `opencontainers/selinux`, SFTP CLI progress bar, Connect My Computer helper
