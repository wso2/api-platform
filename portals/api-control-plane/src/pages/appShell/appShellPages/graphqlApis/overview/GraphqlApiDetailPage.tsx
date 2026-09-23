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

import { useMemo } from 'react';
import {
  Box,
  Breadcrumbs,
  Button,
  Card,
  Chip,
  IconButton,
  Link,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Copy, Edit, Lock, Rocket } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink, useParams } from 'react-router-dom';

import { useGateways, type Gateway } from '@/api/resources/gateways';
import { useGraphQLApi, useGraphQLApiSdl } from '@/api/resources/graphqlApis';
import { useDeployments } from '@/api/resources/graphqlApis/deployments';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useFormatters } from '@/i18n/useFormatters';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { versionLabel } from '@/utils/versionLabel';
import { GraphqlIcon } from '../../apis/create/uiConfig';
import { GraphqlOverviewTab } from './GraphqlOverviewTab';
import { GraphqlProgressBanner } from './GraphqlProgressBanner';

const messages = defineMessages({
  context: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.context.label',
    defaultMessage: 'Context:',
    description: 'Label for the API base path shown in the API detail header, e.g. "/orders".',
  },
  copyContext: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.copyContext',
    defaultMessage: 'Copy API context',
    description: 'Accessible label for the button that copies the API context.',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.created.label',
    defaultMessage: 'Created',
    description: 'Label before the API creation time in the API detail header.',
  },
  unknownCreator: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.unknownCreator',
    defaultMessage: '—',
  },
  by: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.by.label',
    defaultMessage: 'by',
    description: 'Label between the API creation time and creator.',
  },
  deployToGateway: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.deployToGateway',
    defaultMessage: 'Deploy to Gateway',
    description: 'Button on the API overview header that opens the API\'s deployment page.',
  },
  editApi: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.editApi',
    defaultMessage: 'Edit API details',
    description: 'Accessible label and tooltip for the button beside the API name, which opens the edit page.',
  },
  gatewayManaged: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.gatewayManaged',
    defaultMessage: 'Gateway-managed',
    description: 'Chip marking an API that was discovered from a gateway and cannot be edited here.',
  },
  gatewayManagedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.gatewayManagedHint',
    defaultMessage: 'Discovered from a data-plane gateway, so it is read-only in this console.',
    description: 'Tooltip explaining the gateway-managed chip.',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.placeholder',
    defaultMessage: 'No description',
    description: 'Shown in place of the API description when the API has none. Rendered in italics as an absence, not as a value.',
  },
  typeChip: {
    id: 'api.create.apiType.graphQl.title',
    defaultMessage: 'GraphQL API',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlApiDetailPage.notFound',
    defaultMessage: 'GraphQL API not found',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
  breadcrumbApis: {
    id: 'apiListPage.title',
    defaultMessage: 'APIs',
    description: 'Page title for the API list page',
  },
});

const AVATAR_SIZE = 72;

function DescriptionField({ description }: { description: string }) {
  if (!description) {
    return (
      <Typography color="text.disabled" sx={{ fontStyle: 'italic' }} variant="body2">
        <FormattedMessage {...messages.descriptionPlaceholder} />
      </Typography>
    );
  }
  return (
    <Typography
      color="text.secondary"
      sx={{
        display: '-webkit-box',
        fontSize: '0.875rem',
        fontWeight: 400,
        lineHeight: 1.5,
        maxWidth: 860,
        minWidth: 0,
        opacity: 0.8,
        overflow: 'hidden',
        WebkitBoxOrient: 'vertical',
        WebkitLineClamp: 2,
      }}
      variant="body2"
    >
      {description}
    </Typography>
  );
}

/**
 * Overview page for a GraphQL API. No `ScopeGate`: this route lives outside
 * `ConsoleScopeProvider`'s REST-only api-scope matching (see `graphqlApiPath`),
 * so it guards on its own route param instead of `isApiScope`. The
 * organization/project tier still resolves generically through
 * `useConsoleScope`/`useApiScope` — only the API tier is read from
 * `useParams` directly.
 */
