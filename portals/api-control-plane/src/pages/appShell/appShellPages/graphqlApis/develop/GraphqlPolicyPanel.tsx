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

import { Box, Card, CardContent, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { Globe } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useIsPolicyHubConfigured, type PolicySummary } from '@/api/resources/policyHub';
import type { GraphQLApiDetail } from '@/api/resources/graphqlApis';
import { useUpdateGraphQLApi } from '@/api/resources/graphqlApis';
import type { Policy } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { AttachedPolicyList } from '../../develop/policies/AttachedPolicyList';
import { AvailablePoliciesPanel } from '../../develop/policies/AvailablePoliciesPanel';
import { getDraggedPolicy } from '../../develop/policies/policyDnd';
import { PolicyConfigDrawer, type PolicyRef } from '../../develop/policies/PolicyConfigDrawer';
import { SaveBar } from '../../develop/SaveBar';
import { useDirtyTracking } from '../../develop/useDirtyTracking';
import { reorderPolicies, withMajorPolicyVersion } from '../../apis/utils/developEdit';

const messages = defineMessages({
  heading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.heading',
    defaultMessage: 'Policies',
    description: 'Heading over the panel where policies are attached. Noun.',
  },
  globalDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.globalDescription',
    defaultMessage:
      'These policies apply to every request this API receives — a GraphQL API has a single endpoint, so there is no per-resource scope the way a REST API has.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.empty',
    defaultMessage: 'Drag and drop policies here, or use Add Policy.',
    description: '"Add Policy" is the label of a button in this same list.',
  },
  policyUpdated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.policyUpdated',
    defaultMessage: 'Policy updated. Save to apply.',
    description: 'Toast after editing an attached policy; the change is not persisted until Save.',
  },
  policyAttached: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.policyAttached',
    defaultMessage: 'Policy attached. Save to apply.',
    description: 'Toast after attaching a policy; the change is not persisted until Save.',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.saved',
    defaultMessage: 'Policies saved.',
  },
  redeployReminder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlPolicyPanel.redeployReminder',
    defaultMessage: 'Redeploy this API for the change to reach its gateways.',
    description: 'Toast shown alongside "Policies saved." — saving updates the definition only.',
  },
});

type EditState = { policyIndex: number; policy: Policy } | null;

/**
 * Fork of `develop/policies/PolicyPanel.tsx` for a GraphQL API. Reuses every
 * REST-agnostic building block as-is (`AttachedPolicyList`,
 * `AvailablePoliciesPanel`, `PolicyConfigDrawer`, `SaveBar`,
 * `useDirtyTracking`, `useIsPolicyHubConfigured`, `reorderPolicies`,
 * `policyDnd`) — none of them import anything REST-specific beyond the shared
 * `Policy` schema type. What's dropped is the "Resource-wise policies"
 * accordion: `GraphQLAPIConfigData` has no operations list at all (a single
 * POST endpoint, identified by the request body rather than the URL), so
 * there is no per-resource scope to attach anything to — every policy here is
 * API-level, matching what `GraphQLAPI.policies`'s own doc comment says.
 */
