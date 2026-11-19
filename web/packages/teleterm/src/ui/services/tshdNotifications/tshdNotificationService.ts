/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
  cannotProxyVnetConnectionReasonIsCertReissueError,
  cannotProxyVnetConnectionReasonIsInvalidLocalPort,
  notificationRequestOneOfIsCannotProxyGatewayConnection,
  notificationRequestOneOfIsCannotProxyVnetConnection,
} from 'teleterm/helpers';
import {
  formatPortRange,
  publicAddrWithTargetPort,
} from 'teleterm/services/tshd/app';
import { getTargetNameFromUri } from 'teleterm/services/tshd/gateway';
import {
  isSSHGatewayClosedError,
  SSH_IDLE_TIMEOUT_ERROR,
} from 'teleterm/services/tshd/sshGateway';
import { SendNotificationRequest } from 'teleterm/services/tshdEvents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import {
  NotificationContent,
  NotificationKey,
  NotificationsService,
} from 'teleterm/ui/services/notifications';
import type { WorkspacesService } from 'teleterm/ui/services/workspacesService/workspacesService';
import {
  GatewayUri,
  ResourceUri,
  RootClusterUri,
  routing,
} from 'teleterm/ui/uri';

export class TshdNotificationsService {
  constructor(
    private notificationsService: NotificationsService,
    private clustersService: ClustersService,
    private workspacesService: WorkspacesService
  ) {}

  sendNotification(request: SendNotificationRequest) {
    if (this.handleSSHIdleClosed(request)) {
      return;
    }
    const notificationContent = this.getNotificationContent(request);
    this.notificationsService.notifyError(notificationContent);
  }

  /**
   * tshd reports SSH gateway teardown through CannotProxyGatewayConnection
   * with a sentinel error. The gateway is already gone on the daemon side;
   * drop stale UI state so Close Connection / new forwards keep working.
   */
  private handleSSHIdleClosed(request: SendNotificationRequest): boolean {
    const { subject } = request;
    if (
      !notificationRequestOneOfIsCannotProxyGatewayConnection(subject) ||
      !isSSHGatewayClosedError(subject.cannotProxyGatewayConnection.error)
    ) {
      return false;
    }

    const { gatewayUri, targetUri, error } =
      subject.cannotProxyGatewayConnection;
    const gateway = this.clustersService.findGateway(gatewayUri);
    const targetName = gateway?.targetName || getTargetNameFromUri(targetUri);
    const remote = gateway?.targetSubresourceName;
    this.clustersService.removeGatewayFromState(gatewayUri as GatewayUri);
    this.closeGatewayDocuments(gatewayUri);

    const desc = remote ? `${targetName} (${remote})` : targetName;
    const idle = error === SSH_IDLE_TIMEOUT_ERROR;
    this.notificationsService.notifyInfo({
      title: 'SSH port forward closed',
      description: idle
        ? `The local forward to ${desc} was closed after 15 minutes of inactivity.`
        : `The local forward to ${desc} was closed because the SSH session ended.`,
      key: ['sshGatewayIdleClosed', routing.ensureRootClusterUri(targetUri)],
    });
    return true;
  }

  private closeGatewayDocuments(gatewayUri: string) {
    for (const rootClusterUri of Object.keys(
      this.workspacesService.getWorkspaces()
    ) as RootClusterUri[]) {
      const documentsService =
        this.workspacesService.getWorkspaceDocumentService(rootClusterUri);
      if (!documentsService) {
        continue;
      }
      for (const doc of [...documentsService.getDocuments()]) {
        if (doc.kind === 'doc.gateway' && doc.gatewayUri === gatewayUri) {
          documentsService.close(doc.uri);
        }
      }
    }
  }

