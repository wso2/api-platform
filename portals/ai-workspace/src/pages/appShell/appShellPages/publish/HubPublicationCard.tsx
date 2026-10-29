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

import { useId, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link } from 'react-router-dom';
import { Avatar, Box, Button, Card, Chip, chipClasses, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { Circle, ExternalLink, Globe } from '@wso2/oxygen-ui-icons-react';

import { formatRelativeTime } from '../../../../contexts/llmProvider';
import type { PublicationSummaryItem } from '../../../../utils/types';
import { hairline } from './publishStyles';
import { buildViewInPortalUrl, isListedOnPortal, publicationStatusMeta } from './publicationDisplay';

const messages = defineMessages({
  status: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.card.status',
    defaultMessage: 'Status',
  },
  goToPublish: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.card.goToPublish',
    defaultMessage: 'Go to publish',
  },
  viewInHub: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.card.viewInHub',
    defaultMessage: 'View in MCP Hub',
  },
});

type HubPublicationCardProps = {
  mcpProxyId: string;
  orgHandle: string;
  publication: PublicationSummaryItem;
  /** This MCP proxy's publish page for a hub. */
  publishPath: (apiPortalId: string) => string;
};

const AVATAR_SIZE = 72;
const AVATAR_ICON_SIZE = 32;
const DESCRIPTION_LINES = 2;
/** Description line height, so a short description still reserves its two lines. */
const DESCRIPTION_LINE_HEIGHT = 1.5;

/** Keeps the chip's icon and label off its border. */
const chipSx = {
  typography: 'caption',
  [`& .${chipClasses.label}`]: { px: 1.25 },
  [`& .${chipClasses.icon}`]: { ml: 1, mr: -0.5 },
} as const;

function HubAvatar() {
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
      <Globe size={AVATAR_ICON_SIZE} />
    </Avatar>
  );
}

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

/** One MCP Hub with this proxy's status on it. The card is not clickable; "Go to publish" opens the flow. */
export function HubPublicationCard({ mcpProxyId, orgHandle, publication, publishPath }: HubPublicationCardProps) {
  const intl = useIntl();
  const nameId = useId();
  const name = publication.apiPortalName || publication.apiPortalId || '';
  const status = publicationStatusMeta(publication.status);

  // NOT_PUBLISHED has never had a page on the hub to link to.
  const viewInHubHref =
    publication.apiPortalUrl && isListedOnPortal(publication.status)
      ? buildViewInPortalUrl(publication.apiPortalUrl, orgHandle, mcpProxyId)
      : undefined;

  // Without an id there is no publish flow to open.
  const publishHref = publication.apiPortalId ? publishPath(publication.apiPortalId) : undefined;

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height: '100%' }} variant="outlined">
      <Stack spacing={2} sx={{ flex: 1, p: 2.5 }}>
        <Stack alignItems="flex-start" direction="row" spacing={2}>
          <HubAvatar />
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
          <DetailRow label={intl.formatMessage(messages.status)}>
            <Stack alignItems="center" direction="row" spacing={1.25}>
              <Typography color="text.secondary" variant="caption">
                {publication.publicationUpdatedAt ? formatRelativeTime(publication.publicationUpdatedAt) : ''}
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
              state={{ hubName: publication.apiPortalName }}
              to={publishHref ?? ''}
              variant="contained"
            >
              <FormattedMessage {...messages.goToPublish} />
            </Button>
          </Box>
          <Button
            component="a"
            disabled={!viewInHubHref}
            endIcon={<ExternalLink size={16} />}
            href={viewInHubHref}
            rel="noopener noreferrer"
            size="small"
            sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
            target="_blank"
          >
            <FormattedMessage {...messages.viewInHub} />
          </Button>
        </Stack>
      </Stack>
    </Card>
  );
}
