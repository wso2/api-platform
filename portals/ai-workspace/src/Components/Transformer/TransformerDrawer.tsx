/**
 * Picks the translator that applies to one provider.
 *
 * Two stages in one panel: a list of what is available, then the chosen
 * policy's own parameters. It opens on the list, or straight on the
 * configuration of a policy already attached, so re-editing one does not make
 * the user find it again.
 *
 * The list draws from the shared catalogue and from the policies a gateway
 * carries of its own, and both filters stay editable — a pre-filter that cannot
 * be cleared would make anything outside it unreachable.
 */

import React, { useEffect, useMemo, useState } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Divider,
  Drawer,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronLeft,
  ExternalLink,
  Search,
  X,
} from '@wso2/oxygen-ui-icons-react';
import PolicyCategorySelector from '../GuardrailPill/PolicyCategorySelector';
import {
  DEFAULT_TRANSFORMER_SEARCH,
  FALLBACK_POLICY_CATEGORY,
} from '../../hooks/useTransformerPolicies';
import { getPolicyDefinition } from '../../apis/policyHubApis';
import {
  PolicyParameterEditor,
  parsePolicyYaml,
} from '../../pages/appShell/PolicyParameterEditor';
import type {
  ParameterSchema,
  ParameterValues,
  PolicyDefinition,
} from '../../pages/appShell/PolicyParameterEditor';
import { transformerFromPolicy } from '../../utils/proxyProviders';
import { logger } from '../../utils/logger';
import type {
  ProxyProviderTransformer,
  SelectablePolicy,
} from '../../utils/types';

export type TransformerDrawerProps = {
  open: boolean;
  onClose: () => void;
  policies: SelectablePolicy[];
  isLoading?: boolean;
  /** What is attached now, so re-opening lands on it with its saved values. */
  current?: ProxyProviderTransformer | null;
  onApply: (transformer: ProxyProviderTransformer) => void;
  /** Where the catalogue can be browsed in full. */
  policyHubUrl?: string;
};

/** An empty schema, so a policy declaring no parameters still renders a form. */
const EMPTY_PARAMETERS: ParameterSchema = { type: 'object', properties: {} };

/**
 * A gateway's own policies arrive with their declarations already attached, so
 * their form is built from what is in hand rather than from a second request.
 */
const definitionFromInline = (policy: SelectablePolicy): PolicyDefinition => {
  const inline = (policy.definition ?? {}) as {
    description?: string;
    parameters?: ParameterSchema;
    systemParameters?: ParameterSchema;
  };
  return {
    name: policy.name,
    version: policy.version,
    description: policy.description || inline.description || '',
    parameters: inline.parameters ?? EMPTY_PARAMETERS,
    systemParameters: inline.systemParameters,
  };
};

