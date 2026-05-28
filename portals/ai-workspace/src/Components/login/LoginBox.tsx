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
import { Box, Button, Divider, TextField, Typography, Link } from '@wso2/oxygen-ui';
import {
  ArrowLeft,
  GitHub,
  Google,
  // Microsoft,
  ShieldCheck,
} from '@wso2/oxygen-ui-icons-react';
import { useAuthContext } from '@asgardeo/auth-react';
import {
  handleGoogleLogin,
  handleGithubLogin,
  handleMicrosoftLogin,
  handleEnterpriseLogin,
} from '../../auth/login';

/**
 * Simple email validation regex
 */
const isValidEmail = (email: string): boolean =>
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);

export default function LoginBox(): JSX.Element {
  const { signIn } = useAuthContext();
  const [enterpriseMode, setEnterpriseMode] = useState(false);
  const [email, setEmail] = useState('');
  const [emailError, setEmailError] = useState('');

  const onGoogleLogin = () => {
    handleGoogleLogin(signIn, '/');
  };

  const onGithubLogin = () => {
    handleGithubLogin(signIn, '/');
  };

  const onMicrosoftLogin = () => {
    handleMicrosoftLogin(signIn, '/');
  };

  const onEnterpriseContinue = () => {
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      setEmailError('Email is required');
      return;
    }
    if (!isValidEmail(trimmedEmail)) {
      setEmailError('Please enter a valid email address');
      return;
    }
    setEmailError('');
    handleEnterpriseLogin(signIn, '/', trimmedEmail);
  };

  const handleBack = () => {
    setEnterpriseMode(false);
    setEmail('');
    setEmailError('');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      onEnterpriseContinue();
    }
  };

  if (enterpriseMode) {
    return (
      <Box>
        <Box sx={{ mb: 6 }}>
          <Typography variant="h3" gutterBottom>
            Sign in with Enterprise ID
          </Typography>
          <Typography color="text.secondary">
            Enter your enterprise email to continue
          </Typography>
        </Box>

        <Box display="flex" flexDirection="column" gap={3}>
          <TextField
            fullWidth
            label="Email"
            placeholder="Enter your enterprise email"
            type="email"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              if (emailError) setEmailError('');
            }}
            onKeyDown={handleKeyDown}
            error={!!emailError}
            helperText={emailError}
            autoFocus
          />

          <Button
            fullWidth
            variant="contained"
            onClick={onEnterpriseContinue}
          >
            Continue
          </Button>

          <Box sx={{ textAlign: 'center' }}>
            <Link
              component="button"
              underline="hover"
              onClick={handleBack}
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 0.5,
                cursor: 'pointer',
              }}
            >
              <ArrowLeft fontSize="small" />
              Back to sign in options
            </Link>
          </Box>
        </Box>
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ mb: 10 }}>
        <Typography variant="h3" gutterBottom>
          Sign in to AI Workspace
        </Typography>

        <Typography color="text.secondary">
          Continue using your preferred identity provider
        </Typography>
      </Box>

      <Box display="flex" flexDirection="column" gap={3}>
        <Button
          fullWidth
          variant="contained"
          startIcon={<Google />}
          color="secondary"
          onClick={onGoogleLogin}
        >
          Continue with Google
        </Button>

        <Button
          fullWidth
          variant="contained"
          startIcon={<GitHub />}
          color="secondary"
          onClick={onGithubLogin}
        >
          Continue with GitHub
        </Button>

        <Button
          fullWidth
          variant="contained"
          startIcon={<ShieldCheck />}
          color="secondary"
          onClick={onMicrosoftLogin}
        >
          Continue with Microsoft
        </Button>

        <Divider sx={{ my: 1 }}>or</Divider>

        <Button
          fullWidth
          variant="outlined"
          startIcon={<ShieldCheck />}
          onClick={() => setEnterpriseMode(true)}
        >
          Sign in with Enterprise ID
        </Button>
      </Box>
    </Box>
  );
}
