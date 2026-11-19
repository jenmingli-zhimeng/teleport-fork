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
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
)

const (
	// netPollInterval is the fallback poll period used even when netlink is
	// available, so a missed event cannot leave the agent on a dead path.
	netPollInterval = 2 * time.Second
	// netDebounce waits for the routing table to settle after a flap (PPPoE
	// reconnect, WAN failover, DHCP renew) before tearing the session down.
	netDebounce = time.Second
)

// networkFingerprint is the default route plus unicast addresses on that
// path only. LAN/switch port up/down is ignored.
type networkFingerprint struct {
	routes []string
	addrs  []string
}

func (f networkFingerprint) equal(other networkFingerprint) bool {
	if len(f.routes) != len(other.routes) || len(f.addrs) != len(other.addrs) {
		return false
	}
	for i := range f.routes {
		if f.routes[i] != other.routes[i] {
			return false
		}
	}
	for i := range f.addrs {
		if f.addrs[i] != other.addrs[i] {
			return false
		}
	}
	return true
}

func (f networkFingerprint) String() string {
	return fmt.Sprintf("routes=%s addrs=%s", strings.Join(f.routes, ","), strings.Join(f.addrs, ","))
}

func snapshotNetwork() (networkFingerprint, error) {
	routes, err := defaultRoutes()
	if err != nil {
		return networkFingerprint{}, err
	}
	addrs, err := localUnicastAddrs(routedInterfaces(routes))
	if err != nil {
		return networkFingerprint{}, err
	}
	return networkFingerprint{routes: routes, addrs: addrs}, nil
}

// routedInterfaces returns the interface names that currently own a default
// route. LAN/switch port flaps are ignored because those interfaces are not
// on the default path.
func routedInterfaces(routes []string) map[string]struct{} {
	out := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		_, rest, ok := strings.Cut(r, ":")
		if !ok {
			continue
		}
		iface, _, ok := strings.Cut(rest, "/")
		if !ok || iface == "" {
			continue
		}
		out[iface] = struct{}{}
	}
	return out
}

func defaultRoutes() ([]string, error) {
	var routes []string
	if f, err := os.Open("/proc/net/route"); err == nil {
		v4, parseErr := parseIPv4DefaultRoutes(f)
		_ = f.Close()
		if parseErr != nil {
			return nil, parseErr
		}
		routes = append(routes, v4...)
	}
	if f, err := os.Open("/proc/net/ipv6_route"); err == nil {
		v6, parseErr := parseIPv6DefaultRoutes(f)
		_ = f.Close()
		if parseErr != nil {
			return nil, parseErr
		}
		routes = append(routes, v6...)
	}
	sort.Strings(routes)
	return routes, nil
}

func parseIPv4DefaultRoutes(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return nil, sc.Err()
	}
	var routes []string
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		iface, dest, gateway := fields[0], fields[1], fields[2]
		if dest != "00000000" {
			continue
		}
		routes = append(routes, "v4:"+iface+"/"+gateway)
	}
	return routes, sc.Err()
}

func parseIPv6DefaultRoutes(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	var routes []string
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// dest, destPrefix, src, srcPrefix, nextHop, metric, refcnt, use, flags, iface
		if len(fields) < 10 {
			continue
		}
		if fields[1] != "00" {
			continue
		}
		iface := fields[9]
		if iface == "lo" {
			continue
		}
		routes = append(routes, "v6:"+iface+"/"+fields[4])
	}
	return routes, sc.Err()
}

func localUnicastAddrs(routed map[string]struct{}) ([]string, error) {
	if len(routed) == 0 {
		return nil, nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var addrs []string
	for _, iface := range ifaces {
		if _, ok := routed[iface.Name]; !ok {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range ifaceAddrs {
			ip := ipFromAddr(addr)
			if ip == nil || !ip.IsGlobalUnicast() {
				continue
			}
			addrs = append(addrs, iface.Name+"/"+ip.String())
		}
	}
	sort.Strings(addrs)
	return addrs, nil
}

func ipFromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}

type networkWatcher struct {
	snapshot  func() (networkFingerprint, error)
	subscribe func(context.Context) <-chan struct{}
	pollEvery time.Duration
	debounce  time.Duration
	logger    *slog.Logger
	clock     clockwork.Clock
}

func newNetworkWatcher(logger *slog.Logger, clock clockwork.Clock) *networkWatcher {
	return &networkWatcher{
		snapshot:  snapshotNetwork,
		subscribe: subscribeRouteEvents,
		pollEvery: netPollInterval,
		debounce:  netDebounce,
		logger:    logger,
		clock:     clock,
	}
}

// run watches for default-path changes and invokes onChange once the new
// fingerprint has been stable for debounce. Link up/down that does not change
// the default route is ignored. It returns when ctx is cancelled or onChange
// is invoked.
func (w *networkWatcher) run(ctx context.Context, onChange func(from, to networkFingerprint)) {
	baseline, err := w.snapshot()
	if err != nil {
		w.logger.WarnContext(ctx, "Unable to read local network state; falling back to polling.", "error", err)
	}

	var events <-chan struct{}
	if w.subscribe != nil {
		events = w.subscribe(ctx)
	}

	poll := w.clock.NewTicker(w.pollEvery)
	defer poll.Stop()

	var debounce clockwork.Timer
	stopDebounce := func() {
		if debounce != nil {
			debounce.Stop()
			debounce = nil
		}
	}
	defer stopDebounce()

	check := func() {
		snap, err := w.snapshot()
		if err != nil {
			w.logger.DebugContext(ctx, "Network snapshot failed.", "error", err)
			return
		}
		if snap.equal(baseline) {
			stopDebounce()
			return
		}
		if debounce != nil {
			return
		}
		w.logger.InfoContext(ctx, "Network path changed, waiting for it to settle before reconnecting.",
			"from", baseline.String(),
			"to", snap.String(),
		)
		debounce = w.clock.NewTimer(w.debounce)
	}

	var debounceC <-chan time.Time
	for {
		if debounce != nil {
			debounceC = debounce.Chan()
		} else {
			debounceC = nil
		}
		select {
		case <-ctx.Done():
			return
		case <-events:
			check()
		case <-poll.Chan():
			check()
		case <-debounceC:
			snap, err := w.snapshot()
			stopDebounce()
			if err != nil {
				w.logger.DebugContext(ctx, "Network snapshot failed after debounce.", "error", err)
				continue
			}
			if snap.equal(baseline) {
				continue
			}
			onChange(baseline, snap)
			return
		}
	}
}
