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
  SSH_GATEWAY_CLOSED_ERROR,
  SSH_IDLE_TIMEOUT_ERROR,
} from 'teleterm/services/tshd/sshGateway';
import {
  makeRootCluster,
  makeSSHGateway,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeDocumentGatewaySSH } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

test('idle-closed notification removes the gateway and closes the document', () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  const gateway = makeSSHGateway();
  const doc = makeDocumentGatewaySSH({
    gatewayUri: gateway.uri,
    targetUri: gateway.targetUri,
  });
  appContext.addRootClusterWithDoc(cluster, doc);
  appContext.clustersService.setState(draft => {
    draft.gateways.set(gateway.uri, gateway);
  });

  appContext.tshdNotificationsService.sendNotification({
    subject: {
      oneofKind: 'cannotProxyGatewayConnection',
      cannotProxyGatewayConnection: {
        gatewayUri: gateway.uri,
        targetUri: gateway.targetUri,
        error: SSH_IDLE_TIMEOUT_ERROR,
      },
    },
  });

  expect(appContext.clustersService.findGateway(gateway.uri)).toBeUndefined();
  expect(
    appContext.workspacesService
      .getWorkspaceDocumentService(cluster.uri)
      .getDocument(doc.uri)
  ).toBeUndefined();
  const notifications = appContext.notificationsService.getNotifications();
  expect(notifications).toEqual([
    expect.objectContaining({
      severity: 'info',
      content: expect.objectContaining({
        title: 'SSH port forward closed',
        description: expect.stringContaining('15 minutes of inactivity'),
      }),
    }),
  ]);
});

test('dropped SSH session notification removes the gateway and closes the document', () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  const gateway = makeSSHGateway();
  const doc = makeDocumentGatewaySSH({
    gatewayUri: gateway.uri,
    targetUri: gateway.targetUri,
  });
  appContext.addRootClusterWithDoc(cluster, doc);
  appContext.clustersService.setState(draft => {
    draft.gateways.set(gateway.uri, gateway);
  });

  appContext.tshdNotificationsService.sendNotification({
    subject: {
      oneofKind: 'cannotProxyGatewayConnection',
      cannotProxyGatewayConnection: {
        gatewayUri: gateway.uri,
        targetUri: gateway.targetUri,
        error: SSH_GATEWAY_CLOSED_ERROR,
      },
    },
  });

  expect(appContext.clustersService.findGateway(gateway.uri)).toBeUndefined();
  expect(
    appContext.workspacesService
      .getWorkspaceDocumentService(cluster.uri)
      .getDocument(doc.uri)
  ).toBeUndefined();
  const notifications = appContext.notificationsService.getNotifications();
  expect(notifications).toEqual([
    expect.objectContaining({
      severity: 'info',
      content: expect.objectContaining({
        title: 'SSH port forward closed',
        description: expect.stringContaining('SSH session ended'),
      }),
    }),
  ]);
});
