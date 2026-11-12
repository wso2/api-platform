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
  Avatar,
  Box,
  Button,
  ButtonGroup,
  Card,
  Chip,
  chipClasses,
  Divider,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Circle, ExternalLink, Megaphone } from '@wso2/oxygen-ui-icons-react';
import { useId, useRef, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import type { PublicationSummaryItem } from '@/api/resources/apiPublications';
import devportalLogoUrl from '@/assets/icons/devportal-logo.png';
import { useTrackFeature } from '@/featureUsage';
import { useFormatters } from '@/i18n/useFormatters';
import { hairline } from '@/theme/receipes';
import type { PublishTarget } from '../publishTarget';
import { buildViewInPortalUrl, isListedOnPortal, publicationStatusMeta } from '../utils/publicationDisplay';

const messages = defineMessages({
  publish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.publish',
    defaultMessage: 'Publish',
    description:
      'Split button action, offered only while nothing is saved for this API on this portal, that publishes it there straight away.',
  },
  publishing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.publishing',
    defaultMessage: 'Publishing\u2026',
    description: 'The Publish button\u2019s label while its publish runs.',
  },
  moreActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.moreActions',
    defaultMessage: 'More publish options',
    description: 'Accessible label for the arrow of the Publish split button, which opens its other action.',
  },
  configureAndPublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.configureAndPublish',
    defaultMessage: 'Configure & Publish',
    description: 'Card action opening the publish flow for this API on this portal.',
  },
  viewInPortal: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.viewInPortal',
    defaultMessage: 'View in Portal',
    description: 'Link opening this API\'s own page on the portal, in a new tab.',
  },
  statusLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.statusLabel',
    defaultMessage: 'Status',
    description: 'Row label for the API\'s publication status on this portal.',
  },
});

type PortalPublicationCardProps = {
  /** False when the caller may not save the draft a one-click Publish writes; Configure & Publish stays. */
  oneClickPublishAllowed: boolean;
  /** Starts a one-click Publish to this portal; the listing runs it. */
  onPublish: () => void;
  orgHandle: string;
  publication: PublicationSummaryItem;
  /**
   * Set when the caller may not publish this kind of API: the publish buttons
   * are disabled and this explains why.
   */
  publishDeniedTooltip?: string;
  /** Held by the listing, so a card filtered out and back mid-publish still shows it running. */
  publishing: boolean;
  /** Which API, and where its publish pages live. */
  target: PublishTarget;
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

/** Share of the tile the logo fills, leaving the margin around it. */
const LOGO_SIZE = '75%';

/** Square identity tile for a portal, carrying the developer portal's mark. */
function PortalAvatar() {
  return (
    <Avatar
      sx={(theme) => ({
        bgcolor: 'action.hover',
        border: hairline(theme),
        borderColor: 'divider',
        flexShrink: 0,
        height: AVATAR_SIZE,
        width: AVATAR_SIZE,
      })}
      variant="rounded"
    >
      <Box
        alt=""
        component="img"
        src={devportalLogoUrl}
        sx={{ height: LOGO_SIZE, objectFit: 'contain', width: LOGO_SIZE }}
      />
    </Avatar>
  );
}

/** What the split button does: publish straight away, or open the publish flow first. */
type PublishMode = 'publish' | 'review';

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
 * One API Portal, annotated with this API's publication status there and when
 * it last changed. The card itself is not clickable; its button publishes
 * straight away or opens the publish page.
 */
export function PortalPublicationCard({
  oneClickPublishAllowed,
  onPublish,
  orgHandle,
  publication,
  publishDeniedTooltip,
  publishing,
  target,
}: PortalPublicationCardProps) {
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  const trackFeature = useTrackFeature();
  const nameId = useId();
  const name = publication.apiPortalName || publication.apiPortalId || '';
  const status = publicationStatusMeta(publication.status);
  // NOT_PUBLISHED has never had a page on the portal to link to.
  const viewInPortalHref =
    publication.apiPortalUrl && isListedOnPortal(publication.status)
      ? buildViewInPortalUrl(publication.apiPortalUrl, orgHandle, target.apiHandle)
      : undefined;

  // Without an id there is no publish flow to open.
  const publishHref = publication.apiPortalId ? target.publishPath(publication.apiPortalId) : undefined;

  const navigate = useNavigate();
  const [mode, setMode] = useState<PublishMode>('publish');
  const [menuOpen, setMenuOpen] = useState(false);
  const anchorRef = useRef<HTMLDivElement>(null);

  // One-click Publish only while nothing is saved here, so it never has a draft or a live listing to replace.
  // Kept while a publish runs, though a reload meanwhile shows the draft it saved.
  const offerOneClick =
    publishing ||
    (oneClickPublishAllowed && publication.status === 'NOT_PUBLISHED' && !publication.draftUpdatedAt);
  const publishBlocked = !publishHref || publishDeniedTooltip !== undefined || publishing;
  const splitButtonLabel = publishing
    ? messages.publishing
    : mode === 'publish'
      ? messages.publish
      : messages.configureAndPublish;

  const openPublishPage = () => {
    if (!publishHref) return;
    navigate(publishHref, { state: { portalName: publication.apiPortalName } });
  };

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
          <Tooltip title={publishDeniedTooltip ?? ''}>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              {offerOneClick ? (
                <>
                  <ButtonGroup
                    disabled={publishBlocked}
                    ref={anchorRef}
                    size="small"
                    sx={{ display: 'flex', width: '100%' }}
                    variant="contained"
                  >
                    <Button
                      aria-describedby={nameId}
                      loading={publishing}
                      loadingPosition="start"
                      onClick={() => (mode === 'publish' ? onPublish() : openPublishPage())}
                      startIcon={mode === 'publish' && <Megaphone size={16} />}
                      sx={{ '&&': { flex: '1 1 auto', minWidth: 0 }, whiteSpace: 'nowrap' }}
                    >
                      <FormattedMessage {...splitButtonLabel} />
                    </Button>
                    <Button
                      aria-expanded={menuOpen}
                      aria-haspopup="menu"
                      aria-label={intl.formatMessage(messages.moreActions)}
                      onClick={() => setMenuOpen((open) => !open)}
                      sx={{ '&&': { flex: '0 0 40px', minWidth: 40, px: 0 } }}
                    >
                      <ChevronDown size={16} />
                    </Button>
                  </ButtonGroup>
                  <Menu anchorEl={anchorRef.current} onClose={() => setMenuOpen(false)} open={menuOpen}>
                    {(['publish', 'review'] as const).map((option) => (
                      <MenuItem
                        key={option}
                        onClick={() => {
                          setMode(option);
                          setMenuOpen(false);
                        }}
                        selected={option === mode}
                      >
                        <FormattedMessage {...(option === 'publish' ? messages.publish : messages.configureAndPublish)} />
                      </MenuItem>
                    ))}
                  </Menu>
                </>
              ) : (
                <Button
                  aria-describedby={nameId}
                  disabled={publishBlocked}
                  fullWidth
                  onClick={openPublishPage}
                  size="small"
                  sx={{ whiteSpace: 'nowrap' }}
                  variant="contained"
                >
                  <FormattedMessage {...messages.configureAndPublish} />
                </Button>
              )}
            </Box>
          </Tooltip>
          <Button
            component="a"
            disabled={!viewInPortalHref}
            endIcon={<ExternalLink size={16} />}
            href={viewInPortalHref}
            onClick={() => trackFeature('apip.lifecycle.api-portal-open', { api_type: target.apiType })}
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
