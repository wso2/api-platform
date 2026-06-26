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
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  ListingTable,
  Skeleton,
  Stack,
  TablePagination,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronDown,
  ChevronRight,
  Inbox,
  Key,
  Plus,
  Search,
  Trash2,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type {
  ApplicationAssociation,
  MappedAPIKey,
  UserAPIKey,
} from '../../../../../utils/types';
import {
  ROWS_PER_PAGE_OPTIONS,
  dedupeMappedKeys,
  formatDate,
  getKeyStatusColor,
  resolveMappedKeyId,
} from './associationsTabUtils';

type AssociationsTableProps = {
  isLoading: boolean;
  loadError: string | null;
  hasAssociations: boolean;
  showNoSearchResults: boolean;
  filteredAssociationsCount: number;
  paginatedAssociations: ApplicationAssociation[];
  searchValue: string;
  onSearchChange: (value: string) => void;
  onOpenProviderDrawer: () => Promise<void> | void;
  onOpenProxyDrawer: () => Promise<void> | void;
  expandedIds: Set<string>;
  apiKeysMap: Map<string, MappedAPIKey[]>;
  loadingKeyIds: Set<string>;
  removingKeyIds: Set<string>;
  providerKeysMap: Map<string, UserAPIKey[]>;
  loadingProviderKeyIds: Set<string>;
  proxyKeysMap: Map<string, UserAPIKey[]>;
  loadingProxyKeyIds: Set<string>;
  unavailableKeyNames: Set<string>;
  selectionBlockedMessage?: string | null;
  page: number;
  rowsPerPage: number;
  onPageChange: (page: number) => void;
  onRowsPerPageChange: (rowsPerPage: number) => void;
  onToggleExpand: (association: ApplicationAssociation) => Promise<void> | void;
  onOpenManageKeysDrawer: (
    association: ApplicationAssociation
  ) => Promise<void> | void;
  onDeleteAssociation: (association: ApplicationAssociation) => void;
  onRemoveKey: (
    associationId: string,
    key: MappedAPIKey
  ) => Promise<void> | void;
};

function renderPrimaryActions(
  onOpenProviderDrawer: () => Promise<void> | void,
  onOpenProxyDrawer: () => Promise<void> | void
) {
  return (
    <Stack direction="row" spacing={1}>
      <Button
        variant="contained"
        size="small"
        startIcon={<Plus size={16} />}
        onClick={() => void onOpenProviderDrawer()}
      >
        Add LLM Provider
      </Button>
      <Button
        variant="contained"
        size="small"
        startIcon={<Plus size={16} />}
        onClick={() => void onOpenProxyDrawer()}
      >
        Add LLM Proxy
      </Button>
    </Stack>
  );
}

