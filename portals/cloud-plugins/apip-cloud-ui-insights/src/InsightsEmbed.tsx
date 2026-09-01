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

import { Box, PageTitle } from '@wso2/oxygen-ui';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type FC,
} from 'react';

import { fetchViewerToken } from './api/analyticsApi';
import { ErrorState, LoadingState } from './components/StateViews';
import { insightsRuntimeConfig } from './config/runtimeConfig';
import type { InsightsEmbedScope } from './types';
import {
  MOESIF_EMBEDDED_POST_MESSAGE_TYPES,
  buildBasicIframeSrc,
  buildBasicProjectIframeSrc,
  resolveMoesifEmbeddingOrigin,
} from './utils/moesifEmbed';

/** Match choreo-console ProtectedRoute: refresh before Moesif viewer token expiry (~1h). */
const VIEWER_TOKEN_REFRESH_INTERVAL_MS = 50 * 60 * 1000;
/** Stop waiting for Moesif ORG_LOAD_FINISHED / SCHEMA_GEN_FINISHED after this. */
const EMBED_HANDSHAKE_TIMEOUT_MS = 60_000;

export type InsightsEmbedProps = {
  scope: InsightsEmbedScope;
};

/**
 * Embeds Moesif Insights for organization or project scope via wrap/basic.
 *
 * Viewer tokens are fetched from WSO2 Cloud platform-api-service
 * (`/cloud/analytics/id-token`), not from Moesif directly.
 *
 * Does not wrap in `PageContent` — API Control Plane's AppLayout already
 * provides that around the route outlet.
 */
