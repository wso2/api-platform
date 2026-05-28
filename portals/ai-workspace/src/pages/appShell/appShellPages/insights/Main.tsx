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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Grid,
  PageContent,
  PageTitle,
  Skeleton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus } from '@wso2/oxygen-ui-icons-react';
import { useMoesif } from '../../../../contexts/MoesifContext';
import { MOESIF_WEB_URL } from '../../../../config.env';
import { FormattedMessage } from 'react-intl';
import { useNavigate } from 'react-router-dom';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import ErrorAlert from '../../../../Components/common/ErrorAlert';

type MoesifErrorPayload = {
  code?: number;
  message?: string;
};

const MOESIF_EMBEDDED_ORIGIN = new URL(MOESIF_WEB_URL).origin;

const EMBEDDED_POST_MESSAGE_TYPES = {
  ORG_LOAD_FINISHED: 'ORG_LOAD_FINISHED',
  SCHEMA_GEN_FINISHED: 'SCHEMA_GEN_FINISHED',
  SET_TOKEN: 'SET_TOKEN',
};

const IFRAME_SRC = `${MOESIF_WEB_URL}/wrap/basic/ai-overview?embedded_ui=true&isolated_section=true#auth=post`;

const getMoesifErrorPayload = (
  err: Error | null
): MoesifErrorPayload | null => {
  if (!err) return null;

  const responseData = (err as any)?.response?.data;
  if (responseData && typeof responseData === 'object') {
    return responseData as MoesifErrorPayload;
  }

  if (!err.message) return null;
  try {
    const parsed = JSON.parse(err.message);
    if (parsed && typeof parsed === 'object') {
      return parsed as MoesifErrorPayload;
    }
  } catch {
    // Ignore non-JSON messages
  }

  return null;
};

export default function Insights(): JSX.Element {
  const navigate = useNavigate();
  const { currentOrganization, setCurrentProject } = useAppShell();
  const { moesifToken, isLoading, error, fetchMoesifToken } = useMoesif();
  const moesifError = getMoesifErrorPayload(error);
  const isProvisioningError =
    moesifError?.code === 404 ||
    /Moesif organization or app not found/i.test(
      moesifError?.message || error?.message || ''
    );

  const iframeRef = useRef<HTMLIFrameElement>(null);
  const tokenRef = useRef<string | null>(null);
  const [isIframeDomLoaded, setIsIframeDomLoaded] = useState(false);
  const [isIframeLoading, setIsIframeLoading] = useState(true);

  useEffect(() => {
    tokenRef.current = moesifToken?.token ?? null;
  }, [moesifToken]);

  const sendTokenToChild = useCallback(() => {
    if (iframeRef.current && moesifToken?.token) {
      iframeRef.current.contentWindow?.postMessage(
        { type: EMBEDDED_POST_MESSAGE_TYPES.SET_TOKEN, token: moesifToken.token },
        MOESIF_EMBEDDED_ORIGIN
      );
    }
  }, [moesifToken]);

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.origin !== MOESIF_EMBEDDED_ORIGIN) return;
      if (event.data?.type === EMBEDDED_POST_MESSAGE_TYPES.SCHEMA_GEN_FINISHED) {
        setIsIframeLoading(false);
      }
    };
    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, []);

  useEffect(() => {
    if (isIframeDomLoaded && moesifToken?.token) {
      sendTokenToChild();
    }
  }, [moesifToken, isIframeDomLoaded, sendTokenToChild]);

  const handleGoToGateways = () => {
    // Force org-level scope before opening gateways.
    setCurrentProject(null);
    navigate(buildOrgPath(currentOrganization, '/gateways'));
  };

  const handleRetryInsights = () => {
    fetchMoesifToken().catch(() => {
      // Error state is handled by context.
    });
  };

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }}>
          <PageTitle>
            <PageTitle.Header>Insights</PageTitle.Header>
            <PageTitle.SubHeader>
              Explore usage analytics and traffic insights powered by Moesif.
            </PageTitle.SubHeader>
          </PageTitle>
        </Grid>

        <Grid size={{ xs: 12 }}>
          {isLoading ? (
            <Typography variant="body2">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.insights.Main.loading.insights"
                defaultMessage="Loading insights..."
              />
            </Typography>
          ) : isProvisioningError ? (
            <Alert
              severity="warning"
              sx={{
                '& .MuiAlert-icon': {
                  alignItems: 'center',
                },
                '& .MuiAlert-message': {
                  width: '100%',
                },
              }}
            >
              <Stack
                direction={{ xs: 'column', sm: 'row' }}
                spacing={{ xs: 2, sm: 3 }}
                alignItems={{ xs: 'flex-start', sm: 'center' }}
                justifyContent="space-between"
                sx={{ width: '100%' }}
              >
                <Typography variant="body2" sx={{ flex: 1 }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.insights.Main.provisioning.required.message"
                    defaultMessage="You need to add and configure an AI Gateway to enable Insights analytics for this organization."
                  />
                </Typography>
                <Button
                  variant="contained"
                  size="small"
                  onClick={handleGoToGateways}
                  sx={{ width: { xs: '100%', sm: 'auto' } }}
                  startIcon={<Plus size={16} />}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.insights.Main.provisioning.required.add.gateway"
                    defaultMessage="Add AI Gateway"
                  />
                </Button>
              </Stack>
            </Alert>
          ) : error ? (
            <ErrorAlert error={error} onRetry={handleRetryInsights} />
          ) : (
            <Box
              sx={{
                width: '100%',
                minHeight: { xs: '60vh', md: '70vh' },
                // borderRadius: 2,
                overflow: 'hidden',
                border: '1px solid',
                borderColor: 'divider',
                backgroundColor: 'background.paper',
                position: 'relative',
              }}
            >
              {isIframeLoading && (
                <Box
                  sx={{
                    position: 'absolute',
                    inset: 0,
                    p: 2,
                    backgroundColor: 'background.paper',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 2,
                  }}
                >
                  {/* Metric cards row */}
                  <Box sx={{ display: 'flex', gap: 2 }}>
                    {Array.from({ length: 5 }).map((_, i) => (
                      <Skeleton
                        key={i}
                        variant="rectangular"
                        sx={{ flex: 1, height: 120, borderRadius: 2, transform: 'none' }}
                      />
                    ))}
                  </Box>
                  {/* Chart panels row */}
                  <Box sx={{ display: 'flex', gap: 2, flex: 1 }}>
                    <Skeleton
                      variant="rectangular"
                      sx={{ flex: 1, borderRadius: 2, transform: 'none' }}
                    />
                    <Skeleton
                      variant="rectangular"
                      sx={{ flex: 1, borderRadius: 2, transform: 'none' }}
                    />
                  </Box>
                </Box>
              )}
              <Box
                ref={iframeRef}
                component="iframe"
                title="Moesif Insights"
                src={IFRAME_SRC}
                onLoad={() => {
                  setIsIframeDomLoaded(true);
                  const currentToken = tokenRef.current;
                  if (currentToken) {
                    iframeRef.current?.contentWindow?.postMessage(
                      { type: EMBEDDED_POST_MESSAGE_TYPES.SET_TOKEN, token: currentToken },
                      MOESIF_EMBEDDED_ORIGIN
                    );
                  }
                }}
                sx={{
                  width: '100%',
                  height: '100%',
                  minHeight: { xs: '60vh', md: '70vh' },
                  border: 0,
                  display: isIframeLoading ? 'none' : 'block',
                }}
              />
            </Box>
          )}
        </Grid>
      </Grid>
    </PageContent>
  );
}
