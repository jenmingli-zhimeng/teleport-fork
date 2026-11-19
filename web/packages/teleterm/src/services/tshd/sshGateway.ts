/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * Appended to CreateGatewayRequest.target_subresource_name to request a
 * 15-minute idle timeout on an SSH port forward. Must match gateway.SSHIdleMarker.
 */
export const SSH_IDLE_MARKER = '#idle';

/**
 * Sentinel error string returned by tshd when an SSH port forward is closed
 * after sitting idle. Must match gateway.ErrIdleTimeout.
 */
export const SSH_IDLE_TIMEOUT_ERROR =
  'ssh port forward closed after idle timeout';

/**
 * Sentinel error string returned by tshd when an SSH port forward's Serve
 * loop ends for any other reason (dropped SSH session after reconnect
 * failed). Must match gateway.ErrSSHGatewayClosed.
 */
export const SSH_GATEWAY_CLOSED_ERROR = 'ssh port forward closed';

export function isSSHGatewayClosedError(error?: string): boolean {
  return (
    error === SSH_IDLE_TIMEOUT_ERROR || error === SSH_GATEWAY_CLOSED_ERROR
  );
}

export function formatSSHForwardTarget(
  host: string,
  port: string,
  idle?: boolean
): string {
  const hostname = host.trim() || 'localhost';
  const addr = `${hostname}:${port}`;
  return idle ? `${addr}${SSH_IDLE_MARKER}` : addr;
}

export function parseSSHForwardTarget(raw?: string): {
  host: string;
  port: string;
  idle: boolean;
} {
  let value = (raw || '').trim();
  let idle = false;
  if (value.endsWith(SSH_IDLE_MARKER)) {
    idle = true;
    value = value.slice(0, -SSH_IDLE_MARKER.length).trim();
  }
  if (!value) {
    return { host: 'localhost', port: '', idle };
  }
  const lastColon = value.lastIndexOf(':');
  if (lastColon === -1) {
    return { host: 'localhost', port: value, idle };
  }
  return {
    host: value.slice(0, lastColon) || 'localhost',
    port: value.slice(lastColon + 1),
    idle,
  };
}

export function sshForwardLocalURL(
  localAddress: string,
  localPort: string,
  protocol: string
): string | undefined {
  let scheme: string;
  if (protocol === 'HTTPS') {
    scheme = 'https';
  } else if (protocol === 'HTTP') {
    scheme = 'http';
  } else {
    return;
  }
  const host =
    localAddress.includes(':') && !localAddress.startsWith('[')
      ? `[${localAddress}]`
      : localAddress;
  return `${scheme}://${host}:${localPort}`;
}

export function openSSHForwardInBrowser(gateway: {
  localAddress: string;
  localPort: string;
  protocol: string;
}): void {
  const url = sshForwardLocalURL(
    gateway.localAddress,
    gateway.localPort,
    gateway.protocol
  );
  if (url) {
    window.open(url, '_blank', 'noreferrer,noopener');
  }
}
