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

import { useState } from 'react';
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
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Copy } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import { useNotifications } from '@/components/Notifications';
import { gatewayEndpoint } from '../../gateways/utils/gatewayDisplay';

const messages = defineMessages({
  copy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.copy',
    defaultMessage: 'Copy URL',
    description:
      'Tooltip and accessible label of the button that copies the invoke URL to the clipboard.',
  },
  copyFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.copyFailed',
    defaultMessage: 'Failed to copy URL.',
    description: 'Toast shown when the browser refuses the clipboard write.',
  },
  copySucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.copySucceeded',
    defaultMessage: 'URL copied to clipboard.',
    description: 'Toast confirming the invoke URL was copied.',
  },
  description: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.description',
    defaultMessage: 'Change the gateway to generate the gateway specific invoke URL.',
    description:
      'Explains that the URL below is per-gateway, so picking another gateway rewrites it.',
  },
  gatewaysLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.gatewaysLabel',
    defaultMessage: 'Gateways',
    description:
      'Label of the picker choosing which deployed gateway the invoke URL is built for.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.title',
    defaultMessage: 'Invoke URL',
    description:
      'Heading of the section giving the address clients call to reach this API. A noun, not a command.',
  },
  urlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.InvokeUrlPanel.urlLabel',
    defaultMessage: 'URL',
    description: 'Label of the read-only field holding the generated invoke URL.',
  },
});

/** `{endpoint}{context}` with a scheme ensured and slashes normalised. */
export const buildInvokeUrl = (endpoint: string, context?: string): string => {
  const trimmedEndpoint = endpoint.trim();
  if (!trimmedEndpoint) return '';
  const base = /^https?:\/\//i.test(trimmedEndpoint)
    ? trimmedEndpoint.replace(/\/+$/, '')
    : `https://${trimmedEndpoint.replace(/\/+$/, '')}`;
  const trimmedContext = (context || '/').trim();
  const path = trimmedContext.startsWith('/') ? trimmedContext : `/${trimmedContext}`;
  return `${base}${path}`;
};

type InvokeUrlPanelProps = {
  /** Gateways the API is currently deployed on, most recent first. */
  gateways: Gateway[];
  context?: string;
};

/**
 * Invoke URL section of the Overview tab (ai-workspace): pick a deployed
 * gateway, get the gateway-specific invoke URL with a copy affordance.
 */
export function InvokeUrlPanel({ gateways, context }: InvokeUrlPanelProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const [selectedGatewayId, setSelectedGatewayId] = useState(gateways[0]?.id || '');

  const selectedGateway =
    gateways.find((gateway) => gateway.id === selectedGatewayId) ?? gateways[0];
  // The spec models the address as an `endpoints` list; `gatewayEndpoint` picks
  // the one the console builds URLs from, as the whole gateway UI does.
  const invokeUrl = selectedGateway
    ? buildInvokeUrl(gatewayEndpoint(selectedGateway), context)
    : '';

  const copyUrl = () => {
    if (!invokeUrl) return;
    navigator.clipboard
      ?.writeText(invokeUrl)
      .then(() => notify(intl.formatMessage(messages.copySucceeded), 'success'))
      .catch(() => notify(intl.formatMessage(messages.copyFailed), 'error'));
  };

  return (
    <Stack spacing={1.5}>
      <Box>
        <Typography sx={{ fontWeight: 600, mb: 0.5 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.description} />
        </Typography>
      </Box>
      <Grid alignItems="flex-end" container spacing={1}>
        <Grid size={{ md: 4, xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel>
              <FormattedMessage {...messages.gatewaysLabel} />
            </FormLabel>
            <Select
              disabled={gateways.length === 0}
              onChange={(event) => setSelectedGatewayId(String(event.target.value))}
              size="small"
              value={selectedGateway?.id || ''}
            >
              {gateways.map((gateway) => (
                <MenuItem key={gateway.id} value={gateway.id ?? ''}>
                  {gateway.displayName || gateway.id}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>
        <Grid size={{ md: 8, xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel>
              <FormattedMessage {...messages.urlLabel} />
            </FormLabel>
            <TextField
              fullWidth
              size="small"
              slotProps={{
                input: {
                  readOnly: true,
                  endAdornment: (
                    <InputAdornment position="end">
                      <Tooltip arrow title={intl.formatMessage(messages.copy)}>
                        <span>
                          <IconButton
                            aria-label={intl.formatMessage(messages.copy)}
                            disabled={!invokeUrl}
                            onClick={copyUrl}
                            size="small"
                          >
                            <Copy size={16} />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </InputAdornment>
                  ),
                },
              }}
              value={invokeUrl}
            />
          </FormControl>
        </Grid>
      </Grid>
    </Stack>
  );
}
