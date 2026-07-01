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

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { Dispatch, ReactNode, SetStateAction } from 'react';
import {
  Alert,
  Avatar,
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
  TablePagination,
  TextField,
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
import { useApplicationAssociations } from '../../../../../contexts/ApplicationAssociationsContext';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../../hooks/aiWorkspaceSnackbar';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import {
  getLLMProviders,
  getLLMProviderAPIKeys,
} from '../../../../../apis/llmProviderApis';
import { getOrgProxies, getProxies } from '../../../../../apis/proxyApis';
import { getLLMProxyAPIKeys } from '../../../../../apis/llmProxiesApis';
import { getProviderTemplateDisplayName } from '../../../../../utils/providerTemplateDisplay';
import type {
  ApplicationAssociation,
  LLMProvider,
  MappedAPIKey,
  Proxy,
  UserAPIKey,
} from '../../../../../utils/types';

import AnthropicLogo from '../../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../../assets/brands/openAI.png';

const TEMPLATE_LOGO_MAP: Record<string, string> = {
  openai: OpenAILogo,
  anthropic: AnthropicLogo,
  'azure-openai': AzureLogo,
  'azureai-foundry': AzureLogo,
  'aws-bedrock': AWSBedrockLogo,
  awsbedrock: AWSBedrockLogo,
  'google-vertex': GoogleVertexLogo,
  gemini: GoogleGeminiLogo,
  mistralai: MistralAILogo,
  mistral: MistralAILogo,
};

function getTemplateLogo(template?: string): string | undefined {
  return TEMPLATE_LOGO_MAP[(template ?? '').trim().toLowerCase()];
}

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

function getErrorDescription(error: unknown, fallback: string): string {
  return (
    (error as any)?.response?.data?.description ||
    (error as any)?.response?.data?.message ||
    (error instanceof Error ? error.message : null) ||
    fallback
  );
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

function getKeyStatusColor(
  status?: string
): 'success' | 'warning' | 'error' | 'default' {
  const s = (status ?? '').toLowerCase();
  if (s === 'active') return 'success';
  if (s === 'pending') return 'warning';
  if (s === 'inactive' || s === 'expired' || s === 'revoked') return 'error';
  return 'default';
}

function dedupeMappedKeys(keys: MappedAPIKey[]): MappedAPIKey[] {
  const seen = new Set<string>();
  return keys.filter((key) => {
    const keyId = key.keyId || '';
    if (!keyId || seen.has(keyId)) return false;
    seen.add(keyId);
    return true;
  });
}

function resolveMappedKeyId(key: MappedAPIKey): string {
  const candidate = key as MappedAPIKey & { mappedKeyId?: string };
  return candidate.mappedKeyId || key.keyId;
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

const ROWS_PER_PAGE_OPTIONS = [5, 10, 25];

type DrawerEntity = {
  id: string;
  displayName: string;
  description?: string;
};

type SelectionDrawerItemMeta = {
  chip?: ReactNode;
  emptyKeysText: string;
};

type SelectableKeyListProps = {
  keys: UserAPIKey[];
  selectedKeyNames: Set<string>;
  lockedKeyNames?: Set<string>;
  keyStatusByName?: Map<string, string | undefined>;
  emptyText: string;
  onToggleKey: (keyName: string) => void;
};

type AssociationSelectionDrawerProps<T extends DrawerEntity> = {
  open: boolean;
  title: string;
  description: string;
  searchPlaceholder: string;
  searchValue: string;
  onSearchChange: (value: string) => void;
  onClose: () => void;
  isSubmitting: boolean;
  isLoading: boolean;
  loadError: string | null;
  items: T[];
  emptyStateText: string;
  emptySearchText: string;
  linkedIds: Set<string>;
  selectedIds: Set<string>;
  expandedLinkedIds: Set<string>;
  entityKeysMap: Map<string, UserAPIKey[]>;
  mappedKeysMap: Map<string, MappedAPIKey[]>;
  loadingMappedKeyIds: Set<string>;
  loadingEntityKeyIds: Set<string>;
  selectedKeyNamesMap: Map<string, Set<string>>;
  onItemClick: (item: T) => Promise<void> | void;
  onToggleKey: (entityId: string, keyName: string) => void;
  getItemMeta: (item: T) => SelectionDrawerItemMeta;
  addButtonLabel: string;
  isAddDisabled: boolean;
  onAdd: () => Promise<void> | void;
};

type LoadEntityKeysArgs = {
  entityId: string;
  orgUuid: string;
  keysMap: Map<string, UserAPIKey[]>;
  loadingIds: Set<string>;
  setLoadingIds: Dispatch<SetStateAction<Set<string>>>;
  setKeysMap: Dispatch<SetStateAction<Map<string, UserAPIKey[]>>>;
  fetchKeys: (
    entityId: string,
    orgUuid: string
  ) => Promise<{
    items?: UserAPIKey[];
  }>;
  preselectLatest?: boolean;
  setSelectedKeyNamesMap?: Dispatch<SetStateAction<Map<string, Set<string>>>>;
};

function getLatestActiveKey(keys: UserAPIKey[]): UserAPIKey | null {
  return keys.reduce<UserAPIKey | null>((latest, key) => {
    if (!latest) return key;
    return new Date(key.createdAt ?? 0).getTime() >
      new Date(latest.createdAt ?? 0).getTime()
      ? key
      : latest;
  }, null);
}

function toggleSetValue<T>(source: Set<T>, value: T): Set<T> {
  const next = new Set(source);
  if (next.has(value)) {
    next.delete(value);
  } else {
    next.add(value);
  }
  return next;
}

function removeSetValue<T>(source: Set<T>, value: T): Set<T> {
  const next = new Set(source);
  next.delete(value);
  return next;
}

function updateMapSelection(
  source: Map<string, Set<string>>,
  entityId: string,
  nextValues: Set<string>
): Map<string, Set<string>> {
  const next = new Map(source);
  next.set(entityId, nextValues);
  return next;
}

function toggleMapSelectionValue(
  source: Map<string, Set<string>>,
  entityId: string,
  keyName: string
): Map<string, Set<string>> {
  const current = source.get(entityId) ?? new Set<string>();
  return updateMapSelection(source, entityId, toggleSetValue(current, keyName));
}

function filterItemsByQuery<T>(
  items: T[],
  query: string,
  getSearchFields: (item: T) => Array<string | undefined>
): T[] {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) return items;

  return items.filter((item) =>
    getSearchFields(item)
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(normalizedQuery)
  );
}

function buildLinkedKeyPayload(
  selectedKeyNamesMap: Map<string, Set<string>>,
  linkedIds: Set<string>,
  apiKeysMap: Map<string, MappedAPIKey[]>
) {
  return Array.from(selectedKeyNamesMap.entries())
    .filter(([entityId]) => linkedIds.has(entityId))
    .flatMap(([entityId, keyNames]) => {
      const mappedKeyIds = new Set(
        (apiKeysMap.get(entityId) ?? []).map((key) => key.keyId)
      );
      return Array.from(keyNames)
        .filter((keyName) => !mappedKeyIds.has(keyName))
        .map((keyName) => ({
          keyId: keyName,
          associatedEntity: { id: entityId },
        }));
    });
}

function buildAddButtonLabel({
  isSubmitting,
  selectedCount,
  pendingKeyCount,
  entityLabel,
  defaultLabel,
}: {
  isSubmitting: boolean;
  selectedCount: number;
  pendingKeyCount: number;
  entityLabel: string;
  defaultLabel: string;
}): string {
  if (isSubmitting) return 'Adding...';
  if (selectedCount > 0 && pendingKeyCount > 0) {
    return `Add ${selectedCount} ${entityLabel}${
      selectedCount > 1 ? 's' : ''
    } and ${pendingKeyCount} Key${pendingKeyCount > 1 ? 's' : ''}`;
  }
  if (selectedCount > 0) {
    return `Add ${selectedCount} ${entityLabel}${selectedCount > 1 ? 's' : ''}`;
  }
  if (pendingKeyCount > 0) {
    return `Add ${pendingKeyCount} Key${pendingKeyCount > 1 ? 's' : ''}`;
  }
  return defaultLabel;
}

function getVisibleKeys(
  entityKeys: UserAPIKey[],
  mappedKeys: MappedAPIKey[],
  includeMappedOnlyKeys: boolean
): UserAPIKey[] {
  if (!includeMappedOnlyKeys) return entityKeys;

  const mappedOnlyKeys: UserAPIKey[] = mappedKeys
    .filter(
      (mappedKey) =>
        !entityKeys.some(
          (entityKey) => (entityKey.name ?? '') === mappedKey.keyId
        )
    )
    .map((mappedKey) => ({
      name: mappedKey.keyId,
      status: mappedKey.status,
    }));

  return [...entityKeys, ...mappedOnlyKeys];
}

async function loadEntityKeys({
  entityId,
  orgUuid,
  keysMap,
  loadingIds,
  setLoadingIds,
  setKeysMap,
  fetchKeys,
  preselectLatest = false,
  setSelectedKeyNamesMap,
}: LoadEntityKeysArgs): Promise<void> {
  if (keysMap.has(entityId)) {
    if (preselectLatest && setSelectedKeyNamesMap) {
      const latestKey = getLatestActiveKey(keysMap.get(entityId) ?? []);
      setSelectedKeyNamesMap((prev) =>
        updateMapSelection(
          prev,
          entityId,
          latestKey?.name ? new Set([latestKey.name]) : new Set()
        )
      );
    }
    return;
  }

  if (loadingIds.has(entityId)) return;

  setLoadingIds((prev) => new Set(prev).add(entityId));

  try {
    const response = await fetchKeys(entityId, orgUuid);
    const activeKeys = (response.items ?? []).filter(
      (key) => key.status === 'active'
    );
    const latestKey = getLatestActiveKey(activeKeys);

    setKeysMap((prev) => new Map(prev).set(entityId, activeKeys));
    if (preselectLatest && setSelectedKeyNamesMap) {
      setSelectedKeyNamesMap((prev) =>
        updateMapSelection(
          prev,
          entityId,
          latestKey?.name ? new Set([latestKey.name]) : new Set()
        )
      );
    }
  } catch {
    setKeysMap((prev) => new Map(prev).set(entityId, []));
    if (preselectLatest && setSelectedKeyNamesMap) {
      setSelectedKeyNamesMap((prev) =>
        updateMapSelection(prev, entityId, new Set())
      );
    }
  } finally {
    setLoadingIds((prev) => removeSetValue(prev, entityId));
  }
}

function SelectableKeyList({
  keys,
  selectedKeyNames,
  lockedKeyNames = new Set<string>(),
  keyStatusByName = new Map(),
  emptyText,
  onToggleKey,
}: SelectableKeyListProps) {
  if (keys.length === 0) {
    return (
      <Typography variant="caption" color="text.secondary">
        {emptyText}
      </Typography>
    );
  }

  return (
    <Stack spacing={0.25}>
      {keys.map((key) => {
        const keyName = key.name ?? '';
        const isLocked = lockedKeyNames.has(keyName);
        const isSelected = selectedKeyNames.has(keyName);
        const keyStatus = keyStatusByName.get(keyName);

        return (
          <Box
            key={keyName}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1,
              py: 0.75,
              px: 0.5,
              borderRadius: 1,
              cursor: isLocked ? 'default' : 'pointer',
              '&:hover': isLocked ? undefined : { bgcolor: 'action.hover' },
            }}
            onClick={(event) => {
              event.stopPropagation();
              if (isLocked) return;
              onToggleKey(keyName);
            }}
          >
            <Checkbox
              checked={isSelected}
              disabled={isLocked}
              size="small"
              tabIndex={-1}
              disableRipple
              sx={{ p: 0 }}
            />
            <Key size={14} />
            <Typography
              variant="caption"
              fontWeight={500}
              noWrap
              sx={{ flex: 1 }}
            >
              {key.name || '—'}
            </Typography>
            {keyStatus && (
              <Chip
                label={keyStatus}
                size="small"
                variant="outlined"
                color={getKeyStatusColor(keyStatus)}
                sx={{ flexShrink: 0 }}
              />
            )}
            {key.artifactType && (
              <Chip
                label={key.artifactType}
                size="small"
                variant="outlined"
                color="primary"
                sx={{ flexShrink: 0 }}
              />
            )}
          </Box>
        );
      })}
    </Stack>
  );
}

