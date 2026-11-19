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
  ChangeEvent,
  ChangeEventHandler,
  PropsWithChildren,
  useEffect,
  useMemo,
  useState,
} from 'react';
import styled from 'styled-components';

import {
  Alert,
  ButtonSecondary,
  disappear,
  Flex,
  H1,
  LabelInput,
  rotate360,
  Text,
} from 'design';
import { Check, Spinner } from 'design/Icon';
import { LabelContent } from 'design/LabelInput/LabelInput';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import {
  openSSHForwardInBrowser,
  parseSSHForwardTarget,
  sshForwardLocalURL,
} from 'teleterm/services/tshd/sshGateway';
import {
  ConfigFieldInput,
  PortFieldInput,
} from 'teleterm/ui/components/FieldInputs';

export function SSHGateway(props: {
  gateway: Gateway;
  disconnectAttempt: Attempt<void>;
  changeLocalPort(port: string): void;
  changeLocalPortAttempt: Attempt<void>;
  changeRemoteAddr(addr: string): void;
  changeRemoteAddrAttempt: Attempt<void>;
  disconnect(): void;
  remoteLocked?: boolean;
  idleTimeout?: boolean;
}) {
  const { gateway } = props;
  const remote = parseSSHForwardTarget(gateway.targetSubresourceName);
  const [remoteHost, setRemoteHost] = useState(remote.host);
  const [remotePort, setRemotePort] = useState(remote.port);
  const localURL = sshForwardLocalURL(
    gateway.localAddress,
    gateway.localPort,
    gateway.protocol
  );
  const address = localURL || `${gateway.localAddress}:${gateway.localPort}`;

  const handleLocalPortChange = useDebouncedChangeHandler(
    props.changeLocalPort
  );
  const emitRemoteAddr = useMemo(
    () =>
      debounce((host: string, port: string) => {
        if (!host || !port) {
          return;
        }
        props.changeRemoteAddr(`${host}:${port}`);
      }, 1000),
    [props.changeRemoteAddr]
  );

  const handleRemoteHostChange: ChangeEventHandler<
    HTMLInputElement
  > = event => {
    const host = event.target.value;
    setRemoteHost(host);
    if (event.target.reportValidity()) {
      emitRemoteAddr(host, remotePort);
    }
  };
  const handleRemotePortChange: ChangeEventHandler<
    HTMLInputElement
  > = event => {
    const port = event.target.value;
    setRemotePort(port);
    if (event.target.reportValidity()) {
      emitRemoteAddr(remoteHost, port);
    }
  };

  return (
    <Flex
      flexDirection="column"
      maxWidth="680px"
      width="100%"
      mx="auto"
      mt="4"
      px="5"
      gap={3}
    >
      <Flex flexDirection="column" gap={2}>
        <Flex justifyContent="space-between" mb="2" flexWrap="wrap" gap={2}>
          <H1>SSH Port Forward</H1>
          <Flex gap={2}>
            {localURL && (
              <ButtonSecondary
                size="small"
                onClick={() => openSSHForwardInBrowser(gateway)}
              >
                Open in Browser
              </ButtonSecondary>
            )}
            <ButtonSecondary size="small" onClick={props.disconnect}>
              Close Connection
            </ButtonSecondary>
          </Flex>
        </Flex>

        {props.disconnectAttempt.status === 'error' && (
          <Alert details={props.disconnectAttempt.statusText} m={0}>
            Could not close the connection
          </Alert>
        )}

        <Flex as="form" gap={2} flexWrap="wrap">
          <Validation>
            <LabelWithAttemptStatus
              text="Local Port"
              attempt={props.changeLocalPortAttempt}
            >
              <PortFieldInput
                defaultValue={gateway.localPort}
                onChange={handleLocalPortChange}
                mb={0}
              />
            </LabelWithAttemptStatus>
            <LabelWithAttemptStatus
              text="Remote Host"
              attempt={props.changeRemoteAddrAttempt}
              required
            >
              <ConfigFieldInput
                value={remoteHost}
                onChange={handleRemoteHostChange}
                required
                readonly={props.remoteLocked}
                mb={0}
              />
            </LabelWithAttemptStatus>
            <LabelWithAttemptStatus
              text="Remote Port"
              attempt={props.changeRemoteAddrAttempt}
              required
            >
              <PortFieldInput
                value={remotePort}
                onChange={handleRemotePortChange}
                required
                readonly={props.remoteLocked}
                mb={0}
              />
            </LabelWithAttemptStatus>
          </Validation>
        </Flex>
      </Flex>

      <Flex flexDirection="column" gap={2}>
        <div>
          <Text>Access the forwarded port at:</Text>
          <TextSelectCopy mt={1} text={address} bash={false} />
        </div>

        {gateway.gatewayCliCommand?.preview && (
          <div>
            <Text>CLI:</Text>
            <TextSelectCopy
              mt={1}
              text={gateway.gatewayCliCommand.preview}
              bash={false}
            />
          </div>
        )}

        {props.changeLocalPortAttempt.status === 'error' && (
          <Alert details={props.changeLocalPortAttempt.statusText} m={0}>
            Could not change the local port
          </Alert>
        )}

        {props.changeRemoteAddrAttempt.status === 'error' && (
          <Alert details={props.changeRemoteAddrAttempt.statusText} m={0}>
            Could not change the remote address
          </Alert>
        )}

        {props.idleTimeout && (
          <Text>
            This forward closes automatically after 15 minutes with no TCP
            connections.
          </Text>
        )}

        {gateway.protocol === 'HTTPS' && (
          <Text>
            Browsers warn that the certificate does not match 127.0.0.1. This is
            expected for SSH local forwarding; Connect does not terminate TLS
            locally.
          </Text>
        )}
      </Flex>
    </Flex>
  );
}

