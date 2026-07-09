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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Drawer,
  IconButton,
  ListingTable,
  Skeleton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Inbox, Plus, Search, Trash2, X } from '@wso2/oxygen-ui-icons-react';
import { useApplications } from '../../../../../contexts/ApplicationsContext';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../../hooks/aiWorkspaceSnackbar';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import { keyManagementApis } from '../../../../../apis/keyManagementApis';
import type { MappedAPIKey, UserAPIKey } from '../../../../../utils/types';
import { getErrorMessage } from '../../../../../utils/apiError';

type APIKeyTabProps = {
  applicationId: string;
};

function getErrorDescription(error: unknown, fallback: string): string {
  return getErrorMessage(error, fallback);
}

function formatDate(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

function getStatusChipColor(
  status?: string
): 'success' | 'warning' | 'error' | 'default' {
  const normalized = (status ?? '').trim().toLowerCase();
  if (normalized === 'active') return 'success';
  if (normalized === 'pending') return 'warning';
  if (
    normalized === 'inactive' ||
    normalized === 'expired' ||
    normalized === 'revoked'
  ) {
    return 'error';
  }
  return 'default';
}

function resolveMappedKeyId(key: MappedAPIKey): string {
  const candidate = key as MappedAPIKey & { mappedKeyId?: string };
  return candidate.mappedKeyId || key.keyId;
}

function formatAssociatedEntityKind(kind?: string): string {
  if (!kind) return '—';
  if (kind === 'LlmProvider') return 'LLM Provider';
  if (kind === 'LlmProxy') return 'App LLM Proxy';
  return kind;
}

function resolveEntityId(key: MappedAPIKey): string | undefined {
  const candidate = key as MappedAPIKey & {
    entityID?: string;
    entityId?: string;
  };
  return (
    candidate.entityID || candidate.entityId || candidate.associatedEntity?.id
  );
}

export default function APIKeyTab({ applicationId }: APIKeyTabProps) {
  const { getApplicationAPIKeys, addApplicationAPIKeys, removeApplicationAPIKey } =
    useApplications();
  const { currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  const [searchValue, setSearchValue] = useState('');
  const [apiKeys, setApiKeys] = useState<MappedAPIKey[]>([]);
  const [isApiKeysLoading, setIsApiKeysLoading] = useState(false);
  const [apiKeysError, setApiKeysError] = useState<string | null>(null);

  const [addDrawerOpen, setAddDrawerOpen] = useState(false);
  const [availableKeys, setAvailableKeys] = useState<UserAPIKey[]>([]);
  const [isLoadingAvailableKeys, setIsLoadingAvailableKeys] = useState(false);
  const [availableKeysSearchValue, setAvailableKeysSearchValue] = useState('');
  const [selectedKeyIds, setSelectedKeyIds] = useState<Set<string>>(new Set());
  const [isAddingKeys, setIsAddingKeys] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<MappedAPIKey | null>(null);
  const [isRemovingKey, setIsRemovingKey] = useState(false);

  const filteredApiKeys = useMemo(() => {
    const query = searchValue.trim().toLowerCase();
    if (!query) return apiKeys;

    return apiKeys.filter((key) => {
      const haystack = [
        key.keyId,
        key.associatedEntity?.id,
        key.associatedEntity?.kind,
        key.status,
        key.expiresAt,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [apiKeys, searchValue]);

  const alreadyMappedKeyIds = useMemo(
    () => new Set(apiKeys.map((key) => key.keyId).filter(Boolean)),
    [apiKeys]
  );

  const filteredAvailableKeys = useMemo(() => {
    const activeKeys = availableKeys.filter((key) => key.status === 'active');
    const query = availableKeysSearchValue.trim().toLowerCase();
    if (!query) return activeKeys;

    return activeKeys.filter((key) => {
      const haystack = [key.displayName, key.id, key.artifactId, key.artifactType]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [availableKeys, availableKeysSearchValue]);

  const loadApplicationApiKeys = useCallback(async () => {
    if (!applicationId) {
      setApiKeys([]);
      return;
    }

    try {
      setIsApiKeysLoading(true);
      setApiKeysError(null);

      const response = await getApplicationAPIKeys(applicationId);
      setApiKeys(response.list ?? []);
    } catch (error) {
      setApiKeysError(
        getErrorDescription(error, 'Failed to fetch mapped API keys.')
      );
      setApiKeys([]);
    } finally {
      setIsApiKeysLoading(false);
    }
  }, [applicationId, getApplicationAPIKeys]);

  useEffect(() => {
    if (!applicationId) return;
    void loadApplicationApiKeys();
  }, [applicationId, loadApplicationApiKeys]);

  const handleOpenAddDrawer = async () => {
    setAddDrawerOpen(true);
    setSelectedKeyIds(new Set());
    setAvailableKeysSearchValue('');
    setIsLoadingAvailableKeys(true);
    try {
      const response = await keyManagementApis.listUserAPIKeys(
        currentOrganization?.uuid ?? '',
        apimBaseUrl
      );
      setAvailableKeys(response.items ?? []);
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to load available API keys.'),
        'error'
      );
    } finally {
      setIsLoadingAvailableKeys(false);
    }
  };

  const handleToggleKey = (keyId: string) => {
    setSelectedKeyIds((prev) => {
      const next = new Set(prev);
      if (next.has(keyId)) {
        next.delete(keyId);
      } else {
        next.add(keyId);
      }
      return next;
    });
  };

  const handleAddSelectedKeys = async () => {
    if (selectedKeyIds.size === 0 || isAddingKeys) return;

    try {
      setIsAddingKeys(true);

      const keysToAdd = Array.from(selectedKeyIds).map((keyName) => {
        const matched = availableKeys.find((k) => k.id === keyName);
        return {
          keyId: keyName,
          associatedEntity: { id: matched?.artifactId ?? '' },
        };
      });

      await addApplicationAPIKeys(applicationId, { apiKeys: keysToAdd });

      setAddDrawerOpen(false);
      setSelectedKeyIds(new Set());
      showSnackbar('API keys mapped successfully.', 'success');
      await loadApplicationApiKeys();
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to map API keys.'),
        'error'
      );
    } finally {
      setIsAddingKeys(false);
    }
  };

  const handleRemoveApiKey = async () => {
    if (!deleteTarget || isRemovingKey) return;

    const mappedKeyId = resolveMappedKeyId(deleteTarget);
    if (!mappedKeyId) {
      showSnackbar('Mapped key id is missing.', 'error');
      return;
    }

    const entityID = resolveEntityId(deleteTarget);
    if (!entityID) {
      showSnackbar('Associated entity id is missing.', 'error');
      return;
    }

    try {
      setIsRemovingKey(true);

      await removeApplicationAPIKey(applicationId, mappedKeyId, {
        entityID,
      });

      setDeleteTarget(null);
      await loadApplicationApiKeys();
      showSnackbar('API key mapping removed successfully.', 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to remove API key mapping.'),
        'error'
      );
    } finally {
      setIsRemovingKey(false);
    }
  };

  const hasApiKeys = apiKeys.length > 0;
  const hasApiKeySearchQuery = searchValue.trim().length > 0;
  const hasFilteredApiKeys = filteredApiKeys.length > 0;
  const showNoSearchResults =
    hasApiKeys && hasApiKeySearchQuery && !hasFilteredApiKeys;

  return (
    <>
      <ListingTable.Container sx={{ minWidth: 700 }}>
        <ListingTable.Toolbar
          showSearch={hasApiKeys}
          searchValue={searchValue}
          onSearchChange={setSearchValue}
          searchPlaceholder="Search mapped API keys..."
          actions={
            hasApiKeys ? (
              <Button
                variant="contained"
                size="small"
                startIcon={<Plus size={16} />}
                onClick={handleOpenAddDrawer}
              >
                Add API Key
              </Button>
            ) : null
          }
        />

        {apiKeysError ? (
          <Alert severity="error" sx={{ mx: 2, mb: 2 }}>
            {apiKeysError}
          </Alert>
        ) : null}

        {isApiKeysLoading || hasFilteredApiKeys ? (
          <ListingTable>
            <ListingTable.Head>
              <ListingTable.Row>
                <ListingTable.Cell>API Key Name</ListingTable.Cell>
                <ListingTable.Cell>Component Name</ListingTable.Cell>
                <ListingTable.Cell>Component Type</ListingTable.Cell>
                <ListingTable.Cell>Status</ListingTable.Cell>
                <ListingTable.Cell>Expires At</ListingTable.Cell>
                <ListingTable.Cell align="right">Actions</ListingTable.Cell>
              </ListingTable.Row>
            </ListingTable.Head>

            <ListingTable.Body>
              {isApiKeysLoading
                ? Array.from({ length: 3 }).map((_, i) => (
                    // eslint-disable-next-line react/no-array-index-key
                    <ListingTable.Row key={i}>
                      <ListingTable.Cell>
                        <Skeleton variant="text" width="80%" />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Skeleton variant="text" width="70%" />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Skeleton variant="text" width="60%" />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Skeleton variant="text" width="50%" />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Skeleton variant="text" width="40%" />
                      </ListingTable.Cell>
                      <ListingTable.Cell align="right">
                        <Skeleton variant="circular" width={24} height={24} />
                      </ListingTable.Cell>
                    </ListingTable.Row>
                  ))
                : filteredApiKeys.map((key) => (
                    <ListingTable.Row key={resolveMappedKeyId(key)} hover>
                      <ListingTable.Cell>
                        {key.keyId || '—'}
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        {key.associatedEntity?.id || '—'}
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        {formatAssociatedEntityKind(key.associatedEntity?.kind)}
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Chip
                          label={key.status || 'unknown'}
                          size="small"
                          variant="outlined"
                          color={getStatusChipColor(key.status)}
                        />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Tooltip
                          title={
                            key.expiresAt
                              ? new Date(key.expiresAt).toUTCString()
                              : ''
                          }
                        >
                          <span>{formatDate(key.expiresAt)}</span>
                        </Tooltip>
                      </ListingTable.Cell>
                      <ListingTable.Cell align="right">
                        <IconButton
                          size="small"
                          color="error"
                          onClick={() => setDeleteTarget(key)}
                          aria-label={`Delete ${key.keyId}`}
                        >
                          <Trash2 size={16} />
                        </IconButton>
                      </ListingTable.Cell>
                    </ListingTable.Row>
                  ))}
            </ListingTable.Body>
          </ListingTable>
        ) : (
          <ListingTable.EmptyState
            illustration={
              showNoSearchResults ? <Search size={64} /> : <Inbox size={64} />
            }
            title={
              showNoSearchResults ? 'No API keys found' : 'No API keys yet'
            }
            description={
              showNoSearchResults
                ? 'No mapped API keys match your search.'
                : 'Get started by adding your first API key mapping.'
            }
            action={
              showNoSearchResults ? (
                <Button variant="outlined" onClick={() => setSearchValue('')}>
                  Clear search
                </Button>
              ) : (
                <Button
                  variant="contained"
                  startIcon={<Plus size={16} />}
                  onClick={handleOpenAddDrawer}
                >
                  Add API Key
                </Button>
              )
            }
          />
        )}
      </ListingTable.Container>

      <Drawer
        anchor="right"
        open={addDrawerOpen}
        onClose={() => {
          if (!isAddingKeys) {
            setAddDrawerOpen(false);
          }
        }}
        sx={{
          '& .MuiDrawer-paper': {
            width: { xs: '100%', sm: 480 },
            maxWidth: '100%',
          },
        }}
      >
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            p: 2,
            borderBottom: 1,
            borderColor: 'divider',
          }}
        >
          <Typography variant="h6">Add API Keys</Typography>
          <IconButton
            onClick={() => setAddDrawerOpen(false)}
            disabled={isAddingKeys}
            size="small"
          >
            <X size={20} />
          </IconButton>
        </Box>

        <Box sx={{ flex: 1, overflow: 'auto', p: 2 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search API keys..."
            value={availableKeysSearchValue}
            onChange={(event) =>
              setAvailableKeysSearchValue(event.target.value)
            }
            sx={{ mb: 2 }}
            slotProps={{
              input: {
                startAdornment: <Search size={18} />,
              },
            }}
          />

          {isLoadingAvailableKeys ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <CircularProgress />
            </Box>
          ) : filteredAvailableKeys.length === 0 ? (
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ py: 4, textAlign: 'center' }}
            >
              {availableKeysSearchValue.trim()
                ? 'No API keys match your search.'
                : 'No available API keys found.'}
            </Typography>
          ) : (
            <Stack spacing={1.5}>
              {filteredAvailableKeys.map((key) => {
                const keyIdentifier = key.id ?? '';
                const isAlreadyAdded = alreadyMappedKeyIds.has(keyIdentifier);
                const isSelected = selectedKeyIds.has(keyIdentifier);
                return (
                  <Card
                    key={keyIdentifier}
                    sx={{
                      cursor: isAlreadyAdded ? 'not-allowed' : 'pointer',
                      border: 1,
                      borderColor:
                        isSelected && !isAlreadyAdded
                          ? 'primary.main'
                          : 'divider',
                      bgcolor:
                        isSelected && !isAlreadyAdded
                          ? 'action.selected'
                          : 'background.paper',
                      opacity: isAlreadyAdded ? 0.6 : 1,
                      '&:hover': {
                        borderColor: isAlreadyAdded ? 'divider' : 'primary.main',
                      },
                    }}
                    onClick={() => {
                      if (isAlreadyAdded) return;
                      handleToggleKey(keyIdentifier);
                    }}
                  >
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        p: 1.5,
                        gap: 1.5,
                      }}
                    >
                      <Checkbox
                        checked={isAlreadyAdded || isSelected}
                        disabled={isAlreadyAdded}
                        size="small"
                        tabIndex={-1}
                        disableRipple
                      />
                      <Stack spacing={0.25} sx={{ minWidth: 0, flex: 1 }}>
                        <Typography variant="body2" fontWeight={600} noWrap>
                          {key.displayName || key.id || '—'}
                        </Typography>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          noWrap
                        >
                          {key.artifactId
                            ? key.artifactId.length > 30
                              ? `${key.artifactId.slice(0, 30)}...`
                              : key.artifactId
                            : '—'}
                        </Typography>
                      </Stack>
                      {key.artifactType && (
                        <Chip
                          label={key.artifactType}
                          size="small"
                          variant="outlined"
                          sx={{ flexShrink: 0 }}
                          color="primary"
                        />
                      )}
                    </Box>
                  </Card>
                );
              })}
            </Stack>
          )}
        </Box>

        <Box
          sx={{
            p: 2,
            borderTop: 1,
            borderColor: 'divider',
            display: 'flex',
            justifyContent: 'flex-start',
            gap: 1,
          }}
        >
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setAddDrawerOpen(false)}
            disabled={isAddingKeys}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleAddSelectedKeys}
            disabled={selectedKeyIds.size === 0 || isAddingKeys}
          >
            {isAddingKeys ? 'Adding...' : `Add (${selectedKeyIds.size})`}
          </Button>
        </Box>
      </Drawer>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => {
          if (!isRemovingKey) {
            setDeleteTarget(null);
          }
        }}
      >
        <DialogTitle>Remove API key mapping</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to remove mapping{' '}
            <strong>
              {deleteTarget ? resolveMappedKeyId(deleteTarget) : ''}
            </strong>
            ?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setDeleteTarget(null)}
            disabled={isRemovingKey}
          >
            Cancel
          </Button>
          <Button
            color="error"
            onClick={handleRemoveApiKey}
            disabled={isRemovingKey}
          >
            {isRemovingKey ? 'Removing...' : 'Remove'}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
