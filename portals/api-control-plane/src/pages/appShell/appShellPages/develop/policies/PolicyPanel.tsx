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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  CardContent,
  Chip,
  Divider,
  InputAdornment,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Globe, Search } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useIsPolicyHubConfigured, type PolicySummary } from '@/api/resources/policyHub';
import { useUpdateRestApi, type Policy, type RestApi } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { AttachedPolicyList } from './AttachedPolicyList';
import { AvailablePoliciesPanel } from './AvailablePoliciesPanel';
import { SaveBar } from '../SaveBar';
import { useDirtyTracking } from '../useDirtyTracking';
import {
  type EditableOperation,
  methodColor,
  reorderPolicies,
  toEditableOperations,
  withPolicyEdits,
} from '../../apis/utils/developEdit';
import { getDraggedPolicy, type PolicyScope, scopeId } from './policyDnd';
import { PolicyConfigDrawer, type PolicyRef } from './PolicyConfigDrawer';

const messages = defineMessages({
  heading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.heading',
    defaultMessage: 'Policies',
    description: 'Heading over the panel where policies are attached. Noun.',
  },
  hint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.hint',
    defaultMessage: 'Drag policies from the right onto the API or a resource.',
    description: 'Instruction shown when a Policy Hub is configured.',
  },
  hubUnavailable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.hubUnavailable',
    defaultMessage: 'Policy Hub is not configured; policies cannot be added.',
    description: 'Shown instead of the instruction when no Policy Hub URL is set.',
  },
  globalPolicies: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.globalPolicies',
    defaultMessage: 'Global policies (API level)',
    description: 'Section for policies applying to every resource of the API.',
  },
  globalEmpty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.globalEmpty',
    defaultMessage: 'Drag and drop policies here to apply across all resources',
  },
  globalDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.globalDescription',
    defaultMessage: 'Policies applied here will affect all API resources',
  },
  resources: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.resources',
    defaultMessage: 'Resource-wise policies',
    description: "Section listing the API's operations, each with its own policies. Noun.",
  },
  resourcesDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.resourcesDescription',
    defaultMessage: 'Attach policies only to the specific resources',
  },
  searchResources: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.searchResources',
    defaultMessage: 'Search resources',
  },
  allMethods: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.allMethods',
    defaultMessage: 'All methods',
  },
  noResources: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.noResources',
    defaultMessage: 'No resources. Add them in the Routing tab.',
    description: '"Routing" is the name of a sibling page in this console.',
  },
  noMatchingResources: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.noMatchingResources',
    defaultMessage: 'No resources match the filters.',
  },
  resourceEmpty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.resourceEmpty',
    defaultMessage: 'Drag and drop policies here, or use Add Policy.',
    description: '"Add Policy" is the label of a button in this same list.',
  },
  policyUpdated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.policyUpdated',
    defaultMessage: 'Policy updated. Save to apply.',
    description: 'Toast after editing an attached policy; the change is not persisted until Save.',
  },
  policyAttached: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.policyAttached',
    defaultMessage: 'Policy attached. Save to apply.',
    description: 'Toast after attaching a policy; the change is not persisted until Save.',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyPanel.saved',
    defaultMessage: 'Policies saved.',
  },
});

type EditState = { scope: PolicyScope; policyIndex: number; policy: Policy } | null;

/** A left-panel drop target that highlights while a catalog policy hovers it. */
function DropZone({
  active,
  onEnter,
  onLeave,
  onDrop,
  children,
}: {
  active: boolean;
  onEnter: () => void;
  onLeave: () => void;
  onDrop: () => void;
  children: React.ReactNode;
}) {
  return (
    <Box
      onDragLeave={onLeave}
      onDragOver={(event) => {
        if (!getDraggedPolicy()) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = 'copy';
        onEnter();
      }}
      onDrop={(event) => {
        event.preventDefault();
        onDrop();
      }}
      sx={{
        border: '2px dashed',
        borderColor: active ? 'primary.main' : 'transparent',
        borderRadius: 1.5,
        outline: active ? 'none' : undefined,
        p: active ? 0.5 : 0,
        transition: 'border-color .12s',
      }}
    >
      {children}
    </Box>
  );
}

