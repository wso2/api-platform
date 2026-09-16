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
  Divider,
  FormControlLabel,
  Radio,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, ChevronLeft, ChevronRight, FileCode2, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useMemo, useState } from 'react';
import { defineMessages, FormattedMessage } from 'react-intl';

import { DEFAULT_API_SKELETON, PLACEHOLDER_UPSTREAM_URL } from '../utils/apiSkeleton';
import type { ApiCreationWizardDraftState } from '../types';
import { ApiResourcesPreview } from './ApiResourcesPreview';
import { ContractSourceForm, type FetchedContract } from './ContractSourceForm';
import { extractApiDetails } from '../utils/specDetails';

/** The two ways this step can produce a definition. */
type ApproachKey = 'contract' | 'scratch';

/** Whether the user wants to enter an endpoint URL now or skip it. */
type EndpointOption = 'none' | 'custom';

/** The two views rendered by this panel. */
type View = 'cards' | 'contract';

const messages = defineMessages({
  changeSource: {
    id: 'api.create.defineApi.changeSource',
    defaultMessage: 'Change source',
    description: 'Back button label in the contract import view.',
  },
  continue: {
    id: 'api.create.defineApi.continue',
    defaultMessage: 'Continue',
    description: 'Button that advances the wizard from the contract view to the next step.',
  },
  contractDescription: {
    id: 'api.create.defineApi.contract.description',
    defaultMessage: 'Import an API contract from a URL or a file.',
  },
  contractHeading: {
    id: 'api.create.defineApi.contract.heading',
    defaultMessage: 'Point us at your contract',
  },
  contractSubheading: {
    id: 'api.create.defineApi.contract.subheading',
    defaultMessage: 'Import OpenAPI or Swagger definition',
  },
  contractTitle: {
    id: 'api.create.defineApi.contract.title',
    defaultMessage: 'Start with a Contract',
  },
  scratchDescription: {
    id: 'api.create.defineApi.scratch.description',
    defaultMessage: 'Begin with a blank API and fill in the details.',
  },
  scratchHasEndpoint: {
    id: 'api.create.defineApi.scratch.hasEndpoint',
    defaultMessage: 'I have an endpoint URL',
  },
  scratchTitle: {
    id: 'api.create.defineApi.scratch.title',
    defaultMessage: 'Start from Scratch',
  },
});

export type DefineApiPanelProps = {
  /** Type the step works with. Owned by the wizard's earlier step. */
  initialApiTypeKey?: string;
  /** Keeps the wizard footer supplied with the definition currently on screen. */
  onDraftChange: (data: ApiCreationWizardDraftState | null) => void;
  /**
   * Fired when the user's chosen approach changes.
   */
  onApproachChange?: (approach: ApproachKey) => void;
  /**
   * Fired when the user explicitly clicks Continue. The wizard should advance to
   * the next step when this is called.
   */
  onContinue?: () => void;
};

