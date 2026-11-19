/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { dialog, shell, WebContents } from 'electron';

import Logger, { NullService } from 'teleterm/logger';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { registerNavigationHandlers } from './navigationHandler';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.resetAllMocks();
});

jest.mock('electron', () => ({
  dialog: {
    showErrorBox: jest.fn(),
  },
  shell: {
    openExternal: jest.fn(),
  },
}));

const cluster = makeRootCluster();

describe('opening links to', () => {
  test.each([
    {
      name: 'Teleport',
      url: 'https://goteleport.com/',
      allowed: true,
    },
    {
      name: 'Gravitational GitHub',
      url: 'https://github.com/gravitational/',
      allowed: true,
    },
    {
      name: 'cluster SSO host',
      // comes from makeRootCluster
      url: 'https://example.auth0.com/some-path',
      allowed: true,
    },
    {
      name: 'cluster proxy host',
      // comes from makeRootCluster
      url: 'https://teleport-local.com:3080/some-path',
      allowed: true,
    },
    {
      name: 'non-HTTPS URLs',
      url: 'http://goteleport.com',
      allowed: false,
    },
    {
      name: 'non-Gravitational GitHub',
      url: 'https://github.com/abc',
      allowed: false,
    },
    {
      name: 'arbitrary URLs',
      url: 'https://google.com',
      allowed: false,
    },
    {
      name: 'AWS IC',
      url: 'https://d-1234.awsapps.com/start/#/console?account_id=1234',
      allowed: true,
    },
    {
      name: 'AWS IC US gov',
      url: 'https://start.us-gov-home.awsapps.com/directory/foo#/console?account_id=1234',
      allowed: true,
    },
    {
      name: 'loopback HTTP',
      url: 'http://127.0.0.1:1337/',
      allowed: true,
    },
    {
      name: 'loopback localhost HTTP',
      url: 'http://localhost:1337/',
      allowed: true,
    },
    {
      name: 'loopback HTTPS',
      url: 'https://127.0.0.1:8443/',
      allowed: true,
    },
    {
      name: 'loopback IPv6 HTTP',
      url: 'http://[::1]:1337/',
      allowed: true,
    },
    {
      name: 'loopback-looking non-local host',
      url: 'http://127.0.0.1.example.com/',
      allowed: false,
    },
  ])('$name', test => {
    let handler: Parameters<WebContents['setWindowOpenHandler']>[0];
    registerNavigationHandlers(
      {
        setWindowOpenHandler: d => {
          handler = d;
        },
        on: jest.fn(),
      },
      makeRuntimeSettings(),
      { getRootClusters: () => [cluster] },
      new Logger()
    );

    const result = handler({
      url: test.url,
      frameName: '',
      features: '',
      disposition: 'default',
      referrer: undefined,
    });

    expect(result).toEqual({ action: 'deny' });
    /* oxlint-disable jest/no-conditional-expect */
    if (test.allowed) {
      expect(shell.openExternal).toHaveBeenCalledWith(test.url);
      expect(dialog.showErrorBox).not.toHaveBeenCalled();
    } else {
      expect(shell.openExternal).not.toHaveBeenCalled();
      expect(dialog.showErrorBox).toHaveBeenCalledWith(
        'Cannot open this link',
        'The domain does not match any of the allowed domains. Check main.log for more details.'
      );
    }
    /* oxlint-enable jest/no-conditional-expect */
  });
});