  // All returned notification contents should include a key that limits the notification to a
  // single kind per root cluster. This is to avoid spamming the user with notifications if a
  // 3rd-party client is trying to, say, reconnect through a gateway connection.
  private getNotificationContent(
    request: SendNotificationRequest
  ): NotificationContent & {
    key: NotificationKey;
  } {
    const { subject } = request;
    // switch followed by a type guard is awkward, but it helps with ensuring that we get type
    // errors whenever a new request reason is added.
    //
    // Type guards must be called because of how protobuf-ts generates types for oneOf in protos.
    switch (subject.oneofKind) {
      case 'cannotProxyGatewayConnection': {
        if (!notificationRequestOneOfIsCannotProxyGatewayConnection(subject)) {
          return;
        }
        const { gatewayUri, targetUri, error } =
          subject.cannotProxyGatewayConnection;
        const gateway = this.clustersService.findGateway(gatewayUri);
        const clusterName = this.getClusterName(targetUri);
        const rootClusterUri = routing.ensureRootClusterUri(targetUri);
        let targetName: string;
        let targetUser: string;
        let targetDesc: string;

        if (gateway) {
          targetName = gateway.targetName;
          targetUser = gateway.targetUser;
        } else {
          targetName = getTargetNameFromUri(targetUri);
        }

        if (targetUser) {
          targetDesc = `${targetName} as ${targetUser}`;
        } else {
          targetDesc = targetName;
        }

        return {
          title: `Cannot connect to ${targetDesc} (${clusterName})`,
          description: `A connection attempt to ${targetDesc} failed due to an unexpected error: ${error}`,
          key: ['cannotProxyGatewayConnection', rootClusterUri],
        };
      }
      case 'cannotProxyVnetConnection': {
        if (!notificationRequestOneOfIsCannotProxyVnetConnection(subject)) {
          return;
        }
        const { routeToApp, targetUri, reason } =
          subject.cannotProxyVnetConnection;
        const clusterName = this.getClusterName(targetUri);
        const rootClusterUri = routing.ensureRootClusterUri(targetUri);

        switch (reason.oneofKind) {
          case 'certReissueError': {
            if (!cannotProxyVnetConnectionReasonIsCertReissueError(reason)) {
              return;
            }
            const { error } = reason.certReissueError;

            return {
              title: `Cannot connect to ${publicAddrWithTargetPort(routeToApp)}`,
              description: `A connection attempt to the app in the cluster ${clusterName} failed due to an unexpected error: ${error}`,
              key: [
                'cannotProxyVnetConnection',
                'certReissueError',
                rootClusterUri,
              ],
            };
          }
          case 'invalidLocalPort': {
            if (!cannotProxyVnetConnectionReasonIsInvalidLocalPort(reason)) {
              return;
            }

            // Ports are not present if there's more than 10 port ranges defined on an app.
            const ports = reason.invalidLocalPort.tcpPorts
              .map(portRange => formatPortRange(portRange))
              .join(', ');

            let description =
              `A connection attempt on the port ${routeToApp.targetPort} was refused ` +
              `as that port is not included in the target ports of the app ${routeToApp.clusterName} in the cluster ${clusterName}.`;

            if (ports) {
              description += ` Valid ports: ${ports}.`;
            }

            return {
              title: `Invalid port for ${publicAddrWithTargetPort(routeToApp)}`,
              description,
              // 3rd-party clients can potentially make dozens of attempts to connect to an invalid
              // port within a short time. As all notifications from this service go as errors, we
              // don't want to force the user to manually close each notification.
              isAutoRemovable: true,
              key: [
                'cannotProxyVnetConnection',
                'invalidLocalPort',
                rootClusterUri,
              ],
            };
          }
          default: {
            reason satisfies never;
            return;
          }
        }
      }
      default: {
        subject satisfies never;
        return;
      }
    }
  }

  private getClusterName(uri: ResourceUri): string {
    const clusterUri = routing.ensureClusterUri(uri);
    const cluster = this.clustersService.findCluster(clusterUri);

    return cluster?.name || routing.parseClusterName(clusterUri);
  }
}
