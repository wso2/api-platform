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

import { Avatar, Box, Button, Card, CardContent, Chip, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { ChevronRight, Globe, Link2 } from '@wso2/oxygen-ui-icons-react';
import { useId } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { PublicationSummaryItem } from '@/api/resources/apiPublications';
import { publicationChipMeta } from '../utils/publicationDisplay';

const messages = defineMessages({
  goToPublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PortalPublicationCard.goToPublish',
    defaultMessage: 'Go To Publish',
    description: 'Card action opening the publish flow for this API on this portal.',
  },
});

type PortalPublicationCardProps = {
  publication: PublicationSummaryItem;
  onOpen: (publication: PublicationSummaryItem) => void;
};

const AVATAR_SIZE = 48;

/** Square identity tile for a portal — same treatment as an API's kind avatar. */
function PortalAvatar() {
  return (
    <Avatar
      sx={{
        bgcolor: 'primary.light',
        color: 'primary.contrastText',
        flexShrink: 0,
        height: AVATAR_SIZE,
        width: AVATAR_SIZE,
      }}
      variant="rounded"
    >
      <Globe size={Math.round(AVATAR_SIZE * 0.5)} />
    </Avatar>
  );
}

/** Drops the scheme and a trailing slash so the card shows a clean host, not a full URL. */
const displayUrl = (url: string): string => url.replace(/^https?:\/\//i, '').replace(/\/$/, '');

/**
 * One API Portal, annotated with this API's own publication status. Only the
 * "Go To Publish" button opens it; the card itself is not clickable.
 */
export function PortalPublicationCard({ publication, onOpen }: PortalPublicationCardProps) {
  const intl = useIntl();
  const nameId = useId();
  const name = publication.apiPortalName || publication.apiPortalId || '';
  const chipMeta = publicationChipMeta(publication);
  const open = () => onOpen(publication);

  return (
    <Card
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
      }}
    >
      <CardContent sx={{ flex: 1 }}>
        <Stack spacing={1.5}>
          <Stack alignItems="flex-start" direction="row" spacing={1.5}>
            <PortalAvatar />
            <Box sx={{ minWidth: 0 }}>
              <Typography id={nameId} noWrap sx={{ fontWeight: 700 }} variant="h6">
                {name}
              </Typography>
              {chipMeta && (
                <Chip
                  color={chipMeta.color}
                  label={intl.formatMessage(chipMeta.label)}
                  size="small"
                  sx={{ mt: 0.5, typography: 'caption' }}
                  variant="outlined"
                />
              )}
            </Box>
          </Stack>

          {publication.apiPortalDescription && (
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
              {publication.apiPortalDescription}
            </Typography>
          )}

          {publication.apiPortalUrl && (
            <Stack
              alignItems="center"
              direction="row"
              spacing={0.75}
              sx={{ color: 'text.secondary', minWidth: 0 }}
            >
              <Link2 size={14} />
              <Typography
                component="a"
                href={publication.apiPortalUrl}
                noWrap
                rel="noopener noreferrer"
                sx={{ color: 'inherit', textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}
                target="_blank"
                variant="caption"
              >
                {displayUrl(publication.apiPortalUrl)}
              </Typography>
            </Stack>
          )}
        </Stack>
      </CardContent>

      <Divider />

      <Box sx={{ display: 'flex', justifyContent: 'flex-end', px: 2, py: 1.25 }}>
        <Button
          aria-describedby={nameId}
          endIcon={<ChevronRight size={16} />}
          onClick={open}
          size="small"
          variant="outlined"
        >
          <FormattedMessage {...messages.goToPublish} />
        </Button>
      </Box>
    </Card>
  );
}
