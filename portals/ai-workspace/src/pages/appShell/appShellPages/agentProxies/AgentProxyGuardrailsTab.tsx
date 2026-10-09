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

import React, { useMemo, useState } from 'react';
import {
  Box,
  Chip,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import AgentPolicyMapper from './AgentPolicyMapper';
import type { SelectedPolicy } from './AgentPolicyMapper';
import type { ParameterValues } from '../../PolicyParameterEditor/types';

/** The canonical A2A 1.0 operations, from common/agentproto. */
export const A2A_OPERATIONS: ReadonlyArray<{
  name: string;
  description: string;
}> = [
  {
    name: 'SendMessage',
    description: 'Sends a message to the agent and creates or continues a task.',
  },
  {
    name: 'SendStreamingMessage',
    description: 'Sends a message and subscribes to streamed task updates.',
  },
  { name: 'GetTask', description: 'Retrieves the current state of a task.' },
  { name: 'ListTasks', description: 'Lists tasks matching the given filters.' },
  {
    name: 'CancelTask',
    description: 'Requests cancellation of an ongoing task.',
  },
  {
    name: 'SubscribeToTask',
    description: "Reattaches to a task's streamed updates.",
  },
  {
    name: 'CreateTaskPushNotificationConfig',
    description: 'Registers a webhook for task update notifications.',
  },
  {
    name: 'GetTaskPushNotificationConfig',
    description: 'Retrieves a registered push notification config.',
  },
  {
    name: 'ListTaskPushNotificationConfigs',
    description: 'Lists all push notification configs for a task.',
  },
  {
    name: 'DeleteTaskPushNotificationConfig',
    description: 'Removes a registered push notification config.',
  },
  {
    name: 'GetExtendedAgentCard',
    description: 'Returns the protected agent card for an authenticated caller.',
  },
];

export type AgentPolicyState = {
  globalPolicies: SelectedPolicy[];
  operationPolicies: Record<string, SelectedPolicy[]>;
  publicCardPolicies: SelectedPolicy[];
};

type PolicyListHandlers = {
  onAdd: (policy: Omit<SelectedPolicy, 'instanceId'>) => void;
  onUpdate: (instanceId: string, params: ParameterValues) => void;
  onRemove: (instanceId: string) => void;
  onReorder: (draggedInstanceId: string, targetInstanceId: string) => void;
};

type Props = {
  state: AgentPolicyState;
  onChange: (next: AgentPolicyState) => void;
  readOnly: boolean;
};

let instanceCounter = 0;
function nextInstanceId(policyName: string): string {
  instanceCounter += 1;
  return `${policyName}-${Date.now()}-${instanceCounter}`;
}

function applyAdd(
  list: SelectedPolicy[],
  policy: Omit<SelectedPolicy, 'instanceId'>
): SelectedPolicy[] {
  return [...list, { ...policy, instanceId: nextInstanceId(policy.policyName) }];
}

function applyUpdate(
  list: SelectedPolicy[],
  instanceId: string,
  params: ParameterValues
): SelectedPolicy[] {
  return list.map((p) => (p.instanceId === instanceId ? { ...p, params } : p));
}

function applyReorder(
  list: SelectedPolicy[],
  draggedId: string,
  targetId: string
): SelectedPolicy[] {
  const from = list.findIndex((p) => p.instanceId === draggedId);
  const to = list.findIndex((p) => p.instanceId === targetId);
  if (from < 0 || to < 0 || from === to) return list;
  const next = [...list];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

export default function AgentProxyGuardrailsTab({
  state,
  onChange,
  readOnly,
}: Props): React.JSX.Element {
  const [addedOperations, setAddedOperations] = useState<string[]>([]);

  const handlersFor = (
    read: () => SelectedPolicy[],
    write: (next: SelectedPolicy[]) => void
  ): PolicyListHandlers => ({
    onAdd: (policy) => write(applyAdd(read(), policy)),
    onUpdate: (instanceId, params) =>
      write(applyUpdate(read(), instanceId, params)),
    onRemove: (instanceId) =>
      write(read().filter((p) => p.instanceId !== instanceId)),
    onReorder: (draggedId, targetId) =>
      write(applyReorder(read(), draggedId, targetId)),
  });

  const globalHandlers = handlersFor(
    () => state.globalPolicies,
    (next) => onChange({ ...state, globalPolicies: next })
  );

  const cardHandlers = handlersFor(
    () => state.publicCardPolicies,
    (next) => onChange({ ...state, publicCardPolicies: next })
  );

  const operationHandlers = (operation: string): PolicyListHandlers =>
    handlersFor(
      () => state.operationPolicies[operation] ?? [],
      (next) =>
        onChange({
          ...state,
          operationPolicies: { ...state.operationPolicies, [operation]: next },
        })
    );

  // An operation is on screen once it carries policies or the user picked it
  // from the dropdown; canonical order is kept regardless of when it appeared.
  const displayedOperations = useMemo(
    () =>
      A2A_OPERATIONS.filter(
        (operation) =>
          (state.operationPolicies[operation.name]?.length ?? 0) > 0 ||
          addedOperations.includes(operation.name)
      ),
    [addedOperations, state.operationPolicies]
  );

  const selectableOperations = useMemo(
    () =>
      A2A_OPERATIONS.filter(
        (operation) =>
          !displayedOperations.some((shown) => shown.name === operation.name)
      ),
    [displayedOperations]
  );

  const handleAddOperation = (name: string) => {
    if (!name) return;
    setAddedOperations((prev) =>
      prev.includes(name) ? prev : [...prev, name]
    );
  };

  const handleRemoveOperation = (name: string) => {
    setAddedOperations((prev) => prev.filter((item) => item !== name));
    if ((state.operationPolicies[name]?.length ?? 0) > 0) {
      const next = { ...state.operationPolicies };
      delete next[name];
      onChange({ ...state, operationPolicies: next });
    }
  };

  return (
    <Stack spacing={3}>
      <Stack spacing={1.5}>
        <AgentPolicyMapper
          title="Global Operation Policies"
          description="Applies to all A2A operations. Drag policies to change their execution order."
          readOnly={readOnly}
          selectedPolicies={state.globalPolicies}
          onAddPolicy={globalHandlers.onAdd}
          onUpdatePolicy={globalHandlers.onUpdate}
          onRemovePolicy={globalHandlers.onRemove}
          onReorderPolicies={globalHandlers.onReorder}
        />
      </Stack>

      <Divider />

      <Stack spacing={1.5}>
        <Box>
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            Operation-wise Policies
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Appended after the global policies for that operation.
          </Typography>
        </Box>

        <FormControl fullWidth sx={{ maxWidth: 360 }}>
          <FormLabel>Add operation</FormLabel>
          <Select
            size="small"
            displayEmpty
            value=""
            disabled={readOnly || selectableOperations.length === 0}
            onChange={(event) => handleAddOperation(String(event.target.value))}
            MenuProps={{ PaperProps: { sx: { maxHeight: 300 } } }}
            renderValue={() =>
              selectableOperations.length === 0
                ? 'All operations added'
                : 'Select an operation'
            }
          >
            {selectableOperations.map((operation) => (
              <MenuItem key={operation.name} value={operation.name}>
                <Stack spacing={0.25} sx={{ minWidth: 0 }}>
                  <Typography
                    variant="body2"
                    sx={{ fontFamily: 'monospace', fontWeight: 600 }}
                  >
                    {operation.name}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {operation.description}
                  </Typography>
                </Stack>
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <Stack spacing={1.5}>
          {displayedOperations.map((operation) => {
            const attached = state.operationPolicies[operation.name] ?? [];
            const handlers = operationHandlers(operation.name);
            return (
              <Box
                key={operation.name}
                sx={{
                  border: '1px solid',
                  borderColor: (theme) =>
                    `color-mix(in srgb, ${theme.palette.primary.main} 35%, transparent)`,
                  borderRadius: 1,
                }}
              >
                <Stack
                  direction="row"
                  spacing={2}
                  alignItems="center"
                  sx={{
                    p: 1.5,
                    background: (theme) =>
                      `linear-gradient(90deg, color-mix(in srgb, ${theme.palette.primary.main} 10%, transparent) 0%, color-mix(in srgb, ${theme.palette.primary.main} 4%, transparent) 100%)`,
                  }}
                >
                  <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: 'monospace', fontWeight: 600 }}
                    >
                      {operation.name}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {operation.description}
                    </Typography>
                  </Stack>
                  {attached.length > 0 ? (
                    <Chip size="small" label={attached.length} />
                  ) : null}
                  <Tooltip title="Remove operation">
                    <Box component="span">
                      <IconButton
                        size="small"
                        disabled={readOnly}
                        onClick={() => handleRemoveOperation(operation.name)}
                        aria-label={`Remove ${operation.name}`}
                      >
                        <X size={16} />
                      </IconButton>
                    </Box>
                  </Tooltip>
                </Stack>
                <Box sx={{ p: 2, pt: 1.5 }}>
                  <AgentPolicyMapper
                    title="Policies"
                    readOnly={readOnly}
                    selectedPolicies={attached}
                    onAddPolicy={handlers.onAdd}
                    onUpdatePolicy={handlers.onUpdate}
                    onRemovePolicy={handlers.onRemove}
                    onReorderPolicies={handlers.onReorder}
                  />
                </Box>
              </Box>
            );
          })}
          {displayedOperations.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No operation-wise policies yet. Pick an operation above to add
              policies that run only for it.
            </Typography>
          ) : null}
        </Stack>
      </Stack>

      <Divider />

      <Stack spacing={1.5}>
        <AgentPolicyMapper
          title="Public Agent Card Policies"
          description="Applies only to the public Agent Card discovery path."
          readOnly={readOnly}
          selectedPolicies={state.publicCardPolicies}
          onAddPolicy={cardHandlers.onAdd}
          onUpdatePolicy={cardHandlers.onUpdate}
          onRemovePolicy={cardHandlers.onRemove}
          onReorderPolicies={cardHandlers.onReorder}
        />
      </Stack>
    </Stack>
  );
}
