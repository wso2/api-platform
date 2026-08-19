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
import { Avatar, Box, Button, Card, Chip, PageContent, Stack, Tab, Tabs, Typography } from '@wso2/oxygen-ui';

import { useApiDetail } from '../../../../api/hooks/useMvpQueries';
import { ErrorState, LoadingState } from '../../../../components/StateViews';
import { DocumentsTab } from './develop/DocumentsTab';
import { PolicyTab } from './develop/PolicyTab';
import { RoutingTab } from './develop/RoutingTab';
import { OverviewTab } from './overview/OverviewTab';
import { FormattedMessage } from 'react-intl';

export function ApiDetailPage() {
  const detailQuery = useApiDetail();
  const [tab, setTab] = useState(0);

  if (detailQuery.isLoading) return <LoadingState label="Loading API" />;
  if (detailQuery.error || !detailQuery.data) {
    return <ErrorState title="API not found" />;
  }

  const detail = detailQuery.data;

  // Develop tab set mirrors the product: Overview, then Policy/Routing/Documents.
  const tabs = ['Overview', 'Policy', 'Routing', 'Documents'] as const;
  const active = tabs[tab] ?? 'Overview';

  const truncateProviderDisplayName = (
  name?: string | null,
  maxLength = 30
): string => {
  const normalizedName = name?.trim() ?? '';
  if (normalizedName.length <= maxLength) {
    return normalizedName;
  }

  return `${normalizedName.slice(0, maxLength).trim()}…`;
};

  return (
    <PageContent fullWidth>
      <Stack spacing={3} sx={{ mb: 3 }}>
      
         {/* Header card with editable fields */}
        <Card>
          <Box sx={{ p: 2 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'stretch',
                justifyContent: 'space-between',
                gap: 2,
              }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: 2,
                  minWidth: 0,
                }}
              >
                <Avatar
                  color="secondary"
                  sx={{
                    width: 70,
                    height: 70,
                    backgroundColor: 'primary.light',
                    color: 'primary.contrastText',
                    fontSize: 32,
                  }}
                >
                  {(detail.displayName || '\u2014').trim().slice(0, 2).toUpperCase()}
                </Avatar>

                <Box sx={{ minWidth: 0 }}>
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    flexWrap="wrap"
                  >
                    <Typography variant="h3">
                      {truncateProviderDisplayName(detail.displayName || '\u2014')}
                    </Typography>
                    <Chip
                      label={`${detail.version || '1.0'}`}
                      size="small"
                      variant="outlined"
                      color="primary"
                    />
                    {/* Edit page (name/version/context/description). Enabled even
                        for gateway-created proxies — the page keeps the runtime
                        fields read-only and allows only the description. */}
                      {/* <Tooltip
                        title={
                          canUpdateProxy ? 'Edit Proxy' : NO_PERMISSION_TOOLTIP
                        }
                      >
                        <Box component="span">
                          <IconButton
                            component={RouterLink}
                            to={`${proxiesPath}/${detail.id}/edit`}
                            size="small"
                            disabled={!canUpdateProxy}
                            sx={DISABLED_ACTION_SX}
                          >
                            <Edit size={16} />
                          </IconButton>
                        </Box>
                      </Tooltip> */}
                  </Stack>
                  <Stack spacing={0.1} sx={{ mt: 1 }}>
                    <Stack direction="row" alignItems="center" gap={2}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.context.label"
                          defaultMessage="Context :"
                        />
                      </Typography>
                      <Typography variant="body2">
                        {detail.context || '/'}
                      </Typography>
                    </Stack>
                    <Stack direction="row" alignItems="center" gap={2}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.last.updated"
                          defaultMessage={'Last updated :'}
                        />
                      </Typography>
                      <Typography variant="body2">
                        {detail.updatedAt}
                      </Typography>
                    </Stack>
                  </Stack>
                </Box>
              </Box>

              <Stack
                direction="column"
                justifyContent="space-between"
                alignItems="flex-end"
                sx={{ alignSelf: 'stretch' }}
              >
                {/* Deployments remain viewable for gateway-created proxies (deploy/
                    redeploy/restore/undeploy are disabled on the page itself), so the
                    button navigates but is relabelled "View Deployments". */}
                <Button
                  variant="contained"
                  // component={RouterLink}
                  // to={`${proxiesPath}/${detail.id}/deploy`}
                >
                  {/* {isReadOnlyProxy ? 'View Deployments' : 'Deploy to Gateway'} */}
                  Deploy to Gateway
                </Button>
                {/* <DisabledActionTooltip
                  disabled={!canDeleteProxy || Boolean(deleteBlockedReason)}
                  title={
                    !canDeleteProxy ? NO_PERMISSION_TOOLTIP : deleteBlockedReason
                  }
                >
                  <IconButton
                    color="error"
                    disabled={!canDeleteProxy || Boolean(deleteBlockedReason)}
                    onClick={() => setDeleteDialogOpen(true)}
                    aria-label="Delete proxy"
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </DisabledActionTooltip> */}
              </Stack>
            </Box>
          </Box>
        </Card>
      
      </Stack>

      <Tabs
        onChange={(_event, value) => setTab(value)}
        sx={{ mb: 3 }}
        value={tab}
      >
        {tabs.map((label) => (
          <Tab key={label} label={label} />
        ))}
      </Tabs>

      {active === 'Overview' && <OverviewTab detail={detail} />}
      {active === 'Policy' && <PolicyTab detail={detail} />}
      {active === 'Routing' && <RoutingTab detail={detail} />}
      {active === 'Documents' && <DocumentsTab />}
    </PageContent>
  );
}
