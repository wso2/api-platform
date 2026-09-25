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

import React, { useState } from 'react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Eye, EyeOff, HelpCircle, Handshake } from '@wso2/oxygen-ui-icons-react';
import McpMenuIcon from '../../../../../assets/icons/McpMenuIcon';
import type { EndpointValidationResponse } from '../../externalServers/externalServersValidationTypes';
import ExternalServersValidationDetails from '../../externalServers/ExternalServersValidationDetails';

type ResourceType = 'provider' | 'mcp';

export const UNREACHABLE_URL_ERROR = 'URL is unreachable';

const SAMPLE_MCP_SERVER_URL =
  'https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/mcp-everything-server/v1.0/mcp';

type EnterEndpointStepProps = {
  role: string;
  resourceType: ResourceType;
  onResourceTypeChange: (type: ResourceType) => void;
  endpointUrl: string;
  authHeaderName: string;
  authHeaderValue: string;
  validationResult: EndpointValidationResponse | null;
  validationError: string | null;
  isValidating: boolean;
  lastValidatedUrl: string;
  onEndpointUrlChange: (url: string) => void;
  onAuthHeaderNameChange: (name: string) => void;
  onAuthHeaderValueChange: (value: string) => void;
  onValidationResultChange: (result: EndpointValidationResponse | null) => void;
  onValidationErrorChange: (error: string | null) => void;
  onLastValidatedUrlChange: (url: string) => void;
  onValidate: (url: string) => Promise<void>;
};