function AssociationSelectionDrawer<T extends DrawerEntity>({
  open,
  title,
  description,
  searchPlaceholder,
  searchValue,
  onSearchChange,
  onClose,
  isSubmitting,
  isLoading,
  loadError,
  items,
  emptyStateText,
  emptySearchText,
  linkedIds,
  selectedIds,
  expandedLinkedIds,
  entityKeysMap,
  mappedKeysMap,
  loadingMappedKeyIds,
  loadingEntityKeyIds,
  selectedKeyNamesMap,
  onItemClick,
  onToggleKey,
  getItemMeta,
  addButtonLabel,
  isAddDisabled,
  onAdd,
}: AssociationSelectionDrawerProps<T>) {
  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      sx={{
        '& .MuiDrawer-paper': {
          width: { xs: '100%', sm: 520 },
          maxWidth: '100%',
          display: 'flex',
          flexDirection: 'column',
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
          flexShrink: 0,
        }}
      >
        <Stack spacing={0.25}>
          <Typography variant="h6">{title}</Typography>
          <Typography variant="caption" color="text.secondary">
            {description}
          </Typography>
        </Stack>
        <IconButton onClick={onClose} disabled={isSubmitting} size="small">
          <X size={20} />
        </IconButton>
      </Box>

      <Box sx={{ p: 2, flexShrink: 0, pb: 0 }}>
        <TextField
          fullWidth
          size="small"
          placeholder={searchPlaceholder}
          value={searchValue}
          onChange={(event) => onSearchChange(event.target.value)}
          slotProps={{ input: { startAdornment: <Search size={18} /> } }}
        />
      </Box>

      <Box sx={{ flex: 1, overflow: 'auto', p: 2 }}>
        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress />
          </Box>
        ) : loadError ? (
          <Alert severity="error">{loadError}</Alert>
        ) : items.length === 0 ? (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ textAlign: 'center', py: 4 }}
          >
            {searchValue.trim() ? emptySearchText : emptyStateText}
          </Typography>
        ) : (
          <Stack spacing={1.5}>
            {items.map((item) => {
              const isAlreadyLinked = linkedIds.has(item.id);
              const isSelected = selectedIds.has(item.id);
              const isExpanded = isAlreadyLinked
                ? expandedLinkedIds.has(item.id)
                : isSelected;
              const itemKeys = entityKeysMap.get(item.id) ?? [];
              const mappedKeys = mappedKeysMap.get(item.id) ?? [];
              const isLoadingMappedKeys = loadingMappedKeyIds.has(item.id);
              const isLoadingItemKeys = loadingEntityKeyIds.has(item.id);
              const selectedKeys =
                selectedKeyNamesMap.get(item.id) ?? new Set<string>();
              const mappedKeyStatusMap = new Map(
                mappedKeys.map((key) => [key.keyId, key.status])
              );
              const lockedKeyNames = new Set(
                Array.from(mappedKeyStatusMap.keys()).filter(Boolean)
              );
              const visibleKeys = getVisibleKeys(
                itemKeys,
                mappedKeys,
                isAlreadyLinked
              );
              const isKeysLoading = isAlreadyLinked
                ? isLoadingMappedKeys || isLoadingItemKeys
                : isLoadingItemKeys;
              const mergedSelectedKeys = isAlreadyLinked
                ? new Set([
                    ...Array.from(selectedKeys),
                    ...Array.from(lockedKeyNames),
                  ])
                : selectedKeys;
              const itemMeta = getItemMeta(item);

              return (
                <Card
                  key={item.id}
                  sx={{
                    border: 2,
                    borderColor: isExpanded ? 'primary.main' : 'divider',
                    transition: 'border-color 0.15s',
                  }}
                >
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      p: 1.5,
                      gap: 1.5,
                      cursor: 'pointer',
                      '&:hover': { bgcolor: 'action.hover' },
                    }}
                    onClick={() => void onItemClick(item)}
                  >
                    <Checkbox
                      checked={isSelected || isAlreadyLinked}
                      disabled={isAlreadyLinked}
                      size="small"
                      tabIndex={-1}
                      disableRipple
                      sx={{ p: 0, flexShrink: 0 }}
                      onClick={(event) => event.stopPropagation()}
                      onChange={() => void onItemClick(item)}
                    />
                    <Avatar
                      sx={{
                        width: 36,
                        height: 36,
                        flexShrink: 0,
                        fontSize: 13,
                        fontWeight: 600,
                        bgcolor: 'primary.light',
                        color: 'primary.contrastText',
                      }}
                    >
                      {getInitials(item.displayName)}
                    </Avatar>
                    <Stack spacing={0.5} sx={{ minWidth: 0, flex: 1 }}>
                      <Stack
                        direction="row"
                        spacing={1}
                        alignItems="center"
                        flexWrap="wrap"
                      >
                        <Tooltip title={item.displayName.length > 20 ? item.displayName : ''}>
                          <Typography variant="body2" fontWeight={600} noWrap>
                            {item.displayName.length > 30
                              ? `${item.displayName.slice(0, 30)}...`
                              : item.displayName}
                          </Typography>
                        </Tooltip>
                        {itemMeta.chip}
                      </Stack>
                      <Tooltip
                        title={
                          (item.description || '—').length > 20
                            ? item.description || '—'
                            : ''
                        }
                      >
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{
                            display: '-webkit-box',
                            WebkitLineClamp: 2,
                            WebkitBoxOrient: 'vertical',
                            overflow: 'hidden',
                          }}
                        >
                          {item.description || '—'}
                        </Typography>
                      </Tooltip>
                    </Stack>
                    {isExpanded ? (
                      <ChevronDown size={16} />
                    ) : (
                      <ChevronRight size={16} />
                    )}
                  </Box>

                  {isExpanded && (
                    <Box
                      sx={{
                        borderTop: 1,
                        borderColor: 'divider',
                        px: 1.5,
                        pt: 1,
                        pb: 1.5,
                      }}
                    >
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ display: 'block', mb: 1, fontWeight: 600 }}
                      >
                        API Keys
                      </Typography>
                      {isKeysLoading ? (
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 1,
                            py: 0.5,
                          }}
                        >
                          <CircularProgress size={14} />
                          <Typography variant="caption" color="text.secondary">
                            Loading keys...
                          </Typography>
                        </Box>
                      ) : (
                        <SelectableKeyList
                          keys={visibleKeys}
                          selectedKeyNames={mergedSelectedKeys}
                          lockedKeyNames={lockedKeyNames}
                          keyStatusByName={mappedKeyStatusMap}
                          emptyText={itemMeta.emptyKeysText}
                          onToggleKey={(keyName) =>
                            onToggleKey(item.id, keyName)
                          }
                        />
                      )}
                    </Box>
                  )}
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
          gap: 1,
          flexShrink: 0,
        }}
      >
        <Button
          variant="outlined"
          color="secondary"
          onClick={onClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => void onAdd()}
          disabled={isAddDisabled}
        >
          {addButtonLabel}
        </Button>
      </Box>
    </Drawer>
  );
}

