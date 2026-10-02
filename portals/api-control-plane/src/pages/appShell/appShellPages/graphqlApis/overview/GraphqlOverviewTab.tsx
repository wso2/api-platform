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
  Button,
  Card,
  Divider,
  Drawer,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  IconButton,
  Link,
  OutlinedInput,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Globe, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import { useUpdateGraphQLApi, type GraphQLApiDetail } from '@/api/resources/graphqlApis';
import type { Deployment } from '@/api/resources/graphqlApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { routes } from '@/routes/paths';
import { isValidUrl } from '../../apis/utils/developEdit';
import { GraphqlSchemaExplorer } from '../../apis/create/components/graphql/GraphqlSchemaExplorer';
import { DeployedGatewaysPanel } from '../../apis/overview/DeployedGatewaysPanel';
import { InvokeUrlPanel } from '../../apis/overview/InvokeUrlPanel';
import { resuppliedSchemaSource } from '../utils/graphqlApiMetadataUpdate';
import { GraphqlApiKeysPanel } from './GraphqlApiKeysPanel';

const messages = defineMessages({
  schemaTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlOverviewTab.schemaTitle',
    defaultMessage: 'Schema',
  },
  endpointTitle: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.endpoint',
    defaultMessage: 'Endpoint',
    description: 'Label above the URL that requests from this console are sent to. Shown in capitals by the layout, so translate it as ordinary words.',
  },
  endpointNotConfigured: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.notConfigured',
    defaultMessage: 'No endpoint configured',
  },
  // Everything below is the same id/text as REST's own `EndpointsPanel` —
  // same control, same copy, so same id rather than a duplicated translation.
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.cancel',
    defaultMessage: 'Cancel',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.close',
    defaultMessage: 'Close endpoint editor',
  },
  drawerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.drawerTitle',
    defaultMessage: 'Edit endpoint',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.edit',
    defaultMessage: 'Edit endpoint',
  },
  endpointUrlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.endpointLabel',
    defaultMessage: 'Endpoint URL',
  },
  endpointUrlRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.endpointRequired',
    defaultMessage: 'Enter a valid endpoint URL.',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.save',
    defaultMessage: 'Save',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.saved',
    defaultMessage: 'Backend endpoint updated.',
  },
  saveError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.saveError',
    defaultMessage: 'Unable to update the backend endpoint.',
  },
});

/**
 * Backend-endpoint display for a GraphQL API, structurally identical to
 * REST's `EndpointsPanel` (header + edit button, divider, icon-badge + URL
 * row, right-hand drawer to edit) — the two should read as the same control.
 * The one REST feature deliberately not carried over is the multi-endpoint
 * shape: a GraphQL API has exactly one endpoint, never several, so this
 * keeps GraphQL's own singular "Endpoint" title rather than REST's
 * "Endpoints".
 *
 * The save mutation differs from REST's `useUpdateRestApi` because
 * `UpdateGraphQLAPI`'s body is a multipart `{ metadata, sdlFile }` envelope
 * that must also resupply `schemaSource`/`sdl` faithfully — see
 * `resuppliedSchemaSource`'s doc comment, and `GraphqlApiEditPage`, which
 * applies the exact same rule for its own (broader) edit form.
 */
