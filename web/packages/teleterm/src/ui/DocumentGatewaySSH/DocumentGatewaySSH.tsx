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

import { z } from 'zod';

import { parseSSHForwardTarget } from 'teleterm/services/tshd/sshGateway';
import Document from 'teleterm/ui/Document';
import { DocumentGateway } from 'teleterm/ui/services/workspacesService';

import { ConfigFieldInput, PortFieldInput } from '../components/FieldInputs';
import { FormFields, OfflineGateway } from '../components/OfflineGateway';
import { useGateway } from '../DocumentGateway/useGateway';
import { SSHGateway } from './SSHGateway';

export function DocumentGatewaySSH(props: {
  doc: DocumentGateway;
  visible: boolean;
}) {
  const { doc } = props;
  const {
    gateway,
    defaultPort,
    changePort: changeLocalPort,
    changePortAttempt: changeLocalPortAttempt,
    connected,
    connectAttempt,
    disconnect,
    disconnectAttempt,
    reconnect,
    changeTargetSubresourceName: changeRemoteAddr,
    changeTargetSubresourceNameAttempt: changeRemoteAddrAttempt,
  } = useGateway(doc);

  const remote = parseSSHForwardTarget(doc.targetSubresourceName);
  const remoteLocked = !!doc.sshIdleTimeout;

  return (
    <Document visible={props.visible} data-testid={doc.uri}>
      {!connected ? (
        <OfflineGateway
          connectAttempt={connectAttempt}
          gatewayKind="SSH port forward"
          targetName={doc.targetName}
          reconnect={args =>
            reconnect({
              localPort: args.localPort,
              targetSubresourceName: `${args.remoteHost || remote.host}:${args.remotePort || remote.port}`,
            })
          }
          formSchema={reconnectSchema}
          renderFormControls={(isProcessing: boolean) => (
            <>
              <PortFieldInput
                name={FormFields.LocalPort}
                label="Local Port (optional)"
                defaultValue={defaultPort}
                mb={0}
                readonly={isProcessing}
              />
              <ConfigFieldInput
                name={FormFields.RemoteHost}
                label="Remote Host"
                defaultValue={remote.host}
                required
                mb={0}
                readonly={isProcessing || remoteLocked}
              />
              <PortFieldInput
                name={FormFields.RemotePort}
                label="Remote Port"
                defaultValue={remote.port}
                required
                mb={0}
                readonly={isProcessing || remoteLocked}
              />
            </>
          )}
        />
      ) : (
        <SSHGateway
          gateway={gateway}
          disconnect={disconnect}
          disconnectAttempt={disconnectAttempt}
          changeLocalPort={changeLocalPort}
          changeLocalPortAttempt={changeLocalPortAttempt}
          changeRemoteAddr={changeRemoteAddr}
          changeRemoteAddrAttempt={changeRemoteAddrAttempt}
          remoteLocked={remoteLocked}
          idleTimeout={!!doc.sshIdleTimeout}
        />
      )}
    </Document>
  );
}

const reconnectSchema = z.object({
  [FormFields.LocalPort]: z.string(),
  [FormFields.RemoteHost]: z.string(),
  [FormFields.RemotePort]: z.string(),
});
