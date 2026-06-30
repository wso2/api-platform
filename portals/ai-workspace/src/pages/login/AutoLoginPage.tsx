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
import { useNavigate } from 'react-router-dom';
import { Box, LinearProgress, Stack, Typography } from '@wso2/oxygen-ui';
import Logo from '../../Components/Logo';
import { useAppAuth } from '../../contexts/AppAuthContext';

// In BFF mode the OAuth handshake is owned by the server (/api/auth/login →
// /api/auth/callback). This page just kicks off that redirect for OIDC, or sends
// already-authenticated users back to the app.
export default function AutoLoginPage() {
  const { isAuthenticated, isLoading, login } = useAppAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (isLoading) return;
    if (isAuthenticated) {
      navigate('/', { replace: true });
      return;
    }
    void login();
  }, [isLoading, isAuthenticated, login, navigate]);

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
