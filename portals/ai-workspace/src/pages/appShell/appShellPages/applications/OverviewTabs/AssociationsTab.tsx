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
import type { Dispatch, SetStateAction } from 'react';
import {
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
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { useParams } from 'react-router-dom';
import { applicationApis } from '../../../../../apis/applicationApis';
import {
  getLLMProviders,
  getLLMProviderAPIKeys,
} from '../../../../../apis/llmProviderApis';
import { getLLMProxyAPIKeys } from '../../../../../apis/llmProxiesApis';
import { getOrgProxies, getProxies } from '../../../../../apis/proxyApis';
import AnthropicLogo from '../../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../../assets/brands/openAI.png';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import { useApplicationAssociations } from '../../../../../contexts/ApplicationAssociationsContext';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../../hooks/aiWorkspaceSnackbar';
import { getProviderTemplateDisplayName } from '../../../../../utils/providerTemplateDisplay';
import type {
  ApplicationAssociation,
  LLMProvider,
  MappedAPIKey,
  Proxy,
  UserAPIKey,
} from '../../../../../utils/types';
import AssociationSelectionDrawer, {
  SelectableKeyList,
} from './AssociationSelectionDrawer';
import AssociationsTable from './AssociationsTable';
import {
  dedupeMappedKeys,
  getInitials,
  getVisibleKeys,
  resolveMappedKeyId,
} from './associationsTabUtils';

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
  unavailableKeyNames?: Set<string>;
  setSelectedKeyNamesMap?: Dispatch<SetStateAction<Map<string, Set<string>>>>;
};

function getTemplateLogo(template?: string): string | undefined {
  return TEMPLATE_LOGO_MAP[(template ?? '').trim().toLowerCase()];
}