export function GraphqlPolicyPanel({ api }: { api: GraphQLApiDetail }) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const update = useUpdateGraphQLApi();
  const hubEnabled = useIsPolicyHubConfigured();
  const graphqlApiId = api.id;

  const [apiPolicies, setApiPolicies] = useState<Policy[]>(api.policies ?? []);
  const [picked, setPicked] = useState<PolicySummary | null>(null);
  const [editing, setEditing] = useState<EditState>(null);
  const [dropActive, setDropActive] = useState(false);

  const { dirty, markSaved } = useDirtyTracking({ apiPolicies });

  const openAdd = () => {
    setEditing(null);
    setPicked(null);
  };

  const openEdit = (index: number) => {
    setPicked(null);
    setEditing({ policyIndex: index, policy: apiPolicies[index] });
  };

  const confirmPolicy = (policy: Policy) => {
    if (editing) {
      setApiPolicies((current) =>
        current.map((p, i) => (i === editing.policyIndex ? policy : p)),
      );
    } else {
      setApiPolicies((current) => [...current, policy]);
    }
    notify(intl.formatMessage(editing ? messages.policyUpdated : messages.policyAttached), 'success');
    closeFlow();
  };

  const closeFlow = () => {
    setPicked(null);
    setEditing(null);
  };

  const removeAt = (index: number) =>
    setApiPolicies((current) => current.filter((_p, i) => i !== index));
  const reorderAt = (from: number, to: number) =>
    setApiPolicies((current) => reorderPolicies(current, from, to));

  const cancel = () => {
    setApiPolicies(api.policies ?? []);
    closeFlow();
  };

  const save = () => {
    // The API was fetched by handle, so `id` is present; the guard exists for
    // the same reason PolicyPanel's does — a PUT to the collection URL would
    // otherwise be issued if it were ever missing.
    if (!graphqlApiId) return;
    update.mutate(
      {
        graphqlApiId,
        body: {
          metadata: {
            ...api,
            policies: apiPolicies.map(withMajorPolicyVersion),
            // `GraphQLAPIDetail` (the GET shape `api` comes from) doesn't echo
            // back the `schemaSource`/`sdlUrl`/`sdl` the API was originally
            // created with, so there's nothing faithful to resupply here. The
            // service's own Update handler treats an unset schemaSource as
            // 'introspection' by default and re-resolves against the existing
            // upstream URL — explicit here only because the request type
            // requires the field; a failed resolution falls back to the
            // already-stored schema rather than blanking it (see
            // GraphQLAPIService.Update), so this is safe either way.
            schemaSource: 'introspection',
          },
        },
      },
      {
        onSuccess: () => {
          markSaved();
          notify(intl.formatMessage(messages.saved), 'success');
          notify(intl.formatMessage(messages.redeployReminder), 'info');
        },
      },
    );
  };

  const configRef: PolicyRef | null = editing
    ? { name: editing.policy.name, version: editing.policy.version, displayName: editing.policy.name }
    : picked;

  return (
    <Stack spacing={2}>
      <Stack alignItems="flex-start" direction={{ xs: 'column', md: 'row' }} spacing={2}>
        <Box sx={{ flex: 1, minWidth: 0, width: '100%' }}>
          <Card sx={{ height: '100%', overflow: 'hidden' }} variant="outlined">
            <Box sx={{ px: 2, py: 1.5 }}>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                <FormattedMessage {...messages.heading} />
              </Typography>
            </Box>
            <Divider />
            <CardContent sx={{ px: { lg: 5, xs: 2 }, py: 2 }}>
              <Card sx={{ borderRadius: 2, p: 2.5 }}>
                <Stack alignItems="center" direction="row" spacing={1.25}>
                  <Box sx={{ bgcolor: 'background.paper', borderRadius: 1, display: 'flex', p: 1 }}>
                    <Globe size={18} />
                  </Box>
                  <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
                    <FormattedMessage {...messages.heading} />
                  </Typography>
                </Stack>
                <Typography color="text.secondary" sx={{ mb: 2, mt: 0.75 }} variant="body2">
                  <FormattedMessage {...messages.globalDescription} />
                </Typography>
                <Box
                  onDragLeave={() => setDropActive(false)}
                  onDragOver={(event) => {
                    if (!getDraggedPolicy()) return;
                    event.preventDefault();
                    event.dataTransfer.dropEffect = 'copy';
                    setDropActive(true);
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    const dragged = getDraggedPolicy();
                    setDropActive(false);
                    if (!dragged) return;
                    setEditing(null);
                    setPicked({
                      name: dragged.name,
                      version: dragged.version,
                      displayName: dragged.displayName,
                      provider: '',
                      categories: [],
                      tags: [],
                      isLatest: true,
                    });
                  }}
                  sx={{
                    border: '2px dashed',
                    borderColor: dropActive ? 'primary.main' : 'transparent',
                    borderRadius: 1.5,
                    p: dropActive ? 0.5 : 0,
                    transition: 'border-color .12s',
                  }}
                >
                  <AttachedPolicyList
                    canAdd={hubEnabled}
                    emptyText={intl.formatMessage(messages.empty)}
                    onAdd={openAdd}
                    onEdit={openEdit}
                    onReorder={reorderAt}
                    onRemove={removeAt}
                    policies={apiPolicies}
                    showHeader={false}
                  />
                </Box>
              </Card>
            </CardContent>
          </Card>
        </Box>

        {hubEnabled && (
          <Box sx={{ flexShrink: 0, width: { xs: '100%', md: 400 } }}>
            <Card sx={{ height: '100%', overflow: 'hidden' }} variant="outlined">
              <CardContent sx={{ height: 620, p: 2 }}>
                <AvailablePoliciesPanel
                  onSelect={(policy) => {
                    setEditing(null);
                    setPicked(policy);
                  }}
                />
              </CardContent>
            </Card>
          </Box>
        )}
      </Stack>

      <SaveBar dirty={dirty} disabled={!graphqlApiId} onCancel={cancel} onSave={save} saving={update.isPending} />

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
