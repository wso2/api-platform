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

import { useState, type JSX } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, Box, Button, Link, Typography } from '@wso2/oxygen-ui';
import { Building2, Lock } from '@wso2/oxygen-ui-icons-react';
import { useAppAuth } from '../../contexts/AppAuthContext';

const ORG_HANDLE_STORAGE_KEY = 'ai_workspace_org_handle';

export default function LoginBox(): JSX.Element {
  const { login } = useAppAuth();
  const navigate = useNavigate();
  const [signInError, setSignInError] = useState('');

  const orgHandle = localStorage.getItem(ORG_HANDLE_STORAGE_KEY) ?? '';

  const handleSignIn = async () => {
    setSignInError('');
    try {
      await login();
    } catch (err) {
      setSignInError(
        err instanceof Error ? err.message : 'Sign in failed. Please try again.'
      );
    }
  };

  return (
    <Box>
      <Box sx={{ mb: 8 }}>
        <Typography variant="h3" gutterBottom>
          Sign in to AI Workspace
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 3 }}>
          Sign in with your username and password to continue
        </Typography>

        {orgHandle && (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              px: 2,
              py: 1.5,
              borderRadius: 2,
              border: '1px solid',
              borderColor: 'divider',
              bgcolor: 'action.hover',
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25 }}>
              <Building2 size={16} />
              <Box>
                <Typography variant="caption" color="text.secondary" display="block" lineHeight={1.2}>
                  Signing into
                </Typography>
                <Typography variant="body2" fontWeight={600}>
                  {orgHandle}
                </Typography>
              </Box>
            </Box>
            <Link
              component="button"
              variant="body2"
              onClick={() => navigate('/getting-started')}
              sx={{ cursor: 'pointer', whiteSpace: 'nowrap' }}
            >
              Switch org
            </Link>
          </Box>
        )}
      </Box>

      {signInError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {signInError}
        </Alert>
      )}

      <Box display="flex" flexDirection="column" gap={3}>
        <Button
          fullWidth
          variant="contained"
          startIcon={<Lock />}
          onClick={handleSignIn}
        >
          Sign in
        </Button>
      </Box>
    </Box>
  );
}
