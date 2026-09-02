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

import { useState, type ReactNode } from 'react';
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Lock, Pencil } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';

import { useRestApi, useUpdateRestApi, type RestApi } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useFormatters } from '@/i18n/useFormatters';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { LifecycleChip, TransportChips, VersionChip } from '../listing/components/RestApiChips';
import { apiInitials } from '../utils/restApiDisplay';
import { OverviewTab } from './OverviewTab';

const messages = defineMessages({
  context: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.context.label',
    defaultMessage: 'Context',
    description: 'Label for the API base path shown in the API detail header, e.g. "/orders".',
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
  descriptionSaved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.saved',
    defaultMessage: 'Description updated.',
    description: 'Confirmation shown after the API description is saved.',
  },
  editCancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.cancel',
    defaultMessage: 'Cancel',
    description: 'Discards an unsaved edit to the API description.',
  },
  editDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.edit',
    defaultMessage: 'Edit description',
    description:
      'Accessible label and tooltip for the pencil button that opens the description for editing.',
  },
  editFieldLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.fieldLabel',
    defaultMessage: 'Description',
    description: 'Label of the text box used to edit the API description.',
  },
  editSave: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.description.save',
    defaultMessage: 'Save',
    description: 'Commits an edit to the API description.',
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
  lastUpdated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.lastUpdated.label',
    defaultMessage: 'Last updated',
    description: 'Label for when the API definition last changed, shown in the API detail header.',
  },
  transports: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiDetailPage.transports.label',
    defaultMessage: 'Transports',
    description: 'Label for the protocols an API is exposed over (HTTP, HTTPS).',
  },
});

/** Edge of the square kind tile, the monogram inside it, and the fallback icon. */
const AVATAR_SIZE = 70;
const AVATAR_FONT_SIZE = 32;
const AVATAR_ICON_SIZE = 32;

/** Label column of the metadata rows, wide enough to align their values. */
const LABEL_COLUMN = 88;

/** Keeps the description editor from stretching the full width of the header. */
const EDIT_FIELD_MAX_WIDTH = 560;

/** Inline API description editor.
 * Read mode clamps to two lines, shows an italic placeholder when empty,
 * and hides editing for `readOnly` APIs.
 */
function DescriptionField({ api, restApiId }: { api: RestApi; restApiId: string }) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const updateApi = useUpdateRestApi();
  const [draft, setDraft] = useState<string | null>(null);

  const isEditing = draft !== null;
  const description = api.description?.trim() ?? '';

  const save = () => {
    const next = (draft ?? '').trim();
    // Nothing to persist: close rather than spend a PUT on an identical value.
    if (next === description) {
      setDraft(null);
      return;
    }
    // The spec's update body is the whole `RESTAPI`, so the fetched object is
    // spread back with the edit applied.
    updateApi.mutate(
      { restApiId, body: { ...api, description: next } },
      {
        onSuccess: () => {
          setDraft(null);
          notify(intl.formatMessage(messages.descriptionSaved), 'success');
        },
      },
    );
  };

  if (isEditing) {
    return (
      <Stack spacing={1} sx={{ maxWidth: EDIT_FIELD_MAX_WIDTH }}>
        <TextField
          autoFocus
          disabled={updateApi.isPending}
          fullWidth
          label={intl.formatMessage(messages.editFieldLabel)}
          maxRows={6}
          multiline
          onChange={(event) => setDraft(event.target.value)}
          size="small"
          value={draft}
        />
        <Stack direction="row" spacing={1}>
          <Button disabled={updateApi.isPending} onClick={save} size="small" variant="contained">
            <FormattedMessage {...messages.editSave} />
          </Button>
          <Button disabled={updateApi.isPending} onClick={() => setDraft(null)} size="small">
            <FormattedMessage {...messages.editCancel} />
          </Button>
        </Stack>
      </Stack>
    );
  }

  return (
    <Stack alignItems="flex-start" direction="row" spacing={0.5} sx={{ minWidth: 0 }}>
      {description ? (
        <Typography
          color="text.secondary"
          sx={{
            display: '-webkit-box',
            overflow: 'hidden',
            WebkitBoxOrient: 'vertical',
            WebkitLineClamp: 2,
          }}
          variant="body2"
        >
          {description}
        </Typography>
      ) : (
        <Typography color="text.disabled" sx={{ fontStyle: 'italic' }} variant="body2">
          <FormattedMessage {...messages.descriptionPlaceholder} />
        </Typography>
      )}
      {!api.readOnly && (
        <Tooltip title={intl.formatMessage(messages.editDescription)}>
          <IconButton
            aria-label={intl.formatMessage(messages.editDescription)}
            onClick={() => setDraft(description)}
            size="small"
            sx={{ flexShrink: 0 }}
          >
            <Pencil size={14} />
          </IconButton>
        </Tooltip>
      )}
    </Stack>
  );
}

