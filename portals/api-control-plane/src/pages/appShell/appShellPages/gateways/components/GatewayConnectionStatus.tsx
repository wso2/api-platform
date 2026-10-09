/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useEffect, useState } from 'react';
import { Alert, Box, CircularProgress, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, type MessageDescriptor } from 'react-intl';

const messages = defineMessages({
  connected: {
    id: 'gateways.detail.GetStarted.status.connected',
    defaultMessage: 'Connected. This gateway is ready for deployments.',
  },
  waiting: {
    id: 'gateways.detail.GetStarted.status.waiting',
    defaultMessage: 'Waiting for your gateway to connect…',
    description: 'Shown under the start command while the console polls for the gateway.',
  },
  slowTitle: {
    id: 'gateways.detail.GetStarted.status.slow.title',
    defaultMessage: 'Still not connected? Check that:',
  },
  slowRunning: {
    id: 'gateways.detail.GetStarted.status.slow.running',
    defaultMessage: 'the containers are running: docker compose ps',
    description: '"docker compose ps" is a command; keep it exactly as written.',
  },
  slowLogs: {
    id: 'gateways.detail.GetStarted.status.slow.logs',
    defaultMessage:
      'the controller logs show no connection errors: docker compose logs gateway-controller',
    description: 'The command after the colon must be kept exactly as written.',
  },
  slowHost: {
    id: 'gateways.detail.GetStarted.status.slow.host',
    defaultMessage:
      'the control plane address in the settings is reachable from the gateway’s machine',
  },
  slowToken: {
    id: 'gateways.detail.GetStarted.status.slow.token',
    defaultMessage: 'the token in the settings is the one generated above, copied in full',
  },
});

/** How long to wait before offering troubleshooting. Pulling images takes a while. */
const SLOW_AFTER_MS = 3 * 60_000;

const TROUBLESHOOTING: MessageDescriptor[] = [
  messages.slowRunning,
  messages.slowLogs,
  messages.slowHost,
  messages.slowToken,
];

/**
 * The live answer to "did it work?" under the start command. The page polls
 * the gateway while it is not connected, so this flips on its own; nothing
 * here asks the user to reload.
 *
 * Troubleshooting only appears once a token has been generated here and some
 * minutes have passed: before that, "not connected yet" is the expected state,
 * not a problem.
 */
export function GatewayConnectionStatus({
  isConnected,
  waitingSinceToken,
}: {
  isConnected: boolean;
  /** Whether a token was generated on this visit, which starts the slow-connect timer. */
  waitingSinceToken: boolean;
}) {
  const [slow, setSlow] = useState(false);

  useEffect(() => {
    if (isConnected || !waitingSinceToken) {
      setSlow(false);
      return undefined;
    }
    const timer = window.setTimeout(() => setSlow(true), SLOW_AFTER_MS);
    return () => window.clearTimeout(timer);
  }, [isConnected, waitingSinceToken]);

  return (
    <Box aria-live="polite" role="status">
      {isConnected ? (
        <Alert severity="success">
          <FormattedMessage {...messages.connected} />
        </Alert>
      ) : (
        <Stack spacing={1.5}>
          <Stack alignItems="center" direction="row" spacing={1.5}>
            <CircularProgress aria-hidden size={16} />
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.waiting} />
            </Typography>
          </Stack>
          {slow && (
            <Alert severity="info">
              <Typography sx={{ fontWeight: 600 }} variant="body2">
                <FormattedMessage {...messages.slowTitle} />
              </Typography>
              <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
                {TROUBLESHOOTING.map((item) => (
                  <Typography component="li" key={item.id} variant="body2">
                    <FormattedMessage {...item} />
                  </Typography>
                ))}
              </Box>
            </Alert>
          )}
        </Stack>
      )}
    </Box>
  );
}
