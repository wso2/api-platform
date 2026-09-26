/**
 * Everything about one attached provider, in one panel.
 *
 * A proxy's providers are a list of rows; what distinguishes them — which
 * provider, what translates for it, how it authenticates — is too much to sit
 * in a row, so the row carries what tells them apart and this carries the rest.
 *
 * It edits a copy. Nothing reaches the proxy until the panel is saved, and
 * nothing reaches the server until the page is, so an abandoned panel leaves
 * both untouched.
 */

import React, { useEffect, useMemo, useState } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Drawer,
  FormControl,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Copy, X } from '@wso2/oxygen-ui-icons-react';
import TransformerStatusCard from './TransformerStatusCard';
import TransformerDrawer from './TransformerDrawer';
import { resolveTransformer } from '../../utils/transformerResolution';
import { resolvedTransformerFor } from '../../utils/proxyProviders';
import { resolveApiKeyAuthDisplay } from '../../utils/apiKeyAuthDisplay';
import { getLLMProvider, createLLMProviderAPIKey } from '../../apis/llmProviderApis';
import { getPolicyDefinition } from '../../apis/policyHubApis';
import { parsePolicyYaml } from '../../pages/appShell/PolicyParameterEditor';
import type { ParameterSchema } from '../../pages/appShell/PolicyParameterEditor';
import { PLATFORM_API_BASE_URL } from '../../paths';
import { buildApiKeyResourceName } from '../../utils/apiKeyNaming';
import { logger } from '../../utils/logger';
import { getErrorMessage } from '../../utils/apiError';
import type {
  LLMProvider,
  PolicyParameterDefinition,
  ProxyProviderEntry,
  ProxyProviderTransformer,
  SelectablePolicy,
} from '../../utils/types';

/**
 * A policy's parameters as the picker's own listing does not carry them.
 *
 * Declarations are published separately from the listing, so a translator read
 * from the catalogue arrives with none. Reading them here is what lets this
 * panel say which of them a provider has not set.
 */
const parametersFromSchema = (
  schema?: ParameterSchema
): PolicyParameterDefinition[] =>
  Object.entries(schema?.properties ?? {}).map(([name, property]) => ({
    name,
    type: property.type ?? 'string',
    description: property.description,
    required: (schema?.required ?? []).includes(name),
    default: property.default,
  }));

export type ProviderSettingsDrawerProps = {
  open: boolean;
  onClose: () => void;
  /** The attachment being edited, or null while one is being added. */
  entry: ProxyProviderEntry | null;
  /** The format the proxy accepts, which decides what needs translating. */
  inboundTemplate?: string;
  interfaceLabel?: string;
  providerOptions: LLMProvider[];
  /** Providers already attached elsewhere on this proxy, which cannot be chosen. */
  attachedProviderIds: string[];
  policies: SelectablePolicy[];
  policiesLoaded: boolean;
  organizationId: string;
  canGenerateApiKey?: boolean;
  disabled?: boolean;
  onSave: (next: ProxyProviderEntry, detail: LLMProvider | null) => void;
};

