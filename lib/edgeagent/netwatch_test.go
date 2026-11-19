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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestParseIPv4DefaultRoutes(t *testing.T) {
	const table = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0101A8C0	0003	0	0	0	00000000	0	0	0
eth0	0001A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
wwan0	00000000	0100A8C0	0003	0	0	100	00000000	0	0	0
`
	routes, err := parseIPv4DefaultRoutes(strings.NewReader(table))
	require.NoError(t, err)
	require.Equal(t, []string{"v4:eth0/0101A8C0", "v4:wwan0/0100A8C0"}, routes)
}

func TestParseIPv6DefaultRoutes(t *testing.T) {
	const table = `00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 00000000 00000000 00000000 00000001 wwan0
fd000000000000000000000000000000 08 00000000000000000000000000000000 00 00000000000000000000000000000000 00000100 00000000 00000000 00000001 br-lan
00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 00000000 00000000 00000000 00000001 lo
`
	routes, err := parseIPv6DefaultRoutes(strings.NewReader(table))
	require.NoError(t, err)
	require.Equal(t, []string{"v6:wwan0/00000000000000000000000000000000"}, routes)
}

func TestRoutedInterfacesIgnoresLAN(t *testing.T) {
	got := routedInterfaces([]string{
		"v4:eth0/0101A8C0",
		"v6:wwan0/00000000000000000000000000000000",
	})
	require.Equal(t, map[string]struct{}{
		"eth0":  {},
		"wwan0": {},
	}, got)
	require.NotContains(t, got, "br-lan")
	require.NotContains(t, got, "lan1")
}

func TestNetworkFingerprintEqual(t *testing.T) {
	a := networkFingerprint{routes: []string{"v4:eth0/gw"}, addrs: []string{"eth0/192.0.2.10"}}
	b := networkFingerprint{routes: []string{"v4:eth0/gw"}, addrs: []string{"eth0/192.0.2.10"}}
	c := networkFingerprint{routes: []string{"v4:wwan0/gw"}, addrs: []string{"wwan0/198.51.100.8"}}
	require.True(t, a.equal(b))
	require.False(t, a.equal(c))
}

func TestNetworkWatcherReconnectsAfterRouteChange(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	baseline := networkFingerprint{
		routes: []string{"v4:eth0/0101A8C0"},
		addrs:  []string{"eth0/192.0.2.10"},
	}
	changed := networkFingerprint{
		routes: []string{"v4:wwan0/0100A8C0"},
		addrs:  []string{"wwan0/198.51.100.8"},
	}

	var mu sync.Mutex
	current := baseline
	events := make(chan struct{}, 1)
	w := &networkWatcher{
		snapshot: func() (networkFingerprint, error) {
			mu.Lock()
			defer mu.Unlock()
			return current, nil
		},
		subscribe: func(context.Context) <-chan struct{} { return events },
		pollEvery: time.Hour,
		debounce:  time.Second,
		logger:    slog.Default(),
		clock:     clock,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fired := make(chan networkFingerprint, 1)
	go w.run(ctx, func(_, to networkFingerprint) { fired <- to })

	require.NoError(t, clock.BlockUntilContext(ctx, 1))

	mu.Lock()
	current = changed
	mu.Unlock()
	events <- struct{}{}

	require.NoError(t, clock.BlockUntilContext(ctx, 2))
	clock.Advance(time.Second)

	select {
	case got := <-fired:
		require.Equal(t, changed, got)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconnect after network change")
	}
}

func TestNetworkWatcherIgnoresFlapThatReverts(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	baseline := networkFingerprint{
		routes: []string{"v4:eth0/0101A8C0"},
		addrs:  []string{"eth0/192.0.2.10"},
	}
	changed := networkFingerprint{
		routes: []string{"v4:wwan0/0100A8C0"},
		addrs:  []string{"wwan0/198.51.100.8"},
	}

	var mu sync.Mutex
	current := baseline
	events := make(chan struct{}, 1)
	w := &networkWatcher{
		snapshot: func() (networkFingerprint, error) {
			mu.Lock()
			defer mu.Unlock()
			return current, nil
		},
		subscribe: func(context.Context) <-chan struct{} { return events },
		pollEvery: time.Hour,
		debounce:  time.Second,
		logger:    slog.Default(),
		clock:     clock,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fired := make(chan struct{})
	go w.run(ctx, func(_, _ networkFingerprint) { close(fired) })

	require.NoError(t, clock.BlockUntilContext(ctx, 1))

	mu.Lock()
	current = changed
	mu.Unlock()
	events <- struct{}{}
	require.NoError(t, clock.BlockUntilContext(ctx, 2))

	mu.Lock()
	current = baseline
	mu.Unlock()
	events <- struct{}{}
	// Let the watcher observe the revert and cancel the debounce timer
	// before we advance the fake clock.
	time.Sleep(50 * time.Millisecond)
	clock.Advance(time.Second)

	select {
	case <-fired:
		t.Fatal("reconnect should not fire when the path reverts before debounce")
	case <-time.After(200 * time.Millisecond):
	}
}
