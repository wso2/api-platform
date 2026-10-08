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
import { useState } from 'react';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  IconButton,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { GatewayArtifactDeleteWarning } from '../../../../utils/readOnlyArtifacts';
import { formatRelativeTime } from '../../../../contexts/ApplicationsContext';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import {
  DISABLED_ACTION_SX,
  NO_PERMISSION_TOOLTIP,
} from '../../../../auth/permissions';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage } from '../../../../utils/apiError';

/** Rows shown inline before the panel defers to the full listing page. */
const ITEM_PREVIEW_COUNT = 5;

/** A row is the name over its subtitle; the gap is the stack spacing either
 * side of the divider. The list area holds a full preview's worth of rows so
 * the panel keeps one height however many a kind actually has. */
const ITEM_ROW_HEIGHT = 40;
const ITEM_ROW_GAP = 25;
const LIST_MIN_HEIGHT =
  ITEM_PREVIEW_COUNT * ITEM_ROW_HEIGHT + (ITEM_PREVIEW_COUNT - 1) * ITEM_ROW_GAP;

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
  /** Chip shown beside the name, e.g. the provider template a resource uses. */
  chipLabel?: string;
  chipLogo?: string;
  /** Gateway-created resources warn before deletion. */
  readOnly?: boolean;
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
  /** Singular, user-facing name of the kind, used in the delete dialog. */
  itemLabel: string;
  canDelete: boolean;
  onItemDelete: (id: string) => Promise<void>;
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
  itemLabel,
  canDelete,
  onItemDelete,
}: KindDetailPanelProps): React.JSX.Element {
  const [deleteTarget, setDeleteTarget] = useState<KindDetailItem | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const showSnackbar = useAIWorkspaceSnackbar();

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await onItemDelete(deleteTarget.id);
      showSnackbar(`${itemLabel} deleted successfully.`, 'success');
    } catch (error) {
      showSnackbar(
        getErrorMessage(error, `Failed to delete ${itemLabel}.`),
        'error'
      );
    } finally {
      setIsDeleting(false);
      setDeleteTarget(null);
    }
  };

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

        <Box sx={{ minHeight: LIST_MIN_HEIGHT }}>
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
                      <Stack
                        direction="row"
                        spacing={1}
                        alignItems="center"
                        flexWrap="wrap"
                      >
                        <Typography
                          variant="body1"
                          sx={{ fontWeight: 600 }}
                          noWrap
                        >
                          {truncateWords(item.displayName || 'No Name', 12)}
                        </Typography>
                        {item.chipLabel ? (
                          <Chip
                            label={` ${item.chipLabel}`}
                            size="small"
                            variant="outlined"
                            color="primary"
                            sx={{ borderRadius: 0.5 }}
                            icon={
                              item.chipLogo ? (
                                <Box
                                  component="img"
                                  src={item.chipLogo}
                                  alt=""
                                  sx={{
                                    width: 16,
                                    height: 16,
                                    objectFit: 'contain',
                                  }}
                                />
                              ) : undefined
                            }
                          />
                        ) : null}
                      </Stack>
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

                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                  >
                    {item.updatedAt ? (
                      <Stack direction="row" spacing={0.75} alignItems="center">
                        <Clock size={14} />
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          noWrap
                        >
                          {formatRelativeTime(item.updatedAt)}
                        </Typography>
                      </Stack>
                    ) : null}
                    <Tooltip title={canDelete ? '' : NO_PERMISSION_TOOLTIP}>
                      <Box component="span">
                        <IconButton
                          size="small"
                          color="error"
                          disabled={!canDelete}
                          onClick={(event) => {
                            event.stopPropagation();
                            setDeleteTarget(item);
                          }}
                          aria-label={`Delete ${item.displayName}`}
                        >
                          <Trash2 size={16} />
                        </IconButton>
                      </Box>
                    </Tooltip>
                  </Stack>
                </Box>
              ))}
            </Stack>
          )}
        </Box>
      </CardContent>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
        }}
      >
        <DialogTitle>Delete {itemLabel}</DialogTitle>
        <DialogContent>
          {deleteTarget?.readOnly ? (
            <GatewayArtifactDeleteWarning
              artifactType={itemLabel}
              artifactName={deleteTarget.displayName}
            />
          ) : null}
          <DialogContentText>
            Are you sure you want to delete {deleteTarget?.displayName}?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            disabled={isDeleting}
            onClick={() => setDeleteTarget(null)}
          >
            Cancel
          </Button>
          <Button
            color="error"
            disabled={isDeleting}
            onClick={() => void handleDeleteConfirm()}
          >
            {isDeleting ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
}
