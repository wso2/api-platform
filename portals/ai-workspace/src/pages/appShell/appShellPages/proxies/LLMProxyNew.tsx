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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Link as RouterLink, useLocation, useNavigate } from 'react-router-dom';
import {
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Copy,
  Plus,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  createLLMProviderAPIKey,
  getLLMProvider,
} from '../../../../apis/llmProviderApis';
import {
  createSecret,
  deleteSecret,
  buildSecretPlaceholder,
  generateSecretHandle,
} from '../../../../apis/secretApis';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { useProxies } from '../../../../contexts/proxy';
import {
  LLMProviderProvider,
  useLLMProvider,
  useLLMProviders,
} from '../../../../contexts/llmProvider';
import {
  ProviderTemplatesProvider,
  useProviderTemplates,
} from '../../../../contexts/llmProvider/providerTemplate';
import InboundInterfaceSelect from '../../../../Components/InboundInterface/InboundInterfaceSelect';
import TransformerStatusCard from '../../../../Components/Transformer/TransformerStatusCard';
import TransformerDrawer from '../../../../Components/Transformer/TransformerDrawer';
import ProviderRow from '../../../../Components/Transformer/ProviderRow';
import useTransformerPolicies from '../../../../hooks/useTransformerPolicies';
import { resolveTransformer } from '../../../../utils/transformerResolution';
import { resolvedTransformerFor } from '../../../../utils/proxyProviders';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { truncateProviderDisplayName } from '../../../../utils/providerTemplateDisplay';
import { resolveApiKeyAuthDisplay } from '../../../../utils/apiKeyAuthDisplay';
import type {
  CreateProxyRequest,
  LLMProvider,
  ProxyProviderEntry,
  ProxyProviderTransformer,
} from '../../../../utils/types';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { logger } from '../../../../utils/logger';
import { getErrorMessage, getFieldErrors } from '../../../../utils/apiError';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { NO_PERMISSION_TOOLTIP, SCOPES } from '../../../../auth/permissions';

type FormState = {
  name: string;
  context: string;
  providerId: string;
  version: string;
  description: string;
};

/**
 * One extra provider being entered on this page.
 *
 * Held here rather than persisted as it is entered: a proxy is created once,
 * with every provider it has, so nothing reaches the server until Create. That
 * is also what lets a failed create leave the page exactly as the user left it.
 *
 * `apiKeyValue` is whatever the user typed and `generatedApiKeyValue` whatever
 * was minted for this provider. They are held apart so the field a user typed
 * into is never filled in behind them, and either is exchanged for a stored
 * secret during create, so neither leaves this page as plain text.
 */
type AdditionalProviderDraft = {
  /** Stable across re-orders and removals, so React keys survive editing. */
  key: string;
  providerId: string;
  apiKeyValue: string;
  generatedApiKeyValue: string;
  /**
   * The header this provider expects its key in, taken from the provider's own
   * record. Carried on the draft because the provider list does not describe
   * how a provider authenticates — only the provider itself does.
   */
  apiKeyHeader: string;
  transformer: ProxyProviderTransformer | null;
};

/** Identifies the primary when the transformer drawer acts on it. */
const PRIMARY_TARGET = '__primary__';

let additionalProviderKeySeq = 0;
const newAdditionalProviderDraft = (): AdditionalProviderDraft => {
  additionalProviderKeySeq += 1;
  return {
    key: `provider-${additionalProviderKeySeq}`,
    providerId: '',
    apiKeyValue: '',
    generatedApiKeyValue: '',
    apiKeyHeader: '',
    transformer: null,
  };
};

// Backend field names (from CreateProxyRequest) mapped onto this form's state keys.
// "provider.auth.*" errors are not mapped here — best-effort only, they surface
// via the general error banner instead of forcing an unclear match.
const FIELD_NAME_MAP: Partial<Record<string, keyof FormState>> = {
  displayName: 'name',
  description: 'description',
  version: 'version',
  context: 'context',
  'provider.id': 'providerId',
};

/** Derive a URL-safe proxy id from the display name (same logic as LLM provider). */
const toProxyId = (name: string): string =>
  name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const buildApiKeyResourceName = (displayName: string): string => {
  const normalizedDisplayName = displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalizedDisplayName || 'api-key';
};

type LLMProxyNewContentProps = {
  selectedProviderId: string;
  onSelectedProviderIdChange: (providerId: string) => void;
  lockedProviderId: string;
  isProviderSelectionLocked: boolean;
  preselectedProvider: LLMProvider | null;
  preselectedProjectId: string;
};

