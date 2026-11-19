// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package edgeagent

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils/log"
)

const (
	embeddedReexecEnv = "TELEPORT_UNSTABLE_DISABLE_EMBEDDED_REEXEC"

	connectRetryFirst = time.Second
	connectRetryStep  = 2 * time.Second
	connectRetryMax   = 30 * time.Second

	healthPingInterval = 20 * time.Second
	healthPingTimeout  = 10 * time.Second
	healthPingFails    = 2

	// edgeKeepAliveInterval is short enough to notice WAN/interface failover
	// on routers without waiting for the cluster default of 5 minutes.
	edgeKeepAliveInterval = 15 * time.Second
	edgeKeepAliveCount    = 3
)

// Process is the teleport-edge agent runtime.
type Process struct {
	cfg      *Config
	logger   *slog.Logger
	storage  *Storage
	resolver reversetunnelclient.Resolver
	clock    clockwork.Clock
}

// Run loads config, joins the cluster, and serves SSH over a reverse tunnel
// until the context is cancelled or a termination signal is received.
// Transient connect failures and local network-path changes reconnect in
// process instead of exiting.
func Run(ctx context.Context, cfg *Config) error {
	if err := ApplyMemoryLimit(cfg); err != nil {
		return trace.Wrap(err)
	}
	enableEmbeddedReexec()

	if cfg.Teleport.Insecure {
		lib.SetInsecureDevMode(true)
	}

	logger, _, _, err := log.Initialize(log.Config{
		Output:      cfg.Teleport.Log.Output,
		Severity:    cfg.Teleport.Log.Severity,
		Format:      cfg.Teleport.Log.Format.Output,
		ExtraFields: cfg.Teleport.Log.Format.ExtraFields,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	storage, err := NewStorage(cfg.Teleport.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	p := &Process{
		cfg:     cfg,
		logger:  logger.With(teleport.ComponentKey, "edge"),
		storage: storage,
		clock:   clockwork.NewRealClock(),
	}
	return p.serve(ctx)
}

func (p *Process) serve(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  connectRetryFirst,
		Step:   connectRetryStep,
		Max:    connectRetryMax,
		Jitter: retryutils.HalfJitter,
		Clock:  p.clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := p.runSession(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			p.logger.ErrorContext(ctx, "Failed to connect to cluster, retrying.",
				"error", err,
				"backoff", retry.Duration(),
			)
		} else {
			p.logger.WarnContext(ctx, "Edge session ended, reconnecting.")
			retry.Reset()
		}

		select {
		case <-ctx.Done():
			return nil
		case <-retry.After():
			retry.Inc()
		}
	}
}

func (p *Process) newResolver(ctx context.Context) (reversetunnelclient.Resolver, error) {
	return reversetunnelclient.CachingResolver(
		ctx,
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   ctx,
			ProxyAddr: p.cfg.Teleport.ProxyServer,
			Insecure:  p.cfg.Teleport.Insecure || lib.IsInsecureDevMode(),
		}),
		p.clock,
	)
}