function getErrorDescription(error: unknown, fallback: string): string {
  return (
    (error as any)?.response?.data?.description ||
    (error as any)?.response?.data?.message ||
    (error instanceof Error ? error.message : null) ||
    fallback
  );
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

function getLatestActiveKey(keys: UserAPIKey[]): UserAPIKey | null {
  return keys.reduce<UserAPIKey | null>((latest, key) => {
    if (!latest) return key;
    return new Date(key.createdAt ?? 0).getTime() >
      new Date(latest.createdAt ?? 0).getTime()
      ? key
      : latest;
  }, null);
}

function getLatestSelectableKey(
  keys: UserAPIKey[],
  unavailableKeyNames: Set<string>
): UserAPIKey | null {
  return getLatestActiveKey(
    keys.filter((key) => {
      const keyName = key.name ?? '';
      return Boolean(keyName) && !unavailableKeyNames.has(keyName);
    })
  );
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

function pruneUnavailableSelectionMap(
  source: Map<string, Set<string>>,
  unavailableKeyNames: Set<string>
): Map<string, Set<string>> {
  const next = new Map<string, Set<string>>();

  source.forEach((keyNames, entityId) => {
    const filteredKeyNames = new Set(
      Array.from(keyNames).filter((keyName) => !unavailableKeyNames.has(keyName))
    );

    if (filteredKeyNames.size > 0) {
      next.set(entityId, filteredKeyNames);
    }
  });

  return next;
}

function formatReservedKeyMessage(owners: Set<string>): string {
  const ownerNames = Array.from(owners).filter(Boolean);
  if (ownerNames.length === 0) {
    return 'This API key is already used in another application.';
  }
  if (ownerNames.length === 1) {
    return `Already used in application ${ownerNames[0]}.`;
  }
  return `Already used in ${ownerNames.length} other applications.`;
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
  apiKeysMap: Map<string, MappedAPIKey[]>,
  unavailableKeyNames: Set<string>
) {
  return Array.from(selectedKeyNamesMap.entries())
    .filter(([entityId]) => linkedIds.has(entityId))
    .flatMap(([entityId, keyNames]) => {
      const mappedKeyIds = new Set(
        (apiKeysMap.get(entityId) ?? []).map((key) => key.keyId)
      );
      return Array.from(keyNames)
        .filter(
          (keyName) =>
            !mappedKeyIds.has(keyName) && !unavailableKeyNames.has(keyName)
        )
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

function buildDisabledKeyStateByEntity(
  entityKeysMap: Map<string, UserAPIKey[]>,
  unavailableKeyNames: Set<string>,
  reservedKeyOwners: Map<string, Set<string>>
): {
  disabledKeyNamesByEntity: Map<string, Set<string>>;
  disabledReasonsByEntity: Map<string, Map<string, string>>;
} {
  const disabledKeyNamesByEntity = new Map<string, Set<string>>();
  const disabledReasonsByEntity = new Map<string, Map<string, string>>();

  entityKeysMap.forEach((keys, entityId) => {
    const disabledKeyNames = new Set<string>();
    const disabledReasons = new Map<string, string>();

    keys.forEach((key) => {
      const keyName = key.name ?? '';
      if (!keyName || !unavailableKeyNames.has(keyName)) {
        return;
      }

      disabledKeyNames.add(keyName);
      disabledReasons.set(
        keyName,
        formatReservedKeyMessage(reservedKeyOwners.get(keyName) ?? new Set())
      );
    });

    disabledKeyNamesByEntity.set(entityId, disabledKeyNames);
    disabledReasonsByEntity.set(entityId, disabledReasons);
  });

  return { disabledKeyNamesByEntity, disabledReasonsByEntity };
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
  unavailableKeyNames = new Set<string>(),
  setSelectedKeyNamesMap,
}: LoadEntityKeysArgs): Promise<void> {
  if (keysMap.has(entityId)) {
    if (preselectLatest && setSelectedKeyNamesMap) {
      const latestKey = getLatestSelectableKey(
        keysMap.get(entityId) ?? [],
        unavailableKeyNames
      );
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
    const latestKey = getLatestSelectableKey(activeKeys, unavailableKeyNames);

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

export default function AssociationsTab() {
  const { applicationId = '' } = useParams<{ applicationId: string }>();
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

  const [reservedKeyOwners, setReservedKeyOwners] = useState<
    Map<string, Set<string>>
  >(new Map());
  const [isReservedKeysLoading, setIsReservedKeysLoading] = useState(false);
  const [reservedKeysLoadError, setReservedKeysLoadError] = useState<
    string | null
  >(null);

  const [searchValue, setSearchValue] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);

  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());
  const [apiKeysMap, setApiKeysMap] = useState<Map<string, MappedAPIKey[]>>(
    new Map()
  );
  const [loadingKeyIds, setLoadingKeyIds] = useState<Set<string>>(new Set());
  const [removingKeyIds, setRemovingKeyIds] = useState<Set<string>>(new Set());

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

  const [deleteTarget, setDeleteTarget] =
    useState<ApplicationAssociation | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);

  const [manageKeysDrawerTarget, setManageKeysDrawerTarget] =
    useState<ApplicationAssociation | null>(null);
  const [selectedManageKeyNames, setSelectedManageKeyNames] = useState<
    Set<string>
  >(new Set());
  const [isAddingManagedKeys, setIsAddingManagedKeys] = useState(false);

  const providerAssociations = useMemo(
    () => associations.filter((association) => association.kind === 'LlmProvider'),
    [associations]
  );
  const proxyAssociations = useMemo(
    () => associations.filter((association) => association.kind === 'LlmProxy'),
    [associations]
  );
  const allAssociations = useMemo(() => {
    const seen = new Set<string>();
    return [...providerAssociations, ...proxyAssociations].filter(
      (association) => {
        if (seen.has(association.id)) return false;
        seen.add(association.id);
        return true;
      }
    );
  }, [providerAssociations, proxyAssociations]);

  const linkedProviderIds = useMemo(
    () => new Set(providerAssociations.map((association) => association.id)),
    [providerAssociations]
  );
  const linkedProxyIds = useMemo(
    () => new Set(proxyAssociations.map((association) => association.id)),
    [proxyAssociations]
  );
  const unavailableKeyNames = useMemo(
    () => new Set(reservedKeyOwners.keys()),
    [reservedKeyOwners]
  );
  const selectionBlockedMessage = isReservedKeysLoading
    ? 'Checking whether these API keys are already used in another application.'
    : reservedKeysLoadError;

  useEffect(() => {
    if (!applicationId || !currentOrganization?.uuid) {
      setReservedKeyOwners(new Map());
      setReservedKeysLoadError(null);
      setIsReservedKeysLoading(false);
      return;
    }

    let isMounted = true;

    const loadReservedKeys = async () => {
      try {
        setIsReservedKeysLoading(true);
        setReservedKeysLoadError(null);

        const applicationsResponse = await applicationApis.getApplications(
          currentOrganization.uuid,
          apimBaseUrl,
          {
            projectId: currentProject?.id || undefined,
            limit: 1000,
          }
        );
        const otherApplications = (applicationsResponse.list ?? []).filter(
          (application) => application.id && application.id !== applicationId
        );
        const keyResponses = await Promise.allSettled(
          otherApplications.map(async (application) => ({
            application,
            keysResponse: await applicationApis.getApplicationAPIKeys(
              application.id,
              currentOrganization.uuid,
              apimBaseUrl
            ),
          }))
        );

        if (!isMounted) return;

        const nextReservedKeyOwners = new Map<string, Set<string>>();

        keyResponses.forEach((result) => {
          if (result.status !== 'fulfilled') {
            return;
          }

          const ownerName =
            result.value.application.name || result.value.application.id;

          (result.value.keysResponse.list ?? []).forEach((key) => {
            if (!key.keyId) return;

            const owners = nextReservedKeyOwners.get(key.keyId) ?? new Set();
            owners.add(ownerName);
            nextReservedKeyOwners.set(key.keyId, owners);
          });
        });

        setReservedKeyOwners(nextReservedKeyOwners);

        if (keyResponses.some((result) => result.status === 'rejected')) {
          setReservedKeysLoadError(
            'Could not verify API key usage across all other applications.'
          );
          return;
        }

        setReservedKeysLoadError(null);
      } catch {
        if (!isMounted) return;

        setReservedKeyOwners(new Map());
        setReservedKeysLoadError(
          'Could not load other applications to validate API key availability.'
        );
      } finally {
        if (isMounted) {
          setIsReservedKeysLoading(false);
        }
      }
    };

    void loadReservedKeys();

    return () => {
      isMounted = false;
    };
  }, [
    applicationId,
    apimBaseUrl,
    currentOrganization?.uuid,
    currentProject?.id,
  ]);

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
        unavailableKeyNames,
        setSelectedKeyNamesMap: setSelectedProviderKeyNamesMap,
      });
    },
    [loadingProviderKeyIds, providerKeysMap, unavailableKeyNames]
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
        unavailableKeyNames,
        setSelectedKeyNamesMap: setSelectedProxyKeyNamesMap,
      });
    },
    [loadingProxyKeyIds, proxyKeysMap, unavailableKeyNames]
  );

  const loadAssociationKeys = useCallback(
    async (associationId: string) => {
      if (apiKeysMap.has(associationId) || loadingKeyIds.has(associationId)) {
        return;
      }

      setLoadingKeyIds((prev) => new Set(prev).add(associationId));

      try {
        const response = await listAssociationAPIKeys(associationId);
        setApiKeysMap((prev) =>
          new Map(prev).set(associationId, response.list ?? [])
        );
      } catch {
        setApiKeysMap((prev) => new Map(prev).set(associationId, []));
      } finally {
        setLoadingKeyIds((prev) => removeSetValue(prev, associationId));
      }
    },
    [apiKeysMap, listAssociationAPIKeys, loadingKeyIds]
  );

  const loadAssociationKeysRef = useRef(loadAssociationKeys);
  loadAssociationKeysRef.current = loadAssociationKeys;
  const loadProviderKeysRef = useRef(loadProviderKeys);
  loadProviderKeysRef.current = loadProviderKeys;
  const loadProxyKeysRef = useRef(loadProxyKeys);
  loadProxyKeysRef.current = loadProxyKeys;

  useEffect(() => {
    if (!currentOrganization?.uuid || allAssociations.length === 0) return;

    const orgUuid = currentOrganization.uuid;
    allAssociations.forEach((association) => {
      void loadAssociationKeysRef.current(association.id);
      if (association.kind === 'LlmProvider') {
        void loadProviderKeysRef.current(association.id, orgUuid);
      } else {
        void loadProxyKeysRef.current(association.id, orgUuid);
      }
    });
  }, [allAssociations, currentOrganization?.uuid]);

  useEffect(() => {
    if (unavailableKeyNames.size === 0) return;

    setSelectedProviderKeyNamesMap((prev) =>
      pruneUnavailableSelectionMap(prev, unavailableKeyNames)
    );
    setSelectedProxyKeyNamesMap((prev) =>
      pruneUnavailableSelectionMap(prev, unavailableKeyNames)
    );
    setSelectedManageKeyNames(
      (prev) =>
        new Set(
          Array.from(prev).filter((keyName) => !unavailableKeyNames.has(keyName))
        )
    );
  }, [unavailableKeyNames]);

  const filteredAssociations = useMemo(() => {
    const query = searchValue.trim().toLowerCase();
    if (!query) return allAssociations;

    return allAssociations.filter((association) => {
      const haystack = [association.id, association.name, association.kind]
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

  const loadOrganizationProviders = useCallback(async (): Promise<LLMProvider[]> => {
    if (!currentOrganization?.uuid) return [];

    const response = await getLLMProviders(
      currentOrganization.uuid,
      apimBaseUrl
    );
    const providers = response.list ?? [];
    setOrgProviders(providers);
    return providers;
  }, [apimBaseUrl, currentOrganization?.uuid]);

  const loadOrganizationProxies = useCallback(async (): Promise<Proxy[]> => {
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
  }, [apimBaseUrl, currentOrganization?.uuid, currentProject?.id]);

  const ensureOrgProvidersLoaded = async (): Promise<LLMProvider[]> => {
    if (orgProviders.length > 0) return orgProviders;
    return loadOrganizationProviders();
  };

  const ensureOrgProxiesLoaded = async (): Promise<Proxy[]> => {
    if (orgProxies.length > 0) return orgProxies;
    return loadOrganizationProxies();
  };

  const handleToggleExpand = async (association: ApplicationAssociation) => {
    if (expandedIds.has(association.id)) {
      setExpandedIds((prev) => removeSetValue(prev, association.id));
      return;
    }

    setExpandedIds((prev) => new Set(prev).add(association.id));
    await loadAssociationKeys(association.id);

    if (!currentOrganization?.uuid) return;

    if (association.kind === 'LlmProvider') {
      await loadProviderKeys(association.id, currentOrganization.uuid);
      return;
    }

    await loadProxyKeys(association.id, currentOrganization.uuid);
  };

  const resetProviderDrawerState = () => {
    setSelectedProviderIds(new Set());
    setSelectedProviderKeyNamesMap(new Map());
    setExpandedLinkedProviderIds(new Set());
    setProviderDrawerSearch('');
    setProviderDrawerLoadError(null);
  };

  const handleCloseProviderDrawer = () => {
    if (isAddingProviders) return;
    setProviderDrawerOpen(false);
    resetProviderDrawerState();
  };

  const handleOpenProviderDrawer = async () => {
    setProviderDrawerOpen(true);
    resetProviderDrawerState();
    if (!currentOrganization?.uuid) return;

    setIsProviderDrawerLoading(true);
    try {
      await loadOrganizationProviders();
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
    await loadProviderKeys(
      provider.id,
      currentOrganization.uuid,
      !selectionBlockedMessage
    );
  };

  const handleToggleProviderKey = (providerId: string, keyName: string) => {
    if (selectionBlockedMessage || unavailableKeyNames.has(keyName)) return;

    setSelectedProviderKeyNamesMap((prev) =>
      toggleMapSelectionValue(prev, providerId, keyName)
    );
  };

  const linkedProviderKeyPayload = useMemo(
    () =>
      buildLinkedKeyPayload(
        selectedProviderKeyNamesMap,
        linkedProviderIds,
        apiKeysMap,
        unavailableKeyNames
      ),
    [
      apiKeysMap,
      linkedProviderIds,
      selectedProviderKeyNamesMap,
      unavailableKeyNames,
    ]
  );
  const hasPendingLinkedProviderKeys = linkedProviderKeyPayload.length > 0;

  const handleAddProviders = async () => {
    if (
      (selectedProviderIds.size === 0 && !hasPendingLinkedProviderKeys) ||
      isAddingProviders
    ) {
      return;
    }

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
        ...Array.from(selectedProviderIds).flatMap((providerId) =>
          Array.from(selectedProviderKeyNamesMap.get(providerId) ?? [])
            .filter((keyName) => !unavailableKeyNames.has(keyName))
            .map((keyName) => ({
              keyId: keyName,
              associatedEntity: { id: providerId },
            }))
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

      const providerCount = selectedProviderIds.size;
      const keyCount = linkedProviderKeyPayload.length;
      showSnackbar(
        providerCount > 0 && keyCount > 0
          ? `${providerCount} LLM provider${
              providerCount > 1 ? 's' : ''
            } associated and ${keyCount} API key${
              keyCount > 1 ? 's' : ''
            } added successfully.`
          : providerCount > 0
          ? `${providerCount} LLM provider${
              providerCount > 1 ? 's' : ''
            } associated successfully.`
          : `${keyCount} API key${keyCount > 1 ? 's' : ''} added successfully.`,
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

  const resetProxyDrawerState = () => {
    setSelectedProxyIds(new Set());
    setSelectedProxyKeyNamesMap(new Map());
    setExpandedLinkedProxyIds(new Set());
    setProxyDrawerSearch('');
    setProxyDrawerLoadError(null);
  };

  const handleCloseProxyDrawer = () => {
    if (isAddingProxies) return;
    setProxyDrawerOpen(false);
    resetProxyDrawerState();
  };

  const handleOpenProxyDrawer = async () => {
    setProxyDrawerOpen(true);
    resetProxyDrawerState();
    if (!currentOrganization?.uuid) return;

    setIsProxyDrawerLoading(true);
    try {
      await loadOrganizationProxies();
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
    await loadProxyKeys(
      proxy.id,
      currentOrganization.uuid,
      !selectionBlockedMessage
    );
  };

  const handleToggleProxyKey = (proxyId: string, keyName: string) => {
    if (selectionBlockedMessage || unavailableKeyNames.has(keyName)) return;

    setSelectedProxyKeyNamesMap((prev) =>
      toggleMapSelectionValue(prev, proxyId, keyName)
    );
  };

  const linkedProxyKeyPayload = useMemo(
    () =>
      buildLinkedKeyPayload(
        selectedProxyKeyNamesMap,
        linkedProxyIds,
        apiKeysMap,
        unavailableKeyNames
      ),
    [
      apiKeysMap,
      linkedProxyIds,
      selectedProxyKeyNamesMap,
      unavailableKeyNames,
    ]
  );
  const hasPendingLinkedProxyKeys = linkedProxyKeyPayload.length > 0;

  const handleAddProxies = async () => {
    if (
      (selectedProxyIds.size === 0 && !hasPendingLinkedProxyKeys) ||
      isAddingProxies
    ) {
      return;
    }

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
        ...Array.from(selectedProxyIds).flatMap((proxyId) =>
          Array.from(selectedProxyKeyNamesMap.get(proxyId) ?? [])
            .filter((keyName) => !unavailableKeyNames.has(keyName))
            .map((keyName) => ({
              keyId: keyName,
              associatedEntity: { id: proxyId },
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

      const proxyCount = selectedProxyIds.size;
      const keyCount = linkedProxyKeyPayload.length;
      showSnackbar(
        proxyCount > 0 && keyCount > 0
          ? `${proxyCount} LLM proxy${
              proxyCount > 1 ? 's' : ''
            } associated and ${keyCount} API key${
              keyCount > 1 ? 's' : ''
            } added successfully.`
          : proxyCount > 0
          ? `${proxyCount} LLM proxy${
              proxyCount > 1 ? 's' : ''
            } associated successfully.`
          : `${keyCount} API key${keyCount > 1 ? 's' : ''} added successfully.`,
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
          mappedKeys.map((key) =>
            removeAPIKey(resolveMappedKeyId(key), {
              entityID: resolveEntityId(key),
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

  const handleOpenManageKeysDrawer = async (association: ApplicationAssociation) => {
    if (!currentOrganization?.uuid) return;

    setManageKeysDrawerTarget(association);
    setSelectedManageKeyNames(new Set());

    try {
      if (association.kind === 'LlmProvider') {
        await ensureOrgProvidersLoaded();
      } else {
        await ensureOrgProxiesLoaded();
      }

      await Promise.all([
        loadAssociationKeys(association.id),
        association.kind === 'LlmProvider'
          ? loadProviderKeys(association.id, currentOrganization.uuid)
          : loadProxyKeys(association.id, currentOrganization.uuid),
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
    if (selectionBlockedMessage || unavailableKeyNames.has(keyName)) return;
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
        .filter(
          (keyId) =>
            !existingMappedKeyIds.has(keyId) && !unavailableKeyNames.has(keyId)
        )
        .map((keyId) => ({
          keyId,
          associatedEntity: { id: manageKeysDrawerTarget.id },
        }));

      if (apiKeys.length === 0) {
        handleCloseManageKeysDrawer();
        showSnackbar(
          'Selected API keys are already associated or used in another application.',
          'info'
        );
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

  const handleRemoveKey = async (associationId: string, key: MappedAPIKey) => {
    const mappedKeyId = resolveMappedKeyId(key);
    if (removingKeyIds.has(mappedKeyId)) return;

    setRemovingKeyIds((prev) => new Set(prev).add(mappedKeyId));

    try {
      await removeAPIKey(mappedKeyId, { entityID: resolveEntityId(key) });
      setApiKeysMap((prev) => {
        const next = new Map(prev);
        const currentKeys = next.get(associationId) ?? [];
        next.set(
          associationId,
          currentKeys.filter(
            (currentKey) => resolveMappedKeyId(currentKey) !== mappedKeyId
          )
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
  const showNoSearchResults =
    hasAssociations &&
    searchValue.trim().length > 0 &&
    filteredAssociations.length === 0;

  const providerDrawerKeyState = useMemo(
    () =>
      buildDisabledKeyStateByEntity(
        providerKeysMap,
        unavailableKeyNames,
        reservedKeyOwners
      ),
    [providerKeysMap, reservedKeyOwners, unavailableKeyNames]
  );
  const proxyDrawerKeyState = useMemo(
    () =>
      buildDisabledKeyStateByEntity(
        proxyKeysMap,
        unavailableKeyNames,
        reservedKeyOwners
      ),
    [proxyKeysMap, reservedKeyOwners, unavailableKeyNames]
  );

  const filteredOrgProviders = useMemo(
    () =>
      filterItemsByQuery(
        orgProviders,
        providerDrawerSearch,
        (provider) => [provider.name, provider.description, provider.template]
      ),
    [orgProviders, providerDrawerSearch]
  );
  const filteredOrgProxies = useMemo(
    () =>
      filterItemsByQuery(orgProxies, proxyDrawerSearch, (proxy) => [
        proxy.name,
        proxy.description,
        proxy.version,
        proxy.context,
      ]),
    [orgProxies, proxyDrawerSearch]
  );

  const providerAddButtonLabel = buildAddButtonLabel({
    isSubmitting: isAddingProviders,
    selectedCount: selectedProviderIds.size,
    pendingKeyCount: linkedProviderKeyPayload.length,
    entityLabel: 'Provider',
    defaultLabel: 'Add Providers',
  });
  const proxyAddButtonLabel = buildAddButtonLabel({
    isSubmitting: isAddingProxies,
    selectedCount: selectedProxyIds.size,
    pendingKeyCount: linkedProxyKeyPayload.length,
    entityLabel: 'Proxy',
    defaultLabel: 'Add LLM Proxy',
  });

  const managedIsProvider = manageKeysDrawerTarget?.kind === 'LlmProvider';
  const managedProvider = managedIsProvider
    ? orgProviders.find((provider) => provider.id === manageKeysDrawerTarget?.id) ??
      null
    : null;
  const managedProxy = !managedIsProvider
    ? orgProxies.find((proxy) => proxy.id === manageKeysDrawerTarget?.id) ?? null
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
    () => new Map(managedMappedKeys.map((key) => [key.keyId, key.status])),
    [managedMappedKeys]
  );
  const managedDisabledKeyNames = useMemo(
    () =>
      new Set(
        managedAvailableKeys
          .map((key) => key.name ?? '')
          .filter(
            (keyName) => Boolean(keyName) && unavailableKeyNames.has(keyName)
          )
      ),
    [managedAvailableKeys, unavailableKeyNames]
  );
  const managedDisabledReasons = useMemo(
    () =>
      new Map(
        Array.from(managedDisabledKeyNames).map((keyName) => [
          keyName,
          formatReservedKeyMessage(reservedKeyOwners.get(keyName) ?? new Set()),
        ])
      ),
    [managedDisabledKeyNames, reservedKeyOwners]
  );
  const managedVisibleKeys = getVisibleKeys(
    managedAvailableKeys,
    managedMappedKeys,
    true
  );
  const hasAddableManagedKeys = managedAvailableKeys.some(
    (key) =>
      key.name &&
      !managedMappedKeyStatusMap.has(key.name) &&
      !managedDisabledKeyNames.has(key.name)
  );

  return (
    <>
      <AssociationsTable
        isLoading={isLoading}
        loadError={loadError}
        hasAssociations={hasAssociations}
        showNoSearchResults={showNoSearchResults}
        filteredAssociationsCount={filteredAssociations.length}
        paginatedAssociations={paginatedAssociations}
        searchValue={searchValue}
        onSearchChange={setSearchValue}
        onOpenProviderDrawer={handleOpenProviderDrawer}
        onOpenProxyDrawer={handleOpenProxyDrawer}
        expandedIds={expandedIds}
        apiKeysMap={apiKeysMap}
        loadingKeyIds={loadingKeyIds}
        removingKeyIds={removingKeyIds}
        providerKeysMap={providerKeysMap}
        loadingProviderKeyIds={loadingProviderKeyIds}
        proxyKeysMap={proxyKeysMap}
        loadingProxyKeyIds={loadingProxyKeyIds}
        unavailableKeyNames={unavailableKeyNames}
        selectionBlockedMessage={selectionBlockedMessage}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={setPage}
        onRowsPerPageChange={(nextRowsPerPage) => {
          setRowsPerPage(nextRowsPerPage);
          setPage(0);
        }}
        onToggleExpand={handleToggleExpand}
        onOpenManageKeysDrawer={handleOpenManageKeysDrawer}
        onDeleteAssociation={setDeleteTarget}
        onRemoveKey={handleRemoveKey}
      />

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
        disabledKeyNamesByEntity={
          providerDrawerKeyState.disabledKeyNamesByEntity
        }
        disabledReasonsByEntity={providerDrawerKeyState.disabledReasonsByEntity}
        selectionBlockedMessage={selectionBlockedMessage}
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
        disabledKeyNamesByEntity={proxyDrawerKeyState.disabledKeyNamesByEntity}
        disabledReasonsByEntity={proxyDrawerKeyState.disabledReasonsByEntity}
        selectionBlockedMessage={selectionBlockedMessage}
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
          {manageKeysDrawerTarget ? (
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
                      ? managedProvider?.name
                      : managedProxy?.name) ||
                      manageKeysDrawerTarget.name ||
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
                        ? managedProvider?.name
                        : managedProxy?.name) ||
                        manageKeysDrawerTarget.name ||
                        '—'}
                    </Typography>
                    {managedIsProvider && managedProviderTemplate ? (
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
                    ) : null}
                    {!managedIsProvider && managedProxy?.version ? (
                      <Chip
                        label={`v${managedProxy.version}`}
                        size="small"
                        variant="outlined"
                        color="primary"
                        sx={{ borderRadius: 0.5 }}
                      />
                    ) : null}
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
                    disabledKeyNames={managedDisabledKeyNames}
                    disabledReasonByName={managedDisabledReasons}
                    keyStatusByName={managedMappedKeyStatusMap}
                    selectionBlockedMessage={selectionBlockedMessage}
                    emptyText={`No active API keys available for this ${
                      managedIsProvider ? 'provider' : 'proxy'
                    }.`}
                    onToggleKey={handleToggleManagedKey}
                  />
                )}
              </Box>
            </Card>
          ) : null}
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
            <strong>{deleteTarget?.name || deleteTarget?.id}</strong> from this
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
