# teleport-edge integration

This harness starts a fork-built auth+proxy (`teleport`) on the host and a
`teleport-edge` agent in an Alpine (musl) container, then exercises Shell, file
transfer, TCP port forwarding, and reconnect/heartbeat reliability.

## Prerequisites

```
make teleport tctl tsh teleport-edge
```

Docker is required. `teleport` here is the unmodified all-in-one binary from
this branch (auth and proxy). `teleport-edge` is the reduced node.

## Run

```
integration/edge/run.sh
```

The script:

1. Starts auth+proxy with `session_recording: node-sync`.
2. Creates a node token and starts `teleport-edge` in `alpine:3.20`.
3. Creates a local user and checks `tsh ssh`, `tsh scp` (SFTP), and TCP port forwarding (`tsh ssh -L` / `-R`).
4. Samples idle / 1-session RSS and runs 6 concurrent sessions.
5. Kills and restarts the edge container, then restarts auth, and checks reconnect.
   The agent also watches default-route and WAN-source-address changes so a WAN
   failover rebuilds the reverse tunnel in-process (LAN port up/down is ignored).
6. Creates a lock and checks that a new SSH session is rejected, then removes it.
7. Starts a host CA rotation and checks that SSH still works.

Set `EDGE_ITEST_HOLD=1` to leave the cluster running after a successful pass.

Cluster recording should stay `node-sync` on OpenWrt so session data is not
buffered on flash. If you must use async recording, put `data_dir/log` on
tmpfs or external storage.