const EndpointPanel = ({ api, sdl }: { api: GraphQLApiDetail; sdl?: string }) => {
  const intl = useIntl();
  const { notify } = useNotifications();
  const updateApi = useUpdateGraphQLApi();
  const url = api.upstream?.main?.url;
  // Only a URL-based main upstream is ever editable here — a ref-based one
  // (`upstream.main.ref`) has no literal URL to edit, the same rule
  // `EditGraphqlApiForm`'s `hasUrlUpstream` applies.
  const hasUrlUpstream = url !== undefined;
  // sdl loads separately from api (see useGraphQLApiSdl) and is required to
  // faithfully resupply a non-introspection schemaSource on save; editing
  // before it's loaded risks silently saving an empty schema, so the edit
  // button waits for it — the same precondition `GraphqlApiEditPage` enforces
  // by blocking its whole page on both queries first.
  const canEdit = !api.readOnly && hasUrlUpstream && sdl !== undefined;

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [endpointUrl, setEndpointUrl] = useState('');
  const [touched, setTouched] = useState(false);
  const trimmedUrl = endpointUrl.trim();
  const endpointValid = trimmedUrl !== '' && isValidUrl(trimmedUrl);

  const openDrawer = () => {
    setEndpointUrl(url ?? '');
    setTouched(false);
    setDrawerOpen(true);
  };

  const closeDrawer = () => {
    if (updateApi.isPending) return;
    setDrawerOpen(false);
  };

  const saveEndpoint = () => {
    setTouched(true);
    if (!api.id || !endpointValid || sdl === undefined) return;

    updateApi.mutate(
      {
        graphqlApiId: api.id,
        body: {
          metadata: {
            ...api,
            ...resuppliedSchemaSource(api, sdl),
            upstream: {
              ...api.upstream,
              main: { ...api.upstream?.main, url: trimmedUrl },
            },
          },
        },
      },
      {
        onError: () => notify(intl.formatMessage(messages.saveError), 'error'),
        onSuccess: () => {
          notify(intl.formatMessage(messages.saved), 'success');
          setDrawerOpen(false);
        },
      },
    );
  };

  return (
    <>
      <Card>
        <Stack
          alignItems="center"
          direction="row"
          sx={{ justifyContent: 'space-between', px: 2, py: 1.5 }}
        >
          <Typography sx={{ fontWeight: 600 }} variant="h6">
            <FormattedMessage {...messages.endpointTitle} />
          </Typography>
          {canEdit ? (
            <Tooltip title={intl.formatMessage(messages.edit)}>
              <IconButton
                aria-label={intl.formatMessage(messages.edit)}
                onClick={openDrawer}
                size="small"
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
          ) : null}
        </Stack>
        <Divider />
        <Stack alignItems="center" direction="row" spacing={1.25} sx={{ px: 2, py: 1.5 }}>
          <Box
            sx={{
              alignItems: 'center',
              bgcolor: 'action.hover',
              borderRadius: 1,
              color: 'primary.main',
              display: 'flex',
              flexShrink: 0,
              height: 32,
              justifyContent: 'center',
              width: 32,
            }}
          >
            <Globe size={17} />
          </Box>
          {url ? (
            <Tooltip title={url}>
              <Link
                href={url}
                rel="noopener noreferrer"
                sx={{
                  '&:hover': { textDecoration: 'none' },
                  minWidth: 0,
                  overflow: 'hidden',
                  textDecoration: 'none',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
                target="_blank"
                variant="body2"
              >
                {url}
              </Link>
            </Tooltip>
          ) : (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.endpointNotConfigured} />
            </Typography>
          )}
        </Stack>
      </Card>

      <Drawer
        anchor="right"
        onClose={closeDrawer}
        open={drawerOpen}
        sx={{ '& .MuiDrawer-paper': { width: { md: 480, xs: '100%' } } }}
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
          <Stack
            alignItems="center"
            direction="row"
            spacing={1}
            sx={{ borderBottom: 1, borderColor: 'divider', p: 2 }}
          >
            <IconButton
              aria-label={intl.formatMessage(messages.close)}
              disabled={updateApi.isPending}
              onClick={closeDrawer}
              size="small"
            >
              <ChevronLeft size={20} />
            </IconButton>
            <Typography sx={{ fontWeight: 600 }} variant="h6">
              <FormattedMessage {...messages.drawerTitle} />
            </Typography>
          </Stack>

          <Box sx={{ flex: 1, overflowY: 'auto', p: 3 }}>
            <FormControl error={touched && !endpointValid} fullWidth>
              <FormLabel htmlFor="graphql-overview-backend-endpoint">
                <FormattedMessage {...messages.endpointUrlLabel} />
              </FormLabel>
              <OutlinedInput
                aria-describedby={
                  touched && !endpointValid ? 'graphql-overview-backend-endpoint-error' : undefined
                }
                autoFocus
                id="graphql-overview-backend-endpoint"
                onBlur={() => setTouched(true)}
                onChange={(event) => setEndpointUrl(event.target.value)}
                sx={{ mt: 0.75 }}
                value={endpointUrl}
              />
              {touched && !endpointValid ? (
                <FormHelperText id="graphql-overview-backend-endpoint-error">
                  <FormattedMessage {...messages.endpointUrlRequired} />
                </FormHelperText>
              ) : null}
            </FormControl>
          </Box>

          <Stack
            direction="row"
            spacing={1}
            sx={{ borderTop: 1, borderColor: 'divider', justifyContent: 'flex-end', p: 2 }}
          >
            <Button disabled={updateApi.isPending} onClick={closeDrawer} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button
              disabled={!endpointValid || updateApi.isPending || trimmedUrl === (url ?? '')}
              loading={updateApi.isPending}
              onClick={saveEndpoint}
              variant="contained"
            >
              <FormattedMessage {...messages.save} />
            </Button>
          </Stack>
        </Box>
      </Drawer>
    </>
  );
};

/**
 * Overview tab for a GraphQL API: schema explorer on the left, connectivity
 * details on the right — mirrors `apis/overview/OverviewTab.tsx`'s layout,
 * swapping the REST-only resources panel for the schema explorer already
 * built for the creation wizard, and API keys for `GraphqlApiKeysPanel` (its
 * create-key flow is genuinely different from REST's, see that file).
 * `InvokeUrlPanel` and `DeployedGatewaysPanel` are reused unmodified — both
 * are generic over their own props/a route builder, not REST-specific.
 */
export function GraphqlOverviewTab({
  api,
  sdl,
  deployedGateways,
  deployments,
}: {
  api: GraphQLApiDetail;
  sdl?: string;
  deployedGateways: Gateway[];
  deployments: Deployment[];
}) {
  const deployed = deployedGateways.length > 0;

  return (
    <Grid container spacing={2}>
      <Grid size={{ lg: 8, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          <Card sx={{ display: 'flex', flexDirection: 'column', p: 2 }}>
            <Typography sx={{ fontWeight: 600, mb: 1.5 }} variant="h6">
              <FormattedMessage {...messages.schemaTitle} />
            </Typography>
            <GraphqlSchemaExplorer sdl={sdl} />
          </Card>
        </Stack>
      </Grid>
      <Grid size={{ lg: 4, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          {deployed && (
            <>
              <Card sx={{ p: 2 }}>
                <Stack spacing={2}>
                  <InvokeUrlPanel context={api.context} gateways={deployedGateways} version={api.version} />
                  <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2 }}>
                    <GraphqlApiKeysPanel graphqlApiId={api.id ?? ''} />
                  </Box>
                </Stack>
              </Card>
              <DeployedGatewaysPanel
                apiId={api.id ?? ''}
                deployTo={routes.graphqlApiDeploy}
                deployments={deployments}
                gateways={deployedGateways}
              />
            </>
          )}
          <EndpointPanel api={api} sdl={sdl} />
        </Stack>
      </Grid>
    </Grid>
  );
}
