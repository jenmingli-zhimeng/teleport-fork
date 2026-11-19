# Syncing teleport-edge with upstream

The `edge/v18` branch is merged (not rebased) from official `v18.x` tags so
history stays stable for downstream OpenWrt packaging.

## Procedure

1. `git fetch upstream --tags`
2. `git merge v18.X.Y` (or `v18.X.Y` patch tag)
3. `build.assets/edge/upstream-watch.sh v18.PREV v18.X.Y`
   - If it exits non-zero, read the listed files and port relevant fixes into
     `lib/edgeagent` (join, connector, SSH assembly, identity storage).
4. `go test ./lib/edgeagent ./tool/teleport-edge`
5. `make teleport-edge` and confirm the binary is still under the size budget
   (`build.assets/edge/size-budget.sh`)
6. If the dependency blacklist fails, add a new `//go:build edgeagent` cut
   rather than deleting upstream code. Record it in `lib/edgeagent/PATCHES.md`.
7. Re-run `integration/edge/run.sh` when join, reverse-tunnel, SSH, or SFTP
   code moved.

Do not delete newly merged upstream features that the edge binary does not
use. The `edgeagent` build tag already keeps them out of the agent binary.
