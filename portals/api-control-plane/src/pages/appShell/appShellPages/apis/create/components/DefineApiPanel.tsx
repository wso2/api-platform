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
  alpha,
  Box,
  Card,
  Divider,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { FileCode2, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useValidateOpenApiSpec } from '@/api/resources/restApis';
import { DEFAULT_API_SKELETON } from '../utils/apiSkeleton';
import type { ApiCreationWizardDraftState, ApiType, ContractImport } from '../types';
import { ApiResourcesPreview } from './ApiResourcesPreview';
import { ContractSourceForm, type FetchedContract } from './ContractSourceForm';
import { DesignWithAiPanel } from './DesignWithAiPanel';
import { extractApiDetails } from '../utils/specDetails';
import type { SpecDocument } from '../utils/specText';
import type { SpecIssue } from '../utils/specValidation';

/** The two ways this step can produce a definition. */
type ApproachKey = 'contract' | 'scratch';

/**
 * A definition after it has been edited in the preview pane, with what its
 * re-check said about it. Held separately from what was imported so that
 * re-fetching a contract restores the fetched document rather than the edit,
 * and so the import's own warnings can stop being reported once they describe
 * a document that has since been changed.
 */
type EditedSpec = {
  spec: SpecDocument;
  warnings: SpecIssue[];
};

const messages = defineMessages({
  approachLabel: {
    id: 'api.create.defineApi.approach.label',
    defaultMessage: 'How do you want to define this API?',
    description: 'Accessible name for the pair of approach tabs at the top of the step.',
  },
  contractDescription: {
    id: 'api.create.defineApi.contract.description',
    defaultMessage: 'Import from a URL or a file.',
  },
  contractTitle: {
    id: 'api.create.defineApi.contract.title',
    defaultMessage: 'Start with a contract',
  },
  scratchDescription: {
    id: 'api.create.defineApi.scratch.description',
    defaultMessage: 'Start blank and chat with AI to build it.',
  },
  scratchTitle: {
    id: 'api.create.defineApi.scratch.title',
    defaultMessage: 'Design from scratch',
  },
});

type Approach = {
  description: MessageDescriptor;
  icon: ReactNode;
  key: ApproachKey;
  title: MessageDescriptor;
};

const APPROACHES: Approach[] = [
  {
    description: messages.contractDescription,
    icon: <FileCode2 size={18} />,
    key: 'contract',
    title: messages.contractTitle,
  },
  {
    description: messages.scratchDescription,
    icon: <Pencil size={18} />,
    key: 'scratch',
    title: messages.scratchTitle,
  },
];

export type DefineApiPanelProps = {
  /** Types offered to the contract form. */
  apiTypes?: ApiType[];
  /** Type the step works with. Owned by the wizard's earlier step. */
  initialApiTypeKey?: string;
  /** Starts the GitHub OAuth flow. The button renders either way, inert until wired. */
  onAuthorizeGitHub?: () => void;
  /** Keeps the wizard footer supplied with the definition currently on screen. */
  onDraftChange: (data: ApiCreationWizardDraftState | null) => void;
  /** Re-fetches the SwaggerHub organizations. Inert until the import is wired. */
  onRefreshSwaggerHubOrganizations?: () => void;
};

/**
 * The wizard's "how do you want to define this API?" step.
 *
 * Two approaches sit across the top and share one preview pane: importing a
 * contract fills it with what was fetched, designing from scratch fills it with
 * a skeleton to edit. Back and Next belong to the panel rather than to either
 * approach, so switching between them doesn't move the buttons.
 */
