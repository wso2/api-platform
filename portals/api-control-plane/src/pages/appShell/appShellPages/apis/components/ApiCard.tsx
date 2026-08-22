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
  Card,
  CardActions,
  CardContent,
  CardHeader,
  Chip,
  Divider,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Clock, Lock, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { useIntl } from 'react-intl';

import type { RestApi } from '../../../../../api/resources/restApis';
import { relativeTime } from '../../../../../utils/relativeTime';
import { interactiveCardSx } from '../../../../../theme';
import { TransportChips, VersionChip } from './RestApiChips';
import { apiKindLabel } from '../restApiDisplay';

type ApiCardProps = {
  api: RestApi;
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

/**
 * Marks the delete button so the card can reveal it on hover. One constant, so
 * the selector below and the button it targets can never drift apart.
 */
const DELETE_CLASS = 'ApiCard-delete';

/** Edge of the square kind tile, and the icon sitting inside it. */
const AVATAR_SIZE = 56;
const AVATAR_ICON_SIZE = 32;

/**
 * API card for the grid view, rendering the spec's `RESTAPI` shape.
 *
 * Built from the Card family Oxygen re-exports — `CardHeader` for the
 * monogram/name/version band, `CardContent` for the description, `CardActions`
 * for the footer — so padding, dividers, chip and avatar treatments all come
 * from the theme rather than from `sx` literals here. What is left in `sx` is
 * layout only: the flex column that lets equal-height cards sit in a grid.
 *
 * Deliberately sparse: name, version, what it speaks, where it is live and when
 * it last changed. Context, operation count and lifecycle status live on the
 * API's own page — a grid is for finding an API, and per-card status marks
 * compete with that at a dozen cards on screen.
 */
export function ApiCard({ api, onOpen, onDelete }: ApiCardProps) {
  const { formatMessage } = useIntl();
  const updated = api.updatedAt || api.createdAt;
  const transports = api.transport ?? [];

  return (
    <Card
      onClick={() => onOpen(api)}
      sx={{
        ...interactiveCardSx,
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        // Hide delete until hover; `focus-within` keeps it keyboard-reachable.
        [`&:hover .${DELETE_CLASS}, &:focus-within .${DELETE_CLASS}`]: {
          opacity: 1,
        },
      }}
    >
      <CardHeader
        avatar={
          <Tooltip placement="left" title={apiKindLabel(api.kind)}>
            <Avatar
              sx={() => ({
                bgcolor: 'primary.light',
                color: 'primary.contrastText',
                height: AVATAR_SIZE,
                width: AVATAR_SIZE,
              })}
              variant="rounded"
            >
              <Boxes size={AVATAR_ICON_SIZE} />
            </Avatar>
          </Tooltip>
        }
        slotProps={{
          // `content` has no min-width of its own, so a long name would widen
          // the card instead of truncating. Both slots render a Stack, which
          // cannot legally sit inside the default `span`.
          content: { sx: { minWidth: 0 } },
          subheader: { component: 'div' },
          title: { component: 'div', sx: { mb: 1 } },
        }}
        subheader={
          <Stack
            direction="row"
            spacing={1}
            sx={{ flexWrap: 'wrap', gap: 1 }}
            useFlexGap
          >
            <TransportChips transports={transports} />
            {api.readOnly && (
              <Tooltip title="Discovered from a data-plane gateway — read-only here">
                <Chip
                  icon={<Lock size={12} />}
                  label="Gateway-managed"
                  size="small"
                  variant="outlined"
                />
              </Tooltip>
            )}
          </Stack>
        }
        sx={{ alignItems: 'flex-start' }}
        title={
          <Stack
            alignItems="center"
            direction="row"
            spacing={1}
            sx={{ minWidth: 0 }}
          >
            <Typography component="span" noWrap variant="h5" sx={{ fontWeight: 600}}>
              {api.displayName}
            </Typography>
            <VersionChip version={api.version} />
          </Stack>
        }
      />

      {/* Two-line clamped description; the flex grow is what keeps every
          card's footer on the same line across the grid. */}
      <CardContent sx={{ flex: 1, pt: 0 }}>
        <Typography
          color="text.secondary"
          sx={{
            display: '-webkit-box',
            overflow: 'hidden',
            WebkitBoxOrient: 'vertical',
            WebkitLineClamp: 2,
            fontSize: "0.7rem"
          }}
          variant="body2"
        >
          {api.description || ''}
        </Typography>
      </CardContent>

      <Divider />

      {/* Footer: when it last changed, and the one destructive action. */}
      <CardActions sx={{ justifyContent: 'space-between', px: 2 }}>
        <Stack
          alignItems="center"
          direction="row"
          spacing={1}
          sx={{ color: 'text.secondary' }}
        >
          {updated && (
            <>
              <Clock size={16} />
              <Typography color="text.secondary" variant="caption">
                {formatMessage(
                  {
                    id: 'apiCard.updated',
                    defaultMessage: 'Updated {relative}',
                    description:
                      'Card footer timestamp; {relative} is a phrase such as "3 hours ago".',
                  },
                  { relative: relativeTime(updated) }
                )}
              </Typography>
            </>
          )}
        </Stack>
        {onDelete && (
          <Tooltip title="Delete">
            <IconButton
              aria-label={formatMessage(
                {
                  id: 'apiCard.delete',
                  defaultMessage: 'Delete {apiName}',
                  description: 'Accessible label for deleting an API from API card',
                },
                { apiName: api.displayName }
              )}
              className={DELETE_CLASS}
              onClick={(event) => {
                event.stopPropagation();
                onDelete(api);
              }}
              size="small"
              sx={(theme) => ({
                opacity: 0,
                transition: theme.transitions.create(['opacity', 'color']),
                '&:hover': { color: 'error.main' },
              })}
            >
              <Trash2 size={18} />
            </IconButton>
          </Tooltip>
        )}
      </CardActions>
    </Card>
  );
}
