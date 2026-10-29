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

import React, { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { Box, Button, PageTitle, SearchBar, Stack, Typography } from '@wso2/oxygen-ui';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import { usePublishProject } from '../../../../hooks/usePublishProject';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { getHttpStatus } from '../../../../utils/apiError';
import { logger } from '../../../../utils/logger';
import type { MCPServer, PublicationSummaryItem } from '../../../../utils/types';

import { AppPage } from './AppPage';
import { HubPublicationCard } from './HubPublicationCard';
import { isListedOnPortal } from './publicationDisplay';
import { EmptyState, ErrorState, LoadingState } from './PublishStates';

const messages = defineMessages({
  loading: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.loading',
    defaultMessage: 'Loading MCP Hubs',
  },
  forbiddenTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.forbiddenTitle',
    defaultMessage: 'You don’t have permission',
  },
  forbiddenMessage: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.forbiddenMessage',
    defaultMessage:
      'You don’t have permission to publish MCP proxies to MCP Hubs. Contact your organization admin to request access.',
  },
  emptyTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.emptyTitle',
    defaultMessage: 'No MCP Hubs Available',
  },
  emptyDescription: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.emptyDescription',
    defaultMessage: 'Add an MCP Hub for this organization before publishing an MCP proxy to it.',
  },
  search: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.search',
    defaultMessage: 'Search MCP Hubs',
  },
  noMatchTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.noMatchTitle',
    defaultMessage: 'No matching MCP Hubs',
  },
  noMatchDescription: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.noMatchDescription',
    defaultMessage: 'Try a different MCP Hub name or clear the search.',
  },
  back: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.back',
    defaultMessage: 'Back to MCP Proxy',
  },
  title: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.title',
    defaultMessage: 'Publish {name}',
  },
  subtitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.subtitle',
    defaultMessage: 'Choose the MCP Hub to publish this MCP proxy to.',
  },
  publishedCount: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.hubs.publishedCount',
    defaultMessage: 'Published to {published} of {total} {total, plural, one {MCP Hub} other {MCP Hubs}}',
  },
});

/** Every MCP Hub of the organization with this proxy's status on it. No hubs is an empty state, not an error. */
export default function McpPublishHubs(): React.JSX.Element {
  const intl = useIntl();
  const { serverId = '' } = useParams<{ serverId: string }>();
  const { currentOrganization, organizationId, overviewPath, publishPath } = usePublishProject();

  const [server, setServer] = useState<MCPServer | null>(null);
  const [publications, setPublications] = useState<PublicationSummaryItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<Error | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [search, setSearch] = useState('');

  useEffect(() => {
    if (!organizationId || !serverId) return;
    let cancelled = false;
    void (async () => {
      setIsLoading(true);
      setLoadError(null);
      try {
        const [proxy, summary] = await Promise.all([
          mcpProxiesApis.getMCPServer(serverId, PLATFORM_API_BASE_URL),
          mcpProxiesApis.listMcpProxyApiPortalPublications(serverId, PLATFORM_API_BASE_URL),
        ]);
        if (cancelled) return;
        setServer(proxy);
        setPublications(summary.list);
      } catch (error) {
        logger.error('Failed to load MCP Hubs for publishing:', error);
        if (!cancelled) setLoadError(error instanceof Error ? error : new Error(String(error)));
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [organizationId, serverId, reloadKey]);

  const publishedCount = publications.filter(({ status }) => isListedOnPortal(status)).length;
  const visiblePublications = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return publications;
    return publications.filter((publication) =>
      [publication.apiPortalName, publication.apiPortalDescription, publication.apiPortalUrl]
        .filter(Boolean)
        .some((text) => text?.toLowerCase().includes(term))
    );
  }, [publications, search]);

  if (isLoading) {
    return (
      <AppPage>
        <LoadingState label={intl.formatMessage(messages.loading)} />
      </AppPage>
    );
  }

  if (loadError || !server) {
    return (
      <AppPage>
        {loadError && getHttpStatus(loadError) === 403 ? (
          <ErrorState
            title={intl.formatMessage(messages.forbiddenTitle)}
            message={intl.formatMessage(messages.forbiddenMessage)}
          />
        ) : (
          <ErrorAlert error={loadError} onRetry={() => setReloadKey((key) => key + 1)} />
        )}
      </AppPage>
    );
  }

  return (
    <AppPage>
      <Button
        component={Link}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ alignSelf: 'flex-start' }}
        to={overviewPath(serverId)}
      >
        <FormattedMessage {...messages.back} />
      </Button>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} values={{ name: server.displayName }} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.publishedCount} values={{ published: publishedCount, total: publications.length }} />
          </Typography>
        </PageTitle.Actions>
      </PageTitle>

      {publications.length === 0 ? (
        <EmptyState
          title={intl.formatMessage(messages.emptyTitle)}
          description={intl.formatMessage(messages.emptyDescription)}
        />
      ) : (
        <Stack spacing={3}>
          <SearchBar
            fullWidth
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.search)}
            value={search}
          />

          {visiblePublications.length === 0 ? (
            <EmptyState
              title={intl.formatMessage(messages.noMatchTitle)}
              description={intl.formatMessage(messages.noMatchDescription)}
            />
          ) : (
            <Box
              sx={{
                display: 'grid',
                gap: 2.5,
                gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' },
                // Allow cards to shrink so long names do not widen the grid.
                '& > *': { minWidth: 0 },
              }}
            >
              {visiblePublications.map((publication) => (
                <HubPublicationCard
                  key={publication.apiPortalId}
                  mcpProxyId={serverId}
                  orgHandle={currentOrganization?.handle ?? ''}
                  publication={publication}
                  publishPath={(apiPortalId) =>
                    publishPath(`/${encodeURIComponent(serverId)}/portals/${encodeURIComponent(apiPortalId)}`)
                  }
                />
              ))}
            </Box>
          )}
        </Stack>
      )}
    </AppPage>
  );
}
