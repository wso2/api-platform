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
import {
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  CardHeader,
  Divider,
  Skeleton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

export type SummaryRow = {
  id: string;
  title: string;
  description?: string;
  meta?: string;
  avatarText?: string;
  icon?: ReactNode;
};

type SummaryCardSectionProps = {
  title: string;
  totalCount: number;
  items: SummaryRow[];
  /** Icon shown in the header / empty state to brand the category. */
  icon?: ReactNode;
  maxItems?: number;
  isLoading?: boolean;
  error?: Error | null;
  onRetry?: () => void;
  onItemClick?: (id: string) => void;
  onSeeMore?: () => void;
  onAdd?: () => void;
  addLabel?: string;
  emptyTitle: string;
  emptyDescription?: string;
  emptyActionLabel?: string;
  onEmptyAction?: () => void;
};

/**
 * Category summary card used on the home/overview dashboards. A header with a
 * `Total: N` subheader and Add/See-more actions, loading skeleton rows, an
 * empty state, and a divided list of clickable rows.
 */
export function SummaryCardSection({
  title,
  totalCount,
  items,
  icon,
  maxItems = 5,
  isLoading = false,
  error,
  onRetry,
  onItemClick,
  onSeeMore,
  onAdd,
  addLabel = 'Add new',
  emptyTitle,
  emptyDescription,
  emptyActionLabel,
  onEmptyAction,
}: SummaryCardSectionProps) {
  const visible = items.slice(0, maxItems);
  const hasItems = visible.length > 0;
  const isEmpty = !isLoading && !error && !hasItems;
  const canClick = Boolean(onItemClick);

  return (
    <Card sx={{ height: '100%', minHeight: 300, width: '100%' }}>
      <CardHeader
        action={
          hasItems ? (
            <Stack alignItems="center" direction="row" spacing={1}>
              {onAdd && (
                <Button onClick={onAdd} size="small" variant="text">
                  + {addLabel}
                </Button>
              )}
              {onSeeMore && totalCount > visible.length && (
                <Button onClick={onSeeMore} size="small">
                  <FormattedMessage id="summary.card.see.more" defaultMessage="See more" />
                </Button>
              )}
            </Stack>
          ) : null
        }
        slotProps={{
          subheader: { sx: { fontSize: '0.82rem' } },
          title: { sx: { fontSize: '1rem', fontWeight: 700 } },
        }}
        subheader={`Total: ${totalCount}`}
        title={title}
      />
      <CardContent sx={{ pt: 0 }}>
        {isLoading ? (
          <Stack divider={<Divider />} spacing={1.5}>
            {[0, 1, 2].map((key) => (
              <Box
                key={key}
                sx={{
                  alignItems: 'center',
                  display: 'flex',
                  gap: 1.25,
                  py: 0.5,
                }}
              >
                <Skeleton height={36} variant="circular" width={36} />
                <Box sx={{ flexGrow: 1 }}>
                  <Skeleton height={20} variant="text" width="40%" />
                  <Skeleton height={16} variant="text" width="70%" />
                </Box>
              </Box>
            ))}
          </Stack>
        ) : error ? (
          <Alert
            action={
              onRetry ? (
                <Button color="inherit" onClick={onRetry} size="small">
                  <FormattedMessage id="summary.card.retry" defaultMessage="Retry" />
                </Button>
              ) : undefined
            }
            severity="error"
          >
            {error.message || (
              <FormattedMessage id="summary.card.error" defaultMessage="Unable to load." />
            )}
          </Alert>
        ) : isEmpty ? (
          <Stack
            alignItems="center"
            justifyContent="center"
            spacing={1.5}
            sx={{ py: 4, textAlign: 'center' }}
          >
            <Avatar
              sx={{
                bgcolor: 'action.hover',
                color: 'text.secondary',
                height: 56,
                width: 56,
              }}
            >
              {icon}
            </Avatar>
            <Typography sx={{ fontWeight: 700 }} variant="h6">
              {emptyTitle}
            </Typography>
            {emptyDescription && (
              <Typography color="text.secondary" sx={{ maxWidth: 420 }} variant="body2">
                {emptyDescription}
              </Typography>
            )}
            {emptyActionLabel && onEmptyAction && (
              <Button onClick={onEmptyAction} startIcon={<Plus size={18} />} variant="contained">
                {emptyActionLabel}
              </Button>
            )}
          </Stack>
        ) : (
          <Stack divider={<Divider />} spacing={0.5}>
            {visible.map((item) => (
              <Box
                key={item.id}
                onClick={canClick ? () => onItemClick?.(item.id) : undefined}
                sx={{
                  alignItems: 'center',
                  borderRadius: 1,
                  cursor: canClick ? 'pointer' : 'default',
                  display: 'flex',
                  gap: 1.25,
                  p: 1,
                  ...(canClick && { '&:hover': { bgcolor: 'action.hover' } }),
                }}
              >
                <Avatar
                  sx={{
                    bgcolor: 'primary.light',
                    color: 'primary.contrastText',
                    fontSize: 14,
                    height: 36,
                    width: 36,
                  }}
                >
                  {item.icon || item.avatarText || item?.title.charAt(0).toUpperCase()}
                </Avatar>
                <Box sx={{ minWidth: 0 }}>
                  <Typography noWrap sx={{ fontWeight: 600 }} variant="body1">
                    {item.title}
                  </Typography>
                  {item.description && (
                    <Typography color="text.secondary" noWrap variant="body2">
                      {item.description}
                    </Typography>
                  )}
                </Box>
                {item.meta && (
                  <Typography
                    color="text.secondary"
                    sx={{ flexShrink: 0, ml: 'auto' }}
                    variant="caption"
                  >
                    {item.meta}
                  </Typography>
                )}
              </Box>
            ))}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}
