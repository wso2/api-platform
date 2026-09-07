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

import { alpha, Box, Card, CircularProgress, IconButton, Stack, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

const messages = defineMessages({
  connectedNext: {
    id: 'gateways.detail.SetupBanner.connected.next',
    defaultMessage: 'Every setup step is complete.',
    description: 'Sub-line of the setup banner once the gateway agent has connected.',
  },
  connectedTitle: {
    id: 'gateways.detail.SetupBanner.connected.title',
    defaultMessage: 'Your {name} gateway is connected',
    description:
      'Setup banner headline once the gateway is online. {name} is the gateway name the user chose — never translated.',
  },
  createdNext: {
    id: 'gateways.detail.SetupBanner.created.next',
    defaultMessage: 'Next: Configure your {name} gateway',
    description:
      'Sub-line of the setup banner telling the user which step follows. {name} is the gateway name — never translated.',
  },
  createdTitle: {
    id: 'gateways.detail.SetupBanner.created.title',
    defaultMessage: 'You successfully created your {name} gateway',
    description:
      'Setup banner headline after a gateway is registered. {name} is the gateway name — never translated.',
  },
  dismiss: {
    id: 'gateways.detail.SetupBanner.dismiss',
    defaultMessage: 'Dismiss setup progress',
    description: 'Accessible label for the button that closes the setup progress banner.',
  },
  progressLabel: {
    id: 'gateways.detail.SetupBanner.progress.label',
    defaultMessage: 'Gateway setup progress',
    description: 'Accessible name for the ring showing how many setup steps are done.',
  },
  steps: {
    id: 'gateways.detail.SetupBanner.progress.steps',
    defaultMessage: 'Steps',
    description:
      'Word under the "1/2" count inside the setup progress ring. Plural noun, kept very short.',
  },
  stepsRatio: {
    id: 'gateways.detail.SetupBanner.progress.ratio',
    defaultMessage: '{done}/{total}',
    description: 'Completed setup steps out of the total, e.g. "1/2".',
  },
});

/** Setup is two steps: register the gateway, then connect its agent. */
const TOTAL_STEPS = 2;

/** Diameter and stroke of the progress ring, in px. */
const RING_SIZE = 64;
const RING_THICKNESS = 3;

const PERCENT = 100;

export type GatewaySetupBannerProps = {
  /** The gateway's name, echoed back to the user. Never translated. */
  displayName: string;
  /** Whether the gateway's agent has connected — the second step. */
  isConnected: boolean;
  onDismiss: () => void;
};

/**
 * Banner showing gateway setup progress and the next step.
 *
 * It tracks setup completion, not connection status. The banner is
 * dismissible and uses the primary accent throughout.
 */
export function GatewaySetupBanner({
  displayName,
  isConnected,
  onDismiss,
}: GatewaySetupBannerProps) {
  const intl = useIntl();
  const done = isConnected ? TOTAL_STEPS : 1;
  const ratio = intl.formatMessage(messages.stepsRatio, { done, total: TOTAL_STEPS });

  return (
    <Card
      sx={(theme) => ({
        alignItems: 'center',
        backgroundColor: alpha(theme.palette.primary.main, 0.06),
        borderColor: 'primary.main',
        display: 'flex',
        gap: 2,
        p: 2,
      })}
    >
      <Box sx={{ display: 'inline-flex', flexShrink: 0, position: 'relative' }}>
        <CircularProgress
          size={RING_SIZE}
          sx={{ color: 'divider' }}
          thickness={RING_THICKNESS}
          value={PERCENT}
          variant="determinate"
        />
        <CircularProgress
          aria-label={intl.formatMessage(messages.progressLabel)}
          size={RING_SIZE}
          sx={{ color: 'primary.main', left: 0, position: 'absolute' }}
          thickness={RING_THICKNESS}
          value={(done / TOTAL_STEPS) * PERCENT}
          variant="determinate"
        />
        <Stack
          sx={{
            alignItems: 'center',
            inset: 0,
            justifyContent: 'center',
            position: 'absolute',
          }}
        >
          <Typography sx={{ fontWeight: 700 }} variant="body2">
            {ratio}
          </Typography>
          <Typography color="text.secondary" variant="caption">
            <FormattedMessage {...messages.steps} />
          </Typography>
        </Stack>
      </Box>

      <Stack spacing={0.25} sx={{ flexGrow: 1, minWidth: 0 }}>
        <Typography sx={{ fontWeight: 700 }} variant="h6">
          <FormattedMessage
            {...(isConnected ? messages.connectedTitle : messages.createdTitle)}
            values={{ name: displayName }}
          />
        </Typography>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage
            {...(isConnected ? messages.connectedNext : messages.createdNext)}
            values={{ name: displayName }}
          />
        </Typography>
      </Stack>

      <IconButton
        aria-label={intl.formatMessage(messages.dismiss)}
        onClick={onDismiss}
        size="small"
        sx={{ alignSelf: 'flex-start', flexShrink: 0 }}
      >
        <X size={18} />
      </IconButton>
    </Card>
  );
}
