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

import {
  formatSSHForwardTarget,
  isSSHGatewayClosedError,
  parseSSHForwardTarget,
  SSH_GATEWAY_CLOSED_ERROR,
  SSH_IDLE_TIMEOUT_ERROR,
  sshForwardLocalURL,
} from './sshGateway';

describe('parseSSHForwardTarget', () => {
  test.each([
    {
      raw: 'localhost:80',
      expected: { host: 'localhost', port: '80', idle: false },
    },
    {
      raw: '80',
      expected: { host: 'localhost', port: '80', idle: false },
    },
    {
      raw: 'localhost:443#idle',
      expected: { host: 'localhost', port: '443', idle: true },
    },
    {
      raw: '',
      expected: { host: 'localhost', port: '', idle: false },
    },
  ])('$raw', ({ raw, expected }) => {
    expect(parseSSHForwardTarget(raw)).toEqual(expected);
  });
});

test('formatSSHForwardTarget appends the idle marker', () => {
  expect(formatSSHForwardTarget('localhost', '80')).toBe('localhost:80');
  expect(formatSSHForwardTarget('localhost', '80', true)).toBe(
    'localhost:80#idle'
  );
});

test('isSSHGatewayClosedError matches tshd sentinels', () => {
  expect(isSSHGatewayClosedError(SSH_IDLE_TIMEOUT_ERROR)).toBe(true);
  expect(isSSHGatewayClosedError(SSH_GATEWAY_CLOSED_ERROR)).toBe(true);
  expect(isSSHGatewayClosedError('connection refused')).toBe(false);
});

test('sshForwardLocalURL uses http(s) for HTTP/HTTPS', () => {
  expect(sshForwardLocalURL('127.0.0.1', '1337', 'HTTP')).toBe(
    'http://127.0.0.1:1337'
  );
  expect(sshForwardLocalURL('127.0.0.1', '1337', 'HTTPS')).toBe(
    'https://127.0.0.1:1337'
  );
  expect(sshForwardLocalURL('::1', '1337', 'HTTP')).toBe('http://[::1]:1337');
  expect(sshForwardLocalURL('127.0.0.1', '1337', 'TCP')).toBeUndefined();
});
