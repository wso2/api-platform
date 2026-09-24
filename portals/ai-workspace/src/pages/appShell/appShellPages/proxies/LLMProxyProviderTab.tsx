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

import React, { useState } from 'react';
import { FormattedMessage } from 'react-intl';
import {
  Box,
  Button,
  Chip,
  Divider,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus } from '@wso2/oxygen-ui-icons-react';
import { useLLMProviders } from '../../../../contexts/llmProvider';
import { useProxy } from '../../../../contexts/proxy';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import { useAppShell } from '../../../../contexts/AppShellContext';
import type {
  LLMProvider,
  ProxyApiKeySecurity,
  ProxyProviderEntry,
} from '../../../../utils/types';
import {
  canRemoveProvider,
  effectiveProviderName,
  proxyProviderEntries,
  withPrimaryProvider,
} from '../../../../utils/proxyProviders';
import { resolveTransformer } from '../../../../utils/transformerResolution';
import useTransformerPolicies from '../../../../hooks/useTransformerPolicies';
import ProviderRow from '../../../../Components/Transformer/ProviderRow';
import ProviderSettingsDrawer from '../../../../Components/Transformer/ProviderSettingsDrawer';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';

/**
 * Providers tab — the list of providers this proxy routes to, and what
 * distinguishes them from each other.
 *
 * The rows carry only that: which provider, what translates for it, and the
 * name a client uses to select it. Everything else about one provider is behind
 * its own panel, so the list stays readable as it grows.
 */
export type LLMProxyProviderTabProps = {
  /** Takes the user to where the interface is actually editable. */
  onChangeInDefinition?: () => void;
};

/** A stable key for a row, so React does not reuse one provider's row for another. */
const rowKey = (entry: ProxyProviderEntry, index: number) =>
  `${entry.id}-${index}`;

