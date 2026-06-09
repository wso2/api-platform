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

import React, { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import bcrypt from 'bcryptjs';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  ColorSchemeToggle,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff, ShieldCheck } from 'lucide-react';
import Logo from '../../Components/Logo';
import { SUPER_ADMIN_USERNAME, SUPER_ADMIN_PASSWORD_HASH } from '../../config.env';
import { setStoredToken } from '../../clients/choreoApiClient';
import { SUPER_ADMIN_SESSION_KEY } from '../../contexts/SuperAdminAuthProvider';

export default function AdminLoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!SUPER_ADMIN_PASSWORD_HASH) {
      setError('Super admin login is not configured on this deployment.');
      return;
    }
    if (!username.trim() || !password) {
      setError('Username and password are required.');
      return;
    }

    setLoading(true);
    try {
      const valid = await bcrypt.compare(password, SUPER_ADMIN_PASSWORD_HASH);
      if (!valid || username.trim() !== SUPER_ADMIN_USERNAME) {
        setError('Invalid username or password.');
        return;
      }
      setStoredToken('super-admin-session');
      sessionStorage.setItem(SUPER_ADMIN_SESSION_KEY, 'true');
      // Full reload so AppRoot re-evaluates isSuperAdminSession().
      window.location.href = '/';
    } catch {
      setError('Login failed. Please try again.');
    } finally {
      setLoading(false);
    }
  }, [username, password]);

  if (!SUPER_ADMIN_PASSWORD_HASH) {
    return (
      <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', p: 3 }}>
        <Stack spacing={2} alignItems="center" sx={{ maxWidth: 400, textAlign: 'center' }}>
          <Typography variant="h6" fontWeight={700}>Admin login not configured</Typography>
          <Typography variant="body2" color="text.secondary">
            Set <code>VITE_SUPER_ADMIN_PASSWORD_HASH</code> to enable this login.
          </Typography>
          <Button variant="outlined" onClick={() => navigate('/login')}>Back</Button>
        </Stack>
      </Box>
    );
  }

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: 'background.default', p: 2 }}>
      <Paper elevation={2} sx={{ width: '100%', maxWidth: 420, p: 4, borderRadius: 2 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
          <Logo height={36} />
          <ColorSchemeToggle />
        </Box>

        <Stack spacing={0.5} sx={{ mb: 3 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <ShieldCheck size={20} />
            <Typography variant="h6" fontWeight={700}>Admin Login</Typography>
          </Stack>
          <Typography variant="body2" color="text.secondary">
            For platform administrators only.
          </Typography>
        </Stack>

        {error && (
          <Alert severity="error" onClose={() => setError(null)} sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        <Box component="form" onSubmit={handleSubmit} noValidate>
          <Stack spacing={2}>
            <FormControl fullWidth required disabled={loading}>
              <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>Username</FormLabel>
              <TextField
                placeholder="admin"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                fullWidth autoFocus disabled={loading}
              />
            </FormControl>

            <FormControl fullWidth required disabled={loading}>
              <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>Password</FormLabel>
              <TextField
                type={showPassword ? 'text' : 'password'}
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                fullWidth disabled={loading}
                InputProps={{
                  endAdornment: (
                    <InputAdornment position="end">
                      <IconButton onClick={() => setShowPassword((v) => !v)} edge="end" size="small">
                        {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                      </IconButton>
                    </InputAdornment>
                  ),
                }}
              />
            </FormControl>

            <Button
              type="submit"
              variant="contained"
              color="warning"
              fullWidth
              disabled={loading}
              startIcon={loading ? <CircularProgress size={15} color="inherit" /> : <ShieldCheck size={15} />}
            >
              {loading ? 'Signing in…' : 'Sign In as Admin'}
            </Button>

            <Button variant="text" size="small" onClick={() => navigate('/login')} disabled={loading}>
              Back to sign in
            </Button>
          </Stack>
        </Box>
      </Paper>
    </Box>
  );
}
