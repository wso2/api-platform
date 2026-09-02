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
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  getByPath,
  initValues,
  setByPath,
  topLevelRequiredMissing,
  usePolicyDefinition,
  type ParameterSchema,
  type ParameterValues,
  type PolicySummary,
} from '@/api/resources/policyHub';
import type { Policy } from '@/api/resources/restApis';
import { ErrorState } from '@/components/StateViews';
import { defaultForSchema, SchemaField } from './SchemaField';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.title',
    defaultMessage: 'Configure {policyName}',
    description:
      'Drawer heading. {policyName} is the policy\u2019s catalog name, backend data; do not translate it.',
  },
  backLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.backLabel',
    defaultMessage: 'Back',
    description: 'Accessible label for the icon button returning to the previous step.',
  },
  closeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.closeLabel',
    defaultMessage: 'Close',
    description: 'Accessible label for the icon button dismissing the drawer.',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.loadError',
    defaultMessage: 'Unable to load the policy definition.',
  },
  noParameters: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.noParameters',
    defaultMessage: 'This policy has no configurable parameters.',
  },
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.back',
    defaultMessage: 'Back',
    description: 'Footer button returning to the previous step. Verb.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.cancel',
    defaultMessage: 'Cancel',
  },
  savePolicy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.savePolicy',
    defaultMessage: 'Save policy',
    description: 'Confirms edits to an already-attached policy. Verb phrase.',
  },
  attachPolicy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.attachPolicy',
    defaultMessage: 'Attach policy',
    description: 'Attaches the configured policy to the selected scope. Verb phrase.',
  },
});

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
  onConfirm: (policy: Policy) => void;
}) {
  const intl = useIntl();
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
            <IconButton
              aria-label={intl.formatMessage(messages.backLabel)}
              onClick={onBack}
              size="small"
            >
              <ChevronLeft size={18} />
            </IconButton>
          )}
          <Typography sx={{ flex: 1 }} variant="h6">
            <FormattedMessage
              {...messages.title}
              values={{ policyName: policy?.displayName ?? '' }}
            />
          </Typography>
          <IconButton
            aria-label={intl.formatMessage(messages.closeLabel)}
            onClick={onClose}
            size="small"
          >
            <X size={18} />
          </IconButton>
        </Box>

        <Box sx={{ flex: 1, overflowY: 'auto', p: 2 }}>
          {definitionQuery.isLoading || !schema ? (
            definitionQuery.error ? (
              <ErrorState message={intl.formatMessage(messages.loadError)} />
            ) : (
              <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
                <CircularProgress size={24} />
              </Box>
            )
          ) : definitionQuery.error ? (
            <ErrorState message={intl.formatMessage(messages.loadError)} />
          ) : !hasParams ? (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.noParameters} />
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
          <Button onClick={onBack ?? onClose}>
            <FormattedMessage {...(onBack ? messages.back : messages.cancel)} />
          </Button>
          <Button
            disabled={definitionQuery.isLoading || missingRequired}
            onClick={confirm}
            variant="contained"
          >
            <FormattedMessage
              {...(mode === 'edit' ? messages.savePolicy : messages.attachPolicy)}
            />
          </Button>
        </Box>
      </Box>
    </Drawer>
  );
}
