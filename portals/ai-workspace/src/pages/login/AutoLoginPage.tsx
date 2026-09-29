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

import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Box, Button, LinearProgress, Stack, Typography } from '@wso2/oxygen-ui';
import Logo from '../../Components/Logo';
import { useAppAuth } from '../../contexts/AppAuthContext';

// Messages for the ?error= reasons the BFF's OIDC callback redirects here with (see
// the loginErr* constants in internal/server/handlers.go).
//
// Looked up, never echoed. One of those reasons is a code the IDP itself chose, so
// rendering the parameter directly would reflect upstream input into the page; an
// unrecognised reason gets the generic message instead.
const LOGIN_ERROR_MESSAGES: Record<string, string> = {
  auth_failed: 'We could not complete your sign-in. Please try again.',
  session_failed: 'We could not start your session. Please try again.',
  token_exchange_rejected:
    'Your account could not be granted access to this workspace. If you believe this is a '
    + 'mistake, contact your administrator.',
  upstream_unavailable:
    'Sign-in is temporarily unavailable. This is usually brief — please try again in a moment.',
};
const GENERIC_LOGIN_ERROR = 'We could not complete your sign-in. Please try again.';

// In BFF mode the OAuth handshake is owned by the server (/api/auth/login →
// /api/auth/callback). This page just kicks off that redirect for OIDC, or sends
// already-authenticated users back to the app.
export default function AutoLoginPage() {
  const { isAuthenticated, isLoading, login } = useAppAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const errorReason = searchParams.get('error');

  useEffect(() => {
    if (isLoading) return;
    if (isAuthenticated) {
      navigate('/', { replace: true });
      return;
    }
    // An ?error= means we have just come BACK from a callback that failed. Starting
    // the redirect again here is what turns one failure into a loop: the IDP still
    // has a live session, so it redirects straight back, the callback fails the same
    // way, and we land here again — hammering both the IDP and the BFF until
    // something gives. Wait for the user instead.
    if (errorReason) return;
    void login();
  }, [isLoading, isAuthenticated, errorReason, login, navigate]);

  if (errorReason) {
    return (
      <Box
        sx={{
          display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center',
          height: '100vh', width: '100vw', gap: 4,
        }}
      >
        <Logo height={48} />
        <Stack spacing={2} alignItems="center" sx={{ maxWidth: 420, textAlign: 'center' }}>
          <Typography variant="body1">
            {LOGIN_ERROR_MESSAGES[errorReason] ?? GENERIC_LOGIN_ERROR}
          </Typography>
          <Button variant="contained" onClick={() => { void login(); }}>
            Try again
          </Button>
        </Stack>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: 'flex', flexDirection: 'column',
        alignItems: 'center', justifyContent: 'center',
        height: '100vh', width: '100vw', gap: 4,
      }}
    >
      <Logo height={48} />
      <Box sx={{ width: 200 }}>
        <LinearProgress color="primary" />
      </Box>
      <Stack spacing={1} alignItems="center">
        <Typography variant="body2" color="text.secondary">
          Signing you in…
        </Typography>
      </Stack>
    </Box>
  );
}