export default function AssociationsTab() {
  const {
    associations,
    isLoading,
    error: loadError,
    refreshAssociations,
    addAssociations,
    removeAssociation,
    listAssociationAPIKeys,
    addAPIKeys,
    removeAPIKey,
  } = useApplicationAssociations();
  const { currentOrganization, currentProject } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  // ── Derived associations ──────────────────────────────────────────────────
  const providerAssociations = useMemo(
    () => associations.filter((a) => a.kind === 'LlmProvider'),
    [associations]
  );
  const proxyAssociations = useMemo(
    () => associations.filter((a) => a.kind === 'LlmProxy'),
    [associations]
  );
  const allAssociations = useMemo(() => {
    const seen = new Set<string>();
    return [...providerAssociations, ...proxyAssociations].filter((a) => {
      if (seen.has(a.id)) return false;
      seen.add(a.id);
      return true;
    });
  }, [providerAssociations, proxyAssociations]);

  const linkedProviderIds = useMemo(
    () => new Set(providerAssociations.map((a) => a.id)),
    [providerAssociations]
  );
  const linkedProxyIds = useMemo(
    () => new Set(proxyAssociations.map((a) => a.id)),
    [proxyAssociations]
  );

  // ── Search & pagination ───────────────────────────────────────────────────
  const [searchValue, setSearchValue] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);

  // ── Table expand + mapped keys ────────────────────────────────────────────
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());
  const [apiKeysMap, setApiKeysMap] = useState<Map<string, MappedAPIKey[]>>(
    new Map()
  );
  const [loadingKeyIds, setLoadingKeyIds] = useState<Set<string>>(new Set());
  const [removingKeyIds, setRemovingKeyIds] = useState<Set<string>>(new Set());

  // ── Available keys per entity ─────────────────────────────────────────────
  const [providerKeysMap, setProviderKeysMap] = useState<
    Map<string, UserAPIKey[]>
  >(new Map());
  const [loadingProviderKeyIds, setLoadingProviderKeyIds] = useState<
    Set<string>
  >(new Set());
  const [proxyKeysMap, setProxyKeysMap] = useState<Map<string, UserAPIKey[]>>(
    new Map()
  );
  const [loadingProxyKeyIds, setLoadingProxyKeyIds] = useState<Set<string>>(
    new Set()
  );

  // ── Provider drawer ───────────────────────────────────────────────────────
  const [providerDrawerOpen, setProviderDrawerOpen] = useState(false);
  const [orgProviders, setOrgProviders] = useState<LLMProvider[]>([]);
  const [isProviderDrawerLoading, setIsProviderDrawerLoading] = useState(false);
  const [providerDrawerLoadError, setProviderDrawerLoadError] = useState<
    string | null
  >(null);
  const [providerDrawerSearch, setProviderDrawerSearch] = useState('');
  const [selectedProviderIds, setSelectedProviderIds] = useState<Set<string>>(
    new Set()
  );
  const [selectedProviderKeyNamesMap, setSelectedProviderKeyNamesMap] =
    useState<Map<string, Set<string>>>(new Map());
  const [isAddingProviders, setIsAddingProviders] = useState(false);
  const [expandedLinkedProviderIds, setExpandedLinkedProviderIds] = useState<
    Set<string>
  >(new Set());

  // ── Proxy drawer ──────────────────────────────────────────────────────────
  const [proxyDrawerOpen, setProxyDrawerOpen] = useState(false);
  const [orgProxies, setOrgProxies] = useState<Proxy[]>([]);
  const [isProxyDrawerLoading, setIsProxyDrawerLoading] = useState(false);
  const [proxyDrawerLoadError, setProxyDrawerLoadError] = useState<
    string | null
  >(null);
  const [proxyDrawerSearch, setProxyDrawerSearch] = useState('');
  const [selectedProxyIds, setSelectedProxyIds] = useState<Set<string>>(
    new Set()
  );
  const [selectedProxyKeyNamesMap, setSelectedProxyKeyNamesMap] = useState<
    Map<string, Set<string>>
  >(new Map());
  const [isAddingProxies, setIsAddingProxies] = useState(false);
  const [expandedLinkedProxyIds, setExpandedLinkedProxyIds] = useState<
    Set<string>
  >(new Set());

  // ── Delete ────────────────────────────────────────────────────────────────
  const [deleteTarget, setDeleteTarget] =
    useState<ApplicationAssociation | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);

  // ── Manage keys drawer ────────────────────────────────────────────────────
  const [manageKeysDrawerTarget, setManageKeysDrawerTarget] =
    useState<ApplicationAssociation | null>(null);
  const [selectedManageKeyNames, setSelectedManageKeyNames] = useState<
    Set<string>
  >(new Set());
  const [isAddingManagedKeys, setIsAddingManagedKeys] = useState(false);

  const loadProviderKeys = useCallback(
    async (providerId: string, orgUuid: string, preselectLatest = false) => {
      await loadEntityKeys({
        entityId: providerId,
        orgUuid,
        keysMap: providerKeysMap,
        loadingIds: loadingProviderKeyIds,
        setLoadingIds: setLoadingProviderKeyIds,
        setKeysMap: setProviderKeysMap,
        fetchKeys: getLLMProviderAPIKeys,
        preselectLatest,
        setSelectedKeyNamesMap: setSelectedProviderKeyNamesMap,
      });
    },
    [loadingProviderKeyIds, providerKeysMap]
  );

  const loadProxyKeys = useCallback(
    async (proxyId: string, orgUuid: string, preselectLatest = false) => {
      await loadEntityKeys({
        entityId: proxyId,
        orgUuid,
        keysMap: proxyKeysMap,
        loadingIds: loadingProxyKeyIds,
        setLoadingIds: setLoadingProxyKeyIds,
        setKeysMap: setProxyKeysMap,
        fetchKeys: getLLMProxyAPIKeys,
        preselectLatest,
        setSelectedKeyNamesMap: setSelectedProxyKeyNamesMap,
      });
    },
    [loadingProxyKeyIds, proxyKeysMap]
  );

  const loadAssociationKeys = useCallback(
    async (assocId: string) => {
      if (apiKeysMap.has(assocId) || loadingKeyIds.has(assocId)) return;
      setLoadingKeyIds((prev) => new Set(prev).add(assocId));
      try {
        const response = await listAssociationAPIKeys(assocId);
        setApiKeysMap((prev) =>
          new Map(prev).set(assocId, response.list ?? [])
        );
      } catch {
        setApiKeysMap((prev) => new Map(prev).set(assocId, []));
      } finally {
        setLoadingKeyIds((prev) => {
          const next = new Set(prev);
          next.delete(assocId);
          return next;
        });
      }
    },
    [apiKeysMap, listAssociationAPIKeys, loadingKeyIds]
  );

  const handleToggleExpand = async (assoc: ApplicationAssociation) => {
    if (expandedIds.has(assoc.id)) {
      setExpandedIds((prev) => removeSetValue(prev, assoc.id));
      return;
    }
    setExpandedIds((prev) => new Set(prev).add(assoc.id));
    await loadAssociationKeys(assoc.id);
    if (currentOrganization?.uuid) {
      if (assoc.kind === 'LlmProvider') {
        await loadProviderKeys(assoc.id, currentOrganization.uuid);
      } else {
        await loadProxyKeys(assoc.id, currentOrganization.uuid);
      }
    }
  };

  const loadAssociationKeysRef = useRef(loadAssociationKeys);
  loadAssociationKeysRef.current = loadAssociationKeys;
  const loadProviderKeysRef = useRef(loadProviderKeys);
  loadProviderKeysRef.current = loadProviderKeys;
  const loadProxyKeysRef = useRef(loadProxyKeys);
  loadProxyKeysRef.current = loadProxyKeys;

  useEffect(() => {
    if (!currentOrganization?.uuid || allAssociations.length === 0) return;
    const orgUuid = currentOrganization.uuid;
    allAssociations.forEach((assoc) => {
      void loadAssociationKeysRef.current(assoc.id);
      if (assoc.kind === 'LlmProvider') {
        void loadProviderKeysRef.current(assoc.id, orgUuid);
      } else {
        void loadProxyKeysRef.current(assoc.id, orgUuid);
      }
    });
  }, [allAssociations, currentOrganization?.uuid]);

  const filteredAssociations = useMemo(() => {
    const query = searchValue.trim().toLowerCase();
    if (!query) return allAssociations;
    return allAssociations.filter((a) => {
      const haystack = [a.id, a.displayName, a.kind]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [allAssociations, searchValue]);

  useEffect(() => {
    setPage(0);
  }, [searchValue]);

  const paginatedAssociations = useMemo(
    () =>
      filteredAssociations.slice(page * rowsPerPage, (page + 1) * rowsPerPage),
    [filteredAssociations, page, rowsPerPage]
  );

  const ensureOrgProvidersLoaded = async (): Promise<LLMProvider[]> => {
    if (orgProviders.length > 0) return orgProviders;
    if (!currentOrganization?.uuid) return [];
    const response = await getLLMProviders(
      currentOrganization.uuid,
      apimBaseUrl
    );
    const providers = response.list ?? [];
    setOrgProviders(providers);
    return providers;
  };

  const handleCloseProviderDrawer = () => {
    if (isAddingProviders) return;
    setProviderDrawerOpen(false);
    setSelectedProviderIds(new Set());
    setSelectedProviderKeyNamesMap(new Map());
    setExpandedLinkedProviderIds(new Set());
    setProviderDrawerSearch('');
    setProviderDrawerLoadError(null);
  };

  const handleOpenProviderDrawer = async () => {
    setProviderDrawerOpen(true);
    setSelectedProviderIds(new Set());
    setSelectedProviderKeyNamesMap(new Map());
    setExpandedLinkedProviderIds(new Set());
    setProviderDrawerSearch('');
    setProviderDrawerLoadError(null);
    if (!currentOrganization?.uuid) return;
    setIsProviderDrawerLoading(true);
    try {
      const response = await getLLMProviders(
        currentOrganization.uuid,
        apimBaseUrl
      );
      setOrgProviders(response.list ?? []);
    } catch {
      setProviderDrawerLoadError('Failed to load LLM providers.');
    } finally {
      setIsProviderDrawerLoading(false);
    }
  };

  const handleProviderClick = async (provider: LLMProvider) => {
    if (linkedProviderIds.has(provider.id)) {
      const isExpanded = expandedLinkedProviderIds.has(provider.id);
      setExpandedLinkedProviderIds((prev) => toggleSetValue(prev, provider.id));
      if (!isExpanded) {
        await loadAssociationKeys(provider.id);
        if (currentOrganization?.uuid) {
          await loadProviderKeys(provider.id, currentOrganization.uuid);
        }
      }
      return;
    }
    if (!currentOrganization?.uuid) return;
    if (selectedProviderIds.has(provider.id)) {
      setSelectedProviderIds((prev) => removeSetValue(prev, provider.id));
      return;
    }
    setSelectedProviderIds((prev) => new Set(prev).add(provider.id));
    await loadProviderKeys(provider.id, currentOrganization.uuid, true);
  };

  const handleToggleProviderKey = (providerId: string, keyName: string) => {
    setSelectedProviderKeyNamesMap((prev) =>
      toggleMapSelectionValue(prev, providerId, keyName)
    );
  };

  const linkedProviderKeyPayload = useMemo(
    () =>
      buildLinkedKeyPayload(
        selectedProviderKeyNamesMap,
        linkedProviderIds,
        apiKeysMap
      ),
    [selectedProviderKeyNamesMap, linkedProviderIds, apiKeysMap]
  );
  const hasPendingLinkedProviderKeys = linkedProviderKeyPayload.length > 0;

  const handleAddProviders = async () => {
    if (
      (selectedProviderIds.size === 0 && !hasPendingLinkedProviderKeys) ||
      isAddingProviders
    )
      return;
    setIsAddingProviders(true);
    try {
      if (selectedProviderIds.size > 0) {
        await addAssociations({
          associations: Array.from(selectedProviderIds).map((id) => ({
            id,
            kind: 'LlmProvider',
          })),
        });
      }
      const keyPayload = [
        ...Array.from(selectedProviderIds).flatMap((pid) =>
          Array.from(selectedProviderKeyNamesMap.get(pid) ?? []).map(
            (name) => ({ keyId: name, associatedEntity: { id: pid } })
          )
        ),
        ...linkedProviderKeyPayload,
      ];
      if (keyPayload.length > 0) {
        await addAPIKeys({ apiKeys: keyPayload });
      }
      setExpandedIds(new Set());
      setApiKeysMap(new Map());
      await refreshAssociations();
      handleCloseProviderDrawer();
      const pc = selectedProviderIds.size;
      const kc = linkedProviderKeyPayload.length;
      showSnackbar(
        pc > 0 && kc > 0
          ? `${pc} LLM provider${
              pc > 1 ? 's' : ''
            } associated and ${kc} API key${
              kc > 1 ? 's' : ''
            } added successfully.`
          : pc > 0
          ? `${pc} LLM provider${pc > 1 ? 's' : ''} associated successfully.`
          : `${kc} API key${kc > 1 ? 's' : ''} added successfully.`,
        'success'
      );
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to add association.'),
        'error'
      );
    } finally {
      setIsAddingProviders(false);
    }
  };

  const ensureOrgProxiesLoaded = async (): Promise<Proxy[]> => {
    if (orgProxies.length > 0) return orgProxies;
    if (!currentOrganization?.uuid) return [];
    const response = currentProject?.id
      ? await getProxies(
          currentOrganization.uuid,
          currentProject.id,
          apimBaseUrl
        )
      : await getOrgProxies(currentOrganization.uuid, apimBaseUrl);
    const proxies = response.list ?? [];
    setOrgProxies(proxies);
    return proxies;
  };

  const handleCloseProxyDrawer = () => {
    if (isAddingProxies) return;
    setProxyDrawerOpen(false);
    setSelectedProxyIds(new Set());
    setSelectedProxyKeyNamesMap(new Map());
    setExpandedLinkedProxyIds(new Set());
    setProxyDrawerSearch('');
    setProxyDrawerLoadError(null);
  };

  const handleOpenProxyDrawer = async () => {
    setProxyDrawerOpen(true);
    setSelectedProxyIds(new Set());
    setSelectedProxyKeyNamesMap(new Map());
    setExpandedLinkedProxyIds(new Set());
    setProxyDrawerSearch('');
    setProxyDrawerLoadError(null);
    if (!currentOrganization?.uuid) return;
    setIsProxyDrawerLoading(true);
    try {
      const response = currentProject?.id
        ? await getProxies(
            currentOrganization.uuid,
            currentProject.id,
            apimBaseUrl
          )
        : await getOrgProxies(currentOrganization.uuid, apimBaseUrl);
      setOrgProxies(response.list ?? []);
    } catch {
      setProxyDrawerLoadError('Failed to load LLM proxies.');
    } finally {
      setIsProxyDrawerLoading(false);
    }
  };

  const handleProxyClick = async (proxy: Proxy) => {
    if (linkedProxyIds.has(proxy.id)) {
      const isExpanded = expandedLinkedProxyIds.has(proxy.id);
      setExpandedLinkedProxyIds((prev) => toggleSetValue(prev, proxy.id));
      if (!isExpanded) {
        await loadAssociationKeys(proxy.id);
        if (currentOrganization?.uuid) {
          await loadProxyKeys(proxy.id, currentOrganization.uuid);
        }
      }
      return;
    }
    if (!currentOrganization?.uuid) return;
    if (selectedProxyIds.has(proxy.id)) {
      setSelectedProxyIds((prev) => removeSetValue(prev, proxy.id));
      return;
    }
    setSelectedProxyIds((prev) => new Set(prev).add(proxy.id));
    await loadProxyKeys(proxy.id, currentOrganization.uuid, true);
  };

  const handleToggleProxyKey = (proxyId: string, keyName: string) => {
    setSelectedProxyKeyNamesMap((prev) =>
      toggleMapSelectionValue(prev, proxyId, keyName)
    );
  };

  const linkedProxyKeyPayload = useMemo(
    () =>
      buildLinkedKeyPayload(
        selectedProxyKeyNamesMap,
        linkedProxyIds,
        apiKeysMap
      ),
    [selectedProxyKeyNamesMap, linkedProxyIds, apiKeysMap]
  );
  const hasPendingLinkedProxyKeys = linkedProxyKeyPayload.length > 0;

  const handleAddProxies = async () => {
    if (
      (selectedProxyIds.size === 0 && !hasPendingLinkedProxyKeys) ||
      isAddingProxies
    )
      return;
    setIsAddingProxies(true);
    try {
      if (selectedProxyIds.size > 0) {
        await addAssociations({
          associations: Array.from(selectedProxyIds).map((id) => ({
            id,
            kind: 'LlmProxy',
          })),
        });
      }
      const keyPayload = [
        ...Array.from(selectedProxyIds).flatMap((pid) =>
          Array.from(selectedProxyKeyNamesMap.get(pid) ?? []).map((name) => ({
            keyId: name,
            associatedEntity: { id: pid },
          }))
        ),
        ...linkedProxyKeyPayload,
      ];
      if (keyPayload.length > 0) {
        await addAPIKeys({ apiKeys: keyPayload });
      }
      setExpandedIds(new Set());
      setApiKeysMap(new Map());
      await refreshAssociations();
      handleCloseProxyDrawer();
      const pc = selectedProxyIds.size;
      const kc = linkedProxyKeyPayload.length;
      showSnackbar(
        pc > 0 && kc > 0
          ? `${pc} LLM proxy${pc > 1 ? 's' : ''} associated and ${kc} API key${
              kc > 1 ? 's' : ''
            } added successfully.`
          : pc > 0
          ? `${pc} LLM proxy${pc > 1 ? 's' : ''} associated successfully.`
          : `${kc} API key${kc > 1 ? 's' : ''} added successfully.`,
        'success'
      );
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to add association.'),
        'error'
      );
    } finally {
      setIsAddingProxies(false);
    }
  };

  const handleRemove = async () => {
    if (!deleteTarget || isRemoving) return;
    setIsRemoving(true);
    try {
      const mappedKeysResponse = await listAssociationAPIKeys(deleteTarget.id);
      const mappedKeys = mappedKeysResponse.list ?? [];
      if (mappedKeys.length > 0) {
        await Promise.all(
          mappedKeys
            .filter((key) => Boolean(resolveEntityId(key)))
            .map((key) =>
              removeAPIKey(resolveMappedKeyId(key), {
                entityID: resolveEntityId(key) as string,
              })
            )
        );
      }
      await removeAssociation(deleteTarget.id);
      setExpandedIds((prev) => removeSetValue(prev, deleteTarget.id));
      setApiKeysMap((prev) => {
        const next = new Map(prev);
        next.delete(deleteTarget.id);
        return next;
      });
      const label =
        deleteTarget.kind === 'LlmProvider' ? 'LLM provider' : 'LLM proxy';
      setDeleteTarget(null);
      showSnackbar(`${label} association removed successfully.`, 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to remove association.'),
        'error'
      );
    } finally {
      setIsRemoving(false);
    }
  };

  const handleOpenManageKeysDrawer = async (assoc: ApplicationAssociation) => {
    if (!currentOrganization?.uuid) return;
    setManageKeysDrawerTarget(assoc);
    setSelectedManageKeyNames(new Set());
    try {
      if (assoc.kind === 'LlmProvider') {
        await ensureOrgProvidersLoaded();
      } else {
        await ensureOrgProxiesLoaded();
      }
      await Promise.all([
        loadAssociationKeys(assoc.id),
        assoc.kind === 'LlmProvider'
          ? loadProviderKeys(assoc.id, currentOrganization.uuid)
          : loadProxyKeys(assoc.id, currentOrganization.uuid),
      ]);
    } catch {
      showSnackbar('Failed to load API keys for this association.', 'error');
    }
  };

  const handleCloseManageKeysDrawer = () => {
    if (isAddingManagedKeys) return;
    setManageKeysDrawerTarget(null);
    setSelectedManageKeyNames(new Set());
  };

  const handleToggleManagedKey = (keyName: string) => {
    setSelectedManageKeyNames((prev) => toggleSetValue(prev, keyName));
  };

  const handleAddManagedKeys = async () => {
    if (!manageKeysDrawerTarget || selectedManageKeyNames.size === 0) return;
    setIsAddingManagedKeys(true);
    try {
      const existingMappedKeyIds = new Set(
        (apiKeysMap.get(manageKeysDrawerTarget.id) ?? []).map(
          (key) => key.keyId
        )
      );
      const apiKeys = Array.from(selectedManageKeyNames)
        .filter((keyId) => !existingMappedKeyIds.has(keyId))
        .map((keyId) => ({
          keyId,
          associatedEntity: { id: manageKeysDrawerTarget.id },
        }));
      if (apiKeys.length === 0) {
        handleCloseManageKeysDrawer();
        showSnackbar('All selected API keys are already associated.', 'info');
        return;
      }
      await addAPIKeys({ apiKeys });
      setExpandedIds(new Set());
      setApiKeysMap((prev) => {
        const next = new Map(prev);
        next.delete(manageKeysDrawerTarget.id);
        return next;
      });
      await refreshAssociations();
      await loadAssociationKeysRef.current(manageKeysDrawerTarget.id);
      handleCloseManageKeysDrawer();
      showSnackbar('API keys added successfully.', 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to add API keys.'),
        'error'
      );
    } finally {
      setIsAddingManagedKeys(false);
    }
  };

  const handleRemoveKey = async (assocId: string, key: MappedAPIKey) => {
    const mappedKeyId = resolveMappedKeyId(key);
    if (removingKeyIds.has(mappedKeyId)) return;
    const entityID = resolveEntityId(key);
    if (!entityID) {
      showSnackbar('Associated entity id is missing.', 'error');
      return;
    }
    setRemovingKeyIds((prev) => new Set(prev).add(mappedKeyId));
    try {
      await removeAPIKey(mappedKeyId, { entityID });
      setApiKeysMap((prev) => {
        const next = new Map(prev);
        const currentKeys = next.get(assocId) ?? [];
        next.set(
          assocId,
          currentKeys.filter((k) => resolveMappedKeyId(k) !== mappedKeyId)
        );
        return next;
      });
      showSnackbar('API key removed successfully.', 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to remove API key.'),
        'error'
      );
    } finally {
      setRemovingKeyIds((prev) => removeSetValue(prev, mappedKeyId));
    }
  };
  const hasAssociations = allAssociations.length > 0;
  const hasSearchQuery = searchValue.trim().length > 0;
  const hasFilteredAssociations = filteredAssociations.length > 0;
  const showNoSearchResults =
    hasAssociations && hasSearchQuery && !hasFilteredAssociations;

  // Provider drawer computed
  const filteredOrgProviders = useMemo(() => {
    return filterItemsByQuery(
      orgProviders,
      providerDrawerSearch,
      (provider) => [provider.displayName, provider.description, provider.template]
    );
  }, [orgProviders, providerDrawerSearch]);

  const providerAddButtonLabel = buildAddButtonLabel({
    isSubmitting: isAddingProviders,
    selectedCount: selectedProviderIds.size,
    pendingKeyCount: linkedProviderKeyPayload.length,
    entityLabel: 'Provider',
    defaultLabel: 'Add Providers',
  });

  // Proxy drawer computed
  const filteredOrgProxies = useMemo(() => {
    return filterItemsByQuery(orgProxies, proxyDrawerSearch, (proxy) => [
      proxy.displayName,
      proxy.description,
      proxy.version,
      proxy.context,
    ]);
  }, [orgProxies, proxyDrawerSearch]);

  const proxyAddButtonLabel = buildAddButtonLabel({
    isSubmitting: isAddingProxies,
    selectedCount: selectedProxyIds.size,
    pendingKeyCount: linkedProxyKeyPayload.length,
    entityLabel: 'Proxy',
    defaultLabel: 'Add LLM Proxy',
  });

  // Manage keys drawer computed
  const managedIsProvider = manageKeysDrawerTarget?.kind === 'LlmProvider';
  const managedProvider = managedIsProvider
    ? orgProviders.find((p) => p.id === manageKeysDrawerTarget?.id) ?? null
    : null;
  const managedProxy = !managedIsProvider
    ? orgProxies.find((p) => p.id === manageKeysDrawerTarget?.id) ?? null
    : null;
  const managedProviderLogo = getTemplateLogo(managedProvider?.template);
  const managedProviderTemplate = getProviderTemplateDisplayName(
    managedProvider?.template
  );
  const managedMappedKeys = manageKeysDrawerTarget
    ? dedupeMappedKeys(apiKeysMap.get(manageKeysDrawerTarget.id) ?? [])
    : [];
  const managedAvailableKeys = manageKeysDrawerTarget
    ? managedIsProvider
      ? providerKeysMap.get(manageKeysDrawerTarget.id) ?? []
      : proxyKeysMap.get(manageKeysDrawerTarget.id) ?? []
    : [];
  const isManagedAssocKeysLoading = manageKeysDrawerTarget
    ? loadingKeyIds.has(manageKeysDrawerTarget.id)
    : false;
  const isManagedEntityKeysLoading = manageKeysDrawerTarget
    ? managedIsProvider
      ? loadingProviderKeyIds.has(manageKeysDrawerTarget.id)
      : loadingProxyKeyIds.has(manageKeysDrawerTarget.id)
    : false;
  const managedMappedKeyStatusMap = useMemo(
    () => new Map(managedMappedKeys.map((k) => [k.keyId, k.status])),
    [managedMappedKeys]
  );
  const managedVisibleKeys = getVisibleKeys(
    managedAvailableKeys,
    managedMappedKeys,
    true
  );
  const hasAddableManagedKeys = managedAvailableKeys.some(
    (k) => k.name && !managedMappedKeyStatusMap.has(k.name)
  );
  return (
    <>
      {/* ── Main table ────────────────────────────────────────────────────── */}
      <ListingTable.Container sx={{ minWidth: 600 }}>
        <ListingTable.Toolbar
          showSearch={hasAssociations}
          searchValue={searchValue}
          onSearchChange={setSearchValue}
          searchPlaceholder="Search associations..."
          actions={
            hasAssociations ? (
              <Stack direction="row" spacing={1}>
                <Button
                  variant="contained"
                  size="small"
                  startIcon={<Plus size={16} />}
                  onClick={() => void handleOpenProviderDrawer()}
                >
                  Add LLM Provider
                </Button>
                <Button
                  variant="contained"
                  size="small"
                  startIcon={<Plus size={16} />}
                  onClick={() => void handleOpenProxyDrawer()}
                >
                  Add LLM Proxy
                </Button>
              </Stack>
            ) : null
          }
        />

        {loadError ? (
          <Alert severity="error" sx={{ mx: 2, mb: 2 }}>
            {loadError}
          </Alert>
        ) : null}

        {isLoading || hasFilteredAssociations ? (
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
                ? Array.from({ length: 3 }).map((_, i) => (
                    // eslint-disable-next-line react/no-array-index-key
                    <ListingTable.Row key={i}>
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
                : paginatedAssociations.flatMap((assoc) => {
                    const isProvider = assoc.kind === 'LlmProvider';
                    const isExpanded = expandedIds.has(assoc.id);
                    const isLoadingKeys = loadingKeyIds.has(assoc.id);
                    const keys = dedupeMappedKeys(
                      apiKeysMap.get(assoc.id) ?? []
                    );
                    const hasLoadedMappedKeys = apiKeysMap.has(assoc.id);
                    const entityKeys = isProvider
                      ? providerKeysMap.get(assoc.id) ?? []
                      : proxyKeysMap.get(assoc.id) ?? [];
                    const hasLoadedEntityKeys = isProvider
                      ? providerKeysMap.has(assoc.id)
                      : proxyKeysMap.has(assoc.id);
                    const isLoadingEntityKeys = isProvider
                      ? loadingProviderKeyIds.has(assoc.id)
                      : loadingProxyKeyIds.has(assoc.id);
                    const mappedKeyIds = new Set(
                      keys.map((k) => k.keyId).filter(Boolean)
                    );
                    const canDetermineAddableKeys =
                      hasLoadedMappedKeys && hasLoadedEntityKeys;
                    const hasEntityKeys = entityKeys.some((k) =>
                      Boolean(k.name)
                    );
                    const hasAvailableKeysToAdd = entityKeys.some(
                      (k) => k.name && !mappedKeyIds.has(k.name)
                    );
                    const entityLabel = isProvider ? 'provider' : 'proxy';
                    const addApiKeyTooltip =
                      !canDetermineAddableKeys ||
                      isLoadingEntityKeys ||
                      isLoadingKeys
                        ? 'Loading API keys...'
                        : !hasEntityKeys
                        ? `No API keys available for this ${entityLabel}.`
                        : !hasAvailableKeysToAdd
                        ? `All API keys are already associated for this ${entityLabel}.`
                        : 'Add API Key';
                    const isAddApiKeyDisabled =
                      !canDetermineAddableKeys ||
                      isLoadingEntityKeys ||
                      isLoadingKeys ||
                      !hasAvailableKeysToAdd;

                    const mainRow = (
                      <ListingTable.Row key={`assoc-${assoc.id}`} hover>
                        <ListingTable.Cell sx={{ pr: 0 }}>
                          <IconButton
                            size="small"
                            onClick={() => void handleToggleExpand(assoc)}
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
                          {assoc.displayName || '—'}
                        </ListingTable.Cell>
                        <ListingTable.Cell>
                          <Typography variant="body2">
                            {String(assoc.version ?? '—')}
                          </Typography>
                        </ListingTable.Cell>
                        <ListingTable.Cell>
                          {assoc.kind ? (
                            <Chip
                              label={
                                assoc.kind === 'LlmProvider'
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
                                    void handleOpenManageKeysDrawer(assoc)
                                  }
                                  aria-label={`Add API key for ${
                                    assoc.displayName || assoc.id
                                  }`}
                                >
                                  <Plus size={16} />
                                </IconButton>
                              </span>
                            </Tooltip>
                            <IconButton
                              size="small"
                              color="error"
                              onClick={() => setDeleteTarget(assoc)}
                              aria-label={`Remove ${assoc.displayName || assoc.id}`}
                            >
                              <Trash2 size={16} />
                            </IconButton>
                          </Box>
                        </ListingTable.Cell>
                      </ListingTable.Row>
                    );

                    if (!isExpanded) return [mainRow];

                    const sectionTitleRow = (
                      <ListingTable.Row key={`${assoc.id}-section-title`}>
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
                          <ListingTable.Row key={`${assoc.id}-loading`}>
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
                      : keys.length === 0
                      ? [
                          <ListingTable.Row key={`${assoc.id}-empty`}>
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
                      : keys.map((key) => (
                          <ListingTable.Row
                            key={`apikey-${assoc.id}-${key.keyId}`}
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
                                      void handleRemoveKey(assoc.id, key)
                                    }
                                    aria-label={`Remove API key ${key.keyId}`}
                                  >
                                    {removingKeyIds.has(
                                      resolveMappedKeyId(key)
                                    ) ? (
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
            title={
              showNoSearchResults ? 'No associations found' : 'No Associations'
            }
            description={
              showNoSearchResults
                ? 'No associations match your search.'
                : 'Associate LLM providers and proxies to enable AI capabilities for this application.'
            }
            action={
              showNoSearchResults ? (
                <Button variant="outlined" onClick={() => setSearchValue('')}>
                  Clear search
                </Button>
              ) : (
                <Stack direction="row" spacing={1} justifyContent="center">
                  <Button
                    variant="contained"
                    startIcon={<Plus size={16} />}
                    onClick={() => void handleOpenProviderDrawer()}
                  >
                    Add LLM Provider
                  </Button>
                  <Button
                    variant="contained"
                    startIcon={<Plus size={16} />}
                    onClick={() => void handleOpenProxyDrawer()}
                  >
                    Add LLM Proxy
                  </Button>
                </Stack>
              )
            }
          />
        )}

        {/* ── Pagination ── */}
        {hasFilteredAssociations && (
          <TablePagination
            component="div"
            count={filteredAssociations.length}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={(e) => {
              setRowsPerPage(parseInt(e.target.value, 10));
              setPage(0);
            }}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          />
        )}
      </ListingTable.Container>

      <AssociationSelectionDrawer
        open={providerDrawerOpen}
        title="Add LLM Providers"
        description="Select providers and their API keys to associate with this application."
        searchPlaceholder="Search providers..."
        searchValue={providerDrawerSearch}
        onSearchChange={setProviderDrawerSearch}
        onClose={handleCloseProviderDrawer}
        isSubmitting={isAddingProviders}
        isLoading={isProviderDrawerLoading}
        loadError={providerDrawerLoadError}
        items={filteredOrgProviders}
        emptyStateText="No LLM providers found in this organization."
        emptySearchText="No providers match your search."
        linkedIds={linkedProviderIds}
        selectedIds={selectedProviderIds}
        expandedLinkedIds={expandedLinkedProviderIds}
        entityKeysMap={providerKeysMap}
        mappedKeysMap={apiKeysMap}
        loadingMappedKeyIds={loadingKeyIds}
        loadingEntityKeyIds={loadingProviderKeyIds}
        selectedKeyNamesMap={selectedProviderKeyNamesMap}
        onItemClick={handleProviderClick}
        onToggleKey={handleToggleProviderKey}
        getItemMeta={(provider) => {
          const logoUrl = getTemplateLogo(provider.template);
          const templateDisplayName = getProviderTemplateDisplayName(
            provider.template
          );

          return {
            chip: templateDisplayName ? (
              <Chip
                label={` ${templateDisplayName}`}
                size="small"
                variant="outlined"
                color="primary"
                sx={{ borderRadius: 0.5 }}
                icon={
                  logoUrl ? (
                    <Avatar
                      src={logoUrl}
                      variant="circular"
                      sx={{
                        width: 16,
                        height: 16,
                        '& img': { objectFit: 'contain' },
                      }}
                    />
                  ) : undefined
                }
              />
            ) : undefined,
            emptyKeysText: 'No active API keys available for this provider.',
          };
        }}
        addButtonLabel={providerAddButtonLabel}
        isAddDisabled={
          (selectedProviderIds.size === 0 && !hasPendingLinkedProviderKeys) ||
          isAddingProviders
        }
        onAdd={handleAddProviders}
      />

      <AssociationSelectionDrawer
        open={proxyDrawerOpen}
        title="Add LLM Proxies"
        description="Select proxies and their API keys to associate with this application."
        searchPlaceholder="Search proxies..."
        searchValue={proxyDrawerSearch}
        onSearchChange={setProxyDrawerSearch}
        onClose={handleCloseProxyDrawer}
        isSubmitting={isAddingProxies}
        isLoading={isProxyDrawerLoading}
        loadError={proxyDrawerLoadError}
        items={filteredOrgProxies}
        emptyStateText="No LLM proxies found in this organization."
        emptySearchText="No proxies match your search."
        linkedIds={linkedProxyIds}
        selectedIds={selectedProxyIds}
        expandedLinkedIds={expandedLinkedProxyIds}
        entityKeysMap={proxyKeysMap}
        mappedKeysMap={apiKeysMap}
        loadingMappedKeyIds={loadingKeyIds}
        loadingEntityKeyIds={loadingProxyKeyIds}
        selectedKeyNamesMap={selectedProxyKeyNamesMap}
        onItemClick={handleProxyClick}
        onToggleKey={handleToggleProxyKey}
        getItemMeta={(proxy) => ({
          chip: proxy.version ? (
            <Chip
              label={`v${proxy.version}`}
              size="small"
              variant="outlined"
              color="primary"
              sx={{ borderRadius: 0.5 }}
            />
          ) : undefined,
          emptyKeysText: 'No active API keys available for this proxy.',
        })}
        addButtonLabel={proxyAddButtonLabel}
        isAddDisabled={
          (selectedProxyIds.size === 0 && !hasPendingLinkedProxyKeys) ||
          isAddingProxies
        }
        onAdd={handleAddProxies}
      />

      {/* ── Manage Keys Drawer (shared for provider + proxy) ─────────────── */}
      <Drawer
        anchor="right"
        open={Boolean(manageKeysDrawerTarget)}
        onClose={handleCloseManageKeysDrawer}
        sx={{
          '& .MuiDrawer-paper': {
            width: { xs: '100%', sm: 520 },
            maxWidth: '100%',
            display: 'flex',
            flexDirection: 'column',
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
            flexShrink: 0,
          }}
        >
          <Stack spacing={0.25}>
            <Typography variant="h6">Add API Keys</Typography>
            <Typography variant="caption" color="text.secondary">
              Associate additional API keys with this{' '}
              {managedIsProvider ? 'provider' : 'proxy'}.
            </Typography>
          </Stack>
          <IconButton
            onClick={handleCloseManageKeysDrawer}
            disabled={isAddingManagedKeys}
            size="small"
          >
            <X size={20} />
          </IconButton>
        </Box>

        <Box sx={{ flex: 1, overflow: 'auto', p: 2 }}>
          {!manageKeysDrawerTarget ? null : (
            <Card sx={{ border: 2, borderColor: 'divider' }}>
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  p: 1.5,
                  gap: 1.5,
                  borderBottom: 1,
                  borderColor: 'divider',
                }}
              >
                <Checkbox checked disabled size="small" sx={{ p: 0 }} />
                <Avatar
                  sx={{
                    width: 36,
                    height: 36,
                    flexShrink: 0,
                    fontSize: 13,
                    fontWeight: 600,
                    bgcolor: 'primary.light',
                    color: 'primary.contrastText',
                  }}
                >
                  {getInitials(
                    (managedIsProvider
                      ? managedProvider?.displayName
                      : managedProxy?.displayName) ||
                      manageKeysDrawerTarget.displayName ||
                      ''
                  )}
                </Avatar>
                <Stack spacing={0.5} sx={{ minWidth: 0, flex: 1 }}>
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    flexWrap="wrap"
                  >
                    <Typography variant="body2" fontWeight={600} noWrap>
                      {(managedIsProvider
                        ? managedProvider?.displayName
                        : managedProxy?.displayName) ||
                        manageKeysDrawerTarget.displayName ||
                        '—'}
                    </Typography>
                    {/* Provider: template chip */}
                    {managedIsProvider && managedProviderTemplate && (
                      <Chip
                        label={` ${managedProviderTemplate}`}
                        size="small"
                        variant="outlined"
                        color="primary"
                        sx={{ borderRadius: 0.5 }}
                        icon={
                          managedProviderLogo ? (
                            <Avatar
                              src={managedProviderLogo}
                              variant="circular"
                              sx={{
                                width: 16,
                                height: 16,
                                '& img': { objectFit: 'contain' },
                              }}
                            />
                          ) : undefined
                        }
                      />
                    )}
                    {/* Proxy: version chip */}
                    {!managedIsProvider && managedProxy?.version && (
                      <Chip
                        label={`v${managedProxy.version}`}
                        size="small"
                        variant="outlined"
                        color="primary"
                        sx={{ borderRadius: 0.5 }}
                      />
                    )}
                  </Stack>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{
                      display: '-webkit-box',
                      WebkitLineClamp: 2,
                      WebkitBoxOrient: 'vertical',
                      overflow: 'hidden',
                    }}
                  >
                    {(managedIsProvider
                      ? managedProvider?.description
                      : managedProxy?.description) || '—'}
                  </Typography>
                </Stack>
              </Box>

              <Box sx={{ p: 1.5 }}>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ display: 'block', mb: 1, fontWeight: 600 }}
                >
                  API Keys
                </Typography>
                {isManagedAssocKeysLoading || isManagedEntityKeysLoading ? (
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1,
                      py: 0.5,
                    }}
                  >
                    <CircularProgress size={14} />
                    <Typography variant="caption" color="text.secondary">
                      Loading keys...
                    </Typography>
                  </Box>
                ) : (
                  <SelectableKeyList
                    keys={managedVisibleKeys}
                    selectedKeyNames={
                      new Set([
                        ...Array.from(selectedManageKeyNames),
                        ...Array.from(managedMappedKeyStatusMap.keys()).filter(
                          Boolean
                        ),
                      ])
                    }
                    lockedKeyNames={
                      new Set(
                        Array.from(managedMappedKeyStatusMap.keys()).filter(
                          Boolean
                        )
                      )
                    }
                    keyStatusByName={managedMappedKeyStatusMap}
                    emptyText={`No active API keys available for this ${
                      managedIsProvider ? 'provider' : 'proxy'
                    }.`}
                    onToggleKey={handleToggleManagedKey}
                  />
                )}
              </Box>
            </Card>
          )}
        </Box>

        <Box
          sx={{
            p: 2,
            borderTop: 1,
            borderColor: 'divider',
            display: 'flex',
            gap: 1,
            flexShrink: 0,
          }}
        >
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleCloseManageKeysDrawer}
            disabled={isAddingManagedKeys}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleAddManagedKeys()}
            disabled={
              !hasAddableManagedKeys ||
              selectedManageKeyNames.size === 0 ||
              isAddingManagedKeys
            }
          >
            {isAddingManagedKeys
              ? 'Adding...'
              : `Add ${selectedManageKeyNames.size} Key${
                  selectedManageKeyNames.size > 1 ? 's' : ''
                }`}
          </Button>
        </Box>
      </Drawer>

      {/* ── Delete Confirmation Dialog ───────────────────────────────────── */}
      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => {
          if (!isRemoving) setDeleteTarget(null);
        }}
      >
        <DialogTitle>
          Remove{' '}
          {deleteTarget?.kind === 'LlmProvider' ? 'LLM Provider' : 'LLM Proxy'}{' '}
          association
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to remove{' '}
            <strong>{deleteTarget?.displayName || deleteTarget?.id}</strong> from this
            application?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setDeleteTarget(null)}
            disabled={isRemoving}
            size="small"
          >
            Cancel
          </Button>
          <Button
            color="error"
            onClick={() => void handleRemove()}
            disabled={isRemoving}
            variant="contained"
            size="small"
          >
            {isRemoving ? 'Removing...' : 'Remove'}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
