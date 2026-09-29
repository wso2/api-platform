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

import React from 'react';
import { Link as RouterLink } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Divider,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Plus } from '@wso2/oxygen-ui-icons-react';
import { formatRelativeTime } from '../../../../contexts/ApplicationsContext';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import {
  DISABLED_ACTION_SX,
  NO_PERMISSION_TOOLTIP,
} from '../../../../auth/permissions';

/** Rows shown inline before the panel defers to the full listing page. */
const ITEM_PREVIEW_COUNT = 5;

function truncateWords(text: string, maxWords: number): string {
  const words = text.trim().split(/\s+/);
  if (words.length <= maxWords) return text.trim();
  return `${words.slice(0, maxWords).join(' ')}…`;
}

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

/** One row in the panel, flattened from whichever resource kind is selected. */
export type KindDetailItem = {
  id: string;
  displayName: string;
  /** Secondary line shown under the name. */
  subtitle?: string;
  updatedAt?: string;
};

type KindDetailPanelProps = {
  title: string;
  description: string;
  items: KindDetailItem[];
  /** Total held by the resource, which can exceed what `items` carries. */
  totalCount: number;
  isLoading?: boolean;
  error?: Error | null;
  onRetry: () => void;
  viewAllPath: string;
  createPath?: string;
  createLabel: string;
  canCreate: boolean;
  /** Illustration, heading and blurb shown when the kind holds nothing yet. */
  emptyImage: string;
  emptyTitle: string;
  emptyDescription: string;
  onItemClick: (id: string) => void;
};

export default function KindDetailPanel({
  title,
  description,
  items,
  totalCount,
  isLoading,
  error,
  onRetry,
  viewAllPath,
  createPath,
  createLabel,
  canCreate,
  emptyImage,
  emptyTitle,
  emptyDescription,
  onItemClick,
}: KindDetailPanelProps): React.JSX.Element {
  const visibleItems = items.slice(0, ITEM_PREVIEW_COUNT);
  const hasMore = totalCount > ITEM_PREVIEW_COUNT;
  const hasItems = items.length > 0;

  return (
    <Card>
      <CardContent>
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={2}
          alignItems={{ xs: 'flex-start', sm: 'center' }}
          sx={{ mb: 2 }}
        >
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              {title}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          </Box>
          <Stack direction="row" spacing={1.5} alignItems="center">
            {hasMore ? (
              <Button size="small" component={RouterLink} to={viewAllPath}>
                See more
              </Button>
            ) : null}
            {createPath && hasItems ? (
              <Tooltip title={canCreate ? '' : NO_PERMISSION_TOOLTIP}>
                <Box component="span">
                  <Button
                    variant="contained"
                    size="medium"
                    component={RouterLink}
                    to={createPath}
                    startIcon={<Plus size={18} />}
                    disabled={!canCreate}
                    sx={DISABLED_ACTION_SX}
                  >
                    {createLabel}
                  </Button>
                </Box>
              </Tooltip>
            ) : null}
          </Stack>
        </Stack>

        <Divider sx={{ mb: 1.5 }} />

        {error ? (
          <ErrorAlert error={error} onRetry={onRetry} />
        ) : isLoading ? (
          <Stack spacing={1.5}>
            <Skeleton variant="rectangular" height={52} />
            <Skeleton variant="rectangular" height={52} />
            <Skeleton variant="rectangular" height={52} />
          </Stack>
        ) : items.length === 0 ? (
          <Stack alignItems="center" spacing={2} sx={{ py: 5, textAlign: 'center' }}>
            <Box
              component="img"
              src={emptyImage}
              alt=""
              sx={{ width: 140, maxWidth: '80%' }}
            />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              {emptyTitle}
            </Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 420 }}
            >
              {emptyDescription}
            </Typography>
            {createPath ? (
              <Tooltip title={canCreate ? '' : NO_PERMISSION_TOOLTIP}>
                <Box component="span">
                  <Button
                    variant="contained"
                    component={RouterLink}
                    to={createPath}
                    startIcon={<Plus size={20} />}
                    disabled={!canCreate}
                    sx={DISABLED_ACTION_SX}
                  >
                    {createLabel}
                  </Button>
                </Box>
              </Tooltip>
            ) : null}
          </Stack>
        ) : (
          <Stack divider={<Divider />} spacing={1.5}>
            {visibleItems.map((item) => (
              <Box
                key={item.id}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1.5,
                  width: '100%',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.25,
                    minWidth: 0,
                    cursor: 'pointer',
                  }}
                  onClick={() => onItemClick(item.id)}
                >
                  <Avatar
                    sx={{
                      width: 36,
                      height: 36,
                      fontSize: 16,
                      bgcolor: 'primary.light',
                      color: 'primary.contrastText',
                    }}
                  >
                    {getInitials(item.displayName || '')}
                  </Avatar>
                  <Box sx={{ minWidth: 0, overflow: 'hidden' }}>
                    <Typography variant="body1" sx={{ fontWeight: 600 }} noWrap>
                      {truncateWords(item.displayName || 'No Name', 12)}
                    </Typography>
                    {item.subtitle ? (
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        fontSize="0.7rem"
                        noWrap
                      >
                        {truncateWords(item.subtitle, 12)}
                      </Typography>
                    ) : null}
                  </Box>
                </Box>

                {item.updatedAt ? (
                  <Stack
                    direction="row"
                    spacing={0.75}
                    alignItems="center"
                    sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                  >
                    <Clock size={14} />
                    <Typography variant="caption" color="text.secondary" noWrap>
                      {formatRelativeTime(item.updatedAt)}
                    </Typography>
                  </Stack>
                ) : null}
              </Box>
            ))}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}
