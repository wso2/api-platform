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
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Clock, Copy, Edit, Lock, Rocket } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';

import { useRestApi } from '@/api/resources/restApis';
import type { Gateway } from '@/api/resources/gateways';
import { useDeployments } from '@/api/resources/restApis/deployments';
import { useRestApiGateways } from '@/api/resources/restApis/apiGateways/apiGateways.hooks';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useFormatters } from '@/i18n/useFormatters';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiKindChip, VersionChip } from '../listing/components/RestApiChips';
import { apiInitials } from '../utils/restApiDisplay';
import { OverviewTab } from './OverviewTab';
import { ProgressBanner } from './ProgressBanner';

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
    description: "Button on the API overview header that opens the API's deployment page.",
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.placeholder',
    defaultMessage: 'No description',
    description:
      'Shown in place of the API description when the API has none. Rendered in italics as an absence, not as a value.',
  },
  editApi: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.editApi',
    defaultMessage: 'Edit API details',
    description:
      'Accessible label and tooltip for the button beside the API name, which opens the edit page.',
  },
  gatewayManaged: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.gatewayManaged',
    defaultMessage: 'Gateway-managed',
    description:
      'Chip marking an API that was discovered from a gateway and cannot be edited here.',
  },
  gatewayManagedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.gatewayManagedHint',
    defaultMessage: 'Discovered from a data-plane gateway, so it is read-only in this console.',
    description: 'Tooltip explaining the gateway-managed chip.',
  },
  transports: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.transports.label',
    defaultMessage: 'Transports',
    description: 'Label for the protocols an API is exposed over (HTTP, HTTPS).',
  },
});

/** Edge of the square kind tile, the monogram inside it, and the fallback icon. */
const AVATAR_SIZE = 72;
const AVATAR_FONT_SIZE = 32;
const AVATAR_ICON_SIZE = 32;

/**
 * The API description, read-only.
 *
 * Clamped to two lines, with an italic placeholder standing in for an absent
 * description so the row reads as an absence rather than as a value. Editing
 * lives on the dedicated edit page, reached from the button beside the title.
 */
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

// No `ScopeGate`: this page is the API tier of the sidebar's Overview item, which
// degrades to a shallower tier rather than linking here without an API.
export function ApiDetailPage() {
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);
  const apiGatewaysQuery = useRestApiGateways(apiQuery.data?.id);
  const deploymentsQuery = useDeployments(apiQuery.data?.id);
  const { dateTime, relativeTime } = useFormatters();
  const intl = useIntl();
  const deployedGateways = useMemo((): Gateway[] => {
    const gateways = apiGatewaysQuery.data?.list ?? [];
    const deployments = deploymentsQuery.data?.list ?? [];
    const latestByGateway = new Map<string, number>();
    deployments
      .filter((deployment) => deployment.status === 'DEPLOYED')
      .forEach((deployment) => {
        const time = new Date(deployment.createdAt || 0).getTime();
        const current = latestByGateway.get(deployment.gatewayId);
        if (current === undefined || time > current)
          latestByGateway.set(deployment.gatewayId, time);
      });
    return gateways
      .filter((gateway) => gateway.isDeployed || latestByGateway.has(gateway.id ?? ''))
      .sort(
        (a, b) => (latestByGateway.get(b.id ?? '') || 0) - (latestByGateway.get(a.id ?? '') || 0),
      );
  }, [apiGatewaysQuery.data, deploymentsQuery.data]);

  if (apiQuery.isLoading) return <LoadingState label="Loading API" />;
  if (apiQuery.error || !apiQuery.data) {
    return <ErrorState title="API not found" />;
  }

  const api = apiQuery.data;
  const restApiId = api.id ?? params.apiHandler ?? '';

  // The page is the API tier of the sidebar's Overview item, so it only ever
  // mounts with all three handles in the URL; `apiPath` degrades to the
  // scope-less alias for anything still missing.
  const deployPath = routes.apiDeploy(
    params.orgHandle ?? '',
    params.projectHandler ?? null,
    params.apiHandler ?? null,
  );

  // Same reasoning: the page only mounts fully scoped, so the edit page's path
  // is always the fully-scoped one.
  const editPath = routes.apiEdit(
    params.orgHandle ?? '',
    params.projectHandler ?? '',
    params.apiHandler ?? '',
  );

  const displayName = api.displayName || restApiId;
  const context = api.context || '/';
  return (
    <>
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
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 2,
              minWidth: 0,
            }}
          >
            <Avatar
              sx={{
                bgcolor: 'primary.light',
                color: 'primary.contrastText',
                flexShrink: 0,
                height: AVATAR_SIZE,
                width: AVATAR_SIZE,
                fontSize: AVATAR_FONT_SIZE,
              }}
              variant="rounded"
            >
              {apiInitials(displayName) || <Boxes size={AVATAR_ICON_SIZE} />}
            </Avatar>

            <Stack spacing={1.5} sx={{ minWidth: 0 }}>
              {/* Identity: name, version, lifecycle, and whether this console
                  may edit the API at all. */}
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
                  <VersionChip version={api.version} />
                  <ApiKindChip kind={api.kind} />
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
        <ProgressBanner api={api} deployed={deployedGateways.length > 0} />
      </Card>

      <OverviewTab
        api={api}
        deployedGateways={deployedGateways}
        deployments={deploymentsQuery.data?.list ?? []}
      />
    </>
  );
}
