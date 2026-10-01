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

import { Avatar, Box, Button, Card, Chip, chipClasses, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { Circle, ExternalLink, Globe } from '@wso2/oxygen-ui-icons-react';
import { useId, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link } from 'react-router-dom';

import type { PublicationSummaryItem } from '@/api/resources/apiPublications';
import { useFormatters } from '@/i18n/useFormatters';
import { routes } from '@/routes/paths';
import { hairline } from '@/theme/receipes';
import { buildViewInPortalUrl, isListedOnPortal, publicationStatusMeta } from '../utils/publicationDisplay';

const messages = defineMessages({
  goToPublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.goToPublish',
    defaultMessage: 'Go to publish',
    description: 'Card action opening the publish flow for this API on this portal.',
  },
  viewInPortal: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.viewInPortal',
    defaultMessage: 'View in portal',
    description: 'Link opening this API\'s own page on the portal, in a new tab.',
  },
  statusLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.statusLabel',
    defaultMessage: 'Status',
    description: 'Row label for the API\'s publication status on this portal.',
  },
});

type PortalPublicationCardProps = {
  apiHandle: string;
  orgHandle: string;
  projectHandle: string;
  publication: PublicationSummaryItem;
};

const AVATAR_SIZE = 72;
const DESCRIPTION_LINES = 2;
/** Description line height, so a short description still reserves its two lines. */
const DESCRIPTION_LINE_HEIGHT = 1.5;

/** Breathing room around a chip's icon and label, so neither sits against the border. */
const chipSx = {
  typography: 'caption',
  [`& .${chipClasses.label}`]: { px: 1.25 },
  [`& .${chipClasses.icon}`]: { ml: 1, mr: -0.5 },
} as const;

/** Square identity tile for a portal. */
function PortalAvatar() {
  return (
    <Avatar
      sx={(theme) => ({
        bgcolor: 'action.hover',
        border: hairline(theme),
        borderColor: 'divider',
        color: 'text.secondary',
        flexShrink: 0,
        height: AVATAR_SIZE,
        width: AVATAR_SIZE,
      })}
      variant="rounded"
    >
      <Globe size={AVATAR_SIZE / 2} />
    </Avatar>
  );
}

/** A label on the left, its value on the right. */
function DetailRow({ children, label }: { children: ReactNode; label: ReactNode }) {
  return (
    <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={2} sx={{ minHeight: 24 }}>
      <Typography color="text.secondary" variant="body2">
        {label}
      </Typography>
      {children}
    </Stack>
  );
}

/**
 * One API Portal, annotated with this API's own publication state: its status,
 * when it last changed, and whether a draft is waiting. The card itself is not
 * clickable; "Go to publish" opens the flow.
 */
export function PortalPublicationCard({ apiHandle, orgHandle, projectHandle, publication }: PortalPublicationCardProps) {
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  const nameId = useId();
  const name = publication.apiPortalName || publication.apiPortalId || '';
  const status = publicationStatusMeta(publication.status);
  // NOT_PUBLISHED has never had a page on the portal to link to.
  const viewInPortalHref =
    publication.apiPortalUrl && isListedOnPortal(publication.status)
      ? buildViewInPortalUrl(publication.apiPortalUrl, orgHandle, apiHandle)
      : undefined;

  // Without an id there is no publish flow to open.
  const publishHref = publication.apiPortalId
    ? routes.apiPortalPublish(orgHandle, projectHandle, apiHandle, publication.apiPortalId)
    : undefined;

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height: '100%' }} variant="outlined">
      <Stack spacing={2} sx={{ flex: 1, p: 2.5 }}>
        <Stack alignItems="flex-start" direction="row" spacing={2}>
          <PortalAvatar />
          <Stack spacing={0.5} sx={{ flex: 1, minWidth: 0, pt: 0.5 }}>
            <Typography id={nameId} sx={{ fontWeight: 600, overflowWrap: 'break-word' }} variant="subtitle2">
              {name}
            </Typography>
            <Typography
              color="text.secondary"
              sx={{
                display: '-webkit-box',
                lineHeight: DESCRIPTION_LINE_HEIGHT,
                minHeight: `${DESCRIPTION_LINES * DESCRIPTION_LINE_HEIGHT}em`,
                overflow: 'hidden',
                WebkitBoxOrient: 'vertical',
                WebkitLineClamp: DESCRIPTION_LINES,
              }}
              variant="body2"
            >
              {publication.apiPortalDescription}
            </Typography>
          </Stack>
        </Stack>

        <Divider />

        <Box sx={{ flex: 1 }}>
          <DetailRow label={<FormattedMessage {...messages.statusLabel} />}>
            <Stack alignItems="center" direction="row" spacing={1.25}>
              <Typography color="text.secondary" variant="caption">
                {relativeTime(publication.publicationUpdatedAt)}
              </Typography>
              <Chip
                color={status.color}
                icon={<Circle fill="currentColor" size={8} />}
                label={intl.formatMessage(status.label)}
                size="small"
                sx={chipSx}
                variant="outlined"
              />
            </Stack>
          </DetailRow>
        </Box>

        <Stack alignItems="center" direction="row" spacing={2}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Button
              aria-describedby={nameId}
              component={Link}
              disabled={!publishHref}
              fullWidth
              size="small"
              state={{ portalName: publication.apiPortalName }}
              to={publishHref ?? ''}
              variant="contained"
            >
              <FormattedMessage {...messages.goToPublish} />
            </Button>
          </Box>
          <Button
            component="a"
            disabled={!viewInPortalHref}
            endIcon={<ExternalLink size={16} />}
            href={viewInPortalHref}
            rel="noopener noreferrer"
            size="small"
            sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
            target="_blank"
          >
            <FormattedMessage {...messages.viewInPortal} />
          </Button>
        </Stack>
      </Stack>
    </Card>
  );
}
