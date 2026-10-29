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

import type { MouseEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';
import { Button, Card, IconButton, Stack } from '@wso2/oxygen-ui';
import { Trash2 } from '@wso2/oxygen-ui-icons-react';

import { NO_PERMISSION_TOOLTIP, SCOPES } from '../../../../auth/permissions';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import type { MCPServer } from '../../../../utils/types';
import { DisabledActionTooltip } from '../../../../utils/readOnlyArtifacts';
import { overviewButtonSx } from '../../../../utils/overviewHeaderStyles';
import OverviewIdentity from './OverviewIdentity';

const messages = defineMessages({
  deleteLabel: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.hubHeader.delete',
    defaultMessage: 'Delete {name}',
  },
  viewDeployments: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.viewDeployments',
    defaultMessage: 'View Deployments',
  },
  deployToGateway: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.deployToGateway',
    defaultMessage: 'Deploy to Gateway',
  },
  publishToHub: {
    id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.overview.hubHeader.publishToHub',
    defaultMessage: 'Publish to MCP Hub',
  },
});

type HubOverviewHeaderProps = {
  server: MCPServer;
  /** Passed in so both headers show the same monogram. */
  initials: string;
  isReadOnlyServer: boolean;
  /** Why the proxy cannot be deleted right now, or `null` when it can. */
  deleteBlockedReason: string | null;
  /** Where the publish flow starts: this proxy's MCP Hub list. */
  publishUrl: string;
  onDeleteClick: () => void;
  /** Stops a link from leaving the page while it has unsaved changes. */
  onBlockedNavigation: (event: MouseEvent<HTMLElement>) => void;
};

/** The Overview header where publishing goes to MCP Hubs. Scopes are checked here; page state is passed in. */
export default function HubOverviewHeader({
  server,
  initials,
  isReadOnlyServer,
  deleteBlockedReason,
  publishUrl,
  onDeleteClick,
  onBlockedNavigation,
}: HubOverviewHeaderProps) {
  const intl = useIntl();
  const { hasPermission } = useAppAuth();
  const canDelete = hasPermission(SCOPES.MCP_PROXY_DELETE);
  const canDeploy = hasPermission(SCOPES.MCP_PROXY_DEPLOYMENT_CREATE);
  const canViewDeployments = hasPermission(SCOPES.MCP_PROXY_DEPLOYMENT_READ);
  const canOpenPublishPage = hasPermission(SCOPES.MCP_PROXY_API_PORTAL_MANAGE);

  return (
    <Card sx={{ overflow: 'hidden' }}>
      <Stack alignItems="flex-start" direction="row" flexWrap="wrap" gap={3} justifyContent="space-between" sx={{ p: 3 }}>
        <OverviewIdentity initials={initials} server={server} />

        <Stack alignItems="flex-end" justifyContent="space-between" spacing={2} sx={{ alignSelf: 'stretch' }}>
          <Stack alignItems="flex-end" spacing={1.5}>
            {/* For gateway-created (read-only) proxies, and for users holding only the
                deployment-read scope, the deployments remain viewable (deploy/redeploy/
                restore/undeploy are disabled on the page itself), so the button navigates
                but is relabelled "View Deployments". Without even deployment-read there is
                nothing to see, so it is disabled. */}
            <DisabledActionTooltip disabled={!canViewDeployments} title={NO_PERMISSION_TOOLTIP}>
              <Button
                component={RouterLink}
                disabled={!canViewDeployments}
                onClick={isReadOnlyServer ? undefined : onBlockedNavigation}
                sx={overviewButtonSx()}
                to="deploy"
                variant="contained"
              >
                <FormattedMessage
                  {...(isReadOnlyServer || !canDeploy ? messages.viewDeployments : messages.deployToGateway)}
                />
              </Button>
            </DisabledActionTooltip>
            <DisabledActionTooltip disabled={!canOpenPublishPage} title={NO_PERMISSION_TOOLTIP}>
              <Button
                component={RouterLink}
                disabled={!canOpenPublishPage}
                onClick={onBlockedNavigation}
                sx={overviewButtonSx()}
                to={publishUrl}
                variant="outlined"
              >
                <FormattedMessage {...messages.publishToHub} />
              </Button>
            </DisabledActionTooltip>
          </Stack>
          <DisabledActionTooltip
            disabled={!canDelete || Boolean(deleteBlockedReason)}
            title={!canDelete ? NO_PERMISSION_TOOLTIP : (deleteBlockedReason ?? undefined)}
          >
            <IconButton
              aria-label={intl.formatMessage(messages.deleteLabel, { name: server.displayName })}
              color="error"
              data-cyid="delete-mcp-proxy-button"
              disabled={!canDelete || Boolean(deleteBlockedReason)}
              onClick={onDeleteClick}
            >
              <Trash2 size={20} />
            </IconButton>
          </DisabledActionTooltip>
        </Stack>
      </Stack>
    </Card>
  );
}