func (p *Process) runSession(ctx context.Context) error {
	resolver, err := p.newResolver(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	p.resolver = resolver

	conn, err := p.connect(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	go p.watchNetwork(sessionCtx, sessionCancel)
	go p.watchHealth(sessionCtx, sessionCancel, conn)

	inventoryHandle, err := p.startInventory(sessionCtx, conn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer inventoryHandle.Close()

	return trace.Wrap(p.runNode(sessionCtx, conn, inventoryHandle))
}

func (p *Process) watchNetwork(ctx context.Context, reconnect context.CancelFunc) {
	newNetworkWatcher(p.logger, p.clock).run(ctx, func(from, to networkFingerprint) {
		p.logger.WarnContext(ctx, "Local network path changed, reconnecting reverse tunnel.",
			"from", from.String(),
			"to", to.String(),
		)
		reconnect()
	})
}

func (p *Process) watchHealth(ctx context.Context, reconnect context.CancelFunc, conn *Connector) {
	ticker := p.clock.NewTicker(healthPingInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
		}

		pingCtx, cancel := context.WithTimeout(ctx, healthPingTimeout)
		_, err := conn.Client.Ping(pingCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			failures = 0
			continue
		}
		failures++
		p.logger.WarnContext(ctx, "Auth health ping failed.", "error", err, "failures", failures)
		if failures >= healthPingFails {
			p.logger.WarnContext(ctx, "Auth health ping failed repeatedly, reconnecting reverse tunnel.")
			reconnect()
			return
		}
	}
}

func (p *Process) startInventory(ctx context.Context, conn *Connector) (inventory.DownstreamHandle, error) {
	getHello := func(context.Context) (*proto.UpstreamInventoryHello, error) {
		return &proto.UpstreamInventoryHello{
			ServerID: conn.HostUUID(),
			Version:  teleport.Version,
			Services: []string{string(types.RoleInstance), string(types.RoleNode)},
			Hostname: p.cfg.Teleport.Nodename,
		}, nil
	}
	handle, err := inventory.NewDownstreamHandle(
		func(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
			return conn.Client.InventoryControlStream(ctx)
		},
		getHello,
		inventory.WithDownstreamClock(p.clock),
	)
	if err != nil {
		return nil, trace.Wrap(err, "building inventory handle")
	}
	handle.RegisterPingHandler(func(sender inventory.DownstreamSender, ping *proto.DownstreamInventoryPing) {
		systemClock := p.clock.Now().UTC()
		p.logger.InfoContext(ctx, "Handling incoming inventory ping.", "id", ping.GetID())
		if err := sender.Send(ctx, &proto.UpstreamInventoryPong{
			ID:          ping.GetID(),
			SystemClock: timestamppb.New(systemClock),
		}); err != nil {
			p.logger.WarnContext(ctx, "Failed to respond to inventory ping.", "id", ping.ID, "error", err)
		}
	})
	return handle, nil
}

func (p *Process) GetRotation(role types.SystemRole) (*types.Rotation, error) {
	st, err := p.storage.GetState(context.Background(), role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &st.Spec.Rotation, nil
}

func enableEmbeddedReexec() {
	if os.Getenv(embeddedReexecEnv) != "" {
		return
	}
	// Upstream disables the helper unless this env var is explicitly "false".
	_ = os.Setenv(embeddedReexecEnv, "false")
}

const rotationCheckInterval = 15 * time.Minute

func (p *Process) watchRotation(ctx context.Context, conn *Connector) {
	ticker := time.NewTicker(rotationCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.maybeRotate(ctx, conn); err != nil {
				p.logger.WarnContext(ctx, "Certificate rotation check failed.", "error", err)
			}
		}
	}
}

func (p *Process) maybeRotate(ctx context.Context, conn *Connector) error {
	ca, err := conn.Client.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: conn.ClusterName(),
		Type:       types.HostCA,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	st, err := p.storage.GetState(ctx, types.RoleNode)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if st != nil && st.Spec.Rotation.CurrentID == ca.GetRotation().CurrentID && ca.GetRotation().State == types.RotationStateStandby {
		ident := conn.serverState.Load().identity
		if ident != nil && ident.XCert != nil && time.Until(ident.XCert.NotAfter) > time.Hour {
			return nil
		}
	}
	p.logger.InfoContext(ctx, "Re-registering host certificates.", "rotation_state", ca.GetRotation().State)
	instanceIdent := conn.clientState.Load().identity
	nodeIdent, err := p.reRegister(ctx, conn.Client, instanceIdent, types.RoleNode, ca.GetRotation())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := p.storage.WriteIdentity(ctx, state.IdentityCurrent, *nodeIdent); err != nil {
		return trace.Wrap(err)
	}
	if err := p.storage.WriteState(ctx, types.RoleNode, state.StateV2{
		Spec: state.StateSpecV2{
			Rotation:            ca.GetRotation(),
			InitialLocalVersion: teleport.Version,
		},
	}); err != nil {
		return trace.Wrap(err)
	}
	serverState, err := newConnectorState(nodeIdent)
	if err != nil {
		return trace.Wrap(err)
	}
	conn.serverState.Store(serverState)
	return nil
}
