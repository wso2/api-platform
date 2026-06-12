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

import { useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from 'react-oidc-context';
import { Box, LinearProgress, Stack, Typography } from '@wso2/oxygen-ui';
import Logo from '../../Components/Logo';

export default function AutoLoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();

  // True when the browser has arrived here as the OAuth callback (?code=...).
  // In that case we wait for react-oidc-context to finish processing rather
  // than triggering a second signinRedirect.
  const isOAuthCallback = useMemo(
    () => new URLSearchParams(window.location.search).has('code'),
    [],
  );

  useEffect(() => {
    if (auth.isLoading) return;

    if (auth.isAuthenticated) {
      // Already signed in — navigate to app root and let PostSignInInit handle org routing.
      navigate('/', { replace: true });
      return;
    }

    if (!isOAuthCallback) {
      auth.signinRedirect();
    }
  }, [auth.isLoading, auth.isAuthenticated, isOAuthCallback, auth, navigate]);

  if (auth.error) {
    return (
      <Box
        sx={{
          display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center',
          height: '100vh', width: '100vw', gap: 3, px: 3,
        }}
      >
        <Logo height={44} />
        <Stack spacing={1} alignItems="center" sx={{ maxWidth: 400, textAlign: 'center' }}>
          <Typography variant="h6" fontWeight={700}>Sign-in failed</Typography>
          <Typography variant="body2" color="text.secondary">
            {auth.error.message || 'An unexpected error occurred during authentication.'}
          </Typography>
        </Stack>
        <Box
          component="button"
          onClick={() => auth.signinRedirect()}
          sx={{
            px: 3, py: 1, borderRadius: 1, border: 'none',
            bgcolor: 'primary.main', color: 'primary.contrastText',
            fontSize: '0.875rem', fontWeight: 600, cursor: 'pointer',
            '&:hover': { bgcolor: 'primary.dark' },
          }}
        >
          Try again
        </Box>
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
      <Typography variant="body2" color="text.secondary">
        Signing you in…
      </Typography>
    </Box>
  );
}