function LLMProxyNewContent({
  selectedProviderId,
  onSelectedProviderIdChange,
  lockedProviderId,
  isProviderSelectionLocked,
  preselectedProvider,
  preselectedProjectId,
}: LLMProxyNewContentProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const { createProxy } = useProxies();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const canCreateProxy = hasPermission(SCOPES.LLM_PROXY_CREATE);
  // Generating a provider key is a distinct grant from creating a proxy: the
  // API-key operations accept only ap:llm_provider:api_key:{create,manage}, so
  // a user who may create proxies can still be unable to mint the key. They can
  // always paste one into the manual field instead.
  const canGenerateProviderApiKey = hasPermission(
    SCOPES.LLM_PROVIDER_API_KEY_CREATE
  );
  const { providersResponse, isLoading: isProvidersLoading } =
    useLLMProviders();
  const {
    templatesResponse,
    isLoading: isTemplatesLoading,
    error: templatesError,
  } = useProviderTemplates();

  const { provider: selectedProvider, isLoading: isSelectedProviderLoading } =
    useLLMProvider();
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
    setCurrentProject,
  } = useAppShell();
  const projectFromState = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => project.id === preselectedProjectId
      ) ?? null,
    [preselectedProjectId, projectsForCurrentOrganization]
  );
  const effectiveProject = currentProject ?? projectFromState;
  const isProjectLevel = Boolean(effectiveProject?.id);
  const proxiesPath = isProjectLevel
    ? buildProjectPath(currentOrganization, effectiveProject, '/proxies')
    : buildOrgPath(currentOrganization, '/proxies');

  const providerOptions = providersResponse.list;

  const [formState, setFormState] = useState<FormState>(() => ({
    name: '',
    context: '/',
    providerId: lockedProviderId || selectedProviderId || '',
    version: 'v1.0',
    description: '',
  }));

  const [isCreating, setIsCreating] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof FormState, string>>>({});
  const [isApiKeyModalOpen, setIsApiKeyModalOpen] = useState(false);
  const [apiKeyDisplayName, setApiKeyDisplayName] = useState('');
  const [isGeneratingApiKey, setIsGeneratingApiKey] = useState(false);
  const [apiKeyError, setApiKeyError] = useState<string | null>(null);
  const [generatedKeyForDisplay, setGeneratedKeyForDisplay] = useState<
    string | null
  >(null);
  // Providers beyond the first. The page opens with none, so creating a
  // single-provider proxy involves exactly the decisions it always did.
  const [additionalProviders, setAdditionalProviders] = useState<
    AdditionalProviderDraft[]
  >([]);
  // Whether the user has chosen an interface themselves. Once they have, the
  // default below stops moving under them.
  const [hasChosenInterface, setHasChosenInterface] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  /**
   * The provider currently being filled in, if any. Held apart from the list so
   * an abandoned form leaves what is already attached untouched.
   */
  const [stagedProvider, setStagedProvider] =
    useState<AdditionalProviderDraft | null>(null);
  /**
   * Which attached provider is open for editing: the primary, or a draft key.
   *
   * Editing reopens the same form the provider was added with, against a working
   * copy — so abandoning it leaves the attached provider exactly as it was,
   * rather than removing it.
   */
  const [editingKey, setEditingKey] = useState<string | null>(null);
  /** The working copy of an attached extra provider while it is being edited. */
  const [editDraft, setEditDraft] = useState<AdditionalProviderDraft | null>(
    null
  );
  /** What the primary was before its edit began, so cancelling restores it. */
  const [primarySnapshot, setPrimarySnapshot] = useState<{
    providerId: string;
    manualApiKeyValue: string;
    generatedApiKeyValue: string | null;
    transformer: ProxyProviderTransformer | null;
  } | null>(null);
  /**
   * Credentials to put back once a cancelled provider change has settled.
   *
   * Changing the primary provider clears its credential, which is right while
   * editing and wrong while undoing an edit. The restore is handed to that same
   * effect rather than racing it.
   */
  const pendingPrimaryRestore = useRef<{
    manualApiKeyValue: string;
    generatedApiKeyValue: string | null;
    providerId: string;
  } | null>(null);
  /**
   * The provider whose form is open, read in full.
   *
   * Whether a provider needs a key, and which header it goes in, is declared on
   * the provider itself — the list this page selects from carries neither — so
   * an extra provider's form cannot be drawn correctly without reading it.
   */
  const [openProviderDetail, setOpenProviderDetail] =
    useState<LLMProvider | null>(null);
  const [isOpenProviderLoading, setIsOpenProviderLoading] = useState(false);
  /** Which provider a generated key is being minted for. */
  const [apiKeyTarget, setApiKeyTarget] = useState<string>(PRIMARY_TARGET);
  /** Which provider the transformer drawer is acting on: the primary, or a draft key. */
  const [transformerTarget, setTransformerTarget] = useState<string | null>(
    null
  );
  /** The primary's own translator, when the interface differs from its format. */
  const [primaryTransformer, setPrimaryTransformer] =
    useState<ProxyProviderTransformer | null>(null);
  // The format this proxy accepts from clients. Empty means "take it from the
  // primary provider", which is what a single-provider proxy wants.
  const [inboundTemplate, setInboundTemplate] = useState<string>('');

  const [selectedProviderApiKeyValue, setSelectedProviderApiKeyValue] =
    useState<string | null>(null);
  const [
    selectedProviderApiKeyProviderId,
    setSelectedProviderApiKeyProviderId,
  ] = useState('');
  const [manualApiKeyValue, setManualApiKeyValue] = useState('');

  const latestProviderId = useMemo(() => {
    if (providerOptions.length === 0) return '';

    const getLastUpdatedTime = (value?: string): number => {
      if (!value) return 0;
      const timestamp = new Date(value).getTime();
      return Number.isNaN(timestamp) ? 0 : timestamp;
    };

    const latestProvider = providerOptions.reduce((latest, current) => {
      const latestTime = getLastUpdatedTime(
        latest.lastUpdated ?? latest.updatedAt ?? latest.createdAt
      );
      const currentTime = getLastUpdatedTime(
        current.lastUpdated ?? current.updatedAt ?? current.createdAt
      );
      return currentTime > latestTime ? current : latest;
    }, providerOptions[0]);

    return latestProvider.id ?? '';
  }, [providerOptions]);

  useEffect(() => {
    setFormState((prev) => {
      if (isProviderSelectionLocked && lockedProviderId) {
        if (prev.providerId === lockedProviderId) {
          return prev;
        }
        return {
          ...prev,
          providerId: lockedProviderId,
        };
      }

      const hasValidCurrentProvider =
        prev.providerId &&
        providerOptions.some((provider) => provider.id === prev.providerId);

      if (hasValidCurrentProvider) {
        return prev;
      }

      if (prev.providerId && isProvidersLoading) {
        return prev;
      }

      if (!latestProviderId) {
        return prev;
      }

      return {
        ...prev,
        providerId: latestProviderId,
      };
    });
  }, [
    isProviderSelectionLocked,
    isProvidersLoading,
    latestProviderId,
    lockedProviderId,
    providerOptions,
  ]);

  useEffect(() => {
    if (formState.providerId !== selectedProviderId) {
      onSelectedProviderIdChange(formState.providerId);
    }
  }, [formState.providerId, onSelectedProviderIdChange, selectedProviderId]);

  useEffect(() => {
    if (!preselectedProjectId) {
      return;
    }
    if (currentProject?.id === preselectedProjectId) {
      return;
    }
    const matchedProject = projectsForCurrentOrganization.find(
      (project) => project.id === preselectedProjectId
    );
    if (matchedProject) {
      setCurrentProject?.(matchedProject);
    }
  }, [
    currentProject?.id,
    preselectedProjectId,
    projectsForCurrentOrganization,
    setCurrentProject,
  ]);

  const providerDetail = useMemo(() => {
    if (selectedProvider?.id === formState.providerId) {
      return selectedProvider;
    }
    if (preselectedProvider?.id === formState.providerId) {
      return preselectedProvider;
    }
    return null;
  }, [formState.providerId, preselectedProvider, selectedProvider]);

  /**
   * The format this proxy should accept by default.
   *
   * With one provider it is that provider's own format, so nothing needs
   * translating and the form reads as it always did. With several it is
   * OpenAI's, which has the widest translator coverage and so leaves the most
   * providers matching automatically — falling back to the primary's own format
   * when the catalogue does not carry OpenAI's.
   */
  const defaultInboundTemplate = useMemo(() => {
    if (additionalProviders.length === 0) {
      return providerDetail?.template ?? '';
    }
    const openAiTemplate = templatesResponse.list.find(
      (template) => template.id === 'openai' && template.enabled !== false
    );
    return openAiTemplate?.id ?? providerDetail?.template ?? '';
  }, [additionalProviders.length, templatesResponse.list, providerDetail]);

  // Derivation runs only while the user has not chosen for themselves. Once
  // they have, it stops permanently rather than moving under their choice.
  useEffect(() => {
    if (hasChosenInterface) {
      return;
    }
    setInboundTemplate(defaultInboundTemplate);
  }, [defaultInboundTemplate, hasChosenInterface]);

  const { policies: transformerPolicies, isLoaded: transformerPoliciesLoaded } =
    useTransformerPolicies();

  /** The interface's display name, for wording that names it. */
  const inboundInterfaceLabel = useMemo(
    () =>
      templatesResponse.list.find((template) => template.id === inboundTemplate)
        ?.displayName ?? inboundTemplate,
    [templatesResponse.list, inboundTemplate]
  );

  /**
   * What translates for a provider on this form, decided by the same rules the
   * detail page uses so a proxy reads the same before and after it is created.
   */
  const resolutionForProvider = (
    providerId: string,
    chosenTransformer?: ProxyProviderTransformer | null
  ) => {
    const provider = providerOptions.find((option) => option.id === providerId);
    return resolveTransformer({
      inboundTemplate,
      providerTemplate: provider?.template,
      chosenTransformer,
      policies: transformerPolicies,
      policiesLoaded: transformerPoliciesLoaded,
      interfaceLabel: inboundInterfaceLabel,
      providerLabel: provider?.displayName,
    });
  };

  /** Attaches or detaches a translator on whichever provider opened the drawer. */
  const applyTransformerToTarget = (
    transformer: ProxyProviderTransformer | null
  ) => {
    if (transformerTarget === PRIMARY_TARGET) {
      setPrimaryTransformer(transformer);
      return;
    }
    if (openDraft?.key === transformerTarget) {
      updateOpenDraft((prev) => ({ ...prev, transformer }));
      return;
    }
    setAdditionalProviders((prev) =>
      prev.map((entry) =>
        entry.key === transformerTarget ? { ...entry, transformer } : entry
      )
    );
  };

  /** The translator a provider is saved with, by the rule every write shares. */
  const transformerToPersist = (
    providerId: string,
    chosen: ProxyProviderTransformer | null
  ): ProxyProviderTransformer | undefined =>
    resolvedTransformerFor(chosen, resolutionForProvider(providerId, chosen));

  /**
   * Providers not already attached, so the same one cannot be added twice —
   * two rows resolving to the same request handle would make routing to either
   * of them ambiguous.
   */
  /** Whether the proxy has more than one provider, or is about to. */
  const hasSeveralProviders =
    additionalProviders.length > 0 || stagedProvider !== null;
  /**
   * The extra provider currently being filled in — newly staged or an attached
   * one being edited. Only ever one at a time, so one form's worth of supporting
   * state serves both.
   */
  const openDraft = stagedProvider ?? editDraft;
  /** Whether any provider form is open, which is what closes the others down. */
  const isProviderFormOpen = stagedProvider !== null || editingKey !== null;
  /** Edits whichever extra-provider form is open, staged or attached. */
  const updateOpenDraft = (
    change: (draft: AdditionalProviderDraft) => AdditionalProviderDraft
  ) => {
    if (stagedProvider) {
      setStagedProvider((prev) => (prev ? change(prev) : prev));
      return;
    }
    setEditDraft((prev) => (prev ? change(prev) : prev));
  };
  /** The primary shows its own fields when it is the only one, or while edited. */
  const isPrimaryExpanded =
    !hasSeveralProviders || editingKey === PRIMARY_TARGET;

  const availableProviderOptions = (keepProviderId: string) =>
    providerOptions.filter(
      (provider) =>
        provider.id === keepProviderId ||
        (provider.id !== formState.providerId &&
          !additionalProviders.some(
            (entry) => entry.providerId === provider.id
          ))
    );

  /**
   * Makes an attached provider the primary, and demotes the current one into
   * the list rather than dropping it.
   *
   * Promotion reorders and nothing else. The inbound interface is deliberately
   * untouched: an existing proxy has clients depending on its request format,
   * and swapping which provider is primary must not silently change it.
   */
  const promoteProvider = (draftKey: string) => {
    const promoted = additionalProviders.find(
      (entry) => entry.key === draftKey
    );
    if (!promoted) {
      return;
    }
    const demotedPrimary: AdditionalProviderDraft = {
      key: `provider-demoted-${Date.now()}`,
      providerId: formState.providerId,
      apiKeyValue: manualApiKeyValue,
      generatedApiKeyValue:
        selectedProviderApiKeyProviderId === formState.providerId
          ? (selectedProviderApiKeyValue ?? '')
          : '',
      apiKeyHeader: selectedProviderApiKeyName,
      transformer: primaryTransformer,
    };
    setAdditionalProviders((prev) => [
      ...prev.filter((entry) => entry.key !== draftKey),
      demotedPrimary,
    ]);
    // Handed to the effect that clears credentials on a provider change rather
    // than set alongside it. That effect runs after this batch, so a value set
    // here would be wiped — and a generated key, shown once, would be gone.
    pendingPrimaryRestore.current = {
      providerId: promoted.providerId,
      manualApiKeyValue: promoted.apiKeyValue,
      generatedApiKeyValue: promoted.generatedApiKeyValue || null,
    };
    setFormState((prev) => ({ ...prev, providerId: promoted.providerId }));
    onSelectedProviderIdChange(promoted.providerId);
    setPrimaryTransformer(promoted.transformer);
    setHasChosenInterface(true);
  };

  useEffect(() => {
    const openProviderId = openDraft?.providerId;
    if (!openProviderId || !currentOrganization?.uuid) {
      setOpenProviderDetail(null);
      return undefined;
    }
    let abandoned = false;
    // Dropped before the fetch, for the same reason the panel does: a detail
    // held over from the previous provider answers for the wrong one.
    setOpenProviderDetail(null);
    setIsOpenProviderLoading(true);
    getLLMProvider(
      openProviderId,
      currentOrganization.uuid,
      PLATFORM_API_BASE_URL
    )
      .then((detail) => {
        if (!abandoned) {
          setOpenProviderDetail(detail);
        }
      })
      .catch((error) => {
        if (abandoned) {
          return;
        }
        logger.error('Failed to load the selected provider:', error);
        setOpenProviderDetail(null);
      })
      .finally(() => {
        if (!abandoned) {
          setIsOpenProviderLoading(false);
        }
      });
    return () => {
      abandoned = true;
    };
  }, [openDraft?.providerId, currentOrganization?.uuid]);

  /**
   * Opens the primary for editing, remembering what it was.
   *
   * Everything it edits is page state that other things read, so it is changed
   * in place and undone from this record rather than held in a copy.
   */
  const beginPrimaryEdit = () => {
    setPrimarySnapshot({
      providerId: formState.providerId,
      manualApiKeyValue,
      generatedApiKeyValue:
        selectedProviderApiKeyProviderId === formState.providerId
          ? selectedProviderApiKeyValue
          : null,
      transformer: primaryTransformer,
    });
    setEditingKey(PRIMARY_TARGET);
  };

  const cancelPrimaryEdit = () => {
    const snapshot = primarySnapshot;
    setEditingKey(null);
    setPrimarySnapshot(null);
    if (!snapshot) {
      return;
    }
    setPrimaryTransformer(snapshot.transformer);
    if (snapshot.providerId === formState.providerId) {
      setManualApiKeyValue(snapshot.manualApiKeyValue);
      setSelectedProviderApiKeyValue(snapshot.generatedApiKeyValue);
      setSelectedProviderApiKeyProviderId(
        snapshot.generatedApiKeyValue ? snapshot.providerId : ''
      );
      return;
    }
    pendingPrimaryRestore.current = {
      providerId: snapshot.providerId,
      manualApiKeyValue: snapshot.manualApiKeyValue,
      generatedApiKeyValue: snapshot.generatedApiKeyValue,
    };
    setFormState((prev) => ({ ...prev, providerId: snapshot.providerId }));
    onSelectedProviderIdChange(snapshot.providerId);
  };

  /** Edits an attached provider through a copy, so cancelling changes nothing. */
  const beginProviderEdit = (draft: AdditionalProviderDraft) => {
    setEditDraft({ ...draft });
    setEditingKey(draft.key);
  };

  const cancelProviderEdit = () => {
    setEditDraft(null);
    setEditingKey(null);
  };

  const commitProviderEdit = () => {
    if (!editDraft) {
      return;
    }
    const edited: AdditionalProviderDraft = {
      ...editDraft,
      apiKeyHeader: openProviderRequiresApiKey ? openProviderApiKeyName : '',
      apiKeyValue: openProviderRequiresApiKey ? editDraft.apiKeyValue : '',
      generatedApiKeyValue: openProviderRequiresApiKey
        ? editDraft.generatedApiKeyValue
        : '',
    };
    setAdditionalProviders((prev) =>
      prev.map((entry) => (entry.key === edited.key ? edited : entry))
    );
    setEditDraft(null);
    setEditingKey(null);
  };

  const selectedProviderRequiresApiKey = Boolean(
    providerDetail?.security?.enabled && providerDetail.security.apiKey?.enabled
  );
  /**
   * Whether the provider whose form is open authenticates with a key at all. A
   * provider that does not gets no key field: an input nothing is done with
   * reads as a required step and makes the form look incomplete.
   */
  const openProviderRequiresApiKey = Boolean(
    openProviderDetail?.security?.enabled &&
    openProviderDetail.security.apiKey?.enabled
  );
  const openProviderApiKeyName = resolveApiKeyAuthDisplay(
    openProviderDetail?.security,
    openProviderDetail?.globalPolicies
  ).headerName;
  const selectedProviderApiKeyName = resolveApiKeyAuthDisplay(
    providerDetail?.security,
    providerDetail?.globalPolicies
  ).headerName;
  const projectSlug = getProjectSlug(effectiveProject);
  const generatedProxyId = toProxyId(formState.name);
  const computedContext = projectSlug
    ? `/${projectSlug}/${generatedProxyId}`
    : `/${generatedProxyId}`;
  const [contextOverride, setContextOverride] = useState<string | null>(null);
  const effectiveContext = contextOverride ?? computedContext;

  const isGeneratedKeyReady =
    selectedProviderApiKeyProviderId === formState.providerId &&
    Boolean(selectedProviderApiKeyValue);
  const isManualKeyReady = Boolean(manualApiKeyValue.trim());
  const isSelectedProviderApiKeyReady =
    !selectedProviderRequiresApiKey || isGeneratedKeyReady || isManualKeyReady;

  const isCreateProxyDisabled =
    // A provider described but not yet added is not on the list Create reads.
    // Creating anyway would quietly make a proxy without it, and without
    // saying so — the entered credential and translator simply gone.
    isProviderFormOpen ||
    !formState.name.trim() ||
    !formState.providerId ||
    !effectiveProject?.id ||
    !providerDetail ||
    !isSelectedProviderApiKeyReady ||
    isCreating;

  // The submit button used to disable itself with no explanation, which reads as
  // a permission problem even when it is only a missing field. Name the actual
  // blocker instead.
  const createProxyBlockedReason = (() => {
    if (!isCreateProxyDisabled || isCreating) return '';
    if (isProviderFormOpen) {
      return stagedProvider
        ? 'Add the provider you are describing, or cancel it, before creating.'
        : 'Finish editing the open provider before creating.';
    }
    if (!formState.name.trim()) return 'Enter a name for the proxy.';
    if (!formState.providerId) return 'Select an LLM provider.';
    if (!effectiveProject?.id) return 'Select a project.';
    if (!providerDetail) return 'Loading the selected provider details.';
    if (!isSelectedProviderApiKeyReady) {
      return canGenerateProviderApiKey
        ? 'This provider requires an API key. Generate one or enter it manually.'
        : 'This provider requires an API key. Enter it manually, or ask your admin to generate one.';
    }
    return '';
  })();

  useEffect(() => {
    // A credential belongs to the provider it was entered for, so changing the
    // provider clears it. The one exception is undoing a change: the credential
    // being put back is the one this provider already had.
    const restore = pendingPrimaryRestore.current;
    pendingPrimaryRestore.current = null;
    const restores = restore?.providerId === formState.providerId;
    setApiKeyError(null);
    setGeneratedKeyForDisplay(null);
    setSelectedProviderApiKeyValue(
      restores ? restore.generatedApiKeyValue : null
    );
    setSelectedProviderApiKeyProviderId(restores ? formState.providerId : '');
    setManualApiKeyValue(restores ? restore.manualApiKeyValue : '');
    setApiKeyDisplayName('');
    setIsApiKeyModalOpen(false);
  }, [formState.providerId]);

  const lockedProviderDisplayName = useMemo(() => {
    if (!isProviderSelectionLocked || !lockedProviderId) {
      return '';
    }
    if (preselectedProvider?.id === lockedProviderId) {
      return truncateProviderDisplayName(preselectedProvider.displayName);
    }
    const option = providerOptions.find(
      (provider) => provider.id === lockedProviderId
    );
    if (option?.displayName) {
      return truncateProviderDisplayName(option.displayName);
    }
    return truncateProviderDisplayName(lockedProviderId);
  }, [
    isProviderSelectionLocked,
    lockedProviderId,
    preselectedProvider,
    providerOptions,
  ]);

  const handleCreate = async () => {
    const trimmedName = formState.name.trim();
    const generatedId = toProxyId(trimmedName);
    const payloadProjectId = effectiveProject?.id ?? '';
    if (
      !trimmedName ||
      !generatedId ||
      !formState.providerId ||
      !payloadProjectId ||
      !providerDetail ||
      !isSelectedProviderApiKeyReady
    ) {
      return;
    }

    let createdSecretHandle: string | null = null;
    // Every provider can mint one, so a failed create has several to undo.
    const createdSecretHandles: string[] = [];
    try {
      setIsCreating(true);
      setFieldErrors({});

      // Encrypt the provider API key as a secret before storing it in the proxy
      // config — even though it is a platform-issued key, it is still a credential
      // that should not be persisted in plain text.
      // Type, header and value move as one unit: an absent/'none' type carries no
      // credential, so the header and value are cleared with it rather than being
      // inherited from the provider and contradicting the type.
      const inheritedAuthType = providerDetail?.upstream?.main?.auth?.type || 'none';
      const inheritsCredential = inheritedAuthType !== 'none';
      let providerAuthType = inheritedAuthType;
      let providerAuthHeader = inheritsCredential
        ? (providerDetail?.upstream?.main?.auth?.header ?? '')
        : '';
      let providerAuthValue = inheritsCredential
        ? (providerDetail?.upstream?.main?.auth?.value ?? '')
        : '';
      if (selectedProviderRequiresApiKey) {
        const rawKey = manualApiKeyValue.trim() || selectedProviderApiKeyValue || '';
        const isAlreadyPlaceholder = rawKey.includes('{{ secret ');
        if (isAlreadyPlaceholder) {
          providerAuthType = 'api-key';
          providerAuthHeader = selectedProviderApiKeyName;
          providerAuthValue = rawKey;
        } else {
          const secretHandle = generateSecretHandle();
          const secretResponse = await createSecret({
            id: secretHandle,
            displayName: `${generatedId} Provider API Key`,
            description: `Auto-generated secret for LLM proxy ${generatedId}`,
            value: rawKey,
            type: 'GENERIC',
          });
          logger.info('Created secret for LLM proxy provider auth', {
            secretHandle: secretResponse.id,
            proxyId: generatedId,
          });
          createdSecretHandle = secretResponse.id;
          providerAuthType = 'api-key';
          providerAuthHeader = selectedProviderApiKeyName;
          providerAuthValue = buildSecretPlaceholder(secretResponse.id);
        }
      }

      // Each extra provider's credential is stored the same way the first one's
      // is, so every provider on the proxy authenticates on the same terms. A
      // provider left without one is carried without an auth block rather than
      // with an empty one, which is how the server tells "no credential" from
      // "a credential I could not read".
      const additionalProviderEntries: ProxyProviderEntry[] = [];
      for (const draft of additionalProviders) {
        if (!draft.providerId) {
          continue;
        }
        const draftTransformer = transformerToPersist(
          draft.providerId,
          draft.transformer
        );
        // A typed key wins over a minted one: it is the later of the two the
        // user can have supplied, since minting clears nothing they typed.
        const providerKey =
          draft.apiKeyValue.trim() || draft.generatedApiKeyValue;
        if (!providerKey) {
          additionalProviderEntries.push({
            id: draft.providerId,
            isPrimary: false,
            ...(draftTransformer ? { transformer: draftTransformer } : {}),
          });
          continue;
        }
        const secretHandle = generateSecretHandle();
        const secretResponse = await createSecret({
          id: secretHandle,
          displayName: `${generatedId} ${draft.providerId} API Key`,
          description: `Auto-generated secret for LLM proxy ${generatedId}, provider ${draft.providerId}`,
          value: providerKey,
          type: 'GENERIC',
        });
        createdSecretHandles.push(secretResponse.id);
        additionalProviderEntries.push({
          id: draft.providerId,
          isPrimary: false,
          auth: {
            type: 'api-key',
            // The header the provider itself declares. Sending every provider's
            // key in the same one would authenticate against whichever provider
            // happened to agree with that guess.
            header: draft.apiKeyHeader || 'Authorization',
            value: buildSecretPlaceholder(secretResponse.id),
          },
          ...(draftTransformer ? { transformer: draftTransformer } : {}),
        });
      }

      const primaryPersistedTransformer = transformerToPersist(
        formState.providerId,
        primaryTransformer
      );

      const payload: CreateProxyRequest = {
        id: generatedId,
        displayName: trimmedName,
        description:
          formState.description.trim() ||
          intl.formatMessage({
            id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.no.description.provided.for.this.proxy',
            defaultMessage: 'No description provided for this proxy.',
          }),
        version: formState.version.trim() || 'v1.0',
        projectId: payloadProjectId,
        context: effectiveContext,
        providers: [
          {
            id: formState.providerId,
            isPrimary: true,
            auth: {
              type: providerAuthType,
              header: providerAuthHeader,
              value: providerAuthValue,
            },
            ...(primaryPersistedTransformer
              ? { transformer: primaryPersistedTransformer }
              : {}),
          },
          ...additionalProviderEntries,
        ],
        ...(inboundTemplate ? { inboundTemplate } : {}),
        openapi: providerDetail?.openapi ?? '',
        policies: [],
        security: {
          enabled: Boolean(providerDetail?.security?.enabled),
          apiKey: {
            enabled: Boolean(providerDetail?.security?.apiKey?.enabled),
            key: providerDetail?.security?.apiKey?.key ?? '',
            in: providerDetail?.security?.apiKey?.in ?? 'header',
            valuePrefix: providerDetail?.security?.apiKey?.valuePrefix,
          },
        },
      };

      const newProxy = await createProxy(payload);
      navigate(
        isProjectLevel
          ? buildProjectPath(
              currentOrganization,
              effectiveProject,
              `/proxies/${newProxy.id}`
            )
          : buildOrgPath(currentOrganization, `/proxies/${newProxy.id}`),
        {
          state: { proxyAdded: true },
        }
      );
    } catch (error) {
      // Compensate: delete every orphaned secret if proxy creation failed. The
      // entered configuration is deliberately left untouched — a failed create
      // must not cost the user the list they just built.
      const orphanedSecretHandles = [
        ...(createdSecretHandle ? [createdSecretHandle] : []),
        ...createdSecretHandles,
      ];
      orphanedSecretHandles.forEach((secretHandle) => {
        deleteSecret(secretHandle).catch((err) => {
          logger.warn(
            'Could not delete orphaned secret after proxy creation failure',
            {
              secretHandle,
              err,
            }
          );
        });
      });
      const backendFieldErrors = getFieldErrors(error);
      const mappedErrors: Partial<Record<keyof FormState, string>> = {};
      let hasUnmapped = false;
      backendFieldErrors?.forEach(({ field, message }) => {
        const formField = FIELD_NAME_MAP[field];
        if (formField) {
          mappedErrors[formField] = message;
        } else {
          hasUnmapped = true;
        }
      });
      if (Object.keys(mappedErrors).length > 0) {
        setFieldErrors(mappedErrors);
      }
      if (hasUnmapped || Object.keys(mappedErrors).length === 0) {
        const description = getErrorMessage(
          error,
          intl.formatMessage({
            id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.failed.to.create.proxy',
            defaultMessage: 'Failed to create proxy',
          })
        );
        showSnackbar(description, 'error');
      }
    } finally {
      setIsCreating(false);
    }
  };

  const handleOpenApiKeyModal = (target: string) => {
    setApiKeyTarget(target);
    setApiKeyError(null);
    setGeneratedKeyForDisplay(null);
    setApiKeyDisplayName('');
    setIsApiKeyModalOpen(true);
  };

  const handleCloseApiKeyModal = () => {
    if (isGeneratingApiKey) return;
    setIsApiKeyModalOpen(false);
    setApiKeyDisplayName('');
    setApiKeyError(null);
    setGeneratedKeyForDisplay(null);
  };

  const handleGenerateApiKey = async () => {
    const isForPrimary = apiKeyTarget === PRIMARY_TARGET;
    const targetProviderId = isForPrimary
      ? formState.providerId
      : (openDraft?.providerId ?? '');
    if (!currentOrganization?.uuid || !targetProviderId) {
      return;
    }

    const trimmedDisplayName = apiKeyDisplayName.trim();
    if (!trimmedDisplayName) {
      setApiKeyError('Display name is required.');
      return;
    }

    try {
      setIsGeneratingApiKey(true);
      setApiKeyError(null);

      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);

      const response = await createLLMProviderAPIKey(
        targetProviderId,
        currentOrganization.uuid,
        {
          id: buildApiKeyResourceName(trimmedDisplayName),
          displayName: trimmedDisplayName,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        },
        PLATFORM_API_BASE_URL
      );

      setGeneratedKeyForDisplay(response.apiKey);
      if (isForPrimary) {
        setSelectedProviderApiKeyValue(response.apiKey);
        setSelectedProviderApiKeyProviderId(targetProviderId);
      } else {
        // Held on the draft like a typed one: both are exchanged for a stored
        // secret at create time, so the rest of the page need not tell them
        // apart.
        updateOpenDraft((prev) => ({
          ...prev,
          generatedApiKeyValue: response.apiKey,
        }));
      }
      showSnackbar('API key generated successfully.', 'success');
    } catch (error) {
      logger.error('Failed to generate provider API key:', error);
      const description = getErrorMessage(error, 'Failed to generate API key. Please try again.');
      setApiKeyError(description);
      showSnackbar(description, 'error');
    } finally {
      setIsGeneratingApiKey(false);
    }
  };

  const handleCopyGeneratedApiKey = async () => {
    if (!generatedKeyForDisplay) return;

    try {
      await navigator.clipboard.writeText(generatedKeyForDisplay);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = generatedKeyForDisplay;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };

  /**
   * The form one extra provider is described in — the same one whether it is
   * being added or changed afterwards.
   *
   * Editing works on a copy, so the attached provider is untouched until the
   * form is committed, and abandoning the form removes nothing. Adding has
   * nothing to keep, so cancelling simply discards it.
   */
  const renderProviderForm = (
    draft: AdditionalProviderDraft,
    mode: 'add' | 'edit'
  ) => {
    const isAdding = mode === 'add';
    const cyPrefix = isAdding ? 'staged' : 'edit';
    const typedKeyReady = Boolean(draft.apiKeyValue.trim());
    const generatedKeyReady = Boolean(draft.generatedApiKeyValue);
    return (
      <Card
        variant="outlined"
        sx={{ p: 2, borderColor: 'primary.main' }}
        data-cyid={`${cyPrefix}-provider-form`}
      >
        <Stack spacing={2}>
          <Box
            display="flex"
            alignItems="center"
            justifyContent="space-between"
          >
            <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
              {isAdding ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.new.provider"
                  defaultMessage="New provider"
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.edit.provider"
                  defaultMessage="Edit provider"
                />
              )}
            </Typography>
            {/*
              Stated, not editable here: it governs every provider,
              so it belongs with the proxy rather than inside one
              provider's form.
            */}
            <Typography variant="caption" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.inbound.interface.context"
                defaultMessage="Inbound interface: {interfaceName}"
                values={{ interfaceName: inboundInterfaceLabel || '—' }}
              />
            </Typography>
          </Box>

          <FormControl fullWidth>
            <FormLabel sx={{ mb: 0.5 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.llm.service.provider"
                defaultMessage="LLM Provider"
              />
            </FormLabel>
            <Select
              value={draft.providerId}
              onChange={(event) =>
                // A credential belongs to the provider it was entered or
                // minted for, and a translator to the pair of formats, so
                // neither survives a change of provider — nor does a decision
                // to go without one.
                updateOpenDraft((prev) => ({
                  ...prev,
                  providerId: String(event.target.value),
                  apiKeyValue: '',
                  generatedApiKeyValue: '',
                  transformer: null,
                  transformerCleared: false,
                }))
              }
              displayEmpty
              disabled={isProvidersLoading}
              data-cyid={`${cyPrefix}-provider-select`}
            >
              {availableProviderOptions(draft.providerId).map((provider) => (
                <MenuItem key={provider.id} value={provider.id}>
                  {truncateProviderDisplayName(provider.displayName)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <TransformerStatusCard
            resolution={resolutionForProvider(
              draft.providerId,
              draft.transformer
            )}
            transformer={draft.transformer}
            onConfigure={() => setTransformerTarget(draft.key)}
            onRemove={() =>
              updateOpenDraft((prev) => ({ ...prev, transformer: null }))
            }
            // Building a proxy, not reviewing one: a provider that needs no
            // translator has nothing to decide here, so the section is left
            // out rather than stating an absence.
            hideWhenNotNeeded
            data-cyid={`${cyPrefix}-provider-transformer-status`}
          />

          {/*
            Only for a provider that authenticates with a key. One
            that does not gets no field at all, rather than an input
            that reads as a step still to be completed.
          */}
          {openProviderRequiresApiKey && (
            <>
              <Divider />

              <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.api.key"
                  defaultMessage="API Key"
                />
              </Typography>

              <Stack spacing={0.5}>
                <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.enter.api.key.manually.label"
                    defaultMessage="Enter API Key Manually"
                  />
                </Typography>
                <TextField
                  fullWidth
                  size="small"
                  type="password"
                  placeholder={intl.formatMessage({
                    id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.api.key.placeholder',
                    defaultMessage: 'Enter API key',
                  })}
                  value={draft.apiKeyValue}
                  onChange={(event) =>
                    updateOpenDraft((prev) => ({
                      ...prev,
                      apiKeyValue: event.target.value,
                    }))
                  }
                  data-cyid={`${cyPrefix}-provider-api-key`}
                />
                {typedKeyReady && (
                  <Alert severity="success">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.api.key.ready"
                      defaultMessage="API key will be attached when the proxy is created."
                    />
                  </Alert>
                )}
              </Stack>

              <Divider>
                <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.or.divider"
                    defaultMessage="or"
                  />
                </Typography>
              </Divider>

              {/*
                Generating is offered here on the same terms as for
                the first provider — minting a key is a separate
                grant, so the manual field above stays the way in
                for somebody who does not hold it.
              */}
              <Stack spacing={0.5}>
                <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.api.key.label"
                    defaultMessage="Generate API Key"
                  />
                </Typography>
                <Stack
                  direction={{ xs: 'column', sm: 'row' }}
                  spacing={2}
                  alignItems={{ xs: 'flex-start', sm: 'center' }}
                  sx={{
                    px: 1.5,
                    py: 1.5,
                    bgcolor: 'background.paper',
                    border: '1px solid',
                    borderColor: generatedKeyReady ? 'success.main' : 'divider',
                    borderRadius: 1,
                  }}
                >
                  <Box sx={{ flex: 1 }}>
                    {generatedKeyReady ? (
                      <Alert severity="success">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.api.key.ready"
                          defaultMessage="API key will be attached when the proxy is created."
                        />
                      </Alert>
                    ) : (
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.provider.api.key.description"
                          defaultMessage="Generate an API key for the selected LLM provider."
                        />
                      </Typography>
                    )}
                  </Box>
                  <Tooltip
                    title={
                      !canGenerateProviderApiKey
                        ? NO_PERMISSION_TOOLTIP
                        : isOpenProviderLoading
                          ? 'Loading selected provider details.'
                          : ''
                    }
                    placement="top"
                  >
                    <span>
                      <Button
                        variant="contained"
                        size="medium"
                        onClick={() => handleOpenApiKeyModal(draft.key)}
                        disabled={
                          isOpenProviderLoading || !canGenerateProviderApiKey
                        }
                        data-cyid={`${cyPrefix}-provider-generate-api-key`}
                      >
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.api.key"
                          defaultMessage="Generate API Key"
                        />
                      </Button>
                    </span>
                  </Tooltip>
                </Stack>
              </Stack>
            </>
          )}

          <Box sx={{ display: 'flex', justifyContent: 'flex-end', gap: 1 }}>
            <Button
              variant="outlined"
              color="secondary"
              onClick={
                isAdding ? () => setStagedProvider(null) : cancelProviderEdit
              }
              data-cyid={`${cyPrefix}-provider-cancel`}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.cancel"
                defaultMessage="Cancel"
              />
            </Button>
            <Button
              variant="contained"
              // Not while the provider is still being read: whether it takes a
              // key, and which header the key goes in, are unknown until then.
              disabled={!draft.providerId || isOpenProviderLoading}
              onClick={
                isAdding
                  ? () => {
                      setAdditionalProviders((prev) => [
                        ...prev,
                        {
                          ...draft,
                          apiKeyHeader: openProviderRequiresApiKey
                            ? openProviderApiKeyName
                            : '',
                          apiKeyValue: openProviderRequiresApiKey
                            ? draft.apiKeyValue
                            : '',
                          generatedApiKeyValue: openProviderRequiresApiKey
                            ? draft.generatedApiKeyValue
                            : '',
                        },
                      ]);
                      setStagedProvider(null);
                    }
                  : commitProviderEdit
              }
              data-cyid={`${cyPrefix}-provider-${isAdding ? 'add' : 'update'}`}
            >
              {isAdding ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.add.provider.commit"
                  defaultMessage="Add provider"
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.update.provider"
                  defaultMessage="Update provider"
                />
              )}
            </Button>
          </Box>
        </Stack>
      </Card>
    );
  };

  /**
   * The primary's own fields: which provider it is, what translates for it, and
   * how it authenticates.
   *
   * Rendered bare while it is the only provider, and inside its edit form once
   * it has collapsed to a row — the same fields either way, so the primary's
   * credential lives with the primary rather than floating between the rows of
   * providers it does not belong to.
   */
  const renderPrimaryFields = () => (
    <>
      <FormControl fullWidth>
        <FormLabel sx={{ mb: 0.5 }}>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.llm.service.provider"
            defaultMessage="LLM Provider"
          />
        </FormLabel>
        <Select
          value={formState.providerId}
          onChange={(event) =>
            setFormState((prev) => ({
              ...prev,
              providerId: event.target.value,
            }))
          }
          displayEmpty
          disabled={isProvidersLoading || isProviderSelectionLocked}
          data-cyid="proxy-provider-select"
        >
          {isProviderSelectionLocked && lockedProviderId ? (
            <MenuItem value={lockedProviderId}>
              {lockedProviderDisplayName}
            </MenuItem>
          ) : isProvidersLoading ? (
            <MenuItem value="" disabled>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.loading.providers"
                defaultMessage="Loading providers..."
              />
            </MenuItem>
          ) : providerOptions.length === 0 ? (
            <MenuItem value="" disabled>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.no.providers.available"
                defaultMessage="No providers available"
              />
            </MenuItem>
          ) : (
            // A provider already attached is not offered again: two entries for
            // the same provider resolve to the same request handle, so routing
            // to either of them would be ambiguous.
            availableProviderOptions(formState.providerId).map((provider) => (
              <MenuItem key={provider.id} value={provider.id}>
                {truncateProviderDisplayName(provider.displayName)}
              </MenuItem>
            ))
          )}
        </Select>
      </FormControl>

      <TransformerStatusCard
        resolution={resolutionForProvider(
          formState.providerId,
          primaryTransformer
        )}
        transformer={primaryTransformer}
        onConfigure={() => setTransformerTarget(PRIMARY_TARGET)}
        onRemove={() => setPrimaryTransformer(null)}
        hideWhenNotNeeded
        data-cyid="primary-transformer-status"
      />

      {selectedProviderRequiresApiKey && (
        <Stack spacing={1}>
          <Typography variant="body2" sx={{ fontWeight: '600' }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.api.keys"
              defaultMessage="API Keys"
            />
          </Typography>

          <Stack spacing={0.5}>
            <Typography variant="caption" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.enter.api.key.manually.label"
                defaultMessage="Enter API Key Manually"
              />
            </Typography>
            <Stack
              sx={{
                gap: 1,
              }}
            >
              <TextField
                fullWidth
                size="small"
                type="password"
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.api.key.placeholder',
                  defaultMessage: 'Enter API key',
                })}
                value={manualApiKeyValue}
                onChange={(event) => setManualApiKeyValue(event.target.value)}
                data-cyid="proxy-api-key-input"
              />
              {isManualKeyReady && (
                <Alert severity="success">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.api.key.ready"
                    defaultMessage="API key will be attached when the proxy is created."
                  />
                </Alert>
              )}
            </Stack>
          </Stack>

          <Divider>
            <Typography variant="caption" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.or.divider"
                defaultMessage="or"
              />
            </Typography>
          </Divider>

          <Stack spacing={0.5}>
            <Typography variant="caption" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.api.key.label"
                defaultMessage="Generate API Key"
              />
            </Typography>
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              spacing={2}
              alignItems={{ xs: 'flex-start', sm: 'center' }}
              sx={{
                px: 1.5,
                py: 1.5,
                bgcolor: 'background.paper',
                border: '1px solid',
                borderColor: isGeneratedKeyReady ? 'success.main' : 'divider',
                borderRadius: 1,
              }}
            >
              <Box sx={{ flex: 1 }}>
                {isGeneratedKeyReady ? (
                  <Alert severity="success">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.api.key.ready"
                      defaultMessage="API key will be attached when the proxy is created."
                    />
                  </Alert>
                ) : (
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.provider.api.key.description"
                      defaultMessage="Generate an API key for the selected LLM provider."
                    />
                  </Typography>
                )}
              </Box>
              <Tooltip
                title={
                  !canGenerateProviderApiKey
                    ? NO_PERMISSION_TOOLTIP
                    : isSelectedProviderLoading
                      ? 'Loading selected provider details.'
                      : ''
                }
                placement="top"
              >
                <span>
                  <Button
                    variant="contained"
                    size="medium"
                    onClick={() => handleOpenApiKeyModal(PRIMARY_TARGET)}
                    disabled={
                      isSelectedProviderLoading || !canGenerateProviderApiKey
                    }
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.api.key"
                      defaultMessage="Generate API Key"
                    />
                  </Button>
                </span>
              </Tooltip>
            </Stack>
          </Stack>

          {apiKeyError && (
            <Alert severity="error" onClose={() => setApiKeyError(null)}>
              {apiKeyError}
            </Alert>
          )}
        </Stack>
      )}
    </>
  );

  if (!canCreateProxy) {
    return (
      <PageContent fullWidth>
        <Stack spacing={1}>
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.creation.unavailable"
              defaultMessage={'App LLM Proxy creation is unavailable.'}
            />
          </Typography>
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.creation.unavailable.description"
              defaultMessage={
                'You do not have permission to create App LLM Proxies. Please contact your admin.'
              }
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={proxiesPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.back.to.list"
          defaultMessage="Back to list"
        />
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.create.llm.proxy"
              defaultMessage="Create App LLM Proxy"
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 720 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 8 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.name.required"
                  defaultMessage="Name *"
                />
              </FormLabel>
              <TextField
                fullWidth
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.name.placeholder',
                  defaultMessage: 'WSO2 OpenAI Provider Proxy',
                })}
                value={formState.name}
                onChange={(event) => {
                  setFormState((prev) => ({
                    ...prev,
                    name: event.target.value,
                  }));
                  setFieldErrors((prev) => ({ ...prev, name: undefined }));
                }}
                error={Boolean(fieldErrors.name)}
                helperText={fieldErrors.name}
                data-cyid="proxy-name-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.version.required"
                  defaultMessage="Version *"
                />
              </FormLabel>
              <TextField
                fullWidth
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.version.placeholder',
                  defaultMessage: 'v1.0',
                })}
                value={formState.version}
                onChange={(event) => {
                  setFormState((prev) => ({
                    ...prev,
                    version: event.target.value,
                  }));
                  setFieldErrors((prev) => ({ ...prev, version: undefined }));
                }}
                error={Boolean(fieldErrors.version)}
                helperText={fieldErrors.version}
                data-cyid="proxy-version-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.description"
                  defaultMessage="Description"
                />
              </FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={3}
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.description.placeholder',
                  defaultMessage: 'Primary OpenAI provider',
                })}
                value={formState.description}
                onChange={(event) => {
                  setFormState((prev) => ({
                    ...prev,
                    description: event.target.value,
                  }));
                  setFieldErrors((prev) => ({ ...prev, description: undefined }));
                }}
                error={Boolean(fieldErrors.description)}
                helperText={fieldErrors.description}
                data-cyid="proxy-description-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.context"
                  defaultMessage="Context"
                />
              </FormLabel>
              <TextField
                fullWidth
                value={effectiveContext}
                onChange={(event) => setContextOverride(event.target.value)}
                data-cyid="proxy-context-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <Card sx={{ p: 2 }}>
              <Stack spacing={2}>
                <Typography variant="h6">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.configuration"
                    defaultMessage="Provider Configuration"
                  />
                </Typography>

                {/*
                  The primary: its own fields while it is the only provider or
                  while it is being edited, a row otherwise.
                */}
                {isPrimaryExpanded ? (
                  editingKey === PRIMARY_TARGET ? (
                    <Card
                      variant="outlined"
                      sx={{ p: 2, borderColor: 'primary.main' }}
                      data-cyid="primary-provider-form"
                    >
                      <Stack spacing={2}>
                        <Box
                          display="flex"
                          alignItems="center"
                          justifyContent="space-between"
                        >
                          <Typography
                            variant="subtitle2"
                            sx={{ fontWeight: 600 }}
                          >
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.edit.primary.provider"
                              defaultMessage="Primary provider"
                            />
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.inbound.interface.context"
                              defaultMessage="Inbound interface: {interfaceName}"
                              values={{
                                interfaceName: inboundInterfaceLabel || '—',
                              }}
                            />
                          </Typography>
                        </Box>

                        {renderPrimaryFields()}

                        <Box
                          sx={{
                            display: 'flex',
                            justifyContent: 'flex-end',
                            gap: 1,
                          }}
                        >
                          <Button
                            variant="outlined"
                            color="secondary"
                            onClick={cancelPrimaryEdit}
                            data-cyid="primary-provider-cancel"
                          >
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.cancel"
                              defaultMessage="Cancel"
                            />
                          </Button>
                          <Button
                            variant="contained"
                            disabled={!formState.providerId}
                            onClick={() => {
                              setEditingKey(null);
                              setPrimarySnapshot(null);
                            }}
                            data-cyid="primary-provider-update"
                          >
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.update.provider"
                              defaultMessage="Update provider"
                            />
                          </Button>
                        </Box>
                      </Stack>
                    </Card>
                  ) : (
                    renderPrimaryFields()
                  )
                ) : (
                  <ProviderRow
                    displayName={
                      providerOptions.find(
                        (provider) => provider.id === formState.providerId
                      )?.displayName ?? formState.providerId
                    }
                    isPrimary
                    showPrimaryToggle
                    resolution={resolutionForProvider(
                      formState.providerId,
                      primaryTransformer
                    )}
                    onEdit={beginPrimaryEdit}
                    onConfigureTransformer={() =>
                      setTransformerTarget(PRIMARY_TARGET)
                    }
                    disabled={isProviderFormOpen}
                    data-cyid="provider-row-0"
                  />
                )}

                {/*
                  Providers already settled, collapsed to rows. The primary
                  joins them as soon as a second provider is involved, so the
                  only expanded thing is whatever is being filled in.
                */}
                {additionalProviders.map((draft, index) => {
                  if (editingKey === draft.key && editDraft) {
                    return (
                      <React.Fragment key={draft.key}>
                        {renderProviderForm(editDraft, 'edit')}
                      </React.Fragment>
                    );
                  }
                  const draftProvider = providerOptions.find(
                    (option) => option.id === draft.providerId
                  );
                  return (
                    <ProviderRow
                      key={draft.key}
                      displayName={
                        draftProvider?.displayName ?? draft.providerId
                      }
                      isPrimary={false}
                      showPrimaryToggle
                      resolution={resolutionForProvider(
                        draft.providerId,
                        draft.transformer
                      )}
                      onMakePrimary={() => promoteProvider(draft.key)}
                      onEdit={() => beginProviderEdit(draft)}
                      onRemove={() =>
                        setAdditionalProviders((prev) =>
                          prev.filter((entry) => entry.key !== draft.key)
                        )
                      }
                      onConfigureTransformer={() =>
                        setTransformerTarget(draft.key)
                      }
                      // One form at a time: acting on another row while one is
                      // open would leave changes nobody committed.
                      disabled={isProviderFormOpen}
                      data-cyid={`provider-row-${index + 1}`}
                    />
                  );
                })}

                {/* The form being filled in, staged until it is added. */}
                {stagedProvider && renderProviderForm(stagedProvider, 'add')}

                <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<Plus size={16} />}
                    onClick={() => {
                      // Opens a form to fill in, rather than committing an
                      // empty provider to the list. Nothing joins the list
                      // until it is actually described.
                      const draft = newAdditionalProviderDraft();
                      const firstAvailable = availableProviderOptions('')[0];
                      setStagedProvider({
                        ...draft,
                        providerId: firstAvailable?.id ?? '',
                      });
                    }}
                    disabled={isProvidersLoading || isProviderFormOpen}
                    data-cyid="add-provider-button"
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.add.additional.provider"
                      defaultMessage="Add Additional LLM Provider"
                    />
                  </Button>
                </Box>
              </Stack>
            </Card>
          </Grid>

          {/*
            Collapsed by default. Attaching several providers and choosing the
            request format are both uncommon next to creating an ordinary
            single-provider proxy, so they are reachable without being in the
            way of the path nearly everyone takes.
          */}
          <Grid size={{ xs: 12 }}>
            <Button
              size="small"
              startIcon={
                advancedOpen ? (
                  <ChevronDown size={18} />
                ) : (
                  <ChevronRight size={18} />
                )
              }
              onClick={() => setAdvancedOpen((open) => !open)}
              data-cyid="advanced-configurations-toggle"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.advanced.configurations"
                defaultMessage="Advanced Configurations"
              />
            </Button>
            {advancedOpen && (
              <>
                <Divider sx={{ mt: 1, mb: 2 }} />
                <InboundInterfaceSelect
                  value={inboundTemplate}
                  onChange={(templateHandle) => {
                    // Once chosen deliberately, the derived default stops
                    // moving underneath the choice.
                    setHasChosenInterface(true);
                    setInboundTemplate(templateHandle);
                  }}
                  templates={templatesResponse.list}
                  isLoading={isTemplatesLoading}
                  error={templatesError}
                />
              </>
            )}
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button
            variant="contained"
            component={RouterLink}
            to={proxiesPath}
            color="secondary"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.cancel"
              defaultMessage="Cancel"
            />
          </Button>
          <Tooltip title={createProxyBlockedReason} placement="top">
            <span>
              <Button
                variant="contained"
                onClick={handleCreate}
                disabled={isCreateProxyDisabled}
                data-cyid="create-proxy-button"
              >
                {isCreating ? (
                  <CircularProgress size={20} />
                ) : (
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.create.proxy"
                    defaultMessage="Create Proxy"
                  />
                )}
              </Button>
            </span>
          </Tooltip>
        </Box>
      </Box>

      <Dialog
        open={isApiKeyModalOpen}
        onClose={handleCloseApiKeyModal}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          {generatedKeyForDisplay
            ? 'API Key Generated Successfully'
            : 'Generate API Key'}
        </DialogTitle>
        <DialogContent>
          {apiKeyError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {apiKeyError}
            </Alert>
          )}
          {generatedKeyForDisplay ? (
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
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.copy.generated.api.key.warning"
                    defaultMessage="Please copy and save this API key. For security reasons, you won't be able to see it again."
                  />
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
                    header
                  </Typography>
                  <TextField
                    size="small"
                    fullWidth
                    value={
                      apiKeyTarget === PRIMARY_TARGET
                        ? selectedProviderApiKeyName
                        : openProviderApiKeyName
                    }
                    slotProps={{
                      input: {
                        readOnly: true,
                      },
                    }}
                  />
                </Box>
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
                      bgcolor: 'background.paper',
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1,
                      fontFamily: 'monospace',
                      fontSize: '0.875rem',
                    }}
                  >
                    <Box sx={{ flex: 1, wordBreak: 'break-all' }}>
                      {generatedKeyForDisplay}
                    </Box>
                    <IconButton
                      size="small"
                      onClick={() => {
                        void handleCopyGeneratedApiKey();
                      }}
                      sx={{ flexShrink: 0 }}
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
                  value={apiKeyDisplayName}
                  onChange={(event) => setApiKeyDisplayName(event.target.value)}
                  placeholder="Ex: Production Key"
                />
              </FormControl>
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          {generatedKeyForDisplay ? (
            <Button
              onClick={handleCloseApiKeyModal}
              variant="outlined"
              size="small"
            >
              Done
            </Button>
          ) : (
            <>
              <Button
                onClick={handleCloseApiKeyModal}
                variant="outlined"
                color="secondary"
                size="small"
                disabled={isGeneratingApiKey}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                size="small"
                onClick={() => {
                  void handleGenerateApiKey();
                }}
                disabled={isGeneratingApiKey || !apiKeyDisplayName.trim()}
              >
                {isGeneratingApiKey ? (
                  <>
                    <CircularProgress size={16} sx={{ mr: 1 }} />
                    Generating...
                  </>
                ) : (
                  'Generate'
                )}
              </Button>
            </>
          )}
        </DialogActions>
      </Dialog>

      {/*
        One drawer for the whole page: whichever provider asked for it supplies
        the policy currently attached, and receives the choice back.
      */}
      <TransformerDrawer
        open={transformerTarget !== null}
        onClose={() => setTransformerTarget(null)}
        policies={transformerPolicies}
        isLoading={!transformerPoliciesLoaded}
        current={
          transformerTarget === PRIMARY_TARGET
            ? primaryTransformer
            : ((openDraft?.key === transformerTarget
                ? openDraft.transformer
                : additionalProviders.find(
                    (entry) => entry.key === transformerTarget
                  )?.transformer) ?? null)
        }
        onApply={(transformer) => applyTransformerToTarget(transformer)}
      />
    </PageContent>
  );
}

