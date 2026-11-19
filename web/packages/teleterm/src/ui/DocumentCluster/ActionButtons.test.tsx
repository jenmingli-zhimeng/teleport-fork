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

import userEvent from '@testing-library/user-event';

import { render, screen, waitFor } from 'design/utils/testing';

import {
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import { ConnectServerActionButton } from './ActionButtons';

test('server action menu includes HTTP, HTTPS, and forward port items', async () => {
  const user = userEvent.setup();
  const appContext = new MockAppContext({ platform: 'linux' });
  const cluster = makeRootCluster();
  appContext.workspacesService.addWorkspace(cluster);
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = cluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(cluster.uri, cluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <ConnectServerActionButton server={makeServer()} />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await user.click(screen.getByTitle('Open menu'));

  expect(await screen.findByText('Open HTTP (80)')).toBeInTheDocument();
  expect(screen.getByText('Open HTTPS (443)')).toBeInTheDocument();
  expect(screen.getByText('Forward port…')).toBeInTheDocument();
});

test('Open HTTP with a single login creates a gateway document', async () => {
  const user = userEvent.setup();
  const appContext = new MockAppContext({ platform: 'linux' });
  const cluster = makeRootCluster();
  appContext.workspacesService.addWorkspace(cluster);
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = cluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(cluster.uri, cluster);
  });
  const server = makeServer({ logins: ['ec2-user'] });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <ConnectServerActionButton server={server} />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await user.click(screen.getByTitle('Open menu'));
  await user.click(await screen.findByText('Open HTTP (80)'));

  await waitFor(() => {
    expect(
      appContext.workspacesService
        .getActiveWorkspaceDocumentService()
        .getGatewayDocuments()
    ).toHaveLength(1);
  });
  const documents = appContext.workspacesService
    .getActiveWorkspaceDocumentService()
    .getGatewayDocuments();
  expect(documents[0]).toMatchObject({
    kind: 'doc.gateway',
    targetUri: server.uri,
    targetUser: 'ec2-user',
    targetSubresourceName: '127.0.0.1:80',
    sshIdleTimeout: true,
    sshOpenInBrowser: true,
  });
});
