# Edge-agent patches to shared packages

These are the only intentional edits to upstream files. Prefer adding
`//go:build edgeagent` / `//go:build !edgeagent` files over editing
function bodies. When merging an upstream tag, re-run
`build.assets/edge/upstream-watch.sh` and `go test ./tool/teleport-edge`.

## Active patches

| File | Kind | Why |
|------|------|-----|
| `lib/authz/connection.go` | new | Hosts `ConnectionIdentity` so `lib/srv` does not import `lib/auth`. |
| `lib/auth/transport_credentials.go` | alias | `type ConnectionIdentity = authz.ConnectionIdentity` keeps the old name. |
| `lib/srv/session_control.go` | import swap | Uses `authz.ConnectionIdentity` instead of `lib/auth`. |
| `lib/srv/regular/sshserver.go` | split | `SetChildLogConfigFrom` has no `servicecfg` import; router field is untyped; relay getter is a func. |
| `lib/srv/regular/sshserver_servicecfg.go` | new `!edgeagent` | Original `SetChildLogConfig(*servicecfg.Config)`. |
| `lib/srv/regular/sshserver_proxymode.go` | new `!edgeagent` | `SetProxyMode` (imports `lib/proxy`). |
| `lib/srv/regular/sshserver_authorizedkeys.go` | new `!edgeagent` | Authorized-keys watcher. |
| `lib/srv/regular/sshserver_authorizedkeys_edge.go` | new `edgeagent` | No-op watcher. |
| `lib/srv/regular/proxy.go` | tag `!edgeagent` | TCP proxy subsystem pulls `lib/proxy` → desktop. |
| `lib/srv/regular/proxy_edge.go` | new `edgeagent` | NotImplemented stubs. |
| `lib/srv/regular/proxy_types.go` | new | `PROXYHeaderSigner` / `CertAuthorityGetter` shared types. |
| `lib/join/joinclient/join.go` | tag `!edgeagent` | Full join path still aliases `lib/auth/join`. |
| `lib/join/joinclient/join_common.go` | new | Token join helpers shared by both builds. |
| `lib/join/joinclient/join_edge.go` | new `edgeagent` | Token-only `Join` via Proxy, no `lib/auth/join`. |
| `lib/join/joinclient/join_methods.go` | new `!edgeagent` | Full `joinWithMethod`. |
| `lib/join/joinclient/join_methods_edge.go` | new `edgeagent` | Token-only `joinWithMethod`. |
| `lib/join/joinclient/join_{azure,boundkeypair,ec2,iam,oidc,oracle,tpm}.go` | tag `!edgeagent` | Keep cloud join methods out of the edge binary. |
| `lib/reversetunnelclient/api.go` | import trim | `Server` no longer returns `*peer.Client`, so agents do not import quic-go. |
| `lib/reversetunnelclient/peer.go` | tag `!edgeagent` | Proxy-to-proxy `NewPeerDialer`. |
| `lib/reversetunnelclient/fake.go` | tag `!edgeagent` | Test doubles with app/db watchers. |
| `lib/services/app_kube.go` | new `!edgeagent` | Kube-service app discovery (`k8s.io/api`). |
| `lib/services/saml.go`, `oidc.go`, `github.go` | tag `!edgeagent` | SSO connector validate/marshal (`gosaml2`). |
| `lib/services/resource_sso.go` | new `!edgeagent` | SSO resource marshaler registration. |
| `lib/services/enterprise.go` | new | `ErrRequiresEnterprise` (moved out of `github.go`). |
| `lib/services/database_validate.go` | new `!edgeagent` | `ValidateDatabase` (mongo-driver, redis, Azure/GCP). |
| `lib/services/unified_resource.go` | tag `!edgeagent` | Auth/UI resource cache (kube/db/desktop/SAML IdP). |
| `lib/services/saml_idp_crewjam.go` | new `!edgeagent` | crewjam/saml ACS and metadata filtering. |
| `api/types/assertion_info.go` | tag `!edgeagent` | gosaml2 `AssertionInfo` alias. |
| `api/types/assertion_info_edge.go` | new `edgeagent` | JSON-opaque `AssertionInfo` stub. |
| `lib/multiplexer/web.go` | tag `!edgeagent` | Web/db TLS multiplexer (`lib/srv/db/dbutils`). |
| `lib/utils/utils.go` | import trim | DNS-1035 hostname check is local; no apimachinery. |
| `lib/utils/jsontools.go` | import trim | YAML stream decode moved out. |
| `lib/utils/jsontools_yaml.go` | new `!edgeagent` | `ReadYAML` via apimachinery yaml. |
| `lib/services/identity.go` | split | WebAuthn session-data methods moved off the always-compiled Identity interface. |
| `lib/services/identity_webauthn.go` | new `!edgeagent` | `webauthnSessionData` (go-webauthn). |
| `lib/services/identity_webauthn_edge.go` | new `edgeagent` | Empty `webauthnSessionData`. |
| `lib/auth/authclient/clt.go` | import trim | WebAuthn assertion type aliased out. |
| `lib/auth/authclient/clt_webauthn.go` | new `!edgeagent` | Alias to `webauthntypes.CredentialAssertionResponse`. |
| `lib/auth/authclient/clt_webauthn_edge.go` | new `edgeagent` | Opaque assertion placeholder. |
| `lib/services/sigstore_policies.go` | tag `!edgeagent` | Sigstore/TUF policy validation. |
| `session/selinux/selinux_linux.go` | tag `linux && !edgeagent` | opencontainers/selinux. |
| `session/selinux/selinux_linux_edge.go` | new `linux && edgeagent` | SELinux NOP. |
| `session/reexec/reexec_selinux.go` | new `linux && !edgeagent` | `SetExecLabel` for child shells. |
| `session/reexec/reexec_selinux_stub.go` | new `!linux \|\| edgeagent` | SELinux NotImplemented. |
| `lib/sshutils/sftp/sftp_progress.go` | new `!edgeagent` | CLI progress bar. |
| `lib/sshutils/sftp/sftp_progress_edge.go` | new `edgeagent` | Discard progress writer. |
| `lib/srv/authhandlers.go` | import trim | Inline Connect My Computer role name. |
| `lib/services/beam.go` | import trim | DNS-1123 check is local; no apimachinery. |
| `lib/services/groupkind.go` | new | Local `Kind.group` split so access checking does not import apimachinery. |
| `lib/services/access_checker.go`, `access_request.go` | import swap | Use `parseGroupKind` instead of `schema.ParseGroupKind`. |
| `lib/reversetunnel/localsite.go`, `remotesite.go`, `srv.go`, `peer.go`, `cache.go`, `rc_manager.go`, `conn.go`, `transport.go` | tag `!edgeagent` | Proxy-side reverse tunnel (forwards, EICE, `lib/proxy`). |
| `lib/reversetunnel/transport.go` | body trim | Dropped duplicate `parseDialReq` / `getTCPAddr` left after extracting `transport_dial.go`. |
| `lib/reversetunnel/transport_dial.go` | new | `parseDialReq` / `getTCPAddr` for the agent pool. |
| `lib/reversetunnel/agent_metrics.go` | new | `versionRequest` and trusted-cluster Prometheus metric. |
| `lib/reversetunnel/agentpool.go` | split | Remote-cluster transport delegated to tagged helpers. |
| `lib/reversetunnel/agentpool_remote.go` | new `!edgeagent` | Proxy-to-proxy transport. |
| `lib/reversetunnel/agentpool_remote_edge.go` | new `edgeagent` | Logs and closes remote-cluster transports. |
| `lib/reversetunnel/localsite_eice.go` | new `!edgeagent` | AWS EICE reverse-tunnel helper. |
| `lib/inventory/controller.go` | tag `!edgeagent` | Auth-side inventory controller pulls `usagereporter`. |
| `lib/inventory/metadata/metadata_linux.go` | tag `linux && cgo` | glibc version via cgo. |
| `lib/inventory/metadata/metadata_linux_nocgo.go` | new `linux && !cgo` | `/etc/os-release` only. |
| `lib/inventory/metadata/metadata.go` | split | Cloud metadata default moved out. |
| `lib/inventory/metadata/metadata_aws.go` | new `!edgeagent` | EC2 instance-identity lookup. |
| `lib/inventory/metadata/metadata_aws_edge.go` | new `edgeagent` | No cloud metadata. |
| `lib/events/membuffer.go` | tag `!edgeagent` | Uses `aws-sdk-go-v2/feature/s3/manager`. |
| `lib/events/membuffer_edge.go` | new `edgeagent` | Bytes-buffer implementation. |
| `lib/events/fips.go` | tag `!edgeagent` | AWS FIPS endpoint mapping. |
| `lib/events/fips_edge.go` | new `edgeagent` | `UseFIPSQueryParam` constant only. |
| `lib/bpf/bpf_nop.go` | tag `&& !edgeagent` | `New` imported `servicecfg`. |
| `lib/bpf/bpf_nop_edge.go` | new `edgeagent` | `SystemHasBPF() == false`. |
| `lib/labels/cloud.go` | tag `!edgeagent` | Cloud instance-metadata label importer. |
| `lib/modules/modules.go` | import trim | Automatic-upgrades lookup moved out (it pulled k8s client-go). |
| `lib/modules/autoupgrade.go` | new `!edgeagent` | `automaticupgrades.IsEnabled()`. |
| `lib/modules/autoupgrade_edge.go` | new `edgeagent` | Always false; upgrades are via opkg. |
| `lib/utils/aws/arn.go` | new | IAM Role ARN helpers (only `aws/arn`). |
| `lib/utils/aws/{aws,signing,s3,ec2}.go` | tag `!edgeagent` | Full AWS SDK helpers. |
| `lib/utils/aws/ec2_edge.go` | new `edgeagent` | EC2 identity NotImplemented. |
| `session/pam/pam_nop.go` | tag `!pam \|\| !cgo` | Pure-Go PAM stub when CGO is off. |
| `session/shell/shell_unix.go` | tag `!windows && cgo` | Original cgo `getpwnam_r`. |
| `session/shell/shell_unix_nocgo.go` | new `!windows && !cgo` | Reads `/etc/passwd`. |
| `session/uacc/wtmpdb.go` | tag `cgo && !edgeagent` | sqlite wtmpdb. |
| `session/uacc/wtmpdb_stub.go` | new `!cgo \|\| edgeagent` | Disabled wtmpdb. |
| `session/uacc/utmp_linux.go` | tag `linux && cgo && !edgeagent` | libc utmp. |
| `session/uacc/utmp_other.go` | tag `!linux \|\| !cgo \|\| edgeagent` | Disabled utmp. |