const LabelWithAttemptStatus = (
  props: PropsWithChildren<{
    text: string;
    attempt: Attempt<unknown>;
    required?: boolean;
  }>
) => (
  <LabelInput
    mb={0}
    css={`
      width: fit-content;
    `}
  >
    <Flex
      alignItems="center"
      justifyContent="space-between"
      mb={1}
      css={`
        position: relative;
      `}
    >
      <LabelContent required={props.required}>{props.text}</LabelContent>
      {props.attempt.status === 'processing' && (
        <AnimatedSpinner color="interactive.tonal.neutral.2" size="small" />
      )}
      {props.attempt.status === 'success' && (
        <UnmountAfter
          timeoutMs={disappearanceDelayMs + disappearanceDurationMs}
        >
          <DisappearingCheck
            color="success.main"
            size="small"
            title={`${props.text} successfully updated`}
          />
        </UnmountAfter>
      )}
    </Flex>
    {props.children}
  </LabelInput>
);

const useDebouncedChangeHandler = (
  changeFunc: (value: string) => void
): ChangeEventHandler<HTMLInputElement> =>
  useMemo(
    () =>
      debounce((event: ChangeEvent<HTMLInputElement>) => {
        if (event.target.reportValidity()) {
          changeFunc(event.target.value);
        }
      }, 1000),
    [changeFunc]
  );

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s infinite linear;
  position: absolute;
  right: 0;
  top: 0;
`;

const disappearanceDelayMs = 1000;
const disappearanceDurationMs = 200;

const DisappearingCheck = styled(Check)`
  opacity: 1;
  animation: ${disappear};
  animation-delay: ${disappearanceDelayMs}ms;
  animation-duration: ${disappearanceDurationMs}ms;
  animation-fill-mode: forwards;
`;

const UnmountAfter = ({
  timeoutMs,
  children,
}: PropsWithChildren<{ timeoutMs: number }>) => {
  const [isMounted, setIsMounted] = useState(true);

  useEffect(() => {
    const timeout = setTimeout(() => {
      setIsMounted(false);
    }, timeoutMs);

    return () => {
      clearTimeout(timeout);
    };
  }, [timeoutMs]);

  return isMounted ? children : null;
};
