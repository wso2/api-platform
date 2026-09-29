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

import React, { useEffect, useMemo, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
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
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  Link,
  ListingTable,
  MenuItem,
  PageContent,
  Select,
  Skeleton,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronLeft,
  Clock,
  Copy,
  Edit,
  Eye,
  EyeOff,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { formatRelativeTime } from '../proxies/LLMProxyLayout';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { agentProxiesApis } from '../../../../apis/agent/agentProxiesApis';
import { getAgentProxyDeployments } from '../../../../apis/agent/agentProxyDeployApis';
import { getGateways } from '../../../../apis/gatewayApis';
import type { Gateway } from '../../../../apis/gatewayTypes';
import type {
  A2AAgentCardConfig,
  A2AOperationConfig,
  A2ATransport,
  AgentCardDocument,
  AgentCardMode,
  AgentProxy,
  GlobalPolicy,
  UserAPIKey,
} from '../../../../utils/types';
import { getErrorMessage } from '../../../../utils/apiError';
import {
  DisabledActionTooltip,
  GatewayArtifactDeleteWarning,
  GatewayArtifactReadOnlyBanner,
} from '../../../../utils/readOnlyArtifacts';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import {
  NO_PERMISSION_TOOLTIP,
  SCOPES,
} from '../../../../auth/permissions';
import A2AIcon from '../../../../assets/icons/a2a.svg';
import ExternalServerStepBanner from '../quickStart/ExternalServerStepBanner';
import type { ExternalServerStepBannerStepId } from '../quickStart/ExternalServerStepBanner';
import { AGENT_TRANSPORT_OPTIONS } from './AgentProxiesCreateForm';
import AgentProxyCardTab from './AgentProxyCardTab';
import TransportPathField from './TransportPathField';
import AgentProxyGuardrailsTab, {
} from './AgentProxyGuardrailsTab';
import type { AgentPolicyState } from './AgentProxyGuardrailsTab';
import type { SelectedPolicy } from './AgentPolicyMapper';
import { getPolicies } from '../../../../apis/policyHubApis';
import {
  createSecret,
  deleteSecret,
  buildSecretPlaceholder,
  generateSecretHandle,
  extractSecretHandle,
} from '../../../../apis/secretApis';
import type { AgentCardTabState } from './AgentProxyCardTab';

type TabPanelProps = {
  children: React.ReactNode;
  value: number;
  index: number;
};

function TabPanel({ children, value, index }: TabPanelProps): React.JSX.Element {
  return (
    <Box role="tabpanel" hidden={value !== index}>
      {value === index ? children : null}
    </Box>
  );
}

// Stands in for the write-only stored credential, which the API never returns.
const MASKED_CREDENTIAL_VALUE = '******';

/** Auth types that store no credential of their own. */
function isNoCredentialAuthType(type: string): boolean {
  return type === 'none' || type === 'other';
}

const API_KEY_AUTH_POLICY_NAME = 'api-key-auth';

/** Stored path per binding, falling back to the platform default. */
function pathsOf(proxy: AgentProxy): Record<string, string> {
  const stored = proxy.a2a?.operationConfigs?.transports ?? [];
  return Object.fromEntries(
    AGENT_TRANSPORT_OPTIONS.map((option) => [
      option.protocolBinding,
      stored.find(
        (transport) => transport.protocolBinding === option.protocolBinding
      )?.pathPrefix ?? option.pathPrefix,
    ])
  );
}

const TAB_LABELS = [
  'Overview',
  'Agent Card',
  'Guardrails & Policies',
  'Backend Connection',
];

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

const EMPTY_CARD_STATE: AgentCardTabState = {
  publicMode: 'passthrough',
  publicRewriteUrls: true,
  publicContent: '',
  protectedMode: 'passthrough',
  protectedRewriteUrls: true,
  protectedContent: '',
};

const MAX_CARD_BYTES = 1024 * 1024;

function stringifyCard(content?: AgentCardDocument): string {
  return content ? JSON.stringify(content, null, 2) : '';
}

/**
 * Card state from the stored resource. An omitted agentCard, or an omitted
 * public block, both mean passthrough, so the mode is resolved here rather
 * than inferred from whether content is present.
 */
function toCardState(agentProxy: AgentProxy): AgentCardTabState {
  const card = agentProxy.a2a?.agentCard;
  return {
    publicMode: card?.public?.mode ?? 'passthrough',
    publicRewriteUrls: card?.public?.rewriteUrls ?? true,
    publicContent: stringifyCard(card?.public?.content),
    protectedMode: card?.protected?.mode ?? 'passthrough',
    protectedRewriteUrls: card?.protected?.rewriteUrls ?? true,
    protectedContent: stringifyCard(card?.protected?.content),
  };
}

const EMPTY_POLICY_STATE: AgentPolicyState = {
  globalPolicies: [],
  operationPolicies: {},
  publicCardPolicies: [],
};

type StoredPolicy = {
  name: string;
  version: string;
  params?: Record<string, unknown>;
};

/** Stored policy -> SelectedPolicy, resolving display names from Policy Hub. */
function toSelectedPolicies(
  policies: StoredPolicy[] | undefined,
  displayNames: Map<string, string>,
  keyPrefix: string
): SelectedPolicy[] {
  return (policies ?? []).map((policy, index) => ({
    instanceId: `${keyPrefix}-${policy.name}-${index}`,
    policyId: policy.name,
    policyName: policy.name,
    displayName: displayNames.get(policy.name) || policy.name,
    version: policy.version,
    params: policy.params ?? {},
  }));
}

function toStoredPolicies(policies: SelectedPolicy[]): GlobalPolicy[] {
  return policies.map((policy) => ({
    name: policy.policyName,
    version: policy.version,
    ...(policy.params && Object.keys(policy.params).length > 0
      ? { params: policy.params }
      : {}),
  }));
}

function getErrorDescription(error: unknown, fallback: string): string {
  return getErrorMessage(error, fallback);
}

function buildApiKeyResourceName(displayName: string): string {
  const normalizedDisplayName = displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalizedDisplayName || 'api-key';
}

export default function AgentProxyOverview(): React.JSX.Element {
  const navigate = useNavigate();
  const { projectSlug, agentProxyId } = useParams<{
    projectSlug: string;
    agentProxyId: string;
  }>();
  const { currentProject, currentOrganization, projectsForCurrentOrganization } =
    useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const canUpdateAgentProxy = hasPermission(SCOPES.AGENT_PROXY_UPDATE);
  const canDeleteAgentProxy = hasPermission(SCOPES.AGENT_PROXY_DELETE);
  const canReadApiKeys = hasPermission(SCOPES.AGENT_PROXY_API_KEY_READ);
  const canCreateApiKey = hasPermission(SCOPES.AGENT_PROXY_API_KEY_CREATE);
  const canDeleteApiKey = hasPermission(SCOPES.AGENT_PROXY_API_KEY_DELETE);

  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const listPath = buildProjectPath(
    currentOrganization,
    effectiveProject,
    '/agent-proxy'
  );
  const organizationId = currentOrganization?.uuid ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  const [agentProxy, setAgentProxy] = useState<AgentProxy | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSavingChanges, setIsSavingChanges] = useState(false);
  const [tabIndex, setTabIndex] = useState(0);
  const [selectedTransports, setSelectedTransports] = useState<string[]>([]);
  const [editingTransport, setEditingTransport] = useState<string | null>(null);
  const [transportPaths, setTransportPaths] = useState<Record<string, string>>(
    {}
  );
  const [initialTransportPaths, setInitialTransportPaths] = useState<
    Record<string, string>
  >({});
  const [initialTransports, setInitialTransports] = useState<string[]>([]);
  const [cardState, setCardState] = useState<AgentCardTabState>(EMPTY_CARD_STATE);
  const [initialCardState, setInitialCardState] =
    useState<AgentCardTabState>(EMPTY_CARD_STATE);
  const [hasProtectedBlock, setHasProtectedBlock] = useState(false);
  const [endpointUrl, setEndpointUrl] = useState('');
  const [authType, setAuthType] = useState('none');
  const [authHeaderName, setAuthHeaderName] = useState('');
  const [authHeaderValue, setAuthHeaderValue] = useState('');
  const [showAuthHeaderValue, setShowAuthHeaderValue] = useState(false);
  const [isCredentialMasked, setIsCredentialMasked] = useState(false);
  const [hasCredentialChanged, setHasCredentialChanged] = useState(false);
  const [policyState, setPolicyState] =
    useState<AgentPolicyState>(EMPTY_POLICY_STATE);
  const [initialPolicyState, setInitialPolicyState] =
    useState<AgentPolicyState>(EMPTY_POLICY_STATE);
  const [fetchedCard, setFetchedCard] = useState<string | null>(null);
  const [isCardFetching, setIsCardFetching] = useState(false);
  const [cardFetchError, setCardFetchError] = useState<string | null>(null);
  const [deployedGateways, setDeployedGateways] = useState<Gateway[]>([]);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [apiKeys, setApiKeys] = useState<UserAPIKey[]>([]);
  const [isApiKeysLoading, setIsApiKeysLoading] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isKeyDialogOpen, setIsKeyDialogOpen] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [isCreatingKey, setIsCreatingKey] = useState(false);
  const [createdKeyValue, setCreatedKeyValue] = useState<string | null>(null);

  const isReadOnlyAgentProxy = Boolean(agentProxy?.readOnly);
  const isConnectionDisabled = isReadOnlyAgentProxy || !canUpdateAgentProxy;

  useEffect(() => {
    if (!agentProxyId) return;
    let cancelled = false;
    const load = async () => {
      try {
        setIsLoading(true);
        const loaded = await agentProxiesApis.getAgentProxy(
          agentProxyId,
          apimBaseUrl
        );
        if (cancelled) return;
        setAgentProxy(loaded);
        const bindings = (loaded.a2a?.operationConfigs?.transports ?? []).map(
          (transport) => transport.protocolBinding
        );
        setSelectedTransports(bindings);
        setInitialTransports(bindings);
        const loadedPaths = pathsOf(loaded);
        setTransportPaths(loadedPaths);
        setInitialTransportPaths(loadedPaths);
        const card = toCardState(loaded);
        setCardState(card);
        setInitialCardState(card);
        setHasProtectedBlock(Boolean(loaded.a2a?.agentCard?.protected));

        const displayNames = new Map<string, string>();
        try {
          const hub = await getPolicies();
          (hub.data ?? []).forEach((policy) => {
            if (policy.displayName) {
              displayNames.set(policy.name, policy.displayName);
            }
          });
        } catch {
          // fall back to raw policy names
        }
        if (cancelled) return;

        const operationPolicies: Record<string, SelectedPolicy[]> = {};
        (loaded.a2a?.operationConfigs?.operations ?? []).forEach((operation) => {
          operationPolicies[operation.name] = toSelectedPolicies(
            operation.policies,
            displayNames,
            `op-${operation.name}`
          );
        });
        const policies: AgentPolicyState = {
          globalPolicies: toSelectedPolicies(
            loaded.a2a?.operationConfigs?.policies,
            displayNames,
            'global'
          ),
          operationPolicies,
          publicCardPolicies: toSelectedPolicies(
            loaded.a2a?.agentCard?.public?.policies,
            displayNames,
            'card'
          ),
        };
        setPolicyState(policies);
        setInitialPolicyState(policies);
      } catch {
        // silently fail
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [agentProxyId, apimBaseUrl]);

  useEffect(() => {
    if (!agentProxyId) return;
    let cancelled = false;
    const loadGateways = async () => {
      try {
        const [deployments, gateways] = await Promise.all([
          getAgentProxyDeployments(agentProxyId, apimBaseUrl),
          getGateways(organizationId),
        ]);
        if (cancelled) return;
        const deployedIds = new Set(
          (deployments.list ?? [])
            .filter((deployment) => deployment.status === 'DEPLOYED')
            .map((deployment) => deployment.gatewayId)
        );
        const matching = (gateways.list ?? []).filter((gateway) =>
          deployedIds.has(gateway.id)
        );
        setDeployedGateways(matching);
        setSelectedGatewayId((current) =>
          current && matching.some((gateway) => gateway.id === current)
            ? current
            : (matching[0]?.id ?? '')
        );
      } catch {
        // a missing deployment list only hides the Agent Proxy URL
      }
    };
    void loadGateways();
    return () => {
      cancelled = true;
    };
  }, [agentProxyId, apimBaseUrl, organizationId]);

  useEffect(() => {
    if (!agentProxyId || !canReadApiKeys) return;
    let cancelled = false;
    const loadKeys = async () => {
      try {
        setIsApiKeysLoading(true);
        const response = await agentProxiesApis.getAgentProxyAPIKeys(
          agentProxyId,
          apimBaseUrl
        );
        if (!cancelled) setApiKeys(response.list ?? []);
      } catch {
        // silently fail on load
      } finally {
        if (!cancelled) setIsApiKeysLoading(false);
      }
    };
    void loadKeys();
    return () => {
      cancelled = true;
    };
  }, [agentProxyId, apimBaseUrl, canReadApiKeys]);

  // Managed content is what the gateway serves, so it is rendered from the
  // resource. Only passthrough, which stores nothing, goes to the upstream.
  const loadPassthroughCard = React.useCallback(
    async () => {
      if (!agentProxyId) return;
      setIsCardFetching(true);
      setCardFetchError(null);
      try {
        const card = await agentProxiesApis.fetchAgentCard(
          { agentProxyId },
          apimBaseUrl
        );
        setFetchedCard(JSON.stringify(card, null, 2));
      } catch (error) {
        setFetchedCard(null);
        setCardFetchError(
          getErrorDescription(error, 'The upstream agent could not be reached.')
        );
      } finally {
        setIsCardFetching(false);
      }
    },
    [agentProxyId, apimBaseUrl]
  );

  useEffect(() => {
    if (initialCardState.publicMode !== 'passthrough') return;
    void loadPassthroughCard();
  }, [initialCardState.publicMode, loadPassthroughCard]);

  useEffect(() => {
    if (!agentProxy) return;
    setEndpointUrl(agentProxy.upstream?.main?.url ?? '');
    setAuthType(agentProxy.upstream?.main?.auth?.type || 'none');
    setAuthHeaderName(agentProxy.upstream?.main?.auth?.header ?? '');
    // auth.value is write-only and never returned, so auth.header is the only
    // signal that a credential is already stored.
    const hasExistingAuth = Boolean(agentProxy.upstream?.main?.auth?.header);
    setAuthHeaderValue(hasExistingAuth ? MASKED_CREDENTIAL_VALUE : '');
    setIsCredentialMasked(hasExistingAuth);
    setHasCredentialChanged(false);
  }, [agentProxy]);

  const hasBackendConnectionChanges = useMemo(() => {
    if (!agentProxy) return false;
    const savedUrl = agentProxy.upstream?.main?.url ?? '';
    const savedHeaderName = agentProxy.upstream?.main?.auth?.header ?? '';
    const savedType = agentProxy.upstream?.main?.auth?.type || 'none';
    if (endpointUrl.trim() !== savedUrl.trim()) return true;
    if (authType !== savedType) return true;
    if (authHeaderName.trim() !== savedHeaderName.trim()) return true;
    if (!isCredentialMasked && hasCredentialChanged) return true;
    return false;
  }, [
    agentProxy,
    endpointUrl,
    authType,
    authHeaderName,
    isCredentialMasked,
    hasCredentialChanged,
  ]);

  const selectedGateway = useMemo(
    () =>
      deployedGateways.find((gateway) => gateway.id === selectedGatewayId) ??
      null,
    [deployedGateways, selectedGatewayId]
  );

  const generatedInvokeUrl = useMemo(() => {
    const vhost = (
      selectedGateway?.endpoints?.[0] || selectedGateway?.vhost
    )?.trim();
    if (!vhost) return '';
    const normalizedBase = /^https?:\/\//i.test(vhost)
      ? vhost.replace(/\/+$/, '')
      : `https://${vhost.replace(/\/+$/, '')}`;
    const context = (agentProxy?.context || '/').trim();
    const normalizedContext = context.startsWith('/') ? context : `/${context}`;
    return `${normalizedBase}${normalizedContext}`;
  }, [agentProxy?.context, selectedGateway]);

  const hasTransportChanges =
    [...selectedTransports].sort().join(',') !==
      [...initialTransports].sort().join(',') ||
    JSON.stringify(transportPaths) !== JSON.stringify(initialTransportPaths);
  const hasCardChanges =
    JSON.stringify(cardState) !== JSON.stringify(initialCardState);
  const hasPolicyChanges =
    JSON.stringify(policyState) !== JSON.stringify(initialPolicyState);
  const hasUnsavedChanges =
    hasTransportChanges ||
    hasCardChanges ||
    hasPolicyChanges ||
    hasBackendConnectionChanges;

  const hasPolicies = Boolean(
    (agentProxy?.a2a?.operationConfigs?.policies?.length ?? 0) > 0 ||
      (agentProxy?.a2a?.operationConfigs?.operations ?? []).some(
        (operation) => (operation.policies?.length ?? 0) > 0
      )
  );
  const hasApiKeyAuthPolicy = Boolean(
    (agentProxy?.a2a?.operationConfigs?.policies ?? []).some(
      (policy) => policy.name === API_KEY_AUTH_POLICY_NAME
    ) ||
      (agentProxy?.a2a?.operationConfigs?.operations ?? []).some((operation) =>
        (operation.policies ?? []).some(
          (policy) => policy.name === API_KEY_AUTH_POLICY_NAME
        )
      )
  );
  const isDeployed = deployedGateways.length > 0;

  const transportPathFor = (option: Required<A2ATransport>) =>
    transportPaths[option.protocolBinding] ?? option.pathPrefix;

  const handleStepBannerClick = (stepId: ExternalServerStepBannerStepId) => {
    if (stepId === 'add-policies') {
      setTabIndex(2);
    } else if (stepId === 'deploy-to-gateway') {
      navigate('deploy');
    }
  };

  const handleTransportToggle = (protocolBinding: string) => {
    if (isReadOnlyAgentProxy) return;
    setSelectedTransports((prev) =>
      prev.includes(protocolBinding)
        ? prev.filter((binding) => binding !== protocolBinding)
        : [...prev, protocolBinding]
    );
  };

  /**
   * PUT replaces the whole resource, so everything read is sent back and only
   * the edited fields are overridden. Dropping a field here deletes it — an
   * omitted associatedGateways empties the association set.
   */
  const saveAgentProxy = async (
    overrides: Partial<AgentProxy>,
    successMessage: string
  ): Promise<boolean> => {
    if (!agentProxy?.id) return false;
    const { createdAt, createdBy, updatedAt, updatedBy, readOnly, ...rest } =
      agentProxy;
    try {
      setIsSavingChanges(true);
      const updated = await agentProxiesApis.updateAgentProxy(
        agentProxy.id,
        { ...rest, ...overrides },
        apimBaseUrl
      );
      setAgentProxy(updated);
      const bindings = (updated.a2a?.operationConfigs?.transports ?? []).map(
        (transport) => transport.protocolBinding
      );
      setSelectedTransports(bindings);
      setInitialTransports(bindings);
      const savedPaths = pathsOf(updated);
      setTransportPaths(savedPaths);
      setInitialTransportPaths(savedPaths);
      const card = toCardState(updated);
      setCardState(card);
      setInitialCardState(card);
      setHasProtectedBlock(Boolean(updated.a2a?.agentCard?.protected));
      setInitialPolicyState(policyState);
      showSnackbar(successMessage, 'success');
      return true;
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to update Agent Proxy.'),
        'error'
      );
      return false;
    } finally {
      setIsSavingChanges(false);
    }
  };

  /**
   * rewriteUrls is rejected in managed mode and content is forbidden in
   * passthrough, so each mode contributes only its own keys. path and policies
   * are not edited here and are carried over so a save cannot drop them.
   */
  const buildAgentCardConfig = (): A2AAgentCardConfig | undefined => {
    const existing = agentProxy?.a2a?.agentCard;
    const cardPolicies = toStoredPolicies(policyState.publicCardPolicies);
    const carried = {
      ...(existing?.public?.path ? { path: existing.public.path } : {}),
      ...(cardPolicies.length > 0 ? { policies: cardPolicies } : {}),
    };

    const publicCard =
      cardState.publicMode === 'managed'
        ? {
            ...carried,
            mode: 'managed' as const,
            content: JSON.parse(cardState.publicContent) as AgentCardDocument,
          }
        : {
            ...carried,
            mode: 'passthrough' as const,
            rewriteUrls: cardState.publicRewriteUrls,
          };

    // An absent protected block stays absent until it is actually edited.
    const protectedTouched =
      cardState.protectedMode !== initialCardState.protectedMode ||
      cardState.protectedRewriteUrls !== initialCardState.protectedRewriteUrls ||
      cardState.protectedContent !== initialCardState.protectedContent;
    if (!hasProtectedBlock && !protectedTouched) {
      return { public: publicCard };
    }

    const protectedCard =
      cardState.protectedMode === 'managed'
        ? {
            mode: 'managed' as const,
            content: JSON.parse(
              cardState.protectedContent
            ) as AgentCardDocument,
          }
        : {
            mode: 'passthrough' as const,
            rewriteUrls: cardState.protectedRewriteUrls,
          };

    return { public: publicCard, protected: protectedCard };
  };

  const validateCardState = (): string | null => {
    const checks: Array<[AgentCardMode, string, string]> = [
      [cardState.publicMode, cardState.publicContent, 'Public'],
      [cardState.protectedMode, cardState.protectedContent, 'Protected'],
    ];
    for (const [mode, content, label] of checks) {
      if (mode !== 'managed') continue;
      if (!content.trim()) {
        return `${label} card content is required in Managed mode.`;
      }
      if (new Blob([content]).size > MAX_CARD_BYTES) {
        return `${label} card content exceeds the 1 MiB limit.`;
      }
      try {
        JSON.parse(content);
      } catch {
        return `${label} card content is not valid JSON.`;
      }
    }
    return null;
  };

  /**
   * Agent-wide policies plus one entry per operation that has any. An operation
   * with no policies is dropped — the list is not an allowlist, so its absence
   * changes nothing except payload size. Existing resilience values are kept.
   */
  const buildOperationConfigs = (transports: A2ATransport[]) => {
    const existing = agentProxy?.a2a?.operationConfigs;
    const globalPolicies = toStoredPolicies(policyState.globalPolicies);

    const operations: A2AOperationConfig[] = [];
    Object.entries(policyState.operationPolicies).forEach(([name, list]) => {
      const policies = toStoredPolicies(list);
      const resilience = existing?.operations?.find(
        (operation) => operation.name === name
      )?.resilience;
      if (policies.length === 0 && !resilience) return;
      operations.push({
        name,
        ...(policies.length > 0 ? { policies } : {}),
        ...(resilience ? { resilience } : {}),
      });
    });

    return {
      transports,
      ...(globalPolicies.length > 0 ? { policies: globalPolicies } : {}),
      ...(operations.length > 0 ? { operations } : {}),
    };
  };

  const handleSaveChanges = async () => {
    if (!agentProxy?.a2a) return;
    const cardError = validateCardState();
    if (cardError) {
      showSnackbar(cardError, 'error');
      return;
    }
    const transports = AGENT_TRANSPORT_OPTIONS.filter((transport) =>
      selectedTransports.includes(transport.protocolBinding)
    ).map((transport) => {
      return {
        protocolBinding: transport.protocolBinding,
        pathPrefix:
          transportPaths[transport.protocolBinding] ?? transport.pathPrefix,
      };
    });
    // Rotating the credential creates the secret up front so the payload never
    // carries plaintext; the old secret is removed once the update succeeds.
    const isRotatingCredential = !isCredentialMasked && hasCredentialChanged;
    let upstreamPayload = agentProxy.upstream;
    let newSecretHandle: string | null = null;

    if (hasBackendConnectionChanges) {
      const trimmedUrl = endpointUrl.trim();
      const trimmedHeaderName = authHeaderName.trim();
      let authPayload = agentProxy.upstream?.main?.auth;

      if (isNoCredentialAuthType(authType)) {
        authPayload = { type: authType };
      } else if (isRotatingCredential) {
        const trimmedValue = authHeaderValue.trim();
        if (trimmedHeaderName && trimmedValue) {
          try {
            const secretResponse = await createSecret({
              id: generateSecretHandle(),
              displayName: `${agentProxy.displayName} upstream auth`,
              description: `Auto-generated secret for Agent proxy ${agentProxy.displayName}`,
              value: trimmedValue,
              type: 'GENERIC',
            });
            newSecretHandle = secretResponse.id;
            authPayload = {
              type: authType,
              header: trimmedHeaderName,
              value: buildSecretPlaceholder(secretResponse.id),
            };
          } catch {
            showSnackbar('Failed to encrypt upstream auth credential', 'error');
            return;
          }
        } else {
          authPayload = { type: authType, header: trimmedHeaderName };
        }
      } else {
        // Omitting the write-only value retains the stored credential for an
        // unchanged auth configuration.
        authPayload = {
          ...agentProxy.upstream?.main?.auth,
          type: authType,
          header: trimmedHeaderName,
        };
      }

      upstreamPayload = {
        main: {
          ...agentProxy.upstream?.main,
          url: trimmedUrl,
          auth: authPayload,
        },
      };
    }

    const saved = await saveAgentProxy(
      {
        upstream: upstreamPayload,
        a2a: {
          ...agentProxy.a2a,
          operationConfigs: buildOperationConfigs(transports),
          agentCard: buildAgentCardConfig(),
        },
      },
      'Agent Proxy updated successfully.'
    );

    if (saved && isRotatingCredential) {
      const oldHandle = agentProxy.upstream?.main?.auth?.value
        ? extractSecretHandle(agentProxy.upstream.main.auth.value)
        : null;
      if (oldHandle && oldHandle !== newSecretHandle) {
        deleteSecret(oldHandle).catch(() => {
          // best-effort cleanup
        });
      }
    }
  };

  const handleCancelChanges = () => {
    setSelectedTransports(initialTransports);
    setTransportPaths(initialTransportPaths);
    setCardState(initialCardState);
    setPolicyState(initialPolicyState);
  };

  const handleDeleteConfirm = async () => {
    if (!agentProxy?.id) return;
    try {
      setIsDeleting(true);
      await agentProxiesApis.deleteAgentProxy(agentProxy.id, apimBaseUrl);
      showSnackbar('Agent Proxy deleted successfully.', 'success');
      navigate(listPath);
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to delete Agent Proxy.'),
        'error'
      );
    } finally {
      setIsDeleting(false);
      setIsDeleteDialogOpen(false);
    }
  };

  const handleCreateKey = async () => {
    if (!agentProxy?.id || !newKeyName.trim()) return;
    try {
      setIsCreatingKey(true);
      const trimmedKeyName = newKeyName.trim();
      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);

      const created = await agentProxiesApis.createAgentProxyAPIKey(
        agentProxy.id,
        {
          id: buildApiKeyResourceName(trimmedKeyName),
          displayName: trimmedKeyName,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        },
        apimBaseUrl
      );
      setCreatedKeyValue(created.apiKey);
      const response = await agentProxiesApis.getAgentProxyAPIKeys(
        agentProxy.id,
        apimBaseUrl
      );
      setApiKeys(response.list ?? []);
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to generate API key.'),
        'error'
      );
    } finally {
      setIsCreatingKey(false);
    }
  };

  const handleRevokeKey = async (apiKey: UserAPIKey) => {
    if (!agentProxy?.id || !apiKey.id) return;
    try {
      await agentProxiesApis.revokeAgentProxyAPIKey(
        agentProxy.id,
        apiKey.id,
        apimBaseUrl
      );
      setApiKeys((prev) => prev.filter((key) => key.id !== apiKey.id));
      showSnackbar('API key revoked.', 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to revoke API key.'),
        'error'
      );
    }
  };

  const handleCopyCreatedKey = async () => {
    if (!createdKeyValue) return;
    try {
      await navigator.clipboard.writeText(createdKeyValue);
      showSnackbar('API key copied to clipboard.', 'success');
    } catch {
      showSnackbar('Could not copy the API key.', 'error');
    }
  };

  const handleCopyInvokeUrl = async () => {
    if (!generatedInvokeUrl) return;
    try {
      await navigator.clipboard.writeText(generatedInvokeUrl);
      showSnackbar('URL copied to clipboard.', 'success');
    } catch {
      showSnackbar('Could not copy the URL.', 'error');
    }
  };

  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2}>
          <Skeleton variant="rectangular" height={130} />
          <Skeleton variant="rectangular" height={320} />
        </Stack>
      </PageContent>
    );
  }

  if (!agentProxy) {
    return (
      <PageContent fullWidth>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.not.found"
            defaultMessage="Agent Proxy not found."
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={listPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ px: 0, minWidth: 'auto' }}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.back.to.list"
          defaultMessage="Back to List"
        />
      </Button>

      <ExternalServerStepBanner
        serverName={agentProxy.displayName}
        fallbackName="Agent Proxy"
        description="Click each step to secure and deploy your agent proxy."
        hasPolicies={hasPolicies}
        hasDeployments={isDeployed}
        onStepClick={handleStepBannerClick}
      />

      <Stack spacing={3} sx={{ mt: 2, mb: 4 }}>
        <Card>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'stretch',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 2,
              padding: 2,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
              <Avatar
                sx={{
                  width: 72,
                  height: 72,
                  fontWeight: 600,
                  fontSize: 28,
                  bgcolor: 'primary.light',
                  color: 'primary.contrastText',
                }}
              >
                {getInitials(agentProxy.displayName)}
              </Avatar>
              <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  <Chip
                    label=" A2A"
                    size="small"
                    variant="outlined"
                    color="primary"
                    sx={{ borderRadius: 0.5 }}
                    icon={
                      <Box
                        component="img"
                        src={A2AIcon}
                        alt=""
                        sx={{
                          width: 16,
                          height: 16,
                          objectFit: 'contain',
                        }}
                      />
                    }
                  />
                </Stack>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  <Typography variant="h3">{agentProxy.displayName}</Typography>
                  <Chip
                    label={agentProxy.version}
                    size="small"
                    variant="outlined"
                    color="primary"
                  />
                  <Tooltip title="Edit Agent Proxy">
                    <IconButton component={RouterLink} to="edit" size="small">
                      <Edit size={16} />
                    </IconButton>
                  </Tooltip>
                </Stack>
                <Stack spacing={0.2}>
                  <Stack direction="row" alignItems="center" gap={2}>
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.context.label"
                        defaultMessage="Context :"
                      />
                    </Typography>
                    <Typography variant="body2">
                      {agentProxy.context || '/'}
                    </Typography>
                  </Stack>
                  <Stack direction="row" spacing={0.75} alignItems="center">
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.updated"
                        defaultMessage="Last updated :"
                      />
                    </Typography>
                    <Clock size={14} />
                    <Typography variant="caption" color="text.secondary">
                      {formatRelativeTime(agentProxy.updatedAt)}
                    </Typography>
                  </Stack>
                  {agentProxy.createdBy ? (
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.createdBy"
                        defaultMessage="Created by: {createdBy}"
                        values={{ createdBy: agentProxy.createdBy }}
                      />
                    </Typography>
                  ) : null}
                </Stack>
              </Stack>
            </Box>
            <Stack
              direction="column"
              justifyContent="space-between"
              alignItems="flex-end"
              spacing={1.5}
              sx={{ alignSelf: 'stretch' }}
            >
              <Button variant="contained" component={RouterLink} to="deploy">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.deploy"
                  defaultMessage="Deploy to Gateway"
                />
              </Button>
              <DisabledActionTooltip
                disabled={!canDeleteAgentProxy}
                title={NO_PERMISSION_TOOLTIP}
              >
                <IconButton
                  color="error"
                  disabled={!canDeleteAgentProxy}
                  onClick={() => setIsDeleteDialogOpen(true)}
                  aria-label={`Delete ${agentProxy.displayName}`}
                >
                  <Trash2 size={16} />
                </IconButton>
              </DisabledActionTooltip>
            </Stack>
          </Box>
        </Card>

        <Card>
          <Tabs
            value={tabIndex}
            onChange={(_event, value: number) => setTabIndex(value)}
            variant="scrollable"
            allowScrollButtonsMobile
          >
            {TAB_LABELS.map((label) => (
              <Tab key={label} label={label} />
            ))}
          </Tabs>
          <Divider />
          <Box padding={2}>
            <TabPanel value={tabIndex} index={0}>
              <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
                {/* Capped while this column has the row to itself. */}
                <Box
                  sx={{
                    flex: 1,
                    minWidth: 0,
                    maxWidth: isDeployed ? 'none' : 560,
                  }}
                >
                  <Stack spacing={1.5}>
                    <Typography variant="h6" sx={{ fontWeight: 600 }}>
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.transports"
                        defaultMessage="Transports"
                      />
                    </Typography>
                    <Grid container spacing={1.5}>
                      {AGENT_TRANSPORT_OPTIONS.map((transport) => {
                        const existing = (
                          agentProxy.a2a?.operationConfigs?.transports ?? []
                        ).find(
                          (current) =>
                            current.protocolBinding ===
                            transport.protocolBinding
                        );
                        return (
                          <Grid
                            key={transport.protocolBinding}
                            size={{ xs: 12, sm: 6 }}
                          >
                            <Stack
                              direction="row"
                              spacing={1}
                              alignItems="flex-start"
                              onClick={() =>
                                handleTransportToggle(transport.protocolBinding)
                              }
                              sx={{
                                p: 1.5,
                                border: '1px solid',
                                borderColor: 'divider',
                                borderRadius: 1,
                                cursor: isReadOnlyAgentProxy
                                  ? 'default'
                                  : 'pointer',
                              }}
                            >
                              <Checkbox
                                size="small"
                                sx={{ p: 0, mt: 0.25 }}
                                disabled={
                                  isReadOnlyAgentProxy || !canUpdateAgentProxy
                                }
                                checked={selectedTransports.includes(
                                  transport.protocolBinding
                                )}
                                onChange={() =>
                                  handleTransportToggle(
                                    transport.protocolBinding
                                  )
                                }
                                onClick={(event) => event.stopPropagation()}
                              />
                              <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
                                <Typography variant="body2">
                                  {transport.protocolBinding}
                                </Typography>
                                <TransportPathField
                                  value={transportPathFor(transport)}
                                  editing={
                                    editingTransport === transport.protocolBinding
                                  }
                                  onEditingChange={(editing) =>
                                    setEditingTransport(
                                      editing ? transport.protocolBinding : null
                                    )
                                  }
                                  onChange={(path) =>
                                    setTransportPaths((prev) => ({
                                      ...prev,
                                      [transport.protocolBinding]: path,
                                    }))
                                  }
                                />
                              </Stack>
                              <Tooltip title="Edit path">
                                <Box component="span">
                                  <IconButton
                                    size="small"
                                    sx={{ mt: -0.5, mr: -0.5 }}
                                    disabled={
                                      isReadOnlyAgentProxy || !canUpdateAgentProxy
                                    }
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      setEditingTransport(
                                        transport.protocolBinding
                                      );
                                    }}
                                    aria-label={`Edit ${transport.protocolBinding} path`}
                                  >
                                    <Edit size={14} />
                                  </IconButton>
                                </Box>
                              </Tooltip>
                            </Stack>
                          </Grid>
                        );
                      })}
                    </Grid>
                  </Stack>
                </Box>

                {isDeployed ? (
                <>
                  <Divider
                    orientation="vertical"
                    flexItem
                    sx={{ display: { xs: 'none', md: 'block' } }}
                  />
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Stack spacing={2}>
                      <Stack spacing={1.5}>
                        <Stack spacing={0.25}>
                          <Typography variant="h6" sx={{ fontWeight: 600 }}>
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.proxy.url"
                              defaultMessage="Agent Proxy URL"
                            />
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.proxy.url.hint"
                              defaultMessage="Change the Gateway to generate the gateway specific URL. Append the transport path your client uses to reach the agent."
                            />
                          </Typography>
                        </Stack>
                        <Stack direction="row" spacing={1.5}>
                            <FormControl sx={{ flex: 1 }}>
                              <FormLabel>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.gateways"
                                  defaultMessage="Gateways"
                                />
                              </FormLabel>
                              <Select
                                value={selectedGatewayId}
                                onChange={(event) =>
                                  setSelectedGatewayId(
                                    event.target.value as string
                                  )
                                }
                              >
                                {deployedGateways.map((gateway) => (
                                  <MenuItem key={gateway.id} value={gateway.id}>
                                    {gateway.displayName || gateway.name}
                                  </MenuItem>
                                ))}
                              </Select>
                            </FormControl>
                            <FormControl sx={{ flex: 1.4 }}>
                              <FormLabel>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.url"
                                  defaultMessage="URL"
                                />
                              </FormLabel>
                              <TextField
                                fullWidth
                                value={generatedInvokeUrl}
                                slotProps={{
                                  input: {
                                    readOnly: true,
                                    endAdornment: (
                                      <IconButton
                                        size="small"
                                        onClick={() => void handleCopyInvokeUrl()}
                                        aria-label="Copy Agent Proxy URL"
                                      >
                                        <Copy size={14} />
                                      </IconButton>
                                    ),
                                  },
                                }}
                              />
                            </FormControl>
                        </Stack>
                      </Stack>
                      <Divider />
                      <Box>
                        <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.api.keys"
                            defaultMessage="API Keys"
                          />
                        </Typography>
                        <Stack spacing={2}>
                        <Stack
                          direction={{ xs: 'column', sm: 'row' }}
                          spacing={2}
                          alignItems={{ xs: 'flex-start', sm: 'center' }}
                          sx={{
                            p: 2,
                            bgcolor: 'background.paper',
                            border: '1px solid',
                            borderColor: 'divider',
                            borderRadius: 1,
                          }}
                        >
                          <Box sx={{ flex: 1 }}>
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.api.keys.hint"
                                defaultMessage="Generate an API key to authenticate requests to deployed gateways"
                              />
                            </Typography>
                          </Box>
                          <DisabledActionTooltip
                            disabled={!canCreateApiKey}
                            title={NO_PERMISSION_TOOLTIP}
                          >
                            <Button
                              variant="contained"
                              size="medium"
                              disabled={!canCreateApiKey}
                              onClick={() => {
                                setNewKeyName('');
                                setCreatedKeyValue(null);
                                setIsKeyDialogOpen(true);
                              }}
                            >
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.overview.generate.api.key"
                                defaultMessage="Generate API Key"
                              />
                            </Button>
                          </DisabledActionTooltip>
                        </Stack>
                        {!hasApiKeyAuthPolicy && (
                          <Alert severity="info">
                            Keys are not enforced until the{' '}
                            <strong>api-key-auth</strong> policy is attached.
                            Add it from the{' '}
                            <Link
                              component="button"
                              type="button"
                              onClick={() => setTabIndex(2)}
                              sx={{ verticalAlign: 'baseline' }}
                            >
                              Guardrails &amp; Policies
                            </Link>{' '}
                            tab.
                          </Alert>
                        )}
                        {isApiKeysLoading ? (
                          <Box
                            sx={{
                              display: 'flex',
                              justifyContent: 'center',
                              py: 4,
                            }}
                          >
                            <CircularProgress />
                          </Box>
                        ) : apiKeys.length > 0 ? (
                          <ListingTable.Container>
                            <ListingTable>
                              <ListingTable.Head>
                                <ListingTable.Row>
                                  <ListingTable.Cell>Name</ListingTable.Cell>
                                  <ListingTable.Cell>API Key</ListingTable.Cell>
                                  <ListingTable.Cell>
                                    Expires At
                                  </ListingTable.Cell>
                                  <ListingTable.Cell align="right">
                                    Actions
                                  </ListingTable.Cell>
                                </ListingTable.Row>
                              </ListingTable.Head>
                              <ListingTable.Body>
                                {apiKeys.map((apiKey) => (
                                  <ListingTable.Row key={apiKey.id}>
                                    <ListingTable.Cell>
                                      {apiKey.displayName || apiKey.id || '-'}
                                    </ListingTable.Cell>
                                    <ListingTable.Cell>
                                      {apiKey.maskedApiKey || '-'}
                                    </ListingTable.Cell>
                                    <ListingTable.Cell>
                                      <Tooltip
                                        title={
                                          apiKey.expiresAt
                                            ? new Date(
                                                apiKey.expiresAt
                                              ).toUTCString()
                                            : ''
                                        }
                                      >
                                        <span>
                                          {apiKey.expiresAt
                                            ? new Date(
                                                apiKey.expiresAt
                                              ).toLocaleDateString()
                                            : '-'}
                                        </span>
                                      </Tooltip>
                                    </ListingTable.Cell>
                                    <ListingTable.Cell align="right">
                                      <Tooltip
                                        title={
                                          canDeleteApiKey
                                            ? ''
                                            : NO_PERMISSION_TOOLTIP
                                        }
                                      >
                                        <Box component="span">
                                          <IconButton
                                            size="small"
                                            color="error"
                                            disabled={!canDeleteApiKey}
                                            onClick={() =>
                                              void handleRevokeKey(apiKey)
                                            }
                                            aria-label={`Revoke ${apiKey.displayName}`}
                                          >
                                            <Trash2 size={16} />
                                          </IconButton>
                                        </Box>
                                      </Tooltip>
                                    </ListingTable.Cell>
                                  </ListingTable.Row>
                                ))}
                              </ListingTable.Body>
                            </ListingTable>
                          </ListingTable.Container>
                        ) : null}
                        </Stack>
                      </Box>
                    </Stack>
                  </Box>
                </>
                ) : null}
              </Stack>
            </TabPanel>

            <TabPanel value={tabIndex} index={1}>
              <AgentProxyCardTab
                state={cardState}
                onChange={(patch) =>
                  setCardState((prev) => ({ ...prev, ...patch }))
                }
                disabled={isReadOnlyAgentProxy || !canUpdateAgentProxy}
                fetchedCard={fetchedCard}
                isFetching={isCardFetching}
                fetchError={cardFetchError}
                onRefetch={() => void loadPassthroughCard()}
              />
            </TabPanel>

            <TabPanel value={tabIndex} index={2}>
              <AgentProxyGuardrailsTab
                state={policyState}
                onChange={setPolicyState}
                readOnly={isReadOnlyAgentProxy || !canUpdateAgentProxy}
              />
            </TabPanel>

            <TabPanel value={tabIndex} index={3}>
              {isReadOnlyAgentProxy && (
                <GatewayArtifactReadOnlyBanner message="Backend connection is managed by the gateway that created this agent proxy and is read-only here." />
              )}
              <Stack spacing={2} sx={{ maxWidth: 640 }}>
                <FormControl fullWidth>
                  <FormLabel>Agent Endpoint URL</FormLabel>
                  <TextField
                    size="small"
                    value={endpointUrl}
                    onChange={(event) => setEndpointUrl(event.target.value)}
                    disabled={isConnectionDisabled}
                  />
                </FormControl>

                <FormControl fullWidth>
                  <FormLabel>Authentication</FormLabel>
                  <Select
                    size="small"
                    value={authType}
                    disabled={isConnectionDisabled}
                    onChange={(event) => {
                      const nextValue = String(event.target.value);
                      setAuthType(nextValue);
                      if (isNoCredentialAuthType(nextValue)) {
                        setAuthHeaderName('');
                        setAuthHeaderValue('');
                        setIsCredentialMasked(false);
                        setHasCredentialChanged(false);
                      }
                    }}
                  >
                    <MenuItem value="none">none</MenuItem>
                    <MenuItem value="api-key">api-key</MenuItem>
                    <MenuItem value="other">other</MenuItem>
                  </Select>
                  {authType === 'other' ? (
                    <FormHelperText>
                      No credentials are stored for this agent proxy. Use a
                      policy to configure authentication.
                    </FormHelperText>
                  ) : null}
                </FormControl>

                {!isNoCredentialAuthType(authType) ? (
                  <>
                    <FormControl fullWidth>
                      <FormLabel>Authentication Header</FormLabel>
                      <TextField
                        size="small"
                        value={authHeaderName}
                        disabled={isConnectionDisabled}
                        onChange={(event) =>
                          setAuthHeaderName(event.target.value)
                        }
                      />
                    </FormControl>

                    <FormControl fullWidth>
                      <FormLabel>Credentials</FormLabel>
                      <TextField
                        size="small"
                        type={showAuthHeaderValue ? 'text' : 'password'}
                        value={authHeaderValue}
                        disabled={isConnectionDisabled}
                        onFocus={() => {
                          if (isCredentialMasked) {
                            setAuthHeaderValue('');
                            setIsCredentialMasked(false);
                            setHasCredentialChanged(false);
                          }
                        }}
                        onChange={(event) => {
                          setAuthHeaderValue(event.target.value);
                          setHasCredentialChanged(true);
                        }}
                        slotProps={{
                          input: {
                            endAdornment: (
                              <InputAdornment position="end">
                                <IconButton
                                  size="small"
                                  onClick={() =>
                                    setShowAuthHeaderValue((prev) => !prev)
                                  }
                                  aria-label={
                                    showAuthHeaderValue
                                      ? 'Hide credentials'
                                      : 'Show credentials'
                                  }
                                >
                                  {showAuthHeaderValue ? (
                                    <EyeOff size={18} />
                                  ) : (
                                    <Eye size={18} />
                                  )}
                                </IconButton>
                              </InputAdornment>
                            ),
                          },
                        }}
                      />
                    </FormControl>
                  </>
                ) : null}
              </Stack>
            </TabPanel>
          </Box>
        </Card>
      </Stack>

      <Box sx={{ position: 'sticky', bottom: 0, zIndex: 10, mt: 2 }}>
        <Card>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={1}
            alignItems={{ xs: 'flex-start', sm: 'center' }}
            justifyContent="space-between"
            sx={{ p: 2 }}
          >
            <Typography
              variant="body2"
              color={hasUnsavedChanges ? 'warning.main' : 'text.secondary'}
            >
              {hasUnsavedChanges ? 'You have unsaved changes.' : ''}
            </Typography>
            <Stack direction="row" spacing={1}>
              <Button
                variant="outlined"
                color="secondary"
                disabled={!hasUnsavedChanges || isSavingChanges}
                onClick={handleCancelChanges}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                disabled={
                  isReadOnlyAgentProxy ||
                  !canUpdateAgentProxy ||
                  !hasUnsavedChanges ||
                  isSavingChanges
                }
                onClick={() => void handleSaveChanges()}
              >
                {isSavingChanges ? 'Saving...' : 'Save'}
              </Button>
            </Stack>
          </Stack>
        </Card>
      </Box>

      <Dialog
        open={isKeyDialogOpen}
        onClose={() => setIsKeyDialogOpen(false)}
        fullWidth
        maxWidth={createdKeyValue ? 'md' : 'sm'}
      >
        <DialogTitle>
          {createdKeyValue
            ? 'API Key Generated Successfully'
            : 'Generate API Key'}
        </DialogTitle>
        <DialogContent>
          {createdKeyValue ? (
            <Alert
              severity="warning"
              sx={{
                '& .MuiAlert-message': {
                  width: '100%',
                },
              }}
            >
              <Stack spacing={1}>
                <Typography variant="caption" color="text.secondary">
                  Please copy and save this API key. For security reasons, you
                  won&apos;t be able to see it again.
                </Typography>
                <Box
                  display="flex"
                  flexDirection="row"
                  alignItems="center"
                  gap={0.5}
                >
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ flexShrink: 0 }}
                  >
                    API Key
                  </Typography>
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1,
                      p: 1.5,
                      width: '100%',
                      bgcolor: 'background.paper',
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1,
                      fontFamily: 'monospace',
                      fontSize: '0.875rem',
                    }}
                  >
                    <Box sx={{ flex: 1, wordBreak: 'break-all' }}>
                      {createdKeyValue}
                    </Box>
                    <IconButton
                      size="small"
                      onClick={() => void handleCopyCreatedKey()}
                      sx={{ flexShrink: 0 }}
                      aria-label="Copy API key"
                    >
                      <Copy size={16} />
                    </IconButton>
                  </Box>
                </Box>
              </Stack>
            </Alert>
          ) : (
            <Stack spacing={1}>
              <FormControl fullWidth>
                <FormLabel>Key Name</FormLabel>
                <TextField
                  autoFocus
                  size="small"
                  fullWidth
                  value={newKeyName}
                  onChange={(event) => setNewKeyName(event.target.value)}
                  placeholder="Ex: Production Key"
                />
              </FormControl>
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            size="small"
            disabled={isCreatingKey}
            onClick={() => setIsKeyDialogOpen(false)}
          >
            {createdKeyValue ? 'Done' : 'Cancel'}
          </Button>
          {createdKeyValue ? null : (
            <Button
              variant="contained"
              size="small"
              disabled={!newKeyName.trim() || isCreatingKey}
              onClick={() => void handleCreateKey()}
            >
              {isCreatingKey ? (
                <>
                  <CircularProgress size={16} sx={{ mr: 1 }} />
                  Generating...
                </>
              ) : (
                'Generate'
              )}
            </Button>
          )}
        </DialogActions>
      </Dialog>

      <Dialog
        open={isDeleteDialogOpen}
        onClose={() => {
          if (isDeleting) return;
          setIsDeleteDialogOpen(false);
        }}
      >
        <DialogTitle>Delete Agent Proxy</DialogTitle>
        <DialogContent>
          {isReadOnlyAgentProxy ? (
            <GatewayArtifactDeleteWarning
              artifactType="Agent Proxy"
              artifactName={agentProxy.displayName}
            />
          ) : null}
          <DialogContentText>
            Are you sure you want to delete {agentProxy.displayName}?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            disabled={isDeleting}
            onClick={() => setIsDeleteDialogOpen(false)}
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
    </PageContent>
  );
}