const InsightsEmbed: FC<InsightsEmbedProps> = ({ scope }) => {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const tokenRef = useRef<string | null>(null);

  const moesifAppUrl = insightsRuntimeConfig.moesifAppUrl;
  const embeddingOrigin = resolveMoesifEmbeddingOrigin(moesifAppUrl);

  const iframeSrc = useMemo(() => {
    if (scope.level === 'project') {
      return buildBasicProjectIframeSrc(moesifAppUrl, scope.projectId || '');
    }
    return buildBasicIframeSrc(moesifAppUrl);
  }, [moesifAppUrl, scope.level, scope.projectId]);

  const [viewerToken, setViewerToken] = useState<string | null>(null);
  const [tokenError, setTokenError] = useState<string | null>(null);
  const [tokenLoading, setTokenLoading] = useState(true);
  const [isIframeDomLoaded, setIsIframeDomLoaded] = useState(false);
  const [isEmbedReady, setIsEmbedReady] = useState(false);
  const [handshakeError, setHandshakeError] = useState<string | null>(null);
  const [embedAttempt, setEmbedAttempt] = useState(0);

  const pageSubheader = useMemo(() => {
    if (scope.level === 'project') {
      return scope.projectName
        ? `Usage analytics for ${scope.projectName} powered by Moesif.`
        : 'Usage analytics for APIs in this project powered by Moesif.';
    }
    return 'Explore organization-wide usage analytics and traffic insights powered by Moesif.';
  }, [scope.level, scope.projectName]);

  useEffect(() => {
    tokenRef.current = viewerToken;
  }, [viewerToken]);

  useEffect(() => {
    setIsIframeDomLoaded(false);
    setIsEmbedReady(false);
    setHandshakeError(null);
  }, [iframeSrc, scope.level]);

  const retryHandshake = useCallback(() => {
    setHandshakeError(null);
    setIsEmbedReady(false);
    setIsIframeDomLoaded(false);
    setEmbedAttempt((attempt) => attempt + 1);
  }, []);

  const mintToken = useCallback(async () => fetchViewerToken(), []);

  useEffect(() => {
    let cancelled = false;
    setTokenLoading(true);
    setTokenError(null);

    mintToken()
      .then((token) => {
        if (!cancelled) setViewerToken(token);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setTokenError(err instanceof Error ? err.message : String(err));
        }
      })
      .finally(() => {
        if (!cancelled) setTokenLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [mintToken]);

  // choreo-console refreshes in ProtectedRoute on a 50-minute interval and
  // re-posts SET_TOKEN when moesifIdToken changes. We mint here because the
  // token is scoped to this embed rather than app-wide context.
  useEffect(() => {
    if (!viewerToken) return;

    const intervalId = window.setInterval(() => {
      mintToken()
        .then((token) => setViewerToken(token))
        .catch(() => undefined);
    }, VIEWER_TOKEN_REFRESH_INTERVAL_MS);

    return () => window.clearInterval(intervalId);
  }, [mintToken, viewerToken]);

  const sendTokenToChild = useCallback(() => {
    if (!iframeRef.current?.contentWindow || !viewerToken) return;
    iframeRef.current.contentWindow.postMessage(
      { type: MOESIF_EMBEDDED_POST_MESSAGE_TYPES.SET_TOKEN, token: viewerToken },
      embeddingOrigin
    );
  }, [embeddingOrigin, viewerToken]);

  useEffect(() => {
    if (isIframeDomLoaded && viewerToken) sendTokenToChild();
  }, [isIframeDomLoaded, sendTokenToChild, viewerToken]);

  useEffect(() => {
    if (
      tokenLoading ||
      tokenError ||
      handshakeError ||
      !viewerToken ||
      !iframeSrc ||
      isEmbedReady
    ) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setHandshakeError(
        'Moesif Insights did not finish loading in time. Check your network connection and try again.'
      );
    }, EMBED_HANDSHAKE_TIMEOUT_MS);

    return () => window.clearTimeout(timeoutId);
  }, [
    embedAttempt,
    handshakeError,
    iframeSrc,
    isEmbedReady,
    tokenError,
    tokenLoading,
    viewerToken,
  ]);

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.origin !== embeddingOrigin) return;
      const iframeWindow = iframeRef.current?.contentWindow;
      if (!iframeWindow || event.source !== iframeWindow) return;

      switch (event.data?.type) {
        case MOESIF_EMBEDDED_POST_MESSAGE_TYPES.SCHEMA_GEN_FINISHED:
        case MOESIF_EMBEDDED_POST_MESSAGE_TYPES.ORG_LOAD_FINISHED:
          setHandshakeError(null);
          setIsEmbedReady(true);
          break;
        case MOESIF_EMBEDDED_POST_MESSAGE_TYPES.REFRESH_TOKEN:
          mintToken()
            .then((token) => setViewerToken(token))
            .catch(() => undefined);
          break;
        default:
          break;
      }
    };
    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, [embeddingOrigin, mintToken]);

  const showLoader =
    tokenLoading ||
    (!tokenError &&
      !handshakeError &&
      !isEmbedReady &&
      Boolean(viewerToken && iframeSrc));

  const iframeStyle: CSSProperties = {
    backgroundColor: 'transparent',
    border: 'none',
    display: 'block',
    height: '100%',
    width: '100%',
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        // Single scroll surface: constrain the whole view (title + iframe) so the
        // shell page does not scroll separately from Moesif inside the iframe.
        height: 'calc(100dvh - 13rem)',
        minHeight: 480,
        overflow: 'hidden',
        width: '100%',
      }}
    >
      <Box sx={{ flexShrink: 0 }}>
        <PageTitle>
          <PageTitle.Header>Insights</PageTitle.Header>
          <PageTitle.SubHeader>{pageSubheader}</PageTitle.SubHeader>
        </PageTitle>
      </Box>

      {tokenError ? (
        <ErrorState message={tokenError} title="Unable to load Insights" />
      ) : handshakeError ? (
        <ErrorState
          actionLabel="Retry"
          message={handshakeError}
          onAction={retryHandshake}
          title="Unable to load Insights"
        />
      ) : (
        <Box
          id="insights-landing"
          sx={{
            flex: 1,
            minHeight: 0,
            overflow: 'hidden',
            overscrollBehavior: 'contain',
            position: 'relative',
            width: '100%',
          }}
        >
          {showLoader && (
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                inset: 0,
                justifyContent: 'center',
                position: 'absolute',
                width: '100%',
                zIndex: 1,
              }}
            >
              <LoadingState label="Loading Insights" />
            </Box>
          )}
          {viewerToken && iframeSrc && (
            <iframe
              key={`${scope.level}-${iframeSrc}-${embedAttempt}`}
              ref={iframeRef}
              src={iframeSrc}
              title="Moesif Insights"
              allowFullScreen
              onLoad={() => {
                setIsIframeDomLoaded(true);
                const currentToken = tokenRef.current;
                if (currentToken && iframeRef.current?.contentWindow) {
                  iframeRef.current.contentWindow.postMessage(
                    {
                      type: MOESIF_EMBEDDED_POST_MESSAGE_TYPES.SET_TOKEN,
                      token: currentToken,
                    },
                    embeddingOrigin
                  );
                }
              }}
              style={iframeStyle}
            />
          )}
        </Box>
      )}
    </Box>
  );
};

export default InsightsEmbed;