## Accepted residual weight

`lib/services` still lists kube/db/desktop/SAML IdP getter interfaces and SSH
access-checker methods (role certs can name those resources), but SSO
validate/marshal, `ValidateDatabase`, WebAuthn session data, Sigstore/TUF,
crewjam, and the unified resource cache are tagged out.
`aws-sdk-go-v2/aws/arn` is used for IAM role matching. `api/types` still
contains kube/db/desktop protobufs because they live in one generated package.

`lib/bpf` is the NOP implementation only (no eBPF bytecode).

teleport-edge keeps Shell, SFTP/SCP, and SSH TCP port forwarding (`direct-tcpip`
and `tcpip-forward`). X11, PAM, SELinux, host-user creation, ProxyJump, and
Proxy peering are disabled or not compiled.

## Upstream-friendly follow-ups

These would shrink the fork delta if accepted upstream:

1. Register join methods instead of a giant `switch` in `joinWithMethod`.
2. Replace `aws-sdk` `WriteAtBuffer` in `MemBuffer` with a local buffer.
3. Stop defaulting IAM/TPM/EC2 helpers in `RegisterParams.CheckAndSetDefaults` unless those methods are used.
4. Keep reverse-tunnel agent pool types out of the same package as the proxy-side site server.
5. Move IAM ARN string helpers out of `lib/utils/aws` (which imports the full SDK).
6. Keep `GetProxyPeerClient` off `reversetunnelclient.Server` so agents do not import quic-go.
7. Split kube/db/desktop/SAML types out of the `api/types` generated package so agents do not link those protobufs.
