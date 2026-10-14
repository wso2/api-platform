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

import type { ReactNode } from 'react';
import { Box, Button, Card, Chip, IconButton, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Clock, Copy, Edit, Lock, Rocket } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';

import { useFormatters } from '@/i18n/useFormatters';
import { Can, type PermissionOperation } from '@/permissions';
import { ApiThumbnailManager } from '../components/ApiThumbnailManager';
import { ApiKindChip, VersionChip } from '../listing/components/RestApiChips';

// IDs keep the page they were written for, so existing translations carry over.
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

/** The fields of an API the header shows; REST and GraphQL APIs both carry them. */
export type ApiOverviewHeaderApi = {
  context?: string;
  createdAt?: string;
  createdBy?: string;
  description?: string;
  displayName?: string;
  kind?: string;
  readOnly?: boolean;
  version?: string;
};

export type ApiOverviewHeaderProps = {
  api: ApiOverviewHeaderApi;
  /** The id the thumbnail endpoints address the API by. */
  apiId: string;
  /** The thumbnail endpoints' `{apiType}` segment, e.g. `rest-api`. */
  apiType: string;
  /** The progress stepper shown along the bottom of the card. */
  banner: ReactNode;
  /** Whether the caller may edit this API (on top of it not being read-only). */
  canEdit: boolean;
  /** The operation the Deploy button is gated on. */
  deployOperation: PermissionOperation;
  deployPath: string;
  editPath: string;
};

/**
 * The API overview's header card: thumbnail, name, version and kind, the
 * description, who created it and its context, the Edit and Deploy actions,
 * and the progress stepper. Shared by every API kind's overview so they
 * cannot drift apart; each supplies its own paths, permissions and stepper.
 */
export function ApiOverviewHeader({
  api,
  apiId,
  apiType,
  banner,
  canEdit,
  deployOperation,
  deployPath,
  editPath,
}: ApiOverviewHeaderProps) {
  const intl = useIntl();
  const { dateTime, relativeTime } = useFormatters();
  const displayName = api.displayName || apiId;
  const context = api.context || '/';

  return (
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
          <ApiThumbnailManager
            apiType={apiType}
            apiId={apiId}
            displayName={displayName}
            disabled={api.readOnly}
            fontSize={AVATAR_FONT_SIZE}
            iconSize={AVATAR_ICON_SIZE}
            size={AVATAR_SIZE}
          />

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
          {!api.readOnly && canEdit && (
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
          <Can do={deployOperation} denied="disable">
            <Button
              component={RouterLink}
              startIcon={<Rocket size={18} />}
              sx={{ flexShrink: 0 }}
              to={deployPath}
              variant="contained"
            >
              <FormattedMessage {...messages.deployToGateway} />
            </Button>
          </Can>
        </Stack>
      </Box>
      {banner}
    </Card>
  );
}
