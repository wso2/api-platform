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
  FormControl,
  FormLabel,
  InputAdornment,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { FileCode2, Link as LinkIcon, Pencil, Zap } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { DEFAULT_API_SKELETON, PLACEHOLDER_UPSTREAM_URL } from '../utils/apiSkeleton';
import type { ApiCreationWizardDraftState } from '../types';
import { extractApiDetails } from '../utils/specDetails';
import { ApiResourcesPreview } from './ApiResourcesPreview';
import { ContractSourceForm, type FetchedContract } from './ContractSourceForm';
import { GatewayIllustration } from '@/components/illustrations/GatewayIllustration';

type ApproachKey = 'contract' | 'scratch';

const messages = defineMessages({
  contractDescription: {
    id: 'api.create.defineApi.contract.description',
    defaultMessage: 'Import an API contract from a URL or a file.',
  },
  contractTitle: {
    id: 'api.create.defineApi.contract.title',
    defaultMessage: 'Start with a contract',
  },
  endpointDescription: {
    id: 'api.create.defineApi.scratch.endpoint.description',
    defaultMessage:
      'Route every resource to a running service. Calls are proxied through as soon as you publish.',
  },
  endpointLabel: {
    id: 'api.create.defineApi.scratch.endpoint.label',
    defaultMessage: 'Endpoint URL',
  },
  endpointHeading: {
    id: 'api.create.defineApi.scratch.endpoint.heading',
    defaultMessage: 'Backend endpoint',
  },
  endpointPreviewDescription: {
    id: 'api.create.defineApi.scratch.endpoint.preview.description',
    defaultMessage: 'Every API resource will route to the backend endpoint you provide.',
  },
  endpointPreviewTitle: {
    id: 'api.create.defineApi.scratch.endpoint.preview.title',
    defaultMessage: 'Ready to connect',
  },
  sampleUrl: {
    id: 'api.create.defineApi.scratch.endpoint.sampleUrl',
    defaultMessage: 'Try with Sample URL',
  },
  scratchDescription: {
    id: 'api.create.defineApi.scratch.description',
    defaultMessage: 'Begin with a blank API and fill in the details.',
  },
  scratchTitle: {
    id: 'api.create.defineApi.scratch.title',
    defaultMessage: 'Start from scratch',
  },
});

export type DefineApiPanelProps = {
  initialApiTypeKey?: string;
  onDraftChange: (data: ApiCreationWizardDraftState | null) => void;
  onApproachChange?: (approach: ApproachKey) => void;
  onContinue?: () => void;
};

type ApproachTabProps = {
  active: boolean;
  description: ReactNode;
  icon: ReactNode;
  onClick: () => void;
  title: ReactNode;
};

const SAMPLE_BACKEND_URL = 'https://apis.bijira.dev/samples/reading-list-api-service/v1.0/books';

const SampleLink = ({ onClick }: { onClick: () => void }) => (
  <Button
    onClick={onClick}
    size="small"
    startIcon={<Zap size={16} />}
    sx={{ alignSelf: 'flex-start', px: 0, textTransform: 'none' }}
    type="button"
    variant="text"
  >
    <FormattedMessage {...messages.sampleUrl} />
  </Button>
);

const ApproachTab = ({ active, description, icon, onClick, title }: ApproachTabProps) => (
  <Box
    aria-pressed={active}
    component="button"
    onClick={onClick}
    sx={{
      alignItems: 'center',
      bgcolor: active ? 'action.selected' : 'transparent',
      border: 1,
      borderColor: active ? 'primary.main' : 'divider',
      borderRadius: '8px 8px 0 0',
      color: 'text.primary',
      cursor: 'pointer',
      display: 'flex',
      flex: 1,
      gap: 1.5,
      minHeight: 68,
      px: 2,
      py: 1.25,
      textAlign: 'left',
    }}
    type="button"
  >
    <Box
      sx={{
        alignItems: 'center',
        bgcolor: active ? 'primary.main' : 'action.hover',
        borderRadius: 1,
        color: active ? 'primary.contrastText' : 'text.secondary',
        display: 'flex',
        flexShrink: 0,
        height: 40,
        justifyContent: 'center',
        width: 40,
      }}
    >
      {icon}
    </Box>
    <Stack spacing={0.25} sx={{ minWidth: 0 }}>
      <Typography sx={{ fontWeight: 700 }} variant="body1">
        {title}
      </Typography>
      <Typography color="text.secondary" sx={{ opacity: 0.65 }} variant="body2">
        {description}
      </Typography>
    </Stack>
  </Box>
);