export default function ProviderSettingsDrawer({
  open,
  onClose,
  entry,
  inboundTemplate,
  interfaceLabel,
  providerOptions,
  attachedProviderIds,
  policies,
  policiesLoaded,
  organizationId,
  canGenerateApiKey = true,
  disabled = false,
  onSave,
}: ProviderSettingsDrawerProps) {
  const intl = useIntl();
  const [providerId, setProviderId] = useState('');
  const [transformer, setTransformer] =
    useState<ProxyProviderTransformer | null>(null);
  /**
   * A key typed here. Empty means whatever is stored stays as it is.
   *
   * Held apart from a minted one so the field a user typed into is never
   * filled in behind them — the same split the create page makes.
   */
  const [apiKeyValue, setApiKeyValue] = useState('');
  const [generatedApiKeyValue, setGeneratedApiKeyValue] = useState('');
  /** The naming step a key is minted through, so a key is named by its owner. */
  const [isNamingKey, setIsNamingKey] = useState(false);
  const [keyDisplayName, setKeyDisplayName] = useState('');
  /** Shown once, and never again — the server does not return it a second time. */
  const [mintedKeyForDisplay, setMintedKeyForDisplay] = useState<string | null>(
    null
  );
  const [detail, setDetail] = useState<LLMProvider | null>(null);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);
  const [generateError, setGenerateError] = useState<string | null>(null);
  const [isPickingTransformer, setIsPickingTransformer] = useState(false);
  /**
   * Whether the translator was taken off deliberately.
   *
   * Absence alone cannot say: a provider that never had one and a provider
   * whose one was just removed both hold nothing, and only the second must be
   * left alone. Without this the match would be worked out again and put
   * straight back, and removing would do nothing.
   */
  const [wasCleared, setWasCleared] = useState(false);
  /** Parameter declarations per policy, read once each and kept for the panel. */
  const [declaredParameters, setDeclaredParameters] = useState<
    Record<string, PolicyParameterDefinition[]>
  >({});

  const isAdding = entry === null;

  useEffect(() => {
    if (!open) {
      return;
    }
    setProviderId(entry?.id ?? '');
    setTransformer(entry?.transformer ?? null);
    setApiKeyValue('');
    setGeneratedApiKeyValue('');
    setGenerateError(null);
    setIsPickingTransformer(false);
    setWasCleared(false);
    setIsNamingKey(false);
    setKeyDisplayName('');
    setMintedKeyForDisplay(null);
  }, [open, entry]);

  // Whether a provider needs a key, and which header it goes in, is declared on
  // the provider itself — the list this panel selects from carries neither.
  useEffect(() => {
    if (!open || !providerId || !organizationId) {
      setDetail(null);
      return undefined;
    }
    let abandoned = false;
    // Dropped before the fetch, not after it. Held on to, it describes the
    // provider that was selected a moment ago — whether it takes a key, and
    // which header the key goes in — and a commit made in that moment saves
    // the wrong header against the new provider.
    setDetail(null);
    setIsDetailLoading(true);
    getLLMProvider(providerId, organizationId, PLATFORM_API_BASE_URL)
      .then((loaded) => {
        if (!abandoned) {
          setDetail(loaded);
        }
      })
      .catch((error) => {
        if (abandoned) {
          return;
        }
        logger.error('Failed to load the selected provider:', error);
        setDetail(null);
      })
      .finally(() => {
        if (!abandoned) {
          setIsDetailLoading(false);
        }
      });
    return () => {
      abandoned = true;
    };
  }, [open, providerId, organizationId]);

  const selectableProviders = useMemo(
    () =>
      providerOptions.filter(
        (provider) =>
          provider.id === providerId || !attachedProviderIds.includes(provider.id)
      ),
    [providerOptions, attachedProviderIds, providerId]
  );

  const resolution = useMemo(
    () =>
      resolveTransformer({
        inboundTemplate,
        providerTemplate: providerOptions.find(
          (provider) => provider.id === providerId
        )?.template,
        chosenTransformer: transformer,
        // A provider already attached without one is shown as it is stored.
        // A provider being chosen here is shown the match that choosing it
        // will record, because saving this panel is what records it.
        hasNoTransformer:
          wasCleared || (entry !== null && entry.id === providerId),
        policies,
        policiesLoaded,
        interfaceLabel,
        providerLabel: providerOptions.find(
          (provider) => provider.id === providerId
        )?.displayName,
      }),
    [
      inboundTemplate,
      interfaceLabel,
      policies,
      policiesLoaded,
      providerId,
      providerOptions,
      transformer,
      wasCleared,
      entry,
    ]
  );

  // Read for whichever translator applies, so the card can report the ones the
  // provider leaves unset.
  const appliedPolicy = resolution.policy;
  useEffect(() => {
    if (!open || !appliedPolicy || appliedPolicy.definition) {
      return;
    }
    const key = `${appliedPolicy.name}@${appliedPolicy.version}`;
    if (declaredParameters[key]) {
      return;
    }
    let abandoned = false;
    getPolicyDefinition(appliedPolicy.name, appliedPolicy.version)
      .then((response) => {
        if (abandoned) {
          return;
        }
        setDeclaredParameters((prev) => ({
          ...prev,
          [key]: parametersFromSchema(parsePolicyYaml(response).parameters),
        }));
      })
      .catch((error) => {
        // Reported, never enforced: an unread declaration simply means no
        // unset parameters are named, which is what the card already shows.
        logger.error('Failed to load transformer parameters:', error);
      });
    return () => {
      abandoned = true;
    };
  }, [open, appliedPolicy, declaredParameters]);

  const resolutionWithParameters = useMemo(() => {
    if (!appliedPolicy || (appliedPolicy.parameters ?? []).length > 0) {
      return resolution;
    }
    const read = declaredParameters[`${appliedPolicy.name}@${appliedPolicy.version}`];
    return read
      ? { ...resolution, policy: { ...appliedPolicy, parameters: read } }
      : resolution;
  }, [appliedPolicy, declaredParameters, resolution]);

  const requiresApiKey = Boolean(
    detail?.security?.enabled && detail.security.apiKey?.enabled
  );
  const apiKeyHeader = resolveApiKeyAuthDisplay(
    detail?.security,
    detail?.globalPolicies
  ).headerName;
  /** Whether this provider already authenticates with something. */
  const hasStoredCredential = Boolean(
    !isAdding && entry?.id === providerId && entry?.auth?.type === 'api-key'
  );

  const openKeyNaming = () => {
    setGenerateError(null);
    setMintedKeyForDisplay(null);
    setKeyDisplayName('');
    setIsNamingKey(true);
  };

  const closeKeyNaming = () => {
    if (isGenerating) {
      return;
    }
    setIsNamingKey(false);
    setKeyDisplayName('');
    setGenerateError(null);
    setMintedKeyForDisplay(null);
  };

  const handleGenerate = async () => {
    if (!organizationId || !providerId) {
      return;
    }
    // Named by whoever mints it, as on the create page. A name chosen here is
    // what the key is listed under afterwards, so it is not one to invent.
    const displayName = keyDisplayName.trim();
    if (!displayName) {
      setGenerateError('Display name is required.');
      return;
    }
    try {
      setIsGenerating(true);
      setGenerateError(null);
      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);
      const response = await createLLMProviderAPIKey(
        providerId,
        organizationId,
        {
          id: buildApiKeyResourceName(displayName),
          displayName,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        },
        PLATFORM_API_BASE_URL
      );
      setMintedKeyForDisplay(response.apiKey);
      setGeneratedApiKeyValue(response.apiKey);
    } catch (error) {
      logger.error('Failed to generate provider API key:', error);
      setGenerateError(
        getErrorMessage(error, 'Failed to generate API key. Please try again.')
      );
    } finally {
      setIsGenerating(false);
    }
  };

  const handleCopyMintedKey = async () => {
    if (!mintedKeyForDisplay) {
      return;
    }
    try {
      await navigator.clipboard.writeText(mintedKeyForDisplay);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = mintedKeyForDisplay;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };

  const handleSave = () => {
    if (!providerId) {
      return;
    }
    const typedKey = apiKeyValue.trim() || generatedApiKeyValue;
    // An untouched credential is carried back exactly as it was read. Rebuilding
    // it would replace a stored secret reference with the redacted value that
    // came back in its place, which authenticates as nothing.
    const carriedAuth =
      entry?.id === providerId ? entry?.auth : undefined;
    const auth = typedKey
      ? { type: 'api-key', header: apiKeyHeader, value: typedKey }
      : carriedAuth;
    // The translator a match found is recorded here, where a user can see it
    // land and take it off again — a gateway attaches one only where the proxy
    // names it, so a match shown and never written is a provider that silently
    // does not translate. One taken off deliberately is left off.
    const nextTransformer = wasCleared
      ? null
      : resolvedTransformerFor(transformer, resolution);
    onSave(
      {
        // Built field by field rather than spread from the entry being edited.
        // A credential belongs to the provider it was entered for, and a spread
        // carries the old one through untouched whenever nothing new is typed —
        // so swapping the provider behind an attachment would send one vendor's
        // secret to another's upstream.
        id: providerId,
        isPrimary: entry?.isPrimary ?? false,
        // The alias is kept across that swap, deliberately: it is the name
        // clients put in the routing header, and changing the provider behind a
        // name without changing the name is what an alias is for. Dropping it
        // would silently move the attachment to a different handle.
        ...(entry?.alias ? { alias: entry.alias } : {}),
        ...(auth ? { auth } : {}),
        // Cleared explicitly only when there was one to clear. Writing an
        // empty translator onto an entry that never had one would make an
        // untouched provider read as edited.
        ...(nextTransformer
          ? { transformer: nextTransformer }
          : entry?.transformer
            ? { transformer: null }
            : {}),
      } as ProxyProviderEntry,
      detail
    );
    onClose();
  };

  return (
    <>
      <Drawer anchor="right" open={open} onClose={onClose}>
        <Box
          sx={{
            width: { xs: '100vw', sm: 450, md: 520 },
            maxWidth: '100vw',
            display: 'flex',
            flexDirection: 'column',
            height: '100%',
          }}
          data-cyid="provider-settings-drawer"
        >
          <Box sx={{ p: 2 }}>
            <Box
              display="flex"
              alignItems="flex-start"
              justifyContent="space-between"
            >
              <Stack spacing={0.5}>
                <Typography variant="h6">
                  <FormattedMessage
                    id="aiWorkspace.components.providerSettings.title"
                    defaultMessage="Provider settings"
                  />
                </Typography>
                {/*
                  Stated, not editable here: the interface governs every
                  provider, so it belongs with the proxy rather than inside one
                  provider's settings.
                */}
                <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.components.providerSettings.interface"
                    defaultMessage="Inbound interface: {interfaceName}"
                    values={{ interfaceName: interfaceLabel || '—' }}
                  />
                </Typography>
              </Stack>
              <Button
                size="small"
                onClick={onClose}
                sx={{ minWidth: 0 }}
                data-cyid="provider-settings-close"
              >
                <X size={18} />
              </Button>
            </Box>
          </Box>

          <Divider />

          <Box sx={{ p: 2, flex: 1, overflowY: 'auto' }}>
            <Stack spacing={2}>
              <FormControl fullWidth>
                <FormLabel sx={{ mb: 0.5 }}>
                  <FormattedMessage
                    id="aiWorkspace.components.providerSettings.provider"
                    defaultMessage="LLM Provider"
                  />
                  <Typography component="span" color="error.main" sx={{ ml: 0.5 }}>
                    *
                  </Typography>
                </FormLabel>
                <Select
                  value={providerId}
                  displayEmpty
                  disabled={disabled}
                  onChange={(event) => {
                    // A credential belongs to the provider it was entered for,
                    // so it does not survive a change of provider.
                    setProviderId(String(event.target.value));
                    setApiKeyValue('');
                    setGeneratedApiKeyValue('');
                    // A different provider needs a different translator, so
                    // neither the old one nor a decision to go without it
                    // carries over.
                    setTransformer(null);
                    setWasCleared(false);
                  }}
                  data-cyid="provider-settings-provider"
                >
                  {selectableProviders.length === 0 ? (
                    <MenuItem value="" disabled>
                      <FormattedMessage
                        id="aiWorkspace.components.providerSettings.noProviders"
                        defaultMessage="No providers available"
                      />
                    </MenuItem>
                  ) : (
                    selectableProviders.map((provider) => (
                      <MenuItem key={provider.id} value={provider.id}>
                        {provider.displayName}
                      </MenuItem>
                    ))
                  )}
                </Select>
              </FormControl>

              <TransformerStatusCard
                resolution={resolutionWithParameters}
                transformer={transformer}
                onConfigure={
                  disabled ? undefined : () => setIsPickingTransformer(true)
                }
                onRemove={
                  disabled
                    ? undefined
                    : () => {
                        setTransformer(null);
                        setWasCleared(true);
                      }
                }
                disabled={disabled}
                data-cyid="provider-settings-transformer"
              />

              {requiresApiKey && (
                <>
                  <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                    <FormattedMessage
                      id="aiWorkspace.components.providerSettings.apiKey"
                      defaultMessage="API Key"
                    />
                  </Typography>

                  <FormControl fullWidth>
                    <FormLabel sx={{ mb: 0.5 }}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.components.providerSettings.apiKeyLabel"
                          defaultMessage="API Key"
                        />
                      </Typography>
                    </FormLabel>
                    <TextField
                      fullWidth
                      size="small"
                      type="password"
                      value={apiKeyValue}
                      disabled={disabled}
                      // A stored key never comes back from the server, so the
                      // field cannot show it. It says a key is there and takes
                      // a replacement, rather than looking empty and unset.
                      placeholder={
                        hasStoredCredential
                          ? intl.formatMessage({
                              id: 'aiWorkspace.components.providerSettings.apiKeyStored',
                              defaultMessage:
                                'A key is stored - enter one to replace it',
                            })
                          : intl.formatMessage({
                              id: 'aiWorkspace.components.providerSettings.apiKeyPlaceholder',
                              defaultMessage: 'Enter API key',
                            })
                      }
                      onChange={(event) => setApiKeyValue(event.target.value)}
                      data-cyid="provider-settings-api-key"
                    />
                  </FormControl>

                  <Divider>
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.components.providerSettings.or"
                        defaultMessage="or"
                      />
                    </Typography>
                  </Divider>

                  <Stack
                    direction={{ xs: 'column', sm: 'row' }}
                    spacing={2}
                    alignItems={{ xs: 'flex-start', sm: 'center' }}
                    sx={{
                      px: 1.5,
                      py: 1.5,
                      border: '1px solid',
                      borderColor: generatedApiKeyValue
                        ? 'success.main'
                        : 'divider',
                      borderRadius: 1,
                    }}
                  >
                    <Box sx={{ flex: 1 }}>
                      {generatedApiKeyValue ? (
                        <Alert severity="success">
                          <FormattedMessage
                            id="aiWorkspace.components.providerSettings.keyPending"
                            defaultMessage="The new key is applied when this proxy is saved."
                          />
                        </Alert>
                      ) : (
                        <Typography variant="body2" color="text.secondary">
                          {hasStoredCredential ? (
                            <FormattedMessage
                              id="aiWorkspace.components.providerSettings.replaceWithGenerated"
                              defaultMessage="Replace it with a newly generated key."
                            />
                          ) : (
                            <FormattedMessage
                              id="aiWorkspace.components.providerSettings.generateDescription"
                              defaultMessage="Generate an API key for the selected LLM provider."
                            />
                          )}
                        </Typography>
                      )}
                    </Box>
                    <Button
                      variant="contained"
                      onClick={openKeyNaming}
                      disabled={
                        disabled ||
                        !canGenerateApiKey ||
                        isDetailLoading ||
                        isGenerating
                      }
                      data-cyid="provider-settings-generate"
                    >
                      <FormattedMessage
                        id="aiWorkspace.components.providerSettings.generate"
                        defaultMessage="Generate API Key"
                      />
                    </Button>
                  </Stack>

                  {apiKeyValue.trim() && (
                    <Typography variant="caption" color="success.main">
                      <FormattedMessage
                        id="aiWorkspace.components.providerSettings.typedKeyPending"
                        defaultMessage="The key you entered is applied when this proxy is saved."
                      />
                    </Typography>
                  )}
                  {!isNamingKey && generateError && (
                    <Typography variant="caption" color="error.main">
                      {generateError}
                    </Typography>
                  )}
                </>
              )}
            </Stack>
          </Box>

          <Divider />
          <Box sx={{ p: 2, display: 'flex', justifyContent: 'flex-end', gap: 1 }}>
            <Button
              variant="outlined"
              color="secondary"
              onClick={onClose}
              data-cyid="provider-settings-cancel"
            >
              <FormattedMessage
                id="aiWorkspace.components.providerSettings.cancel"
                defaultMessage="Cancel"
              />
            </Button>
            <Button
              variant="contained"
              // Not while the provider is still being read: everything this
              // saves about it is unknown until then.
              disabled={disabled || !providerId || isDetailLoading}
              onClick={handleSave}
              data-cyid="provider-settings-save"
            >
              <FormattedMessage
                id="aiWorkspace.components.providerSettings.save"
                defaultMessage="Save changes"
              />
            </Button>
          </Box>
        </Box>
      </Drawer>

      {/*
        Minting a key, named by whoever mints it. The key it produces is shown
        once and never again — the server does not return it a second time — so
        it is presented with the header it goes in and a way to copy it.
      */}
      <Dialog
        open={isNamingKey}
        onClose={closeKeyNaming}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          {mintedKeyForDisplay
            ? 'API Key Generated Successfully'
            : 'Generate API Key'}
        </DialogTitle>
        <DialogContent>
          {generateError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {generateError}
            </Alert>
          )}
          {mintedKeyForDisplay ? (
            <Alert
              severity="warning"
              sx={{ '& .MuiAlert-message': { width: '100%' } }}
            >
              <Stack spacing={1}>
                <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.components.providerSettings.copyKeyWarning"
                    defaultMessage="Please copy and save this API key. For security reasons, you won't be able to see it again."
                  />
                </Typography>
                <Box display="flex" alignItems="center" gap={0.5}>
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
                    value={apiKeyHeader}
                    slotProps={{ input: { readOnly: true } }}
                  />
                </Box>
                <Box display="flex" alignItems="center" gap={0.5}>
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
                      {mintedKeyForDisplay}
                    </Box>
                    <IconButton
                      size="small"
                      onClick={() => void handleCopyMintedKey()}
                      sx={{ flexShrink: 0 }}
                    >
                      <Copy size={16} />
                    </IconButton>
                  </Box>
                </Box>
              </Stack>
            </Alert>
          ) : (
            <FormControl fullWidth>
              <FormLabel>Key Name</FormLabel>
              <TextField
                autoFocus
                size="small"
                fullWidth
                value={keyDisplayName}
                onChange={(event) => setKeyDisplayName(event.target.value)}
                placeholder="Ex: Production Key"
                data-cyid="provider-settings-key-name"
              />
            </FormControl>
          )}
        </DialogContent>
        <DialogActions>
          {mintedKeyForDisplay ? (
            <Button onClick={closeKeyNaming} variant="outlined" size="small">
              Done
            </Button>
          ) : (
            <>
              <Button
                onClick={closeKeyNaming}
                variant="outlined"
                color="secondary"
                size="small"
                disabled={isGenerating}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                size="small"
                onClick={() => void handleGenerate()}
                disabled={isGenerating || !keyDisplayName.trim()}
                data-cyid="provider-settings-key-generate"
              >
                {isGenerating ? (
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

      {/* The translator picker, opened from the status card above. */}
      <TransformerDrawer
        open={isPickingTransformer}
        onClose={() => setIsPickingTransformer(false)}
        policies={policies}
        isLoading={!policiesLoaded}
        current={transformer}
        onApply={(chosen) => {
          setTransformer(chosen);
          setWasCleared(false);
        }}
      />
    </>
  );
}