/**
 * One `label: value` row in the header's metadata block.
 *
 * A single component for all of them is what keeps the rows aligned: the label
 * always takes the theme's `caption` scale and secondary tone, the value always
 * `body2`, so adding a field cannot introduce a fourth type treatment.
 */
function MetaItem({ children, label }: { children: ReactNode; label: ReactNode }) {
  return (
    <Stack alignItems="center" direction="row" spacing={2} sx={{ minWidth: 0 }}>
      {/* `minWidth` rather than a fixed width: it lines the values up into a
          column for the labels we ship, and a longer translation pushes its own
          row wider instead of clipping. */}
      <Typography
        color="text.secondary"
        sx={{ flexShrink: 0, minWidth: LABEL_COLUMN }}
        variant="caption"
      >
        {label}
      </Typography>
      {children}
    </Stack>
  );
}

// No `ScopeGate`: this page is the API tier of the sidebar's Overview item, which
// degrades to a shallower tier rather than linking here without an API.
export function ApiDetailPage() {
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);
  const { dateTime, relativeTime } = useFormatters();
  const intl = useIntl();

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

  const displayName = api.displayName || restApiId;
  const transports = api.transport ?? [];
  // `updatedAt` is absent until the first edit, so a fresh API reads its
  // creation time rather than showing an empty row.
  const updated = api.updatedAt || api.createdAt;

  return (
    <>
      <Card sx={{ mb: 3 }}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 2,
            justifyContent: 'space-between',
            p: 2,
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

            <Stack spacing={1} sx={{ minWidth: 0 }}>
              {/* Identity: name, version, lifecycle, and whether this console
                  may edit the API at all. */}
              <Stack
                alignItems="center"
                direction="row"
                sx={{ flexWrap: 'wrap', gap: 1, minWidth: 0 }}
                useFlexGap
              >
                <Tooltip title={displayName}>
                  <Typography noWrap variant="h3">
                    {displayName}
                  </Typography>
                </Tooltip>
                <VersionChip version={api.version} />
                {api.lifeCycleStatus && <LifecycleChip status={api.lifeCycleStatus} />}
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

              <DescriptionField api={api} restApiId={restApiId} />

              <Stack spacing={0.5}>
                <MetaItem label={<FormattedMessage {...messages.context} />}>
                  <Typography noWrap variant="body2">
                    {api.context || '/'}
                  </Typography>
                </MetaItem>

                {transports.length > 0 && (
                  <MetaItem label={<FormattedMessage {...messages.transports} />}>
                    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5 }} useFlexGap>
                      <TransportChips transports={transports} />
                    </Stack>
                  </MetaItem>
                )}

                {updated && (
                  <MetaItem label={<FormattedMessage {...messages.lastUpdated} />}>
                    {/* Relative reads faster; the exact stamp is one hover
                        away for anyone auditing a change. */}
                    <Tooltip title={dateTime(updated)}>
                      <Typography variant="body2">{relativeTime(updated)}</Typography>
                    </Tooltip>
                  </MetaItem>
                )}
              </Stack>
            </Stack>
          </Box>

          <Button component={RouterLink} sx={{ flexShrink: 0 }} to={deployPath} variant="contained">
            <FormattedMessage {...messages.deployToGateway} />
          </Button>
        </Box>
      </Card>

      {/* Policy, Routing and Documents used to be tabs here; each is now its own
          page under the sidebar's Develop menu, leaving Overview as the whole of
          this page — so no tab bar. */}
      <OverviewTab api={api} />
    </>
  );
}
