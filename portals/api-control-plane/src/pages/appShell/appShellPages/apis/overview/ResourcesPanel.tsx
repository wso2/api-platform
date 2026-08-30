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

import { Box, Chip, Stack, Typography } from '@wso2/oxygen-ui';
import { PackageSearch } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { RestApi } from '@/api/resources/restApis';
import { methodColor } from '../utils/developEdit';

const messages = defineMessages({
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.empty',
    defaultMessage: 'No available resources.',
    description: 'Shown in place of the operation list when the API definition exposes none.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.title',
    defaultMessage: 'Resources',
    description:
      "Heading of the panel listing the API's operations (its OpenAPI paths). A noun, not a command.",
  },
});

function EmptyResources({ label }: { label: string }) {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={1}
      sx={{ color: 'text.secondary', py: 4, textAlign: 'center' }}
    >
      <PackageSearch size={40} strokeWidth={1.25} />
      <Typography color="text.secondary" variant="body2">
        {label}
      </Typography>
    </Stack>
  );
}

/**
 * Left panel of the Overview tab (ai-workspace "OpenAPI Resources"): the
 * API's operations in a scrollable bordered box.
 */
export function ResourcesPanel({ api }: { api: RestApi }) {
  const intl = useIntl();
  // `operations` is optional on the spec's `RESTAPI`.
  const operations = api.operations ?? [];

  return (
    <Box sx={{ flex: 1, minWidth: 0 }}>
      <Typography sx={{ fontWeight: 600, mb: 0.5 }} variant="h6">
        <FormattedMessage {...messages.title} />
      </Typography>
      <Box
        sx={{
          bgcolor: 'background.paper',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          maxHeight: { md: 520, xs: 320 },
          overflowY: 'auto',
          px: 2,
          py: 1,
        }}
      >
        {operations.length === 0 ? (
          <EmptyResources label={intl.formatMessage(messages.empty)} />
        ) : (
          operations.map((operation) => (
            <Stack
              alignItems="center"
              direction="row"
              key={`${operation.request.method}-${operation.request.path}`}
              spacing={1.5}
              sx={{
                borderBottom: '1px solid',
                borderColor: 'divider',
                py: 1.5,
                '&:last-child': { borderBottom: 'none' },
              }}
            >
              <Chip
                color={methodColor(operation.request.method)}
                label={operation.request.method}
                size="small"
                sx={{ fontWeight: 600, minWidth: 68 }}
              />
              <Box sx={{ minWidth: 0 }}>
                <Typography noWrap sx={{ fontFamily: 'monospace', fontSize: 13.5 }}>
                  {operation.request.path}
                </Typography>
                {(operation.description || operation.name) && (
                  <Typography color="text.secondary" noWrap variant="caption">
                    {operation.description || operation.name}
                  </Typography>
                )}
              </Box>
            </Stack>
          ))
        )}
      </Box>
    </Box>
  );
}
