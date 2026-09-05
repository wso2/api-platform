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
  Card,
  Divider,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { Download, Sparkles } from '@wso2/oxygen-ui-icons-react';
import { useCallback, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { hairline } from '@/theme/receipes';
import { DEFAULT_API_SKELETON } from '../utils/apiSkeleton';
import type { ApiCreationWizardDraftState, ApiType } from '../types';
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
  back: {
    id: 'api.create.defineApi.action.back',
    defaultMessage: 'Back',
    description: 'Returns to the previous step of the API creation wizard.',
  },
  contractDescription: {
    id: 'api.create.defineApi.contract.description',
    defaultMessage: 'Import from a URL or a file.',
  },
  contractTitle: {
    id: 'api.create.defineApi.contract.title',
    defaultMessage: 'Start with a contract',
  },
  next: {
    id: 'api.create.defineApi.action.next',
    defaultMessage: 'Continue',
    description: 'Carries the definition in the preview pane on to the next step.',
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
    icon: <Download size={18} />,
    key: 'contract',
    title: messages.contractTitle,
  },
  {
    description: messages.scratchDescription,
    icon: <Sparkles size={18} />,
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
  /** Called by Back. */
  onBack?: () => void;
  /** Called by Next, with the draft the wizard collects. */
  onDataFetched: (data: ApiCreationWizardDraftState) => void;
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
  onBack,
  onDataFetched,
  onRefreshSwaggerHubOrganizations,
}: DefineApiPanelProps) => {
  const intl = useIntl();
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

  const handleSpecChange = (next: SpecDocument, warnings: SpecIssue[]) => {
    const edit: EditedSpec = { spec: next, warnings };
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

  const handleProceed = () => {
    // The draft the wizard collects — display name, version, context — is read
    // off the definition, which is the next piece of work; advancing with an
    // empty draft keeps this step's contract with the wizard until then.
    // get the basic informarion from the spec and pass it to the onDataFetched callback
    let draft: ApiCreationWizardDraftState = {};
    if (spec) {
      draft = extractApiDetails(spec);
    }
    onDataFetched(draft);
  };

  return (
    <Stack spacing={3}>
      {/* One surface for the whole step: the two approaches sit flush on top of
          the panels they open, like tabs on their own body, rather than
          floating above as separate cards. */}
      <Card variant="outlined">
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
            // The buttons are the card's own header row, so they give up the
            // borders and radii a standalone group would draw.
            '& .MuiToggleButtonGroup-grouped': {
              border: 0,
              borderRadius: 0,
              justifyContent: 'flex-start',
              p: 2,
              textTransform: 'none',
              '&:not(:first-of-type)': {
                border: hairline(theme),
                borderBottom: 0,
                borderColor: 'divider',
                borderRight: 0,
                borderTop: 0,
              },
              '&.Mui-selected': { bgcolor: 'action.selected' },
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

        <Divider />

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
              onSpecChange={handleSpecChange}
              spec={spec}
              warnings={edit?.warnings}
            />
          </Box>
        </Stack>
      </Card>

      {/* Both buttons on the trailing edge, Back beside the action it steps
          away from rather than at the opposite corner of the step. */}
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'flex-end' }}>
        <Button onClick={onBack} type="button" variant="text">
          <FormattedMessage {...messages.back} />
        </Button>
        <Button
          // Nothing to carry forward until the pane has a definition in it.
          disabled={spec === undefined}
          onClick={handleProceed}
          type="button"
          variant="contained"
        >
          {intl.formatMessage(messages.next)}
        </Button>
      </Stack>
    </Stack>
  );
};