export function GraphqlApiDetailPage() {
  const { params } = useConsoleScope();
  const { graphqlApiHandler } = useParams();
  const intl = useIntl();
  const { dateTime, relativeTime } = useFormatters();

  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const sdlQuery = useGraphQLApiSdl(graphqlApiHandler);
  const gatewaysQuery = useGateways();
  const deploymentsQuery = useDeployments(graphqlApiHandler);

  const deployedGateways = useMemo((): Gateway[] => {
    const gateways = gatewaysQuery.data?.list ?? [];
    const deployments = deploymentsQuery.data?.list ?? [];
    const latestByGateway = new Map<string, number>();
    deployments
      .filter((deployment) => deployment.status === 'DEPLOYED')
      .forEach((deployment) => {
        const time = new Date(deployment.createdAt || 0).getTime();
        const current = latestByGateway.get(deployment.gatewayId);
        if (current === undefined || time > current) {
          latestByGateway.set(deployment.gatewayId, time);
        }
      });
    return gateways
      .filter((gateway) => latestByGateway.has(gateway.id ?? ''))
      .sort(
        (a, b) => (latestByGateway.get(b.id ?? '') || 0) - (latestByGateway.get(a.id ?? '') || 0),
      );
  }, [gatewaysQuery.data, deploymentsQuery.data]);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.notFound)} />;

  const api = apiQuery.data;
  const displayName = api.displayName || graphqlApiHandler;
  const context = api.context || '/';
  const orgHandle = params.orgHandle ?? '';
  const projectHandler = params.projectHandler ?? '';
  const deployPath = routes.graphqlApiDeploy(orgHandle, projectHandler, graphqlApiHandler);
  const editPath = routes.graphqlApiEdit(orgHandle, projectHandler, graphqlApiHandler);

  return (
    <>
      <Breadcrumbs sx={{ mb: 2 }}>
        <Link component={RouterLink} to={routes.apis(orgHandle, projectHandler)} underline="hover">
          <FormattedMessage {...messages.breadcrumbApis} />
        </Link>
        <Typography color="text.primary" noWrap variant="body2">
          {displayName}
        </Typography>
      </Breadcrumbs>

      <Card sx={{ mb: 3 }}>
        <Box
          sx={{
            display: 'flex',
            flexDirection: { sm: 'row', xs: 'column' },
            alignItems: 'flex-start',
            gap: 3,
            justifyContent: 'space-between',
            p: { sm: 2.5, xs: 2 },
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2, minWidth: 0 }}>
            <Box
              sx={{
                alignItems: 'center',
                bgcolor: 'action.hover',
                borderRadius: 2,
                display: 'flex',
                flexShrink: 0,
                height: AVATAR_SIZE,
                justifyContent: 'center',
                width: AVATAR_SIZE,
              }}
            >
              <GraphqlIcon />
            </Box>

            <Stack spacing={1.5} sx={{ minWidth: 0 }}>
              <Stack alignItems="flex-start" spacing={1}>
                <Stack alignItems="center" direction="row" spacing={1} sx={{ minWidth: 0 }}>
                  <Tooltip title={displayName}>
                    <Typography noWrap sx={{ fontWeight: 700, lineHeight: 1.2 }} variant="h3">
                      {displayName}
                    </Typography>
                  </Tooltip>
                  {api.readOnly && (
                    <Tooltip title={intl.formatMessage(messages.gatewayManagedHint)}>
                      <Chip
                        icon={<Lock size={12} />}
                        label={intl.formatMessage(messages.gatewayManaged)}
                        size="small"
                        sx={{ flexShrink: 0, typography: 'caption' }}
                        variant="outlined"
                      />
                    </Tooltip>
                  )}
                </Stack>
                <Stack alignItems="center" direction="row" spacing={0.75}>
                  <Chip label={versionLabel(api.version)} size="small" variant="outlined" />
                  <Chip label={intl.formatMessage(messages.typeChip)} size="small" />
                </Stack>
              </Stack>

              <DescriptionField description={api.description?.trim() ?? ''} />

              <Stack
                alignItems="center"
                direction="row"
                divider={<Box sx={{ bgcolor: 'divider', height: 24, width: '1px' }} />}
                spacing={1.5}
                sx={{ flexWrap: 'wrap', rowGap: 1 }}
              >
                {api.createdAt && (
                  <Tooltip title={dateTime(api.createdAt)}>
                    <Stack alignItems="center" direction="row" spacing={0.5}>
                      <Box
                        sx={{
                          alignItems: 'center',
                          color: 'text.secondary',
                          display: 'flex',
                          flexShrink: 0,
                          opacity: 0.75,
                        }}
                      >
                        <Clock color="currentColor" size={16} />
                      </Box>
                      <Typography color="text.secondary" sx={{ opacity: 0.75 }} variant="body2">
                        <FormattedMessage {...messages.created} />
                      </Typography>
                      <Typography variant="body2">{relativeTime(api.createdAt)}</Typography>
                      <Typography color="text.secondary" sx={{ opacity: 0.75 }} variant="body2">
                        <FormattedMessage {...messages.by} />
                      </Typography>
                      <Typography variant="body2">
                        {api.createdBy || intl.formatMessage(messages.unknownCreator)}
                      </Typography>
                    </Stack>
                  </Tooltip>
                )}
                <Stack alignItems="center" direction="row" spacing={1}>
                  <Typography
                    color="text.secondary"
                    sx={{ fontWeight: 400, opacity: 0.75 }}
                    variant="body2"
                  >
                    <FormattedMessage {...messages.context} />
                  </Typography>
                  <Typography noWrap variant="body2">
                    {context}
                  </Typography>
                  <Tooltip title={intl.formatMessage(messages.copyContext)}>
                    <IconButton
                      aria-label={intl.formatMessage(messages.copyContext)}
                      onClick={() => void navigator.clipboard?.writeText(context)}
                      size="small"
                    >
                      <Copy size={16} />
                    </IconButton>
                  </Tooltip>
                </Stack>
              </Stack>
            </Stack>
          </Box>

          <Stack
            alignItems="center"
            direction="row"
            spacing={1.5}
            sx={{ alignSelf: { sm: 'flex-start', xs: 'stretch' }, flexShrink: 0 }}
          >
            {!api.readOnly && (
              <Tooltip title={intl.formatMessage(messages.editApi)}>
                <IconButton
                  aria-label={intl.formatMessage(messages.editApi)}
                  component={RouterLink}
                  sx={{
                    border: '1px solid',
                    borderColor: 'divider',
                    height: 52,
                    width: 52,
                  }}
                  to={editPath}
                >
                  <Edit size={22} />
                </IconButton>
              </Tooltip>
            )}
            <Button
              component={RouterLink}
              startIcon={<Rocket size={18} />}
              sx={{ flexShrink: 0 }}
              to={deployPath}
              variant="contained"
            >
              <FormattedMessage {...messages.deployToGateway} />
            </Button>
          </Stack>
        </Box>
        <GraphqlProgressBanner deployed={deployedGateways.length > 0} />
      </Card>

      <GraphqlOverviewTab
        api={api}
        deployedGateways={deployedGateways}
        deployments={deploymentsQuery.data?.list ?? []}
        sdl={sdlQuery.data}
      />
    </>
  );
}