export default function EnterEndpointStep({
  role,
  resourceType,
  onResourceTypeChange,
  endpointUrl,
  authHeaderName,
  authHeaderValue,
  validationResult,
  validationError,
  isValidating,
  lastValidatedUrl,
  onEndpointUrlChange,
  onAuthHeaderNameChange,
  onAuthHeaderValueChange,
  onValidationResultChange,
  onValidationErrorChange,
  onLastValidatedUrlChange,
  onValidate,
}: EnterEndpointStepProps) {
  const [showAuthHeaderValue, setShowAuthHeaderValue] = useState(false);

  const handleEndpointChange = (value: string) => {
    onEndpointUrlChange(value);
    if (value.trim() !== lastValidatedUrl) {
      onValidationErrorChange(null);
      onValidationResultChange(null);
    }
  };

  const handleTrySampleUrl = () => {
    onEndpointUrlChange(SAMPLE_MCP_SERVER_URL);
    void onValidate(SAMPLE_MCP_SERVER_URL);
  };

  const hasResult = validationResult !== null || validationError === UNREACHABLE_URL_ERROR;

  return (
    <Stack spacing={2.5}>
      <Box>
        <Typography variant="body2" sx={{ mb: 1.25, fontWeight: 600 }}>
          Resource Type
        </Typography>
        <ToggleButtonGroup
          size="medium"
          exclusive
          value={resourceType}
          onChange={(_, value: ResourceType | null) => {
            if (!value) return;
            onResourceTypeChange(value);
          }}
          sx={{
            borderRadius: '6px',
            '& .MuiToggleButtonGroup-grouped': {
              '&:first-of-type': { borderRadius: '6px 0 0 6px' },
              '&:last-of-type': { borderRadius: '0 6px 6px 0' },
            },
            '& .MuiToggleButton-root.Mui-selected': {
              backgroundColor: 'rgba(234, 106, 51, 0.1)',
              color: 'warning.main',
              '&:hover': { backgroundColor: 'rgba(234, 106, 51, 0.16)' },
            },
          }}
        >
          <Tooltip
            title={role === 'developer' ? 'Switch to the admin role to add LLM providers.' : ''}
            disableHoverListener={role !== 'developer'}
          >
            <Box component="span">
              <ToggleButton
                value="provider"
                disabled={role === 'developer'}
                sx={{
                  gap: 1,
                  px: 2,
                  textTransform: 'none',
                  justifyContent: 'flex-start',
                  borderTopLeftRadius: '6px !important',
                  borderBottomLeftRadius: '6px !important',
                  borderTopRightRadius: '0 !important',
                  borderBottomRightRadius: '0 !important',
                }}
              >
                <Handshake size={18} />
                LLM Provider
              </ToggleButton>
            </Box>
          </Tooltip>
          <ToggleButton
            value="mcp"
            sx={{ gap: 1, px: 2, textTransform: 'none', justifyContent: 'flex-start' }}
          >
            <McpMenuIcon size={18} aria-hidden />
            MCP Server
          </ToggleButton>
        </ToggleButtonGroup>
      </Box>

      <Grid container spacing={2} sx={{ alignItems: 'flex-start' }}>
      <Grid size={{ xs: 12, md: hasResult ? 5 : 7 }}>
        <Stack spacing={1.5}>
          <FormControl fullWidth>
            <FormLabel>MCP Proxy Endpoint URL</FormLabel>
            <TextField
              fullWidth
              placeholder="Enter URL of Your MCP Proxy"
              value={endpointUrl}
              onChange={(event) => handleEndpointChange(event.target.value)}
              slotProps={{
                input: {
                  endAdornment: isValidating ? (
                    <InputAdornment position="end">
                      <CircularProgress size={18} />
                    </InputAdornment>
                  ) : null,
                },
              }}
            />
          </FormControl>

          <Button
            variant="text"
            onClick={handleTrySampleUrl}
            sx={{ alignSelf: 'flex-start', px: 0, minWidth: 'auto', py: 0 }}
          >
            Try with Sample URL
          </Button>

          <Accordion sx={{ borderRadius: 1, '&:before': { display: 'none' } }}>
            <AccordionSummary expandIcon={<ChevronDown size={18} />}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Typography sx={{ fontWeight: 500, fontSize: '0.875rem' }}>
                  Advanced Configurations
                </Typography>
                <Tooltip title="If the MCP Proxy is protected, provide security credentials to authenticate with the server.">
                  <HelpCircle size={16} />
                </Tooltip>
              </Stack>
            </AccordionSummary>
            <AccordionDetails>
              <Stack spacing={1.5}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, fontSize: '0.8125rem' }}>
                  Configure Authentication Header
                </Typography>
                <Grid container spacing={1.5}>
                  <Grid size={{ xs: 12, sm: 6 }}>
                    <FormControl fullWidth>
                      <FormLabel>Header</FormLabel>
                      <TextField
                        fullWidth
                        placeholder="Header"
                        value={authHeaderName}
                        onChange={(event) => onAuthHeaderNameChange(event.target.value)}
                      />
                    </FormControl>
                  </Grid>
                  <Grid size={{ xs: 12, sm: 6 }}>
                    <FormControl fullWidth>
                      <FormLabel>Value</FormLabel>
                      <TextField
                        fullWidth
                        placeholder="Value"
                        type={showAuthHeaderValue ? 'text' : 'password'}
                        value={authHeaderValue}
                        onChange={(event) => onAuthHeaderValueChange(event.target.value)}
                        slotProps={{
                          input: {
                            endAdornment: (
                              <InputAdornment position="end">
                                <IconButton
                                  size="small"
                                  onClick={() => setShowAuthHeaderValue((prev) => !prev)}
                                  aria-label={showAuthHeaderValue ? 'Hide header value' : 'Show header value'}
                                >
                                  {showAuthHeaderValue ? <EyeOff size={18} /> : <Eye size={18} />}
                                </IconButton>
                              </InputAdornment>
                            ),
                          },
                        }}
                      />
                    </FormControl>
                  </Grid>
                </Grid>
              </Stack>
            </AccordionDetails>
          </Accordion>

          {validationError && validationError !== UNREACHABLE_URL_ERROR ? (
            <Alert severity="error">{validationError}</Alert>
          ) : null}
          {validationError === UNREACHABLE_URL_ERROR ? (
            <Alert severity="info">
              AI Workspace cannot reach this server URL, but the gateway may still be able to.
              You can proceed and add capabilities manually.
            </Alert>
          ) : null}

          {!hasResult ? (
            <Button
              variant="contained"
              disabled={!endpointUrl.trim() || isValidating}
              onClick={() => void onValidate(endpointUrl.trim())}
              sx={{ alignSelf: 'flex-start' }}
            >
              {isValidating ? 'Validating...' : 'Fetch Server Info'}
            </Button>
          ) : null}
        </Stack>
      </Grid>

      {validationResult ? (
        <Grid size={{ xs: 12, md: 7 }}>
          <ExternalServersValidationDetails validationResult={validationResult} />
        </Grid>
      ) : null}
      </Grid>
    </Stack>
  );
}
