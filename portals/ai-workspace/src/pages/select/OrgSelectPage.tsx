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
import {
  Box,
  Button,
  ColorSchemeToggle,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Building2 } from 'lucide-react';
import Logo from '../../Components/Logo';
import UserMenu from '../../Components/UserMenu';
import { useAppAuth } from '../../contexts/AppAuthContext';

const HANDLE_PATTERN = /^[a-z0-9-]+$/;

export default function OrgSelectPage() {
  const navigate = useNavigate();
  const { user, logout } = useAppAuth();

  const userForMenu = {
    name: user?.name || user?.email || 'User',
    email: user?.email || '',
    role: user?.role ?? undefined,
  };

  const [handle, setHandle] = useState('');
  const [error, setError] = useState<string | undefined>();

  const validate = (value: string): string | undefined => {
    if (!value.trim()) return 'Organization handle is required.';
    if (!HANDLE_PATTERN.test(value)) return 'Only lowercase letters, numbers, and hyphens allowed.';
    if (value.length < 2) return 'Handle must be at least 2 characters.';
    return undefined;
  };

  const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setHandle(e.target.value.toLowerCase());
    setError(undefined);
  }, []);

  const handleSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    const validationError = validate(handle.trim());
    if (validationError) {
      setError(validationError);
      return;
    }
    navigate(`/organizations/${handle.trim()}/home`);
  }, [handle, navigate]);

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', bgcolor: 'background.default' }}>
      <Grid container sx={{ flex: 1 }}>

        {/* ── Left panel (decorative, md+ only) ───────────────────────────── */}
        <Grid
          size={{ xs: 0, md: 7 }}
          sx={{
            display: { xs: 'none', md: 'flex' },
            flexDirection: 'column',
            justifyContent: 'center',
            px: 10,
            py: 8,
            gap: 4,
            background:
              'linear-gradient(135deg, var(--oxygen-palette-primary-dark) 0%, var(--oxygen-palette-primary-main) 100%)',
            color: '#fff',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          {/* decorative circles */}
          {[
            { size: 400, top: -100, right: -100, opacity: 0.05 },
            { size: 250, bottom: -60, left: -60, opacity: 0.08 },
          ].map((c, i) => (
            <Box
              key={i}
              sx={{
                position: 'absolute',
                width: c.size,
                height: c.size,
                borderRadius: '50%',
                bgcolor: `rgba(255,255,255,${c.opacity})`,
                ...(c.top    !== undefined && { top:    c.top }),
                ...(c.right  !== undefined && { right:  c.right }),
                ...(c.bottom !== undefined && { bottom: c.bottom }),
                ...(c.left   !== undefined && { left:   c.left }),
              }}
            />
          ))}

          <Box sx={{ position: 'relative' }}>
            <Logo height={52} />
          </Box>

          <Stack spacing={2} sx={{ position: 'relative' }}>
            <Typography variant="h3" fontWeight="bold" sx={{ color: '#fff' }}>
              Welcome back
            </Typography>
            <Typography
              variant="body1"
              sx={{ color: 'rgba(255,255,255,0.8)', maxWidth: 420 }}
            >
              Enter your organization handle to jump straight into your AI
              workspace — gateways, providers, and proxies ready to go.
            </Typography>
          </Stack>

          <Stack spacing={2} sx={{ position: 'relative' }}>
            {[
              'Connect and manage multiple LLM providers',
              'Deploy AI gateways with policy controls',
              'Build, test, and iterate AI proxies',
              'Monitor usage with built-in analytics',
            ].map((item) => (
              <Stack key={item} direction="row" spacing={1.5} alignItems="center">
                <Box
                  sx={{
                    width: 6, height: 6, borderRadius: '50%',
                    bgcolor: 'rgba(255,255,255,0.7)', flexShrink: 0,
                  }}
                />
                <Typography variant="body2" sx={{ color: 'rgba(255,255,255,0.85)' }}>
                  {item}
                </Typography>
              </Stack>
            ))}
          </Stack>

        </Grid>

        {/* ── Right panel (form) ──────────────────────────────────────────── */}
        <Grid size={{ xs: 12, md: 5 }}>
          <Paper
            square elevation={0}
            sx={{
              display: 'flex', flexDirection: 'column',
              minHeight: '100vh', px: { xs: 3, sm: 6 }, py: 4,
            }}
          >
            {/* header row */}
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
              <Box sx={{ display: { md: 'none' } }}>
                <Logo height={40} />
              </Box>
              <Box sx={{ ml: 'auto', display: 'flex', alignItems: 'center', gap: 1 }}>
                <ColorSchemeToggle />
                <UserMenu user={userForMenu} onLogout={logout} />
              </Box>
            </Box>

            {/* centred content area */}
            <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Box sx={{ width: '100%', maxWidth: 420 }}>
                <Stack spacing={3}>
                  <Stack spacing={0.5}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Building2 size={20} />
                      <Typography variant="h5" fontWeight="bold">
                        Select Organization
                      </Typography>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      Enter the handle of the organization you want to access.
                    </Typography>
                  </Stack>

                  <Divider />

                  <Box component="form" onSubmit={handleSubmit} noValidate>
                    <Stack spacing={2.5}>

                      <FormControl fullWidth required>
                        <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>
                          Organization Handle
                        </FormLabel>
                        <TextField
                          placeholder="e.g. acme"
                          value={handle}
                          onChange={handleChange}
                          error={!!error}
                          fullWidth autoFocus
                          inputProps={{ pattern: '[a-z0-9][-a-z0-9]*' }}
                        />
                        <FormHelperText error={!!error}>
                          {error}
                        </FormHelperText>
                      </FormControl>

                      <Button
                        type="submit"
                        variant="contained"
                        size="large"
                        fullWidth
                      >
                        Continue
                      </Button>

                    </Stack>
                  </Box>
                </Stack>
              </Box>
            </Box>

            {/* footer */}
            <Box component="footer" sx={{ mt: 4, textAlign: 'center' }}>
              <Typography variant="caption" color="text.secondary">
                © {new Date().getFullYear()} WSO2 LLC. — AI Platform local dev
              </Typography>
            </Box>
          </Paper>
        </Grid>
      </Grid>
    </Box>
  );
}