type LLMProxyNewLocationState = {
  preselectedProviderId?: string;
  preselectedProvider?: LLMProvider;
  selectedProjectId?: string;
};

export default function LLMProxyNew() {
  const location = useLocation();
  const state = location.state as LLMProxyNewLocationState | null;
  const lockedProviderId = state?.preselectedProviderId?.trim() ?? '';
  const preselectedProjectId = state?.selectedProjectId?.trim() ?? '';
  const preselectedProvider =
    state?.preselectedProvider?.id === lockedProviderId
      ? state.preselectedProvider
      : null;
  const [selectedProviderId, setSelectedProviderId] =
    useState(lockedProviderId);

  useEffect(() => {
    if (!lockedProviderId) return;
    setSelectedProviderId(lockedProviderId);
  }, [lockedProviderId]);

  return (
    <LLMProviderProvider providerId={selectedProviderId}>
      {/*
        The template catalogue backs the inbound interface choice. Mounted here
        rather than fetched on the page so it uses the same reader, and the same
        cache, as everywhere else templates are listed.
      */}
      <ProviderTemplatesProvider>
        <LLMProxyNewContent
          selectedProviderId={selectedProviderId}
          onSelectedProviderIdChange={setSelectedProviderId}
          lockedProviderId={lockedProviderId}
          isProviderSelectionLocked={Boolean(lockedProviderId)}
          preselectedProvider={preselectedProvider}
          preselectedProjectId={preselectedProjectId}
        />
      </ProviderTemplatesProvider>
    </LLMProviderProvider>
  );
}
