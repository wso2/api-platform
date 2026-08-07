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
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Globe } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';

import { useUpdateApi } from '../../../api/hooks/useMvpQueries';
import type { PolicySummary } from '../../../api/policyHub/policyHubClient';
import { usePolicyHub } from '../../../api/policyHub/usePolicyHub';
import { useNotifications } from '../../../components/Notifications';
import type {
  ApiOperation,
  ApiPolicy,
  ApiDetail,
} from '../../../types/domain';
import { AttachedPolicyList } from './AttachedPolicyList';
import { AvailablePoliciesPanel } from './AvailablePoliciesPanel';
import { SaveBar } from './SaveBar';
import { methodColor, reorderPolicies } from './developEdit';
import {
  getDraggedPolicy,
  type PolicyScope,
  scopeId,
} from './policyDnd';
import { PolicyConfigDrawer, type PolicyRef } from './PolicyConfigDrawer';

type EditState = { scope: PolicyScope; policyIndex: number; policy: ApiPolicy } | null;

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

export function PolicyTab({ detail }: { detail: ApiDetail }) {
  const { notify } = useNotifications();
  const update = useUpdateApi();
  const hubEnabled = usePolicyHub();

  const [apiPolicies, setApiPolicies] = useState<ApiPolicy[]>(detail.policies);
  const [operations, setOperations] = useState<ApiOperation[]>(detail.operations);

  // Add/config flow targeting a scope.
  const [scope, setScope] = useState<PolicyScope | null>(null);
  const [picked, setPicked] = useState<PolicySummary | null>(null);
  const [editing, setEditing] = useState<EditState>(null);
  const [activeZone, setActiveZone] = useState<string | null>(null);

  const policiesFor = (s: PolicyScope): ApiPolicy[] =>
    s.kind === 'api' ? apiPolicies : operations[s.index].policies || [];

  const setPoliciesFor = (s: PolicyScope, next: ApiPolicy[]) => {
    if (s.kind === 'api') setApiPolicies(next);
    else
      setOperations((ops) =>
        ops.map((op, i) => (i === s.index ? { ...op, policies: next } : op))
      );
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

  const confirmPolicy = (policy: ApiPolicy) => {
    if (!scope) return;
    if (editing) {
      setPoliciesFor(
        scope,
        policiesFor(scope).map((p, i) => (i === editing.policyIndex ? policy : p))
      );
    } else {
      setPoliciesFor(scope, [...policiesFor(scope), policy]);
    }
    notify(editing ? 'Policy updated. Save to apply.' : 'Policy attached. Save to apply.', 'success');
    closeFlow();
  };

  const closeFlow = () => {
    setPicked(null);
    setEditing(null);
    setScope(null);
  };

  const removeAt = (s: PolicyScope, i: number) =>
    setPoliciesFor(s, policiesFor(s).filter((_p, idx) => idx !== i));
  const reorderAt = (s: PolicyScope, from: number, to: number) =>
    setPoliciesFor(s, reorderPolicies(policiesFor(s), from, to));

  const save = () => {
    update.mutate(
      { ...detail, policies: apiPolicies, operations },
      {
        onSuccess: () => notify('Policies saved.', 'success'),
        onError: (error) =>
          notify(error instanceof Error ? error.message : 'Save failed', 'error'),
      }
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
          <Card sx={{ height: '100%' }} variant="outlined">
            <CardContent>
              <Typography sx={{ mb: 0.5 }} variant="subtitle1">
                Policies
              </Typography>
              <Typography color="text.secondary" sx={{ mb: 2 }} variant="body2">
                {hubEnabled
                  ? 'Drag policies from the right onto the API or a resource.'
                  : 'Policy Hub is not configured; policies cannot be added.'}
              </Typography>

              {/* Global / API-level */}
              <DropZone
                active={activeZone === scopeId({ kind: 'api' })}
                onDrop={() => dropOnScope({ kind: 'api' })}
                onEnter={() => setActiveZone(scopeId({ kind: 'api' }))}
                onLeave={() => setActiveZone(null)}
              >
                <Box sx={{ mb: 2 }}>
                  <Stack alignItems="center" direction="row" spacing={1} sx={{ mb: 1 }}>
                    <Globe size={16} />
                    <Typography sx={{ fontWeight: 600 }}>
                      Global policies (API level)
                    </Typography>
                  </Stack>
                  <AttachedPolicyList
                    canAdd={hubEnabled}
                    emptyText="Drag and drop policies here to apply at the global level."
                    onAdd={() => {
                      setEditing(null);
                      setScope({ kind: 'api' });
                      setPicked(null);
                      // open the catalog hint by selecting scope; user drags or
                      // clicks a catalog item to proceed
                    }}
                    onEdit={(i) => openEdit({ kind: 'api' }, i)}
                    onReorder={(from, to) => reorderAt({ kind: 'api' }, from, to)}
                    onRemove={(i) => removeAt({ kind: 'api' }, i)}
                    policies={apiPolicies}
                  />
                </Box>
              </DropZone>

              <Divider sx={{ my: 1 }} />
              <Typography sx={{ fontWeight: 600, mb: 1 }}>Resources</Typography>
              {operations.length === 0 ? (
                <Typography color="text.secondary" variant="body2">
                  No resources. Add them in the Routing tab.
                </Typography>
              ) : (
                <Stack spacing={1}>
                  {operations.map((op, index) => {
                    const zid = scopeId({ kind: 'operation', index });
                    return (
                      <DropZone
                        active={activeZone === zid}
                        key={index}
                        onDrop={() => dropOnScope({ kind: 'operation', index })}
                        onEnter={() => setActiveZone(zid)}
                        onLeave={() => setActiveZone(null)}
                      >
                        <Accordion
                          disableGutters
                          sx={{ '&:before': { display: 'none' } }}
                          variant="outlined"
                        >
                          <AccordionSummary expandIcon={<ChevronDown size={18} />}>
                            <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
                              <Chip
                                color={methodColor(op.method)}
                                label={op.method}
                                size="small"
                                sx={{ fontWeight: 700, minWidth: 58 }}
                              />
                              <Typography noWrap sx={{ fontFamily: 'monospace' }}>
                                {op.path || '/'}
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
                            <AttachedPolicyList
                              canAdd={hubEnabled}
                              emptyText="Drag and drop policies here, or use Add Policy."
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
                            />
                          </AccordionDetails>
                        </Accordion>
                      </DropZone>
                    );
                  })}
                </Stack>
              )}
            </CardContent>
          </Card>
        </Box>

        {/* RIGHT: Available Policies (drag source) */}
        {hubEnabled && (
          <Box sx={{ flexShrink: 0, width: { xs: '100%', md: 340 } }}>
            <Card sx={{ height: '100%' }} variant="outlined">
              <CardContent sx={{ height: 560 }}>
                <AvailablePoliciesPanel onSelect={pickFromCatalog} />
              </CardContent>
            </Card>
          </Box>
        )}
      </Stack>

      <SaveBar onSave={save} saving={update.isPending} />

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
