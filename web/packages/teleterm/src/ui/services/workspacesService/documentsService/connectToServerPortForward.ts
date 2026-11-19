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

import { openSSHForwardInBrowser } from 'teleterm/services/tshd/sshGateway';
import { IAppContext } from 'teleterm/ui/types';
import { routing, ServerUri } from 'teleterm/ui/uri';

import { DocumentOrigin } from './types';

export async function connectToServerPortForward(
  ctx: IAppContext,
  target: {
    uri: ServerUri;
    hostname: string;
    login: string;
  },
  options: {
    telemetry: { origin: DocumentOrigin };
    remoteAddr: string;
    localPort?: string;
    idleTimeout?: boolean;
    openInBrowser?: boolean;
    targetProtocol?: string;
  }
): Promise<void> {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  const documentsService =
    ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
  const doc = documentsService.createGatewayDocument({
    targetUri: target.uri,
    origin: options.telemetry.origin,
    targetName: target.hostname,
    targetUser: target.login,
    targetSubresourceName: options.remoteAddr,
    port: options.localPort,
    targetProtocol: options.targetProtocol,
    sshIdleTimeout: options.idleTimeout,
    sshOpenInBrowser: options.openInBrowser,
  });

  const connectionToReuse = ctx.connectionTracker.findConnectionByDocument(doc);

  if (connectionToReuse) {
    await ctx.connectionTracker.activateItem(connectionToReuse.id, {
      origin: options.telemetry.origin,
    });
    if (options.openInBrowser) {
      const gateway = ctx.clustersService.findGatewayByConnectionParams({
        targetUri: doc.targetUri,
        targetUser: doc.targetUser,
        targetSubresourceName: doc.targetSubresourceName,
      });
      if (gateway) {
        openSSHForwardInBrowser(gateway);
      }
    }
    return;
  }

  const { isAtDesiredWorkspace } =
    await ctx.workspacesService.setActiveWorkspace(rootClusterUri);
  if (!isAtDesiredWorkspace) {
    return;
  }
  documentsService.add(doc);
  documentsService.open(doc.uri);
}
