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
  FormHelperText,
  FormLabel,
  InputAdornment,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { FileCode2, Link as LinkIcon, Pencil, Zap } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { isHttpUrl } from '../../utils/basicInfoRules';
import { DEFAULT_API_SKELETON, skeletonFor } from '../utils/apiSkeleton';
import { nameFromEndpoint, versionFromEndpoint } from '../utils/nameFromEndpoint';
import type { ApiCreationWizardDraftState } from '../types';
import { extractApiDetails } from '../utils/specDetails';
import { ApiResourcesPreview } from './ApiResourcesPreview';
import { ContractSourceForm, type FetchedContract } from './ContractSourceForm';
import { GatewayIllustration } from '@/components/illustrations/GatewayIllustration';

type ApproachKey = 'contract' | 'scratch';

const messages = defineMessages({
  contractDescription: {
    id: 'api.create.defineApi.contract.description',
    defaultMessage: 'Import an OpenAPI spec from a URL or a file.',
  },
  contractTitle: {
    id: 'api.create.defineApi.contract.title',
    defaultMessage: 'From an OpenAPI spec',
  },
  endpointDescription: {
    id: 'api.create.defineApi.scratch.endpoint.description',
    defaultMessage:
      'Route every resource to a running service. Calls are proxied through as soon as you deploy it to a gateway.',
  },
  endpointHint: {
    id: 'api.create.defineApi.scratch.endpoint.hint',
    defaultMessage: 'Your API’s base URL. We forward traffic to it; we don’t call it now.',
    description: 'Always-visible line under the backend URL field.',
  },
  endpointInvalid: {
    id: 'api.create.defineApi.scratch.endpoint.invalid',
    defaultMessage:
      'That doesn’t look like a URL yet. Check for a typo, or a missing https:// at the start.',
  },
  endpointLabel: {
    id: 'api.create.defineApi.scratch.endpoint.label',
    defaultMessage: 'Backend URL',
  },
  endpointHeading: {
    id: 'api.create.defineApi.scratch.endpoint.heading',
    defaultMessage: 'Backend endpoint',
  },
  endpointPreviewDescription: {
    id: 'api.create.defineApi.scratch.endpoint.preview.description',
    defaultMessage: 'Every API resource will route to the backend endpoint you provide.',
  },
  endpointPreviewForwardsTo: {
    id: 'api.create.defineApi.scratch.endpoint.preview.forwardsTo',
    defaultMessage: 'FORWARDS TO',
    description: 'Small caps label above the backend URL in the routes preview.',
  },
  endpointPreviewNote: {
    id: 'api.create.defineApi.scratch.endpoint.preview.note',
    defaultMessage:
      'One catch-all route per method. Requests to any path are forwarded unchanged; without a spec we can’t list your API’s paths individually. Add a spec later whenever you want that.',
  },
  endpointPreviewTitle: {
    id: 'api.create.defineApi.scratch.endpoint.preview.title',
    defaultMessage: 'Ready to connect',
  },
  sampleUrl: {
    id: 'api.create.defineApi.scratch.endpoint.sampleUrl',
    defaultMessage: 'Try a sample',
  },
  scratchDescription: {
    id: 'api.create.defineApi.scratch.description',
    defaultMessage: 'Proxy a running service. Every path is forwarded as-is.',
  },
  scratchTitle: {
    id: 'api.create.defineApi.scratch.title',
    defaultMessage: 'From an endpoint',
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

/** A bare example, not instructions: placeholders vanish on focus. */
const ENDPOINT_PLACEHOLDER = 'https://api.example.com/v1';

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
  // The endpoint is checked for shape only; the console never calls it. The
  // error waits until the user leaves the field, so a half-typed URL never
  // flashes red, and clears the moment the value parses.
  const [endpointTouched, setEndpointTouched] = useState(false);
  const endpointValid = isHttpUrl(endpointUrl.trim());
  const endpointError = endpointTouched && endpointUrl.trim() !== '' && !endpointValid;
  const [contract, setContract] = useState<FetchedContract | null>(null);

  const selectApproach = (next: ApproachKey) => {
    setApproach(next);
    onApproachChange?.(next);
  };

  const scratchDraft = useMemo((): ApiCreationWizardDraftState | null => {
    const upstreamUrl = endpointUrl.trim();
    if (!isHttpUrl(upstreamUrl)) return null;

    const details = extractApiDetails(DEFAULT_API_SKELETON);
    const scratchRawText = JSON.stringify(DEFAULT_API_SKELETON, null, 2);
    // Name and version come from the URL where it says them, so an API made
    // from `…/orders/v2` starts as "Orders" 2.0.0 rather than "Untitled API".
    const guessedName = nameFromEndpoint(upstreamUrl);
    const guessedVersion = versionFromEndpoint(upstreamUrl);
    return {
      ...details,
      ...(guessedName ? { displayName: guessedName } : {}),
      ...(guessedVersion ? { version: guessedVersion } : {}),
      upstream: {
        main: { url: upstreamUrl },
      },
      contractImport: {
        specFile: new File([scratchRawText], 'api_definition.json', { type: 'application/json' }),
        // Rebuilt at submit from the details step's values (`skeletonFor`).
        fromSkeleton: true,
      },
    };
  }, [endpointUrl]);

  // The definition an endpoint API would be created with, for the preview.
  // Built only once the URL parses: before that the pane explains itself.
  const endpointPreview = useMemo(() => {
    if (scratchDraft === null) return null;
    const spec = skeletonFor({
      displayName: scratchDraft.displayName ?? 'Untitled API',
      upstreamUrl: endpointUrl.trim(),
      version: scratchDraft.version ?? '1.0.0',
    });
    return { rawText: JSON.stringify(spec, null, 2), spec };
  }, [endpointUrl, scratchDraft]);

  const contractDraft = useMemo((): ApiCreationWizardDraftState | null => {
    if (contract?.spec === undefined) return null;
    // A URL-sourced spec resolves a relative server against its own address.
    const base = extractApiDetails(contract.spec, contract.values.url);
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
                <FormControl error={endpointError} fullWidth required>
                  <FormLabel htmlFor="backend-endpoint">
                    <FormattedMessage {...messages.endpointLabel} />
                  </FormLabel>
                  <OutlinedInput
                    aria-describedby="backend-endpoint-hint"
                    id="backend-endpoint"
                    onBlur={() => setEndpointTouched(true)}
                    onChange={(event) => setEndpointUrl(event.target.value)}
                    placeholder={ENDPOINT_PLACEHOLDER}
                    startAdornment={
                      <InputAdornment position="start">
                        <LinkIcon size={18} />
                      </InputAdornment>
                    }
                    sx={{ mt: 0.75 }}
                    value={endpointUrl}
                  />
                  <FormHelperText id="backend-endpoint-hint">
                    <FormattedMessage
                      {...(endpointError ? messages.endpointInvalid : messages.endpointHint)}
                    />
                  </FormHelperText>
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
              {endpointPreview ? (
                // What will be created, drawn by the same preview a spec
                // uses: the forwarding target once, then the catch-all routes.
                <Stack spacing={1.5}>
                  <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1.5 }}>
                    <Typography
                      color="text.secondary"
                      sx={{ fontWeight: 600, letterSpacing: 0.4 }}
                      variant="caption"
                    >
                      <FormattedMessage {...messages.endpointPreviewForwardsTo} />
                    </Typography>
                    <Typography
                      sx={{ fontFamily: 'monospace', overflowWrap: 'anywhere' }}
                      variant="body2"
                    >
                      {endpointUrl.trim()}
                    </Typography>
                  </Box>
                  <ApiResourcesPreview
                    height={360}
                    rawText={endpointPreview.rawText}
                    spec={endpointPreview.spec}
                  />
                  <Typography color="text.secondary" variant="body2">
                    <FormattedMessage {...messages.endpointPreviewNote} />
                  </Typography>
                </Stack>
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
                  <Typography
                    color="text.secondary"
                    sx={{ maxWidth: 360, mt: 0.5 }}
                    variant="body2"
                  >
                    {intl.formatMessage(messages.endpointPreviewDescription)}
                  </Typography>
                </Stack>
              )}
            </Box>
          </>
        )}
      </Stack>
    </Box>
  );
};
