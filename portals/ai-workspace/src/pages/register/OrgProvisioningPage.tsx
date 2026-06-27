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

import React from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Building2, LogOut } from 'lucide-react';
import Logo from '../../Components/Logo';

interface Props {
  orgName?: string;
  /** True while the registration API call is in flight; false while checking. */
  isProvisioning?: boolean;
  error?: string | null;
  onRetry?: () => void;
  /** When true, render a "session expired" message with a logout action. */
  isSessionExpired?: boolean;
  onLogout?: () => void;
}

export default function OrgProvisioningPage({
  orgName,
  isProvisioning = false,
  error,
  onRetry,
  isSessionExpired = false,
  onLogout,
}: Props) {
  const hasError = isSessionExpired || !!error;

  const title = isSessionExpired
    ? 'Your session has expired'
    : error
      ? 'Workspace setup failed'
      : isProvisioning
        ? 'Setting up your workspace'
        : 'Getting things ready';

  const subtitle = isSessionExpired
    ? 'Your session is no longer valid. Please log in again to continue.'
    : error
      ? (error)
      : isProvisioning
        ? orgName
          ? `Registering ${orgName} on the platform…`
          : 'Registering your organization on the platform…'
        : 'Verifying your organization…';

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'background.default',
        gap: 4,
        px: 3,
      }}
    >
      <Logo height={48} />

      <Box
        sx={{
          width: 72,
          height: 72,
          borderRadius: '50%',
          bgcolor: hasError ? 'error.light' : 'primary.light',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {isSessionExpired ? (
          <LogOut size={32} color="var(--oxygen-palette-error-contrastText)" />
        ) : error ? (
          <Building2 size={32} color="var(--oxygen-palette-error-contrastText)" />
        ) : (
          <CircularProgress size={36} sx={{ color: 'primary.contrastText' }} />
        )}
      </Box>

      <Stack spacing={1} alignItems="center" sx={{ maxWidth: 400, textAlign: 'center' }}>
        <Typography variant="h5" fontWeight="bold">
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {subtitle}
        </Typography>
      </Stack>

      {isSessionExpired ? (
        <Button
          variant="contained"
          color="error"
          startIcon={<LogOut size={16} />}
          onClick={onLogout}
        >
          Logout
        </Button>
      ) : error ? (
        <Stack spacing={2} alignItems="center">
          <Alert severity="error" sx={{ maxWidth: 400 }}>
            {error}
          </Alert>
          {onRetry && (
            <Button variant="contained" onClick={onRetry}>
              Try again
            </Button>
          )}
        </Stack>
      ) : (
        <Typography variant="caption" color="text.disabled">
          This only happens once. Please wait…
        </Typography>
      )}
    </Box>
  );
}
