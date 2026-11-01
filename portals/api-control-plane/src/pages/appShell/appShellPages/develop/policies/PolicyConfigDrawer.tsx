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

import { Box, Chip, CircularProgress, Drawer, IconButton, Typography } from '@wso2/oxygen-ui';
import { ChevronLeft, SlidersHorizontal, X } from '@wso2/oxygen-ui-icons-react';
import { useMemo } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  usePolicyDefinition,
  usePolicyVersions,
  type PolicyDefinition,
  type PolicySummary,
} from '@/api/resources/policyHub';
import type { Policy } from '@/api/resources/restApis';
import { ErrorState } from '@/components/StateViews';
import { versionLabel } from '@/utils/versionLabel';
import { usePolicyDisplayName } from './PolicyChip';
import {
  PolicyParameterEditor,
  type ParameterValues,
  type PolicyFormDefinition,
} from './parameterEditor';

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
  readyToAttach: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.readyToAttach',
    defaultMessage: 'Ready to attach',
    description: 'Footer status when every parameter is valid and the policy can be attached.',
  },
  readyToSave: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.readyToSave',
    defaultMessage: 'Ready to save',
    description: 'Footer status when every parameter is valid and the edits can be saved.',
  },
  attachPolicy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyConfigDrawer.attachPolicy',
    defaultMessage: 'Attach policy',
    description: 'Attaches the configured policy to the selected scope. Verb phrase.',
  },
});

/** Schema keywords that make fields appear without a top-level `properties`. */
const RULE_KEYWORDS = ['anyOf', 'oneOf', 'allOf', 'if'] as const;

/** The numeric major of "v1", "1" or "v1.2.3". */
const majorOf = (version: string): string | undefined => version.match(/^v?(\d+)/i)?.[1];

/** True for a version carrying only a major ("v1"): the hub needs "x.x" to serve a definition. */
const isMajorOnly = (version: string): boolean => /^v?\d+$/i.test(version.trim());

/** Orders dotted versions numerically, so "1.10" outranks "1.9". */
const compareVersions = (a: string, b: string): number => {
  const parts = (v: string) =>
    v
      .replace(/^v/i, '')
      .split('.')
      .map((n) => Number.parseInt(n, 10) || 0);
  const [pa, pb] = [parts(a), parts(b)];
  for (let i = 0; i < Math.max(pa.length, pb.length); i += 1) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
};

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
  // A saved policy keeps only its major ("v1"), but the hub serves a definition by
  // full "x.x" version, so a major-only reference is first resolved to the newest
  // published version of that major.
  const needsResolving = Boolean(policy && isMajorOnly(policy.version));
  const versionsQuery = usePolicyVersions(policy?.name, open && needsResolving);
  const resolvedVersion = useMemo(() => {
    if (!policy) return undefined;
    if (!needsResolving) return policy.version;
    const major = majorOf(policy.version);
    return (versionsQuery.data ?? [])
      .filter((candidate) => majorOf(candidate.version) === major)
      .map((candidate) => candidate.version)
      .sort(compareVersions)
      .pop();
  }, [policy, needsResolving, versionsQuery.data]);
  const versionUnresolved =
    needsResolving &&
    (Boolean(versionsQuery.error) || (versionsQuery.isSuccess && !resolvedVersion));

  const definitionQuery = usePolicyDefinition(policy?.name, resolvedVersion, open && !!policy);

  const definition: PolicyDefinition | undefined = definitionQuery.data;

  // An attached policy only knows its handle, so the title takes the catalog's name.
  const catalogName = usePolicyDisplayName(policy?.name ?? '');
  const titleName =
    definition?.displayName ||
    (policy && catalogName !== policy.name ? catalogName : policy?.displayName) ||
    '';

  // The editor keys its form on this object, so it must stay the same object
  // until the policy itself changes.
  const formDefinition = useMemo<PolicyFormDefinition | undefined>(
    () =>
      definition && {
        name: definition.name,
        version: definition.version,
        displayName: definition.displayName,
        description: definition.description,
        parameters: definition.parameters,
        systemParameters: definition.systemParameters,
      },
    [definition],
  );

  // The editor re-seeds its form whenever this changes identity, which would
  // discard in-progress edits if a parent passed a fresh object on each render.
  const serializedInitialValues = initialValues ? JSON.stringify(initialValues) : undefined;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const existingValues = useMemo(() => initialValues, [serializedInitialValues]);

  // Properties, or rules (`oneOf`, `if`, ...) that the editor turns into fields.
  const hasParams = Boolean(
    definition?.parameters &&
    (Object.keys(definition.parameters.properties ?? {}).length > 0 ||
      RULE_KEYWORDS.some((keyword) => keyword in definition.parameters!)),
  );

  const confirm = (params: ParameterValues) => {
    if (!policy) return;
    onConfirm({ name: policy.name, version: policy.version, params });
  };

  return (
    <Drawer anchor="right" onClose={onClose} open={open}>
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
          maxWidth: '100vw',
          width: 720,
        }}
      >
        <Box
          sx={{
            alignItems: 'center',
            borderBottom: '1px solid',
            borderColor: 'divider',
            display: 'flex',
            gap: 1.5,
            px: 3,
            py: 2,
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
          <Box
            sx={{
              alignItems: 'center',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              color: 'primary.main',
              display: 'flex',
              flexShrink: 0,
              height: 40,
              justifyContent: 'center',
              width: 40,
            }}
          >
            <SlidersHorizontal size={18} />
          </Box>
          <Box sx={{ alignItems: 'center', display: 'flex', flex: 1, gap: 1, minWidth: 0 }}>
            <Typography noWrap variant="h6">
              <FormattedMessage {...messages.title} values={{ policyName: titleName }} />
            </Typography>
            {policy?.version && (
              <Chip
                label={versionLabel(resolvedVersion ?? policy.version)}
                size="small"
                sx={{ color: 'text.secondary', fontFamily: 'monospace', fontSize: '0.7rem' }}
                variant="outlined"
              />
            )}
          </Box>
          <IconButton
            aria-label={intl.formatMessage(messages.closeLabel)}
            onClick={onClose}
            size="small"
          >
            <X size={18} />
          </IconButton>
        </Box>

        <Box sx={{ display: 'flex', flex: 1, flexDirection: 'column', minHeight: 0 }}>
          {definitionQuery.error || versionUnresolved ? (
            <Box sx={{ p: 3 }}>
              <ErrorState message={intl.formatMessage(messages.loadError)} />
            </Box>
          ) : definitionQuery.isLoading || !formDefinition ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
              <CircularProgress size={24} />
            </Box>
          ) : !hasParams ? (
            <Typography color="text.secondary" sx={{ p: 3 }} variant="body2">
              <FormattedMessage {...messages.noParameters} />
            </Typography>
          ) : (
            <PolicyParameterEditor
              cancelLabel={intl.formatMessage(onBack ? messages.back : messages.cancel)}
              existingValues={existingValues}
              hideTitle
              key={`${policy?.name}@${policy?.version}`}
              onCancel={onBack ?? onClose}
              onSubmit={confirm}
              policyDefinition={formDefinition}
              readyLabel={intl.formatMessage(
                mode === 'edit' ? messages.readyToSave : messages.readyToAttach,
              )}
              submitLabel={intl.formatMessage(
                mode === 'edit' ? messages.savePolicy : messages.attachPolicy,
              )}
            />
          )}
        </Box>
      </Box>
    </Drawer>
  );
}
