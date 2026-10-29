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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';
import { Avatar, Chip, IconButton, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Clock, Edit } from '@wso2/oxygen-ui-icons-react';

import { formatRelativeTime } from '../../../../contexts/llmProvider';
import OverviewDescription from '../../../../Components/OverviewDescription/OverviewDescription';
import { OVERVIEW_AVATAR_SX } from '../../../../utils/overviewHeaderStyles';
import type { MCPServer } from '../../../../utils/types';

const messages = defineMessages({
  context: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.identity.context',
    defaultMessage: 'Context :',
  },
  updated: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.identity.updated',
    defaultMessage: 'Last updated :',
  },
  edit: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.identity.edit',
    defaultMessage: 'Edit MCP Proxy',
  },
});

type OverviewIdentityProps = {
  server: MCPServer;
  /** The monogram on the avatar. */
  initials: string;
};

/** The left side of the MCP proxy Overview's header: who the proxy is, shared by both headers. */
export default function OverviewIdentity({ server, initials }: OverviewIdentityProps) {
  const intl = useIntl();

  return (
    <Stack direction="row" spacing={2.5} sx={{ flex: '1 1 380px', minWidth: 0 }}>
      <Avatar
        sx={OVERVIEW_AVATAR_SX}
        variant="rounded"
      >
        {initials}
      </Avatar>
      <Stack spacing={1.5} sx={{ minWidth: 0 }}>
        <Stack alignItems="center" direction="row" flexWrap="wrap" spacing={1.5}>
          <Typography sx={{ overflowWrap: 'anywhere' }} variant="h3">
            {server.displayName}
          </Typography>
          <Chip color="primary" label={server.version} size="small" variant="outlined" />
          {/* Edit page (name/version/context/description). Enabled even for gateway-created
              MCP proxies — the page keeps the runtime fields read-only and allows only the
              description. */}
          <Tooltip title={intl.formatMessage(messages.edit)}>
            <IconButton aria-label={intl.formatMessage(messages.edit)} component={RouterLink} to="edit">
              <Edit size={18} />
            </IconButton>
          </Tooltip>
        </Stack>
        <OverviewDescription text={server.description ?? ''} />
        <Stack spacing={0.25}>
          <Stack alignItems="center" direction="row" gap={2}>
            <Typography color="text.secondary" variant="caption">
              <FormattedMessage {...messages.context} />
            </Typography>
            <Typography variant="body2">{server.context || '/'}</Typography>
          </Stack>
          <Tooltip title={server.updatedAt ? new Date(server.updatedAt).toLocaleString() : ''}>
            <Stack alignItems="center" direction="row" spacing={0.75}>
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.updated} />
              </Typography>
              <Clock size={14} />
              <Typography color="text.secondary" variant="caption">
                {formatRelativeTime(server.updatedAt)}
              </Typography>
            </Stack>
          </Tooltip>
        </Stack>
      </Stack>
    </Stack>
  );
}