export default function TransformerDrawer({
  open,
  onClose,
  policies,
  isLoading = false,
  current,
  onApply,
  policyHubUrl,
}: TransformerDrawerProps) {
  const intl = useIntl();
  // Opens on the category the published catalogue actually carries today. The
  // dedicated one is offered in the selector and starts matching the moment the
  // catalogue is re-published with it.
  const [categories, setCategories] = useState<string[]>([
    FALLBACK_POLICY_CATEGORY,
  ]);
  const [search, setSearch] = useState(DEFAULT_TRANSFORMER_SEARCH);
  const [selected, setSelected] = useState<SelectablePolicy | null>(null);
  // The chosen policy's own parameter declarations. The form is rendered from
  // these rather than from a list held here, so a translator that gains a
  // parameter becomes configurable without a change to this screen.
  const [definition, setDefinition] = useState<PolicyDefinition | null>(null);
  const [definitionLoading, setDefinitionLoading] = useState(false);
  const [definitionError, setDefinitionError] = useState<string | null>(null);

  // Re-opening for a provider that already has a policy lands on that policy's
  // configuration, carrying what was saved, rather than on the list again.
  useEffect(() => {
    if (!open) {
      return;
    }
    const attached = current?.type
      ? policies.find((policy) => policy.name === current.type)
      : undefined;
    setSelected(attached ?? null);
  }, [open, current, policies]);

  // Declarations are fetched for the chosen policy alone: the list can hold
  // dozens, and only one of them is ever being configured.
  useEffect(() => {
    if (!selected) {
      setDefinition(null);
      setDefinitionError(null);
      setDefinitionLoading(false);
      return undefined;
    }
    if (selected.definition) {
      setDefinition(definitionFromInline(selected));
      setDefinitionError(null);
      // An earlier fetch may still be in flight and will never clear this
      // itself, having been abandoned. Left set, the spinner outlives the
      // policy that started it and the form never appears.
      setDefinitionLoading(false);
      return undefined;
    }

    let abandoned = false;
    setDefinition(null);
    setDefinitionError(null);
    setDefinitionLoading(true);
    getPolicyDefinition(selected.name, selected.version)
      .then((response) => {
        if (!abandoned) {
          setDefinition(parsePolicyYaml(response));
        }
      })
      .catch((error) => {
        if (abandoned) {
          return;
        }
        logger.error('Failed to load transformer definition:', error);
        setDefinitionError(
          'Failed to load this transformer\u2019s configuration.'
        );
      })
      .finally(() => {
        if (!abandoned) {
          setDefinitionLoading(false);
        }
      });
    return () => {
      abandoned = true;
    };
  }, [selected]);

  const visiblePolicies = useMemo(() => {
    const term = search.trim().toLowerCase();
    return policies.filter((policy) => {
      // Gateway policies are not in the catalogue to be categorised, so a
      // category filter must not hide them — it would make a policy that
      // exists unreachable from the screen that attaches it.
      const matchesCategory =
        categories.length === 0 ||
        policy.source === 'custom' ||
        (policy.categories ?? []).some((category) =>
          categories.includes(category)
        ) ||
        // A catalogue that has not yet been re-published with the transformer
        // label would otherwise hide every policy behind a filter the user did
        // not set. Falling back to the name keeps them reachable.
        policy.name.includes('-to-');
      const matchesSearch =
        term === '' ||
        policy.displayName.toLowerCase().includes(term) ||
        policy.name.toLowerCase().includes(term);
      return matchesCategory && matchesSearch;
    });
  }, [policies, categories, search]);

  /** What was saved for this policy, so re-opening it shows what is in effect. */
  const existingValues = useMemo(
    () =>
      selected && current?.type === selected.name
        ? ((current?.params as ParameterValues) ?? {})
        : undefined,
    [current, selected]
  );

  const applySelection = (values: ParameterValues) => {
    if (!selected) {
      return;
    }
    onApply(transformerFromPolicy(selected, values));
    onClose();
  };

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box
        sx={{
          width: { xs: '100vw', sm: 450, md: 600 },
          maxWidth: '100vw',
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
        }}
        data-cyid="transformer-drawer"
      >
        <Box sx={{ p: 2 }}>
          <Box
            display="flex"
            alignItems="flex-start"
            justifyContent="space-between"
          >
            <Stack spacing={0.5}>
              {selected && (
                <Button
                  size="small"
                  startIcon={<ChevronLeft size={16} />}
                  onClick={() => setSelected(null)}
                  sx={{ alignSelf: 'flex-start', px: 0 }}
                  data-cyid="transformer-drawer-back"
                >
                  <FormattedMessage
                    id="aiWorkspace.components.transformerDrawer.allTransformers"
                    defaultMessage="All transformers"
                  />
                </Button>
              )}
              <Typography variant="h6">
                {selected ? (
                  selected.displayName
                ) : (
                  <FormattedMessage
                    id="aiWorkspace.components.transformerDrawer.title"
                    defaultMessage="Transformers"
                  />
                )}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {selected ? (
                  selected.version
                ) : (
                  <FormattedMessage
                    id="aiWorkspace.components.transformerDrawer.subtitle"
                    defaultMessage="Choose a transformer policy to configure its parameters."
                  />
                )}
              </Typography>
            </Stack>
            <Button
              size="small"
              onClick={onClose}
              sx={{ minWidth: 0 }}
              data-cyid="transformer-drawer-close"
            >
              <X size={18} />
            </Button>
          </Box>
        </Box>

        <Divider />

        <Box sx={{ p: 2, flex: 1, overflowY: 'auto' }}>
          {!selected ? (
            <Stack spacing={2}>
              <Box display="flex" alignItems="center" justifyContent="flex-end">
                {policyHubUrl && (
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<ExternalLink size={14} />}
                    component="a"
                    href={policyHubUrl}
                    target="_blank"
                    rel="noreferrer"
                    data-cyid="transformer-drawer-policy-hub"
                  >
                    <FormattedMessage
                      id="aiWorkspace.components.transformerDrawer.policyHub"
                      defaultMessage="Policy Hub"
                    />
                  </Button>
                )}
              </Box>

              <PolicyCategorySelector
                value={categories}
                onChange={setCategories}
              />

              <TextField
                fullWidth
                size="small"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.components.transformerDrawer.search',
                  defaultMessage: 'Search transformers',
                })}
                slotProps={{
                  input: {
                    startAdornment: (
                      <Search size={16} style={{ marginRight: 8 }} />
                    ),
                  },
                }}
                data-cyid="transformer-drawer-search"
              />

              {isLoading ? (
                <Box display="flex" justifyContent="center" sx={{ py: 4 }}>
                  <CircularProgress size={24} />
                </Box>
              ) : visiblePolicies.length === 0 ? (
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ py: 2 }}
                >
                  <FormattedMessage
                    id="aiWorkspace.components.transformerDrawer.empty"
                    defaultMessage="No transformer policies match these filters."
                  />
                </Typography>
              ) : (
                <Stack spacing={1}>
                  {visiblePolicies.map((policy) => (
                    <Card
                      key={`${policy.source}-${policy.name}`}
                      variant="outlined"
                      sx={{ p: 1.5, cursor: 'pointer' }}
                      onClick={() => setSelected(policy)}
                      data-cyid={`transformer-option-${policy.name}`}
                    >
                      <Box
                        display="flex"
                        alignItems="center"
                        justifyContent="space-between"
                        gap={1}
                      >
                        <Stack spacing={0.25}>
                          <Typography variant="body2" sx={{ fontWeight: 600 }}>
                            {policy.displayName}
                          </Typography>
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            sx={{
                              fontFamily:
                                'ui-monospace, SFMono-Regular, Menlo, monospace',
                            }}
                          >
                            {policy.name}
                          </Typography>
                        </Stack>
                        <Box display="flex" alignItems="center" gap={0.5}>
                          <Chip
                            size="small"
                            variant="outlined"
                            label={
                              policy.source === 'custom' ? 'Custom' : 'WSO2'
                            }
                          />
                          <Chip
                            size="small"
                            variant="outlined"
                            label={policy.version}
                          />
                        </Box>
                      </Box>
                    </Card>
                  ))}
                </Stack>
              )}
            </Stack>
          ) : definitionLoading ? (
            <Box display="flex" justifyContent="center" sx={{ py: 4 }}>
              <CircularProgress size={24} />
            </Box>
          ) : definitionError ? (
            <Alert severity="error">{definitionError}</Alert>
          ) : definition ? (
            <Card variant="outlined" sx={{ p: 2 }}>
              <PolicyParameterEditor
                policyDefinition={definition}
                policyDisplayName={selected.displayName}
                existingValues={existingValues}
                onCancel={onClose}
                onSubmit={applySelection}
              />
            </Card>
          ) : null}
        </Box>
      </Box>
    </Drawer>
  );
}
