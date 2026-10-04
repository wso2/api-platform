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
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Server } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import { CopyButton } from '../curl/components/CopyButton';

const messages = defineMessages({
  copyEndpoint: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.copyEndpoint',
    defaultMessage: 'Copy endpoint URL',
    description: 'Accessible label for the button copying the gateway invoke URL.',
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
  title: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.title',
    defaultMessage: 'Gateway',
    description: 'Heading of the section selecting which gateway the console tests against.',
  },
});

type GatewaySectionProps = {
  endpoint: string;
  gateways: Gateway[];
  onSelect: (gatewayId: string) => void;
  optionLabel: (gateway: Gateway) => string;
  selectedGatewayId: string;
};

/**
 * Picks which deployed gateway the console targets, and shows the resulting
 * invoke URL.
 *
 * A section rather than a card: this and `TestKeySection` describe one thing
 * between them — where a request goes and what it carries — and two bordered
 * boxes side by side read as two unrelated settings. The page seats both in a
 * single card, which is why nothing here draws a border or a background of its
 * own beyond the endpoint well.
 *
 * There is no health badge, and the "Deployed" one this used to carry went with
 * the redesign. `Gateway` has no health or status field to read, and deployment
 * is already implied: the page renders its deploy-first empty state instead of
 * this card when the API is deployed nowhere.
 */
export function GatewaySection({
  endpoint,
  gateways,
  onSelect,
  optionLabel,
  selectedGatewayId,
}: GatewaySectionProps) {
  const intl = useIntl();

  return (
    <Box sx={{ px: 2, py: 2 }}>
      <Stack spacing={1.5}>
        <Stack alignItems="center" direction="row" spacing={1}>
          <Box sx={{ color: 'primary.main', display: 'flex' }}>
            <Server size={18} />
          </Box>
          <Typography variant="subtitle2">
            <FormattedMessage {...messages.title} />
          </Typography>
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
