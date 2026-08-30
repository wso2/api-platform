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
  Box,
  Button,
  CircularProgress,
  Drawer,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, X } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';

import type { PolicySummary } from '../../../../../api/policyHub/policyHubClient';
import {
  getByPath,
  initValues,
  type ParameterSchema,
  type ParameterValues,
  setByPath,
  topLevelRequiredMissing,
} from '@/api/policyHub/policySchema';
import { usePolicyDefinition } from '@/api/policyHub/usePolicyHub';
import { ErrorState } from '@/components/StateViews';
import type { ApiPolicy } from '@/types/domain';
import { defaultForSchema, SchemaField } from './SchemaField';

/** Minimal reference needed to load a policy's definition. */
export type PolicyRef = Pick<PolicySummary, 'name' | 'version' | 'displayName'>;

/**
 * Configure a policy's parameters (form generated recursively from the policy's
 * JSON-Schema definition). In edit mode the form is pre-filled from
 * `initialValues`.
 */
export function PolicyConfigDrawer({
  policy,
  open,
  mode = 'add',
  initialValues,
  onBack,
  onClose,
  onConfirm,
}: {
  policy: PolicyRef | null;
  open: boolean;
  mode?: 'add' | 'edit';
  initialValues?: Record<string, unknown>;
  onBack?: () => void;
  onClose: () => void;
  onConfirm: (policy: ApiPolicy) => void;
}) {
  const definitionQuery = usePolicyDefinition(policy?.name, policy?.version, open && !!policy);
  const schema: ParameterSchema | undefined = definitionQuery.data?.schema;

  // Values keyed by policy so switching policies resets the form.
  const [valuesByKey, setValuesByKey] = useState<Record<string, ParameterValues>>({});
  const formKey = policy ? `${policy.name}@${policy.version}` : '';
  const values = valuesByKey[formKey] ?? (schema ? initValues(schema, initialValues) : {});

  const update = (next: ParameterValues) =>
    setValuesByKey((prev) => ({ ...prev, [formKey]: next }));

  const onFieldChange = (path: string, value: unknown) => update(setByPath(values, path, value));

  const onAddItem = (path: string, itemSchema: ParameterSchema) => {
    const current = getByPath(values, path);
    const arr = Array.isArray(current) ? current : [];
    update(setByPath(values, `${path}.${arr.length}`, defaultForSchema(itemSchema)));
  };

  const onRemoveItem = (path: string, index: number) => {
    const current = getByPath(values, path);
    const arr = Array.isArray(current) ? [...current] : [];
    arr.splice(index, 1);
    update(setByPath(values, path, arr));
  };

  const missingRequired = useMemo(
    () => (schema ? topLevelRequiredMissing(schema, values) : false),
    [schema, values],
  );

  const hasParams = Boolean(
    schema && schema.properties && Object.keys(schema.properties).length > 0,
  );

  const confirm = () => {
    if (!policy) return;
    onConfirm({ name: policy.name, version: policy.version, params: values });
  };

  return (
    <Drawer anchor="right" onClose={onClose} open={open}>
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', width: 420 }}>
        <Box
          sx={{
            alignItems: 'center',
            borderBottom: '1px solid',
            borderColor: 'divider',
            display: 'flex',
            gap: 1,
            p: 2,
          }}
        >
          {onBack && (
            <IconButton aria-label="Back" onClick={onBack} size="small">
              <ChevronLeft size={18} />
            </IconButton>
          )}
          <Typography sx={{ flex: 1 }} variant="h6">
            Configure {policy?.displayName}
          </Typography>
          <IconButton aria-label="Close" onClick={onClose} size="small">
            <X size={18} />
          </IconButton>
        </Box>

        <Box sx={{ flex: 1, overflowY: 'auto', p: 2 }}>
          {definitionQuery.isLoading || !schema ? (
            definitionQuery.error ? (
              <ErrorState
                message={
                  definitionQuery.error instanceof Error
                    ? definitionQuery.error.message
                    : 'Unable to load the policy definition.'
                }
              />
            ) : (
              <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
                <CircularProgress size={24} />
              </Box>
            )
          ) : definitionQuery.error ? (
            <ErrorState
              message={
                definitionQuery.error instanceof Error
                  ? definitionQuery.error.message
                  : 'Unable to load the policy definition.'
              }
            />
          ) : !hasParams ? (
            <Typography color="text.secondary" variant="body2">
              This policy has no configurable parameters.
            </Typography>
          ) : (
            <Stack spacing={1}>
              <SchemaField
                key={formKey}
                onAddItem={onAddItem}
                onChange={onFieldChange}
                onRemoveItem={onRemoveItem}
                path=""
                schema={schema}
                values={values}
              />
            </Stack>
          )}
        </Box>

        <Box
          sx={{
            borderColor: 'divider',
            borderTop: '1px solid',
            display: 'flex',
            gap: 1,
            justifyContent: 'flex-end',
            p: 2,
          }}
        >
          <Button onClick={onBack ?? onClose}>{onBack ? 'Back' : 'Cancel'}</Button>
          <Button
            disabled={definitionQuery.isLoading || missingRequired}
            onClick={confirm}
            variant="contained"
          >
            {mode === 'edit' ? 'Save policy' : 'Attach policy'}
          </Button>
        </Box>
      </Box>
    </Drawer>
  );
}
