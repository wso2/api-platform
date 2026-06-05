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
import { Alert, Box, Button, Typography } from '@wso2/oxygen-ui';
import { Lock } from '@wso2/oxygen-ui-icons-react';
import { useAppAuth } from '../../contexts/AppAuthContext';

export default function LoginBox(): JSX.Element {
  const { login } = useAppAuth();
  const [signInError, setSignInError] = useState('');

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
      <Box sx={{ mb: 10 }}>
        <Typography variant="h3" gutterBottom>
          Sign in to AI Workspace
        </Typography>
        <Typography color="text.secondary">
          Sign in with your username and password to continue
        </Typography>
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