export function PolicyPanel({ api }: { api: RestApi }) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const update = useUpdateRestApi();
  const hubEnabled = useIsPolicyHubConfigured();
  const restApiId = api.id;

  const [apiPolicies, setApiPolicies] = useState<Policy[]>(api.policies ?? []);
  // Flattened once, lazily: from here the panel owns the edits, so re-deriving
  // from `api` on a later render would silently discard them.
  const [operations, setOperations] = useState<EditableOperation[]>(() =>
    toEditableOperations(api),
  );

  // Add/config flow targeting a scope.
  const [scope, setScope] = useState<PolicyScope | null>(null);
  const [picked, setPicked] = useState<PolicySummary | null>(null);
  const [editing, setEditing] = useState<EditState>(null);
  const [activeZone, setActiveZone] = useState<string | null>(null);
  const [resourceSearch, setResourceSearch] = useState('');
  const [methodFilter, setMethodFilter] = useState('all');

  // Only the fields `save` actually submits count towards "unsaved" — not the
  // resource search/filter or which config drawer is open.
  const { dirty, markSaved } = useDirtyTracking({ apiPolicies, operations });

  const methods = useMemo(
    () => [...new Set(operations.map((operation) => operation.method))].sort(),
    [operations],
  );
  const visibleOperations = operations
    .map((operation, index) => ({ operation, index }))
    .filter(({ operation }) => {
      const matchesMethod = methodFilter === 'all' || operation.method === methodFilter;
      const term = resourceSearch.trim().toLowerCase();
      return matchesMethod && (term === '' || operation.path.toLowerCase().includes(term));
    });

  const policiesFor = (s: PolicyScope): Policy[] =>
    s.kind === 'api' ? apiPolicies : operations[s.index].policies || [];

  const setPoliciesFor = (s: PolicyScope, next: Policy[]) => {
    if (s.kind === 'api') setApiPolicies(next);
    else
      setOperations((ops) => ops.map((op, i) => (i === s.index ? { ...op, policies: next } : op)));
  };

  // Drop a catalog policy onto a scope → open config for that scope.
  const dropOnScope = (s: PolicyScope) => {
    const dragged = getDraggedPolicy();
    setActiveZone(null);
    if (!dragged) return;
    setEditing(null);
    setScope(s);
    setPicked({
      name: dragged.name,
      version: dragged.version,
      displayName: dragged.displayName,
      provider: '',
      categories: [],
      tags: [],
      isLatest: true,
    });
  };

  // Click in the catalog (non-DnD fallback) attaches to whichever scope the user
  // last targeted via its "Add Policy" button, defaulting to API level.
  const pickFromCatalog = (policy: PolicySummary) => {
    setEditing(null);
    setScope((current) => current ?? { kind: 'api' });
    setPicked(policy);
  };

  const openEdit = (s: PolicyScope, index: number) => {
    setPicked(null);
    setScope(s);
    setEditing({ scope: s, policyIndex: index, policy: policiesFor(s)[index] });
  };

  const confirmPolicy = (policy: Policy) => {
    if (!scope) return;
    if (editing) {
      setPoliciesFor(
        scope,
        policiesFor(scope).map((p, i) => (i === editing.policyIndex ? policy : p)),
      );
    } else {
      setPoliciesFor(scope, [...policiesFor(scope), policy]);
    }
    notify(
      intl.formatMessage(editing ? messages.policyUpdated : messages.policyAttached),
      'success',
    );
    closeFlow();
  };

  const closeFlow = () => {
    setPicked(null);
    setEditing(null);
    setScope(null);
  };

  const removeAt = (s: PolicyScope, i: number) =>
    setPoliciesFor(
      s,
      policiesFor(s).filter((_p, idx) => idx !== i),
    );
  const reorderAt = (s: PolicyScope, from: number, to: number) =>
    setPoliciesFor(s, reorderPolicies(policiesFor(s), from, to));

  const cancel = () => {
    setApiPolicies(api.policies ?? []);
    setOperations(toEditableOperations(api));
    closeFlow();
  };

  const save = () => {
    // The API was fetched by handle, so `id` is present; the guard exists
    // because the spec marks it optional and a PUT to `/rest-apis/` would
    // otherwise be issued against the collection.
    if (!restApiId) return;
    update.mutate(
      { restApiId, body: withPolicyEdits(api, { policies: apiPolicies, operations }) },
      // No `onError`: the query client's `onMutationError` already notifies, and
      // a local handler would replace the optimistic rollback in `useUpdateRestApi`.
      {
        onSuccess: () => {
          markSaved();
          notify(intl.formatMessage(messages.saved), 'success');
        },
      },
    );
  };

  const configRef: PolicyRef | null = editing
    ? {
        name: editing.policy.name,
        version: editing.policy.version,
        displayName: editing.policy.name,
      }
    : picked;

  return (
    <Stack spacing={2}>
      <Stack alignItems="flex-start" direction={{ xs: 'column', md: 'row' }} spacing={2}>
        {/* LEFT: Policies (drop targets) */}
        <Box sx={{ flex: 1, minWidth: 0, width: '100%' }}>
          <Card sx={{ height: '100%', overflow: 'hidden' }} variant="outlined">
            <Box sx={{ px: 2, py: 1.5 }}>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                <FormattedMessage {...messages.heading} />
              </Typography>
            </Box>
            <Divider />
            <CardContent sx={{ px: { lg: 5, xs: 2 }, py: 2 }}>
              {/* Global / API-level */}
              <Card
                sx={{
                  borderRadius: 2,
                  mb: 3,
                  p: 2.5,
                }}
              >
                <Stack alignItems="center" direction="row" spacing={1.25}>
                  <Box sx={{ bgcolor: 'background.paper', borderRadius: 1, display: 'flex', p: 1 }}>
                    <Globe size={18} />
                  </Box>
                  <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
                    <FormattedMessage {...messages.globalPolicies} />
                  </Typography>
                </Stack>
                <Typography color="text.secondary" sx={{ mb: 2, mt: 0.75 }} variant="body2">
                  <FormattedMessage {...messages.globalDescription} />
                </Typography>
                <DropZone
                  active={activeZone === scopeId({ kind: 'api' })}

                  onDrop={() => dropOnScope({ kind: 'api' })}
                  onEnter={() => setActiveZone(scopeId({ kind: 'api' }))}
                  onLeave={() => setActiveZone(null)}
                >
                  <AttachedPolicyList
                    canAdd={hubEnabled}
                    emptyText={intl.formatMessage(messages.globalEmpty)}
                    onAdd={() => {
                      setEditing(null);
                      setScope({ kind: 'api' });
                      setPicked(null);
                    }}
                    onEdit={(i) => openEdit({ kind: 'api' }, i)}
                    onReorder={(from, to) => reorderAt({ kind: 'api' }, from, to)}
                    onRemove={(i) => removeAt({ kind: 'api' }, i)}
                    policies={apiPolicies}
                    showHeader={false}
                  />
                </DropZone>
              </Card>

              <Card sx={{ borderRadius: 2, overflow: 'hidden' }}>
                <Accordion
                  disableGutters
                  elevation={0}
                  sx={{ bgcolor: 'transparent', '&:before': { display: 'none' } }}
                >
                  <AccordionSummary
                    expandIcon={<ChevronDown size={18} />}
                    sx={{ px: 2.5, py: 1.25 }}
                  >
                    <Box>
                      <Typography sx={{ fontWeight: 700 }}>
                        <FormattedMessage {...messages.resources} />
                      </Typography>
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage {...messages.resourcesDescription} />
                      </Typography>
                    </Box>
                  </AccordionSummary>
                  <AccordionDetails sx={{ px: 2.5, pb: 2.5 }}>
                    <Stack direction={{ sm: 'row', xs: 'column' }} spacing={1.5} sx={{ mb: 2 }}>
                      <TextField
                        fullWidth
                        onChange={(event) => setResourceSearch(event.target.value)}
                        placeholder={intl.formatMessage(messages.searchResources)}
                        size="small"
                        slotProps={{
                          input: {
                            endAdornment: (
                              <InputAdornment position="end">
                                <Search size={18} />
                              </InputAdornment>
                            ),
                          },
                        }}
                        value={resourceSearch}
                      />
                      <TextField
                        onChange={(event) => setMethodFilter(event.target.value)}
                        select
                        size="small"
                        sx={{ minWidth: 180 }}
                        value={methodFilter}
                      >
                        <MenuItem value="all">
                          <FormattedMessage {...messages.allMethods} />
                        </MenuItem>
                        {methods.map((method) => (
                          <MenuItem key={method} value={method}>
                            {method}
                          </MenuItem>
                        ))}
                      </TextField>
                    </Stack>
                    {visibleOperations.length === 0 ? (
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage
                          {...(operations.length === 0
                            ? messages.noResources
                            : messages.noMatchingResources)}
                        />
                      </Typography>
                    ) : (
                      <Stack spacing={1}>
                        {visibleOperations.map(({ operation: op, index }) => {
                          const zid = scopeId({ kind: 'operation', index });
                          // The "/" fallback is a URL path, not prose — kept out of
                          // JSX so it is never mistaken for translatable text.
                          const displayPath = op.path || '/';
                          return (
                            <Accordion
                              disableGutters
                              key={index}
                              sx={{ '&:before': { display: 'none' } }}
                              variant="outlined"
                            >
                              <AccordionSummary expandIcon={<ChevronDown size={18} />}>
                                <Stack
                                  alignItems="center"
                                  direction="row"
                                  spacing={1.5}
                                  sx={{ minWidth: 0 }}
                                >
                                  <Chip
                                    color={methodColor(op.method)}
                                    label={op.method}
                                    size="small"
                                    sx={{ fontWeight: 700, minWidth: 58 }}
                                  />
                                  <Typography noWrap sx={{ fontFamily: 'monospace' }}>
                                    {displayPath}
                                  </Typography>
                                  {(op.policies?.length || 0) > 0 && (
                                    <Chip
                                      label={op.policies!.length}
                                      size="small"
                                      variant="outlined"
                                    />
                                  )}
                                </Stack>
                              </AccordionSummary>
                              <AccordionDetails>
                                <DropZone
                                  active={activeZone === zid}
                                  onDrop={() => dropOnScope({ kind: 'operation', index })}
                                  onEnter={() => setActiveZone(zid)}
                                  onLeave={() => setActiveZone(null)}
                                >
                                  <AttachedPolicyList
                                    canAdd={hubEnabled}
                                    emptyText={intl.formatMessage(messages.resourceEmpty)}
                                    onAdd={() => {
                                      setEditing(null);
                                      setScope({ kind: 'operation', index });
                                      setPicked(null);
                                    }}
                                    onEdit={(i) => openEdit({ kind: 'operation', index }, i)}
                                    onReorder={(from, to) =>
                                      reorderAt({ kind: 'operation', index }, from, to)
                                    }
                                    onRemove={(i) => removeAt({ kind: 'operation', index }, i)}
                                    policies={op.policies || []}
                                    showHeader={false}
                                  />
                                </DropZone>
                              </AccordionDetails>
                            </Accordion>
                          );
                        })}
                      </Stack>
                    )}
                  </AccordionDetails>
                </Accordion>
              </Card>
            </CardContent>
          </Card>
        </Box>

        {/* RIGHT: Available Policies (drag source) */}
        {hubEnabled && (
          <Box sx={{ flexShrink: 0, width: { xs: '100%', md: 400 } }}>
            <Card sx={{ height: '100%', overflow: 'hidden' }} variant="outlined">
              <CardContent sx={{ height: 620, p: 2 }}>
                <AvailablePoliciesPanel onSelect={pickFromCatalog} />
              </CardContent>
            </Card>
          </Box>
        )}
      </Stack>

      <SaveBar
        dirty={dirty}
        disabled={!restApiId}
        onCancel={cancel}
        onSave={save}
        saving={update.isPending}
      />

      <PolicyConfigDrawer
        initialValues={editing?.policy.params}
        mode={editing ? 'edit' : 'add'}
        onClose={closeFlow}
        onConfirm={confirmPolicy}
        open={Boolean(configRef)}
        policy={configRef}
      />
    </Stack>
  );
}