export default function LLMProxyProviderTab({
  onChangeInDefinition,
}: LLMProxyProviderTabProps = {}) {
  const { proxy, setLocalProxy } = useProxy();
  const { providersResponse, isLoading: isProvidersLoading } =
    useLLMProviders();
  const { templatesResponse } = useProviderTemplates();
  const { currentOrganization } = useAppShell();
  const { hasPermission } = useAppAuth();
  const organizationId = currentOrganization?.uuid ?? '';
  const {
    policies: transformerPolicies,
    isLoaded: transformerPoliciesLoaded,
  } = useTransformerPolicies();

  const isReadOnlyProxy = Boolean(proxy?.readOnly);
  const providerOptions = providersResponse.list;
  // Every provider the proxy is attached to, primary first. The single legacy
  // field is deliberately not read: it names only the primary, so a proxy with
  // several providers would display as if it had one.
  const providerEntries = proxyProviderEntries(proxy);

  /**
   * Which attachment the settings panel is open on: an index into the list, or
   * `add` while a new one is being described. Held as an index rather than an
   * id so the panel stays on the same row if its provider is changed.
   */
  const [openIndex, setOpenIndex] = useState<number | 'add' | null>(null);

  /**
   * The interface named the way the catalogue names it.
   *
   * A proxy stores the handle, which is what routes; a reader recognises the
   * display name. The handle is the fallback rather than the label, so an
   * interface the catalogue no longer lists still says which one it is.
   */
  const inboundHandle =
    proxy?.inboundTemplate ||
    providerOptions.find((provider) => provider.id === providerEntries[0]?.id)
      ?.template ||
    '';
  const interfaceLabel =
    templatesResponse.list.find((template) => template.id === inboundHandle)
      ?.displayName ||
    inboundHandle ||
    '—';

  const providerDisplayName = (entry: ProxyProviderEntry): string =>
    providerOptions.find((provider) => provider.id === entry.id)?.displayName ??
    entry.id;

  /**
   * What translates for this provider, decided in one place so this screen says
   * the same thing as every other screen showing the same provider.
   */
  const resolutionFor = (entry: ProxyProviderEntry) => {
    const provider = providerOptions.find((option) => option.id === entry.id);
    return resolveTransformer({
      // The interface in effect, not only the one stored. A proxy created
      // before the setting existed carries none and runs on its primary
      // provider's format; passing the empty value would make every provider
      // on it — the primary included — read as needing a translator that
      // cannot be matched.
      inboundTemplate: inboundHandle,
      providerTemplate: provider?.template,
      chosenTransformer: entry.transformer,
      // A saved proxy either names a translator or does not. Offering the one
      // that would apply as though it already did is how a provider comes to
      // read as translating when nothing on the proxy translates for it — and
      // how a translator just removed appears to still be there.
      hasNoTransformer: !entry.transformer,
      policies: transformerPolicies,
      policiesLoaded: transformerPoliciesLoaded,
      interfaceLabel,
      providerLabel: provider?.displayName,
    });
  };

  const mapProviderSecurityToProxySecurity = (provider: LLMProvider) => {
    const providerApiKey = provider.security?.apiKey;
    const apiKey: ProxyApiKeySecurity | undefined = providerApiKey
      ? {
          enabled: Boolean(providerApiKey.enabled),
          key: providerApiKey.key ?? '',
          in: providerApiKey.in ?? 'header',
          valuePrefix: providerApiKey.valuePrefix,
        }
      : undefined;
    return { enabled: Boolean(provider.security?.enabled), apiKey };
  };

  const handleMakePrimary = (providerId: string) => {
    if (isReadOnlyProxy) return;
    setLocalProxy((prev) =>
      prev
        ? {
            ...prev,
            providers: withPrimaryProvider(prev.providers ?? [], providerId),
          }
        : prev
    );
  };

  const handleRemoveProvider = (providerId: string) => {
    if (isReadOnlyProxy) return;
    setLocalProxy((prev) => {
      if (!prev) return prev;
      const remaining = (prev.providers ?? []).filter(
        (entry) => entry.id !== providerId
      );
      // A proxy always has a primary. If the one removed was it, the next
      // attachment takes over rather than leaving the proxy without one.
      const hasPrimary = remaining.some((entry) => entry.isPrimary);
      return {
        ...prev,
        providers:
          hasPrimary || remaining.length === 0
            ? remaining
            : withPrimaryProvider(remaining, remaining[0].id),
      };
    });
  };

  /**
   * Writes back what the settings panel produced.
   *
   * The proxy's own identity — its published specification, its host, what it
   * requires of callers — comes from the primary provider, so changing which
   * provider is primary changes those with it. Changing a provider that is not
   * primary changes nothing beyond that attachment.
   */
  const handleSaveEntry = (
    next: ProxyProviderEntry,
    detail: LLMProvider | null
  ) => {
    if (isReadOnlyProxy) return;
    setLocalProxy((prev) => {
      if (!prev) return prev;
      const entries = prev.providers ?? [];
      const isAdding = openIndex === 'add';
      // The panel was opened on a position in the *displayed* list, which is
      // sorted primary-first; the stored list is not. Making the primary a
      // different provider changes one order and not the other, so a position
      // carried across would write an edit over a different provider —
      // removing it and duplicating the one being edited. The entry itself is
      // carried across instead, and found by identity.
      const target = isAdding ? null : openEntry;
      const targetIndex = target ? entries.indexOf(target) : -1;
      const replaced = targetIndex >= 0 ? entries[targetIndex] : undefined;
      if (!isAdding && targetIndex < 0) {
        // The provider being edited is no longer in the list. Writing it back
        // would resurrect something already removed.
        return prev;
      }
      const providers = isAdding
        ? [...entries, { ...next, isPrimary: entries.length === 0 }]
        : entries.map((entry, index) => (index === targetIndex ? next : entry));
      // Re-derived only when the primary is actually a different provider.
      // Deriving it again from the same provider would rewrite fields nobody
      // edited, and a panel closed without a change would read as an edit.
      const primaryProviderChanged = isAdding
        ? entries.length === 0
        : Boolean(next.isPrimary) && replaced?.id !== next.id;
      const inheritsProxyIdentity = Boolean(detail) && primaryProviderChanged;
      return {
        ...prev,
        providers,
        ...(inheritsProxyIdentity && detail
          ? {
              vhost: detail.vhost?.trim() || undefined,
              openapi: detail.openapi ?? '',
              security: mapProviderSecurityToProxySecurity(detail),
            }
          : {}),
      };
    });
  };

  const openEntry =
    typeof openIndex === 'number' ? (providerEntries[openIndex] ?? null) : null;

  return (
    <Stack spacing={2}>
      {/*
        The interface constrains this whole list — it decides what each provider
        needs translating to — so it is stated here. It is not editable here: it
        belongs to the definition, and a link goes there rather than a second
        control that could disagree with the first.
      */}
      <Box
        display="flex"
        alignItems="center"
        gap={1.5}
        data-cyid="providers-inbound-interface"
      >
        <Typography variant="body2" color="text.secondary">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.inbound.interface"
            defaultMessage="Inbound interface"
          />
        </Typography>
        <Chip size="small" variant="outlined" label={interfaceLabel} />
        <Button
          size="small"
          onClick={onChangeInDefinition}
          data-cyid="change-in-definition"
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.change.in.definition"
            defaultMessage="Change in Definition"
          />
        </Button>
      </Box>

      <Divider />

      <Typography variant="h6" sx={{ fontWeight: 600 }}>
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.providers.count"
          defaultMessage="Providers {count}"
          values={{ count: providerEntries.length }}
        />
      </Typography>

      {/*
        One row per attached provider. The name shown alongside the translator
        is the one a client puts in a routing header to select it — the alias
        when the provider has one, its id otherwise. Showing the id for an
        aliased provider would show something that does not route.
      */}
      <Stack spacing={1} data-cyid="proxy-provider-list">
        {providerEntries.map((entry, index) => (
          <ProviderRow
            key={rowKey(entry, index)}
            variant="plain"
            displayName={providerDisplayName(entry)}
            requestHandle={effectiveProviderName(entry)}
            isPrimary={Boolean(entry.isPrimary)}
            showPrimaryToggle={providerEntries.length > 1}
            resolution={resolutionFor(entry)}
            onMakePrimary={() => handleMakePrimary(entry.id)}
            onEdit={() => setOpenIndex(index)}
            // Withheld on the last remaining row rather than offered and then
            // refused: a proxy always has a provider.
            onRemove={
              canRemoveProvider(providerEntries)
                ? () => handleRemoveProvider(entry.id)
                : undefined
            }
            onConfigureTransformer={() => setOpenIndex(index)}
            disabled={isReadOnlyProxy}
            data-cyid={`provider-row-${index}`}
          />
        ))}
      </Stack>

      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button
          size="small"
          variant="outlined"
          startIcon={<Plus size={16} />}
          disabled={isReadOnlyProxy || isProvidersLoading}
          onClick={() => setOpenIndex('add')}
          data-cyid="add-provider-button"
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.add.provider"
            defaultMessage="Add Additional LLM Provider"
          />
        </Button>
      </Box>

      <ProviderSettingsDrawer
        open={openIndex !== null}
        onClose={() => setOpenIndex(null)}
        entry={openEntry}
        inboundTemplate={inboundHandle}
        interfaceLabel={interfaceLabel}
        providerOptions={providerOptions}
        attachedProviderIds={providerEntries
          .filter((_, index) => index !== openIndex)
          .map((entry) => entry.id)}
        policies={transformerPolicies}
        policiesLoaded={transformerPoliciesLoaded}
        organizationId={organizationId}
        canGenerateApiKey={hasPermission(SCOPES.LLM_PROVIDER_API_KEY_CREATE)}
        disabled={isReadOnlyProxy}
        onSave={handleSaveEntry}
      />
    </Stack>
  );
}
