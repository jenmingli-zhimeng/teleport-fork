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

import { FormEvent, useState } from 'react';

import { ButtonIcon, ButtonPrimary, ButtonSecondary, Flex, H2 } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';
import { P } from 'design/Text/Text';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import {
  ConfigFieldInput,
  PortFieldInput,
} from 'teleterm/ui/components/FieldInputs';

export type ForwardPortDialogMode = 'http' | 'https' | 'manual';

export function ForwardPortDialog(props: {
  hostname: string;
  logins: string[];
  mode: ForwardPortDialogMode;
  onCancel(): void;
  onConfirm(result: {
    login: string;
    remoteHost: string;
    remotePort: string;
    localPort?: string;
  }): void;
}) {
  const loginOptions: Option[] = props.logins.map(login => ({
    value: login,
    label: login,
  }));
  const [login, setLogin] = useState<Option>(loginOptions[0]);
  const remoteLocked = props.mode !== 'manual';
  const defaultRemotePort =
    props.mode === 'https' ? '443' : props.mode === 'http' ? '80' : '';
  const title =
    props.mode === 'http'
      ? `Open HTTP on ${props.hostname}`
      : props.mode === 'https'
        ? `Open HTTPS on ${props.hostname}`
        : `Forward a port on ${props.hostname}`;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    const remoteHost = (formData.get('remoteHost') as string) || '127.0.0.1';
    const remotePort = formData.get('remotePort') as string;
    const localPort = (formData.get('localPort') as string) || undefined;
    if (!login?.value || !remotePort) {
      return;
    }
    props.onConfirm({
      login: login.value,
      remoteHost,
      remotePort,
      localPort,
    });
  };

  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '440px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={4}>
        <H2>{title}</H2>
        <ButtonIcon onClick={props.onCancel} color="text.slightlyMuted">
          <Cross size="small" />
        </ButtonIcon>
      </DialogHeader>
      <Validation>
        <form onSubmit={submit}>
          <DialogContent mb={4}>
            <Flex flexDirection="column" gap={3}>
              {props.mode !== 'manual' && (
                <P>
                  Forwards {props.hostname} 127.0.0.1:{defaultRemotePort} to a
                  local port and opens it in the browser. The forward closes
                  after 15 minutes with no connections.
                </P>
              )}
              <FieldSelect
                label="SSH Login"
                options={loginOptions}
                value={login}
                onChange={option => setLogin(option as Option)}
                rule={requiredField('SSH login is required')}
                required
              />
              <ConfigFieldInput
                name="remoteHost"
                label="Remote Host"
                defaultValue="127.0.0.1"
                required
                readonly={remoteLocked}
                mb={0}
              />
              <PortFieldInput
                name="remotePort"
                label="Remote Port"
                defaultValue={defaultRemotePort}
                required
                readonly={remoteLocked}
                mb={0}
              />
              <PortFieldInput
                name="localPort"
                label="Local Port (optional)"
                mb={0}
              />
            </Flex>
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary type="submit" size="large" mr={3}>
              {props.mode === 'manual' ? 'Forward' : 'Open'}
            </ButtonPrimary>
            <ButtonSecondary
              type="button"
              size="large"
              onClick={props.onCancel}
            >
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </form>
      </Validation>
    </DialogConfirmation>
  );
}