export const DefineApiPanel = ({
  apiTypes,
  initialApiTypeKey,
  onAuthorizeGitHub,
  onDraftChange,
  onRefreshSwaggerHubOrganizations,
}: DefineApiPanelProps) => {
  const intl = useIntl();
  const validateSpec = useValidateOpenApiSpec();
  const [approach, setApproach] = useState<ApproachKey>('contract');
  const [contract, setContract] = useState<FetchedContract | null>(null);
  // One edit per approach, so switching tabs to look at the other one and back
  // doesn't throw away what was typed.
  const [contractEdit, setContractEdit] = useState<EditedSpec | null>(null);
  const [scratchEdit, setScratchEdit] = useState<EditedSpec | null>(null);

  /**
   * A different contract underneath - fetched, or cleared because the form's
   * inputs moved on from it; retires the edit built on the previous one.
   *
   * Stable identity matters: the form reports the current contract from an
   * effect keyed on this callback, so a fresh function each render would fire
   * that effect every render and wipe the edit as fast as it was made.
   */
  const handleContractChange = useCallback((next: FetchedContract | null) => {
    setContract(next);
    setContractEdit(null);
  }, []);

  const handleBeforeSave = useCallback(
    async (specText: string): Promise<string[] | null> => {
      try {
        const result = await validateSpec.mutateAsync(specText);
        if (!result.isValid) return result.errors.map((e) => e.message);
        return null;
      } catch {
        return null;
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [validateSpec.mutateAsync],
  );

  const handleSpecChange = (next: SpecDocument) => {
    const edit: EditedSpec = { spec: next, warnings: [] };
    if (approach === 'scratch') {
      setScratchEdit(edit);
      return;
    }
    setContractEdit(edit);
  };

  // Scratch always has something to show and carry forward; a contract has to
  // be fetched first. Either way an edit made here supersedes what it started
  // from.
  const edit = approach === 'scratch' ? scratchEdit : contractEdit;
  const spec = edit?.spec ?? (approach === 'scratch' ? DEFAULT_API_SKELETON : contract?.spec);

  const draft = useMemo((): ApiCreationWizardDraftState | null => {
    if (spec === undefined) return null;
    const base = extractApiDetails(spec);
    // Both approaches submit via import-openapi: contract passes the fetched spec,
    // scratch passes the skeleton (or whatever the user has edited).
    const specBlob = new Blob([JSON.stringify(spec, null, 2)], { type: 'application/json' });
    return {
      ...base,
      contractImport: {
        specFile: new File([specBlob], 'openapi.json', { type: 'application/json' }),
      },
    };
  }, [spec]);

  useEffect(() => {
    onDraftChange(draft);
    return () => onDraftChange(null);
  }, [draft, onDraftChange]);

  return (
    <Stack spacing={3}>
      {/* One surface for the whole step: the two approaches sit flush on top of
          the panels they open, like tabs on their own body, rather than
          floating above as separate cards. */}
      <Card sx={{ border: 0, overflow: 'visible' }} variant="outlined">
        <ToggleButtonGroup
          aria-label={intl.formatMessage(messages.approachLabel)}
          exclusive
          fullWidth
          onChange={(_event, next: ApproachKey | null) => {
            // `exclusive` reports null when the active button is clicked
            // again; keep the current approach rather than clearing it.
            if (next !== null) {
              setApproach(next);
            }
          }}
          sx={(theme) => ({
            p: 0,
            '& .MuiToggleButtonGroup-grouped': {
              border: `1px solid ${alpha(theme.palette.text.primary, 0.32)}`,
              borderBottom: 0,
              borderRadius: `${theme.shape.borderRadius}px ${theme.shape.borderRadius}px 0 0`,
              flex: 1,
              justifyContent: 'flex-start',
              p: 2,
              textTransform: 'none',
              '&:not(:first-of-type)': {
                borderLeft: `1px solid ${alpha(theme.palette.text.primary, 0.32)}`,
                marginLeft: 0,
              },
              '&.Mui-selected, &.Mui-selected:hover': {
                bgcolor: 'action.selected',
                border: `1px solid ${theme.palette.primary.main}`,
                borderBottom: 0,
                borderRadius: `${theme.shape.borderRadius}px ${theme.shape.borderRadius}px 0 0`,
              },
            },
          })}
          value={approach}
        >
          {APPROACHES.map((candidate) => {
            const selected = candidate.key === approach;

            return (
              <ToggleButton key={candidate.key} value={candidate.key}>
                <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', width: '100%' }}>
                  <Box
                    sx={{
                      alignItems: 'center',
                      bgcolor: selected ? 'primary.main' : 'action.hover',
                      borderRadius: 1,
                      color: selected ? 'primary.contrastText' : 'text.secondary',
                      display: 'flex',
                      flexShrink: 0,
                      height: 34,
                      justifyContent: 'center',
                      width: 34,
                    }}
                  >
                    {candidate.icon}
                  </Box>
                  <Stack spacing={0.25} sx={{ minWidth: 0, textAlign: 'left' }}>
                    <Typography color="text.primary" sx={{ fontWeight: 700 }} variant="body1">
                      <FormattedMessage {...candidate.title} />
                    </Typography>
                    <Typography color="text.secondary" variant="body2">
                      <FormattedMessage {...candidate.description} />
                    </Typography>
                  </Stack>
                </Stack>
              </ToggleButton>
            );
          })}
        </ToggleButtonGroup>

        <Stack
          direction={{ lg: 'row', xs: 'column' }}
          divider={
            <Divider
              flexItem
              orientation="vertical"
              // One rule that reads correctly both ways: a vertical line
              // between the halves side by side, a horizontal one once the
              // layout stacks them.
              sx={{
                borderBottomWidth: { lg: 0, xs: 'thin' },
                borderRightWidth: { lg: 'thin', xs: 0 },
              }}
            />
          }
          sx={(theme) => ({
            border: 1,
            borderColor: 'primary.main',
            borderRadius: `0 0 ${theme.shape.borderRadius}px ${theme.shape.borderRadius}px`,
            borderTop: 0,
            position: 'relative',
            '&::before': {
              bgcolor: 'primary.main',
              content: '""',
              height: '1px',
              left: approach === 'contract' ? '50%' : 0,
              position: 'absolute',
              top: 0,
              width: '50%',
            },
          })}
        >
          <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
            {approach === 'contract' ? (
              <ContractSourceForm
                apiTypes={apiTypes}
                // Fetched warnings describe the import; after edits they no
                // longer match and the pane shows new warnings.
                definitionEdited={contractEdit !== null}
                initialApiTypeKey={initialApiTypeKey}
                onAuthorizeGitHub={onAuthorizeGitHub}
                onContractChange={handleContractChange}
                onRefreshSwaggerHubOrganizations={onRefreshSwaggerHubOrganizations}
              />
            ) : (
              <DesignWithAiPanel />
            )}
          </Box>

          <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
            <ApiResourcesPreview
              onBeforeSave={handleBeforeSave}
              onSpecChange={handleSpecChange}
              spec={spec}
              warnings={edit?.warnings}
            />
          </Box>
        </Stack>
      </Card>
    </Stack>
  );
};
