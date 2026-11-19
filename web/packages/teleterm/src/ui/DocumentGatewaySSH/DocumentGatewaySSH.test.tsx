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

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeRootCluster,
  makeSSHGateway,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import type * as docs from 'teleterm/ui/services/workspacesService';
import { ServerUri } from 'teleterm/ui/uri';

import { DocumentGatewaySSH } from './DocumentGatewaySSH';

beforeEach(() => {
  jest.restoreAllMocks();
});

test('reconnects an SSH port forward', async () => {
  const user = userEvent.setup();
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  const gateway = makeSSHGateway();
  const doc: docs.DocumentGateway = {
    uri: '/docs/1',
    kind: 'doc.gateway',
    targetName: gateway.targetName,
    targetUri: gateway.targetUri as ServerUri,
    targetUser: gateway.targetUser,
    targetSubresourceName: gateway.targetSubresourceName,
    gatewayUri: gateway.uri,
    origin: 'resource_table',
    title: '',
    status: '',
  };
  appContext.addRootClusterWithDoc(cluster, doc);

  jest
    .spyOn(appContext.tshd, 'createGateway')
    .mockReturnValueOnce(
      new MockedUnaryCall(undefined, new Error('Something went wrong'))
    )
    .mockReturnValueOnce(new MockedUnaryCall(gateway));

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <DocumentGatewaySSH visible doc={doc} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  expect(
    await screen.findByText('Could not establish the connection')
  ).toBeInTheDocument();

  await user.click(screen.getByText('Reconnect'));

  expect(await screen.findByText('Close Connection')).toBeInTheDocument();
  expect(screen.getByText('SSH Port Forward')).toBeInTheDocument();
});

test('locks the remote address for one-click HTTP forwards', async () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  const gateway = makeSSHGateway();
  const doc: docs.DocumentGateway = {
    uri: '/docs/1',
    kind: 'doc.gateway',
    targetName: gateway.targetName,
    targetUri: gateway.targetUri as ServerUri,
    targetUser: gateway.targetUser,
    targetSubresourceName: gateway.targetSubresourceName,
    gatewayUri: gateway.uri,
    origin: 'resource_table',
    title: '',
    status: '',
    sshIdleTimeout: true,
  };
  appContext.addRootClusterWithDoc(cluster, doc);
  appContext.clustersService.setState(draft => {
    draft.gateways.set(gateway.uri, gateway);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <DocumentGatewaySSH visible doc={doc} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  expect(await screen.findByText('SSH Port Forward')).toBeInTheDocument();
  expect(screen.getByLabelText(/Remote Host/)).toHaveAttribute('readonly');
  expect(screen.getByLabelText(/Remote Port/)).toHaveAttribute('readonly');
  expect(
    screen.getByText(/closes automatically after 15 minutes/)
  ).toBeInTheDocument();
});

test('Close Connection succeeds if tshd has already removed the gateway', async () => {
  const user = userEvent.setup();
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  const gateway = makeSSHGateway();
  const doc: docs.DocumentGateway = {
    uri: '/docs/1',
    kind: 'doc.gateway',
    targetName: gateway.targetName,
    targetUri: gateway.targetUri as ServerUri,
    targetUser: gateway.targetUser,
    targetSubresourceName: gateway.targetSubresourceName,
    gatewayUri: gateway.uri,
    origin: 'resource_table',
    title: '',
    status: '',
  };
  appContext.addRootClusterWithDoc(cluster, doc);
  appContext.clustersService.setState(draft => {
    draft.gateways.set(gateway.uri, gateway);
  });

  jest.spyOn(appContext.tshd, 'removeGateway').mockReturnValue(
    new MockedUnaryCall(
      undefined,
      Object.assign(new Error(`gateway is not found: ${gateway.uri}`), {
        name: 'RpcError',
        code: 'NOT_FOUND',
      })
    )
  );

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <DocumentGatewaySSH visible doc={doc} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  expect(await screen.findByText('Close Connection')).toBeInTheDocument();
  await user.click(screen.getByText('Close Connection'));

  await waitFor(() => {
    expect(
      appContext.workspacesService
        .getWorkspaceDocumentService(cluster.uri)
        .getDocument(doc.uri)
    ).toBeUndefined();
  });
  expect(appContext.clustersService.findGateway(gateway.uri)).toBeUndefined();
  expect(
    screen.queryByText('Could not close the connection')
  ).not.toBeInTheDocument();
});