export default function AssociationsTable({
  isLoading,
  loadError,
  hasAssociations,
  showNoSearchResults,
  filteredAssociationsCount,
  paginatedAssociations,
  searchValue,
  onSearchChange,
  onOpenProviderDrawer,
  onOpenProxyDrawer,
  expandedIds,
  apiKeysMap,
  loadingKeyIds,
  removingKeyIds,
  providerKeysMap,
  loadingProviderKeyIds,
  proxyKeysMap,
  loadingProxyKeyIds,
  unavailableKeyNames,
  selectionBlockedMessage = null,
  page,
  rowsPerPage,
  onPageChange,
  onRowsPerPageChange,
  onToggleExpand,
  onOpenManageKeysDrawer,
  onDeleteAssociation,
  onRemoveKey,
}: AssociationsTableProps) {
  return (
    <ListingTable.Container sx={{ minWidth: 600 }}>
      <ListingTable.Toolbar
        showSearch={hasAssociations}
        searchValue={searchValue}
        onSearchChange={onSearchChange}
        searchPlaceholder="Search associations..."
        actions={
          hasAssociations
            ? renderPrimaryActions(onOpenProviderDrawer, onOpenProxyDrawer)
            : null
        }
      />

      {loadError ? (
        <Alert severity="error" sx={{ mx: 2, mb: 2 }}>
          {loadError}
        </Alert>
      ) : null}

      {isLoading || filteredAssociationsCount > 0 ? (
        <ListingTable>
          <ListingTable.Head>
            <ListingTable.Row>
              <ListingTable.Cell sx={{ width: 32, pr: 0 }} />
              <ListingTable.Cell>Name</ListingTable.Cell>
              <ListingTable.Cell>Version</ListingTable.Cell>
              <ListingTable.Cell>Type</ListingTable.Cell>
              <ListingTable.Cell align="right">Actions</ListingTable.Cell>
            </ListingTable.Row>
          </ListingTable.Head>
          <ListingTable.Body>
            {isLoading
              ? Array.from({ length: 3 }).map((_, index) => (
                  <ListingTable.Row key={index}>
                    <ListingTable.Cell />
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
              : paginatedAssociations.flatMap((association) => {
                  const isProvider = association.kind === 'LlmProvider';
                  const isExpanded = expandedIds.has(association.id);
                  const isLoadingKeys = loadingKeyIds.has(association.id);
                  const mappedKeys = dedupeMappedKeys(
                    apiKeysMap.get(association.id) ?? []
                  );
                  const hasLoadedMappedKeys = apiKeysMap.has(association.id);
                  const entityKeys = isProvider
                    ? providerKeysMap.get(association.id) ?? []
                    : proxyKeysMap.get(association.id) ?? [];
                  const hasLoadedEntityKeys = isProvider
                    ? providerKeysMap.has(association.id)
                    : proxyKeysMap.has(association.id);
                  const isLoadingEntityKeys = isProvider
                    ? loadingProviderKeyIds.has(association.id)
                    : loadingProxyKeyIds.has(association.id);
                  const mappedKeyIds = new Set(
                    mappedKeys.map((key) => key.keyId).filter(Boolean)
                  );
                  const canDetermineAddableKeys =
                    hasLoadedMappedKeys && hasLoadedEntityKeys;
                  const hasEntityKeys = entityKeys.some((key) =>
                    Boolean(key.name)
                  );
                  const hasAvailableKeysToAdd = entityKeys.some(
                    (key) =>
                      key.name &&
                      !mappedKeyIds.has(key.name) &&
                      !unavailableKeyNames.has(key.name)
                  );
                  const entityLabel = isProvider ? 'provider' : 'proxy';
                  const addApiKeyTooltip = selectionBlockedMessage
                    ? selectionBlockedMessage
                    : !canDetermineAddableKeys ||
                      isLoadingEntityKeys ||
                      isLoadingKeys
                    ? 'Loading API keys...'
                    : !hasEntityKeys
                    ? `No API keys available for this ${entityLabel}.`
                    : !hasAvailableKeysToAdd
                    ? `No unused API keys are available for this ${entityLabel}.`
                    : 'Add API Key';
                  const isAddApiKeyDisabled =
                    Boolean(selectionBlockedMessage) ||
                    !canDetermineAddableKeys ||
                    isLoadingEntityKeys ||
                    isLoadingKeys ||
                    !hasAvailableKeysToAdd;

                  const mainRow = (
                    <ListingTable.Row key={`assoc-${association.id}`} hover>
                      <ListingTable.Cell sx={{ pr: 0 }}>
                        <IconButton
                          size="small"
                          onClick={() => void onToggleExpand(association)}
                          aria-label={isExpanded ? 'Collapse' : 'Expand'}
                        >
                          {isExpanded ? (
                            <ChevronDown size={16} />
                          ) : (
                            <ChevronRight size={16} />
                          )}
                        </IconButton>
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        {association.name || '—'}
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <Typography variant="body2">
                          {String(association.version ?? '—')}
                        </Typography>
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        {association.kind ? (
                          <Chip
                            label={
                              association.kind === 'LlmProvider'
                                ? 'LLM Provider'
                                : 'LLM Proxy'
                            }
                            size="small"
                            variant="outlined"
                            color="primary"
                          />
                        ) : (
                          '—'
                        )}
                      </ListingTable.Cell>
                      <ListingTable.Cell align="right">
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'flex-end',
                            gap: 0.8,
                          }}
                        >
                          <Tooltip title={addApiKeyTooltip}>
                            <span>
                              <IconButton
                                size="small"
                                color="primary"
                                disabled={isAddApiKeyDisabled}
                                onClick={() =>
                                  void onOpenManageKeysDrawer(association)
                                }
                                aria-label={`Add API key for ${
                                  association.name || association.id
                                }`}
                              >
                                <Plus size={16} />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() => onDeleteAssociation(association)}
                            aria-label={`Remove ${
                              association.name || association.id
                            }`}
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </Box>
                      </ListingTable.Cell>
                    </ListingTable.Row>
                  );

                  if (!isExpanded) return [mainRow];

                  const sectionTitleRow = (
                    <ListingTable.Row key={`${association.id}-section-title`}>
                      <ListingTable.Cell />
                      <ListingTable.Cell colSpan={4}>
                        <Box sx={{ pl: 2, pt: 0.75, pb: 0.25 }}>
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            sx={{
                              display: 'block',
                              fontWeight: 600,
                              letterSpacing: 0.4,
                            }}
                          >
                            Associated API Keys
                          </Typography>
                        </Box>
                      </ListingTable.Cell>
                    </ListingTable.Row>
                  );

                  const keyRows = isLoadingKeys
                    ? [
                        <ListingTable.Row key={`${association.id}-loading`}>
                          <ListingTable.Cell />
                          <ListingTable.Cell colSpan={4}>
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 1,
                                pl: 2,
                                py: 0.5,
                              }}
                            >
                              <CircularProgress size={14} />
                              <Typography
                                variant="caption"
                                color="text.secondary"
                              >
                                Loading API keys...
                              </Typography>
                            </Box>
                          </ListingTable.Cell>
                        </ListingTable.Row>,
                      ]
                    : mappedKeys.length === 0
                    ? [
                        <ListingTable.Row key={`${association.id}-empty`}>
                          <ListingTable.Cell />
                          <ListingTable.Cell colSpan={4}>
                            <Typography
                              variant="caption"
                              color="text.secondary"
                              sx={{ pl: 2, py: 0.5, display: 'block' }}
                            >
                              No API keys mapped to this {entityLabel}.
                            </Typography>
                          </ListingTable.Cell>
                        </ListingTable.Row>,
                      ]
                    : mappedKeys.map((key) => (
                        <ListingTable.Row
                          key={`apikey-${association.id}-${key.keyId}`}
                          sx={{ bgcolor: 'action.hover' }}
                        >
                          <ListingTable.Cell />
                          <ListingTable.Cell>
                            <Stack
                              direction="row"
                              spacing={0.75}
                              alignItems="center"
                              sx={{ pl: 2 }}
                            >
                              <Key size={14} color="disabled" />
                              <Typography variant="caption" fontWeight={500}>
                                {key.keyId || '—'}
                              </Typography>
                            </Stack>
                          </ListingTable.Cell>
                          <ListingTable.Cell>
                            {key.status ? (
                              <Chip
                                label={key.status}
                                size="small"
                                variant="outlined"
                                color={getKeyStatusColor(key.status)}
                              />
                            ) : (
                              '—'
                            )}
                          </ListingTable.Cell>
                          <ListingTable.Cell>
                            <Typography
                              variant="caption"
                              color="text.secondary"
                            >
                              Expired At {formatDate(key.expiresAt)}
                            </Typography>
                          </ListingTable.Cell>
                          <ListingTable.Cell>
                            <Tooltip title="Remove API key">
                              <span>
                                <IconButton
                                  size="small"
                                  color="error"
                                  disabled={removingKeyIds.has(
                                    resolveMappedKeyId(key)
                                  )}
                                  onClick={() =>
                                    void onRemoveKey(association.id, key)
                                  }
                                  aria-label={`Remove API key ${key.keyId}`}
                                >
                                  {removingKeyIds.has(resolveMappedKeyId(key)) ? (
                                    <CircularProgress size={14} />
                                  ) : (
                                    <X size={14} />
                                  )}
                                </IconButton>
                              </span>
                            </Tooltip>
                          </ListingTable.Cell>
                        </ListingTable.Row>
                      ));

                  return [mainRow, sectionTitleRow, ...keyRows];
                })}
          </ListingTable.Body>
        </ListingTable>
      ) : (
        <ListingTable.EmptyState
          illustration={
            showNoSearchResults ? <Search size={64} /> : <Inbox size={64} />
          }
          title={showNoSearchResults ? 'No associations found' : 'No Associations'}
          description={
            showNoSearchResults
              ? 'No associations match your search.'
              : 'Associate LLM providers and proxies to enable AI capabilities for this application.'
          }
          action={
            showNoSearchResults ? (
              <Button variant="outlined" onClick={() => onSearchChange('')}>
                Clear search
              </Button>
            ) : (
              renderPrimaryActions(onOpenProviderDrawer, onOpenProxyDrawer)
            )
          }
        />
      )}

      {filteredAssociationsCount > 0 ? (
        <TablePagination
          component="div"
          count={filteredAssociationsCount}
          page={page}
          onPageChange={(_, nextPage) => onPageChange(nextPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(event) =>
            onRowsPerPageChange(parseInt(event.target.value, 10))
          }
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        />
      ) : null}
    </ListingTable.Container>
  );
}