export const DefineApiPanel = ({
  initialApiTypeKey,
  onDraftChange,
  onApproachChange,
  onContinue,
}: DefineApiPanelProps) => {
  const [view, setView] = useState<View>('cards');

  // Scratch card selection and endpoint sub-choice
  const [scratchSelected, setScratchSelected] = useState(false);
  const [endpointOption, setEndpointOption] = useState<EndpointOption>('none');
  const [endpointUrl, setEndpointUrl] = useState('');

  // Contract state
  const [contract, setContract] = useState<FetchedContract | null>(null);

  /**
   * The draft for the scratch approach. The upstream URL is either the user's
   * custom entry or the placeholder — the configure step distinguishes them.
   */
  const scratchDraft = useMemo((): ApiCreationWizardDraftState => {
    const base = extractApiDetails(DEFAULT_API_SKELETON);
    const specBlob = new Blob([JSON.stringify(DEFAULT_API_SKELETON, null, 2)], {
      type: 'application/json',
    });
    const customUrl = endpointOption === 'custom' ? endpointUrl.trim() : '';
    const upstreamUrl = customUrl || PLACEHOLDER_UPSTREAM_URL;
    return {
      ...base,
      upstream: { main: { url: upstreamUrl } },
      contractImport: {
        specFile: new File([specBlob], 'openapi.json', { type: 'application/json' }),
      },
    };
  }, [endpointOption, endpointUrl]);

  const contractDraft = useMemo((): ApiCreationWizardDraftState | null => {
    if (contract?.spec === undefined) return null;
    const base = extractApiDetails(contract.spec);
    const rawText = contract.rawText;
    if (rawText !== undefined) {
      const isJson = rawText.trimStart().startsWith('{');
      const contentType = isJson ? 'application/json' : 'application/yaml';
      const fileName = isJson ? 'openapi.json' : 'openapi.yaml';
      const rawBlob = new Blob([rawText], { type: contentType });
      return {
        ...base,
        contractImport: {
          specFile: new File([rawBlob], fileName, { type: contentType }),
        },
      };
    }
    return null;
  }, [contract]);

  useEffect(() => {
    if (view === 'cards') {
      onDraftChange(scratchSelected ? scratchDraft : null);
    } else {
      onDraftChange(contractDraft ?? scratchDraft);
    }
    return () => onDraftChange(null);
  }, [contractDraft, onDraftChange, scratchDraft, scratchSelected, view]);

  const handleBackToCards = () => {
    setView('cards');
    setScratchSelected(false);
    onApproachChange?.('scratch');
  };

  if (view === 'cards') {
    return (
      <Stack spacing={2}>
        <Stack direction={{ sm: 'row', xs: 'column' }} spacing={2} sx={{ alignItems: 'stretch' }}>

          {/* Scratch card — expands on click to show endpoint radio options */}
          <Box
            onClick={() => {
              if (!scratchSelected) {
                setScratchSelected(true);
                onApproachChange?.('scratch');
              }
            }}
            role={scratchSelected ? undefined : 'button'}
            sx={(theme) => ({
              border: '1px solid',
              borderColor: scratchSelected ? 'primary.main' : 'divider',
              borderRadius: 1,
              cursor: scratchSelected ? 'default' : 'pointer',
              flex: 1,
              p: 3,
              ...(scratchSelected
                ? { boxShadow: `0 0 0 1px ${theme.palette.primary.main}` }
                : {
                    '&:hover': {
                      borderColor: 'primary.main',
                      boxShadow: `0 0 0 1px ${theme.palette.primary.main}`,
                    },
                  }),
            })}
            tabIndex={scratchSelected ? -1 : 0}
            onKeyDown={(e) => {
              if (!scratchSelected && (e.key === 'Enter' || e.key === ' ')) {
                e.preventDefault();
                setScratchSelected(true);
                onApproachChange?.('scratch');
              }
            }}
          >
            <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
              <Box
                sx={{
                  alignItems: 'center',
                  bgcolor: 'action.hover',
                  borderRadius: 1,
                  display: 'flex',
                  flexShrink: 0,
                  justifyContent: 'center',
                  p: 1.25,
                }}
              >
                <Pencil size={18} />
              </Box>
              <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
                <Typography sx={{ fontWeight: 700 }} variant="body1">
                  <FormattedMessage {...messages.scratchTitle} />
                </Typography>
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.scratchDescription} />
                </Typography>
              </Stack>
            </Stack>

            {/* Endpoint radio — always visible */}
            <Box onClick={(e) => e.stopPropagation()} sx={{ mt: 2 }}>
              <FormControlLabel
                control={
                  <Radio
                    checked={endpointOption === 'custom'}
                    onClick={() => {
                      setScratchSelected(true);
                      setEndpointOption((prev) => (prev === 'custom' ? 'none' : 'custom'));
                      onApproachChange?.('scratch');
                    }}
                    size="small"
                  />
                }
                label={<FormattedMessage {...messages.scratchHasEndpoint} />}
              />
              {endpointOption === 'custom' && (
                <Box sx={{ mt: 1, pl: 3.5 }}>
                  <TextField
                    autoFocus
                    fullWidth
                    onChange={(e) => setEndpointUrl(e.target.value)}
                    placeholder={PLACEHOLDER_UPSTREAM_URL}
                    size="small"
                    value={endpointUrl}
                  />
                </Box>
              )}
            </Box>
          </Box>

          {/* Contract card */}
          <Box
            onClick={() => {
              setScratchSelected(false);
              setView('contract');
              onApproachChange?.('contract');
            }}
            role="button"
            sx={(theme) => ({
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              cursor: 'pointer',
              flex: 1,
              p: 3,
              '&:hover': {
                borderColor: 'primary.main',
                boxShadow: `0 0 0 1px ${theme.palette.primary.main}`,
              },
            })}
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                setScratchSelected(false);
                setView('contract');
                onApproachChange?.('contract');
              }
            }}
          >
            <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
              <Box
                sx={{
                  alignItems: 'center',
                  bgcolor: 'action.hover',
                  borderRadius: 1,
                  display: 'flex',
                  flexShrink: 0,
                  justifyContent: 'center',
                  p: 1.25,
                }}
              >
                <FileCode2 size={18} />
              </Box>
              <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
                <Typography sx={{ fontWeight: 700 }} variant="body1">
                  <FormattedMessage {...messages.contractTitle} />
                </Typography>
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.contractDescription} />
                </Typography>
              </Stack>
              <ChevronRight size={18} />
            </Stack>
          </Box>
        </Stack>
      </Stack>
    );
  }

  // Contract view
  return (
    <Stack spacing={2}>
      <Button
        onClick={handleBackToCards}
        startIcon={<ChevronLeft size={18} />}
        sx={{ alignSelf: 'flex-start' }}
        variant="text"
      >
        <FormattedMessage {...messages.changeSource} />
      </Button>

      <Box>
        <Typography sx={{ fontWeight: 700, mb: 0.5 }} variant="h2">
          <FormattedMessage {...messages.contractHeading} />
        </Typography>
        <Typography color="text.secondary" variant="body1">
          <FormattedMessage {...messages.contractSubheading} />
        </Typography>
      </Box>

      <Box
        sx={{
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          display: 'flex',
          flexDirection: { lg: 'row', xs: 'column' },
          overflow: 'hidden',
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
          <ContractSourceForm
            initialApiTypeKey={initialApiTypeKey}
            onContractChange={setContract}
          />
        </Box>

        <Divider
          flexItem
          orientation="vertical"
          sx={{
            borderBottomWidth: { lg: 0, xs: 'thin' },
            borderRightWidth: { lg: 'thin', xs: 0 },
          }}
        />

        <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
          <ApiResourcesPreview
            rawText={contract?.rawText}
            spec={contract?.spec}
          />
        </Box>
      </Box>

      {onContinue !== undefined ? (
        <Stack direction="row" sx={{ justifyContent: 'flex-end' }}>
          <Button
            endIcon={<ArrowRight size={18} />}
            onClick={() => onContinue()}
            variant="contained"
          >
            <FormattedMessage {...messages.continue} />
          </Button>
        </Stack>
      ) : null}
    </Stack>
  );
};
