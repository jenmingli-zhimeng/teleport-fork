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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/session/networking/x11"
	"github.com/gravitational/teleport/session/pam/pamcfg"
	"github.com/gravitational/teleport/session/reexec"
)

// runNode starts the SSH node and reverse-tunnel agent pool.
// Copied and reduced from lib/service.TeleportProcess.initSSH
// (WATCH: lib/service/service.go initSSH).
func (p *Process) runNode(ctx context.Context, conn *Connector, inventoryHandle inventory.DownstreamHandle) error {
	logger := p.logger.With(teleport.ComponentKey, teleport.ComponentNode)

	accessPoint := conn.Client
	if p.cfg.Teleport.Cache.Enabled {
		logger.WarnContext(ctx, "Node cache is not compiled into teleport-edge; using direct Auth access to save memory.")
	}

	lim, err := limiter.NewLimiter(limiter.Config{
		MaxConnections: p.cfg.Edge.MaxConnections,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	asyncEmitter, err := events.NewAsyncEmitter(events.AsyncEmitterConfig{
		Inner: conn.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() { _ = asyncEmitter.Close() }()

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Logger:    logger,
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer lockWatcher.Close()

	streamEmitter := &events.StreamerAndEmitter{Emitter: asyncEmitter, Streamer: conn.Client}

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   conn.Client,
		AccessPoint:  accessPoint,
		LockEnforcer: lockWatcher,
		Emitter:      streamEmitter,
		Component:    teleport.ComponentNode,
		Logger:       logger.With("subsystem", "sessionctrl"),
		ServerID:     conn.HostUUID(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	proxyGetter := reversetunnel.NewConnectedProxyGetter()

	s, err := regular.New(
		ctx,
		utils.NetAddr{},
		p.cfg.Teleport.Nodename,
		conn.ServerGetHostSigners,
		accessPoint,
		p.cfg.Teleport.DataDir,
		"",
		utils.NetAddr{},
		conn.Client,
		regular.SetUUID(conn.HostUUID()),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetLimiter(lim),
		regular.SetEmitter(streamEmitter),
		regular.SetLabels(p.cfg.Labels(), services.CommandLabels(p.cfg.CmdLabels()), nil),
		regular.SetUseTunnel(true),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetAllowTCPForwarding(true),
		regular.SetX11ForwardingConfig(&x11.ServerConfig{Enabled: false}),
		regular.SetPAMConfig(&pamcfg.PAMConfig{Enabled: false}),
		regular.SetLockWatcher(lockWatcher),
		regular.SetAllowFileCopying(true),
		regular.SetConnectedProxyGetter(proxyGetter),
		regular.SetCreateHostUser(false),
		regular.SetInventoryControlHandle(inventoryHandle),
		regular.SetSessionController(sessionController),
		regular.SetSELinuxEnabled(false),
		regular.SetRotationGetter(p.GetRotation),
		regular.SetChildLogConfigFrom(&srv.ChildLogConfig{
			ExecLogConfig: reexec.ExecLogConfig{
				Level:       new(slog.LevelVar),
				Format:      p.cfg.Teleport.Log.Format.Output,
				ExtraFields: p.cfg.Teleport.Log.Format.ExtraFields,
			},
		}),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Start(); err != nil {
		return trace.Wrap(err)
	}

	agentPool, err := reversetunnel.NewAgentPool(ctx, reversetunnel.AgentPoolConfig{
		Component:            teleport.ComponentNode,
		HostUUID:             conn.HostID(),
		Resolver:             conn.TunnelProxyResolver(),
		Client:               conn.Client,
		AccessPoint:          conn.Client,
		AuthMethods:          conn.ClientAuthMethods(),
		Cluster:              conn.ClusterName(),
		Server:               s,
		ConnectedProxyGetter: proxyGetter,
		KeepAliveInterval:    edgeKeepAliveInterval,
		KeepAliveCount:       edgeKeepAliveCount,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := agentPool.Start(); err != nil {
		return trace.Wrap(err)
	}
	logger.InfoContext(ctx, "SSH service is starting in tunnel mode.",
		"version", teleport.Version,
		"host_uuid", conn.HostUUID(),
	)

	go p.watchRotation(ctx, conn)

	<-ctx.Done()
	logger.InfoContext(ctx, "Shutting down.")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		logger.WarnContext(ctx, "SSH server shutdown returned an error.", "error", err)
	}
	s.Wait()
	agentPool.Stop()
	agentPool.Wait()
	logger.InfoContext(ctx, "Exited.")
	return nil
}
