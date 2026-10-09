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

import { Stack, Typography } from '@wso2/oxygen-ui';
import { FileCode2, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { DEFAULT_API_SKELETON, PLACEHOLDER_UPSTREAM_URL } from '../utils/apiSkeleton';
import type { ApiCreationWizardDraftState } from '../types';
import { extractApiDetails } from '../utils/specDetails';
import { ApiResourcesPreview } from './ApiResourcesPreview';
import { BackendEndpointField } from './BackendEndpointField';
import { ContractSourceForm, type FetchedContract } from './ContractSourceForm';
import { DefineApproachLayout, type DefineApproach } from './DefineApproachLayout';
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

const SAMPLE_BACKEND_URL = 'https://apis.bijira.dev/samples/reading-list-api-service/v1.0/books';

/** REST's approaches, in tab order. */
const APPROACHES: DefineApproach<ApproachKey>[] = [
  {
    description: <FormattedMessage {...messages.scratchDescription} />,
    icon: <Pencil size={20} />,
    key: 'scratch',
    title: <FormattedMessage {...messages.scratchTitle} />,
  },
  {
    description: <FormattedMessage {...messages.contractDescription} />,
    icon: <FileCode2 size={20} />,
    key: 'contract',
    title: <FormattedMessage {...messages.contractTitle} />,
  },
];

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
    if (rawText === undefined) return null;
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
  }, [contract]);

  useEffect(() => {
    onDraftChange(approach === 'contract' ? contractDraft : scratchDraft);
    return () => onDraftChange(null);
  }, [approach, contractDraft, onDraftChange, scratchDraft]);

  return (
    <DefineApproachLayout
      approaches={APPROACHES}
      form={
        approach === 'contract' ? (
          <ContractSourceForm initialApiTypeKey={initialApiTypeKey} onContractChange={setContract} />
        ) : (
          <BackendEndpointField
            description={<FormattedMessage {...messages.endpointDescription} />}
            heading={<FormattedMessage {...messages.endpointHeading} />}
            inputId="backend-endpoint"
            label={<FormattedMessage {...messages.endpointLabel} />}
            onChange={setEndpointUrl}
            onSample={() => setEndpointUrl(SAMPLE_BACKEND_URL)}
            placeholder={PLACEHOLDER_UPSTREAM_URL}
            sampleLabel={<FormattedMessage {...messages.sampleUrl} />}
            value={endpointUrl}
          />
        )
      }
      onChange={selectApproach}
      preview={
        approach === 'contract' ? (
          <ApiResourcesPreview height={472} rawText={contract?.rawText} spec={contract?.spec} />
        ) : (
          <Stack
            sx={{
              alignItems: 'center',
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
        )
      }
      value={approach}
    />
  );
};
