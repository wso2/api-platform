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

import {
  Box,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, Info, Server, Waypoints } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import { segmentedSwitchSx } from '@/theme/receipes';
import { CopyButton } from '../curl/components/CopyButton';
import type { TestCallMode } from '../utils/callMode';

const messages = defineMessages({
  callModeLabel: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.callModeLabel',
    defaultMessage: 'Send requests through the proxy or directly',
    description: 'Accessible label for the toggle between the two ways of sending a request.',
  },
  callModeHintLabel: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.callModeHintLabel',
    defaultMessage: 'What this routing choice means',
    description: 'Accessible label for the info button explaining the selected routing mode.',
  },
  copyEndpoint: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.copyEndpoint',
    defaultMessage: 'Copy endpoint URL',
    description: 'Accessible label for the button copying the gateway invoke URL.',
  },
  directHint: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.directHint',
    defaultMessage:
      'Requests go straight from your browser, so the gateway must be reachable from this machine and allow this origin with a CORS policy.',
    description:
      'Caption under the endpoint while Direct is selected. States what the user takes on by choosing it — the browser, not the portal, now has to reach the gateway.',
  },
  directMode: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.directMode',
    defaultMessage: 'Direct',
    description:
      'Toggle option sending the request from the browser straight to the gateway. An adverb describing how the request travels, not a command.',
  },
  endpoint: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.endpoint',
    defaultMessage: 'Endpoint',
    description:
      'Label above the URL that requests from this console are sent to. Shown in capitals by the layout, so translate it as ordinary words.',
  },
  gatewayLabel: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.gatewayLabel',
    defaultMessage: 'Gateway',
    description: 'Label of the picker choosing which deployed gateway to test against.',
  },
  noGateways: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.noGateways',
    defaultMessage: 'No deployed gateway.',
    description:
      'Shown in place of the picker when the API is not deployed anywhere. States the fact only — the page-level banner carries the call to action, so this must not repeat it.',
  },
  proxyHint: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.proxyHint',
    defaultMessage: 'Requests go through a proxy, so the gateway does not need a CORS policy.',
    description:
      'Caption under the endpoint while Through proxy is selected. Explains the benefit of the default, so the user can tell what they would give up by switching.',
  },
  proxyMode: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.proxyMode',
    defaultMessage: 'Through proxy',
    description:
      'Toggle option relaying the request via the portal server. Describes the route a request takes, not a command.',
  },
  title: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.title',
    defaultMessage: 'Gateway',
    description: 'Heading of the section selecting which gateway the console tests against.',
  },
});

type GatewaySectionProps = {
  /**
   * How the Console view sends a request. Omitted — together with
   * `onCallModeChange` — by the cURL view, which has no transport to choose:
   * a copied command always leaves from the user's own terminal.
   */
  callMode?: TestCallMode;
  endpoint: string;
  gateways: Gateway[];
  onCallModeChange?: (mode: TestCallMode) => void;
  onSelect: (gatewayId: string) => void;
  optionLabel: (gateway: Gateway) => string;
  selectedGatewayId: string;
};

/**
 * Renders the gateway selector, request transport controls, and invoke URL.
 *
 * This component is rendered as a section within the page card. Together with
 * `TestKeySection`, it describes the request destination and credentials
 * without introducing an additional card boundary.
 *
 * The proxy/direct transport control is colocated with the endpoint because it
 * specifies how that endpoint is reached. The endpoint caption describes the
 * effect of the selected transport mode.
 *
 * The component does not render a health or deployment-status indicator.
 * Deployment availability is represented by the page-level empty state when
 * no gateways are deployed.
 */
export function GatewaySection({
  callMode,
  endpoint,
  gateways,
  onCallModeChange,
  onSelect,
  optionLabel,
  selectedGatewayId,
}: GatewaySectionProps) {
  const intl = useIntl();

  /** Both halves arrive together or not at all; neither is useful alone. */
  const showCallMode = callMode !== undefined && onCallModeChange !== undefined;

  return (
    <Box sx={{ px: 2, py: 2 }}>
      <Stack spacing={1.5}>
        <Stack
          alignItems="center"
          direction="row"
          justifyContent="space-between"
          spacing={1}
          sx={{ minHeight: 36 }}
        >
          <Stack alignItems="center" direction="row" spacing={1}>
            <Box sx={{ color: 'primary.main', display: 'flex' }}>
              <Server size={18} />
            </Box>
            <Typography variant="subtitle2">
              <FormattedMessage {...messages.title} />
            </Typography>
          </Stack>

          {showCallMode && (
            <Stack alignItems="center" direction="row" spacing={1}>
              <ToggleButtonGroup
                aria-label={intl.formatMessage(messages.callModeLabel)}
                exclusive
                onChange={(_event, next) => next && onCallModeChange(next as TestCallMode)}
                size="small"
                sx={segmentedSwitchSx}
                value={callMode}
              >
                <ToggleButton value="proxy">
                  <Stack alignItems="center" direction="row" spacing={1}>
                    <Waypoints size={16} />
                    <span>
                      <FormattedMessage {...messages.proxyMode} />
                    </span>
                  </Stack>
                </ToggleButton>
                <ToggleButton value="direct">
                  <Stack alignItems="center" direction="row" spacing={1}>
                    <ArrowRight size={16} />
                    <span>
                      <FormattedMessage {...messages.directMode} />
                    </span>
                  </Stack>
                </ToggleButton>
              </ToggleButtonGroup>

              {/* The consequence of the selected mode, on demand rather than
                  as a permanent line under the endpoint: it is the same two
                  sentences every time, so once read it is noise. */}
              <Tooltip
                arrow
                placement="bottom-end"
                title={
                  <Stack spacing={0.5} sx={{ py: 0.5 }}>
                    <Typography sx={{ fontWeight: 'fontWeightBold' }} variant="caption">
                      <FormattedMessage
                        {...(callMode === 'direct' ? messages.directMode : messages.proxyMode)}
                      />
                    </Typography>
                    <Typography variant="caption">
                      <FormattedMessage
                        {...(callMode === 'direct' ? messages.directHint : messages.proxyHint)}
                      />
                    </Typography>
                  </Stack>
                }
              >
                <IconButton
                  aria-label={intl.formatMessage(messages.callModeHintLabel)}
                  size="small"
                  sx={{ color: 'primary.main' }}
                >
                  <Info size={18} />
                </IconButton>
              </Tooltip>
            </Stack>
          )}
        </Stack>

        {gateways.length === 0 ? (
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.noGateways} />
          </Typography>
        ) : (
          <Grid container spacing={1}>
            <Grid size={{ sm: 4, xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel id="test-console-gateway-label" sx={{ display: 'none' }}>
                  <FormattedMessage {...messages.gatewayLabel} />
                </FormLabel>
                <Select
                  labelId="test-console-gateway-label"
                  onChange={(event) => onSelect(String(event.target.value))}
                  size="small"
                  value={selectedGatewayId}
                >
                  {gateways.map((gateway) => (
                    <MenuItem key={gateway.id} value={gateway.id ?? ''}>
                      {optionLabel(gateway)}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ sm: 8, xs: 12 }}>
              <TextField
                fullWidth
                size="small"
                slotProps={{
                  htmlInput: {
                    'aria-label': intl.formatMessage(messages.endpoint),
                  },
                  input: {
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <CopyButton
                          getValue={() => endpoint}
                          label={intl.formatMessage(messages.copyEndpoint)}
                          variant="icon"
                        />
                      </InputAdornment>
                    ),
                  },
                }}
                value={endpoint}
              />
            </Grid>
          </Grid>
        )}
      </Stack>
    </Box>
  );
}