export const DefineApiPanel = ({
  initialApiTypeKey,
  onDraftChange,
  onApproachChange,
}: DefineApiPanelProps) => {
  const intl = useIntl();
  const [approach, setApproach] = useState<ApproachKey>('scratch');
  const [endpointUrl, setEndpointUrl] = useState('');
  const [contract, setContract] = useState<FetchedContract | null>(null);

  const selectApproach = (next: ApproachKey) => {
    setApproach(next);
    onApproachChange?.(next);
  };

  const scratchDraft = useMemo((): ApiCreationWizardDraftState | null => {
    const upstreamUrl = endpointUrl.trim();
    if (!upstreamUrl) return null;

    const details = extractApiDetails(DEFAULT_API_SKELETON);
    const scratchRawText = JSON.stringify(DEFAULT_API_SKELETON, null, 2);
    return {
      ...details,
      upstream: {
        main: { url: upstreamUrl },
      },
      contractImport: {
        specFile: new File([scratchRawText], 'api_definition.json', { type: 'application/json' }),
      },
    };
  }, [endpointUrl]);

  const contractDraft = useMemo((): ApiCreationWizardDraftState | null => {
    if (contract?.spec === undefined) return null;
    const base = extractApiDetails(contract.spec);
    const rawText = contract.rawText;
    if (rawText !== undefined) {
      const isJson = rawText.trimStart().startsWith('{');
      const contentType = isJson ? 'application/json' : 'application/yaml';
      let fileName = contract.values.file?.name;
      if (!fileName) {
        fileName = isJson ? 'api_definition.json' : 'api_definition.yaml';
      }
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
    onDraftChange(approach === 'contract' ? contractDraft : scratchDraft);
    return () => onDraftChange(null);
  }, [approach, contractDraft, onDraftChange, scratchDraft]);

  return (
    <Box>
      <Stack
        direction={{ md: 'row', xs: 'column' }}
        sx={{
          '& > button + button': { ml: { md: '-1px', xs: 0 }, mt: { md: 0, xs: '-1px' } },
        }}
      >
        <ApproachTab
          active={approach === 'scratch'}
          description={<FormattedMessage {...messages.scratchDescription} />}
          icon={<Pencil size={20} />}
          onClick={() => selectApproach('scratch')}
          title={<FormattedMessage {...messages.scratchTitle} />}
        />
        <ApproachTab
          active={approach === 'contract'}
          description={<FormattedMessage {...messages.contractDescription} />}
          icon={<FileCode2 size={20} />}
          onClick={() => selectApproach('contract')}
          title={<FormattedMessage {...messages.contractTitle} />}
        />
      </Stack>
      <Stack
        direction={{ lg: 'row', xs: 'column' }}
        sx={{
          border: 1,
          borderColor: 'primary.main',
          borderRadius: '0 0 8px 8px',
          minHeight: 520,
          mt: '-1px',
          overflow: 'hidden',
        }}
      >
        {approach === 'contract' ? (
          <>
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
              <ApiResourcesPreview height={472} rawText={contract?.rawText} spec={contract?.spec} />
            </Box>
          </>
        ) : (
          <>
            <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
              <Stack spacing={2.5}>
                <Box>
                  <Typography sx={{ fontWeight: 700 }} variant="h3">
                    <FormattedMessage {...messages.endpointHeading} />
                  </Typography>
                  <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
                    <FormattedMessage {...messages.endpointDescription} />
                  </Typography>
                </Box>
                <FormControl fullWidth>
                  <FormLabel htmlFor="backend-endpoint">
                    <FormattedMessage {...messages.endpointLabel} />
                  </FormLabel>
                  <OutlinedInput
                    id="backend-endpoint"
                    onChange={(event) => setEndpointUrl(event.target.value)}
                    placeholder={PLACEHOLDER_UPSTREAM_URL}
                    startAdornment={
                      <InputAdornment position="start">
                        <LinkIcon size={18} />
                      </InputAdornment>
                    }
                    sx={{ mt: 0.75 }}
                    value={endpointUrl}
                  />
                  <SampleLink onClick={() => setEndpointUrl(SAMPLE_BACKEND_URL)} />
                </FormControl>
              </Stack>
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
              <Stack
                sx={{
                  alignItems: 'center',
                  // bgcolor: 'action.hover',
                  border: 1,
                  borderColor: 'divider',
                  borderRadius: 2,
                  height: '100%',
                  justifyContent: 'center',
                  minHeight: 420,
                  p: 3,
                  textAlign: 'center',
                }}
              >
                <GatewayIllustration />
                <Typography sx={{ fontWeight: 700, mt: 2 }} variant="body1">
                  {intl.formatMessage(messages.endpointPreviewTitle)}
                </Typography>
                <Typography color="text.secondary" sx={{ maxWidth: 360, mt: 0.5 }} variant="body2">
                  {intl.formatMessage(messages.endpointPreviewDescription)}
                </Typography>
              </Stack>
            </Box>
          </>
        )}
      </Stack>
    </Box>
  );
};
