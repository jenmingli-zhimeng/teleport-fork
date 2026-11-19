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
  makeRootCluster,
  makeServer,
  makeSSHGateway,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { IAppContext } from 'teleterm/ui/types';

import { connectToServerPortForward } from './connectToServerPortForward';

test('creates an SSH gateway document', async () => {
  const appContext = new MockAppContext();
  setTestCluster(appContext);
  const server = makeServer();

  await connectToServerPortForward(
    appContext,
    { uri: server.uri, hostname: server.hostname, login: 'ec2-user' },
    {
      telemetry: { origin: 'resource_table' },
      remoteAddr: 'localhost:80',
      idleTimeout: true,
      openInBrowser: true,
      targetProtocol: 'HTTP',
    }
  );

  const documents = appContext.workspacesService
    .getActiveWorkspaceDocumentService()
    .getGatewayDocuments();
  expect(documents).toHaveLength(1);
  expect(documents[0]).toEqual({
    gatewayUri: undefined,
    kind: 'doc.gateway',
    origin: 'resource_table',
    port: undefined,
    status: '',
    targetName: 'foo',
    targetSubresourceName: 'localhost:80',
    targetUri: server.uri,
    targetUser: 'ec2-user',
    title: 'ec2-user@foo (localhost:80)',
    uri: expect.any(String),
    targetProtocol: 'HTTP',
    sshIdleTimeout: true,
    sshOpenInBrowser: true,
  });
});

test('reuses an existing connection and reopens the browser', async () => {
  jest.spyOn(window, 'open').mockImplementation();
  const appContext = new MockAppContext();
  setTestCluster(appContext);
  const server = makeServer();
  const gateway = makeSSHGateway();
  appContext.clustersService.setState(draft => {
    draft.gateways.set(gateway.uri, gateway);
  });

  await connectToServerPortForward(
    appContext,
    { uri: server.uri, hostname: server.hostname, login: 'ec2-user' },
    {
      telemetry: { origin: 'resource_table' },
      remoteAddr: 'localhost:80',
      idleTimeout: true,
      openInBrowser: true,
      targetProtocol: 'HTTP',
    }
  );

  jest.spyOn(appContext.connectionTracker, 'activateItem');

  await connectToServerPortForward(
    appContext,
    { uri: server.uri, hostname: server.hostname, login: 'ec2-user' },
    {
      telemetry: { origin: 'resource_table' },
      remoteAddr: 'localhost:80',
      idleTimeout: true,
      openInBrowser: true,
      targetProtocol: 'HTTP',
    }
  );

  expect(appContext.connectionTracker.activateItem).toHaveBeenCalled();
  expect(window.open).toHaveBeenCalledWith(
    'http://127.0.0.1:1337',
    '_blank',
    'noreferrer,noopener'
  );
  expect(
    appContext.workspacesService
      .getActiveWorkspaceDocumentService()
      .getGatewayDocuments()
  ).toHaveLength(1);
});

function setTestCluster(appContext: IAppContext): void {
  const testCluster = makeRootCluster();
  appContext.workspacesService.addWorkspace(testCluster);
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });
}
