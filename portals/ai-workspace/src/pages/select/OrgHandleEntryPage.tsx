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
import bcrypt from 'bcryptjs';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Collapse,
  ColorSchemeToggle,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Building2, ChevronRight, Eye, EyeOff, ShieldCheck } from 'lucide-react';
import Logo from '../../Components/Logo';
import { SUPER_ADMIN_USERNAME, SUPER_ADMIN_PASSWORD_HASH } from '../../config.env';
import { setStoredToken } from '../../clients/choreoApiClient';
import { SUPER_ADMIN_SESSION_KEY } from '../../contexts/SuperAdminAuthProvider';

const HANDLE_PATTERN = /^[a-z0-9-]+$/;

interface Props {
  onConfirm: (handle: string) => void;
  onSuperAdminLogin: () => void;
  initialHandle?: string;
  externalError?: string | null;
  isFetching?: boolean;
}

type ActivePanel = 'none' | 'org' | 'superadmin';

export default function OrgHandleEntryPage({ onConfirm, onSuperAdminLogin, initialHandle = '', externalError, isFetching }: Props) {
  const [active, setActive] = useState<ActivePanel>('none');

  // Org handle state
  const [handle, setHandle] = useState(initialHandle);
  const [handleError, setHandleError] = useState<string | undefined>();

  // Super admin state
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [adminError, setAdminError] = useState<string | null>(null);
  const [adminLoading, setAdminLoading] = useState(false);

  const togglePanel = (panel: ActivePanel) => {
    setActive((prev) => (prev === panel ? 'none' : panel));
    setHandleError(undefined);
    setAdminError(null);
  };

  const validateHandle = (value: string): string | undefined => {
    if (!value.trim()) return 'Organization handle is required.';
    if (!HANDLE_PATTERN.test(value)) return 'Only lowercase letters, numbers, and hyphens are allowed.';
    if (value.length < 2) return 'Handle must be at least 2 characters.';
    return undefined;
  };

  const handleOrgSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    const err = validateHandle(handle.trim());
    if (err) { setHandleError(err); return; }
    onConfirm(handle.trim());
  }, [handle, onConfirm]);

  const handleAdminSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setAdminError(null);

    if (!SUPER_ADMIN_PASSWORD_HASH) {
      setAdminError('Super admin login is not configured on this deployment.');
      return;
    }
    if (!username.trim() || !password) {
      setAdminError('Username and password are required.');
      return;
    }

    setAdminLoading(true);
    try {
      const valid = await bcrypt.compare(password, SUPER_ADMIN_PASSWORD_HASH);
      if (!valid || username.trim() !== SUPER_ADMIN_USERNAME) {
        setAdminError('Invalid username or password.');
        return;
      }
      setStoredToken('super-admin-session');
      sessionStorage.setItem(SUPER_ADMIN_SESSION_KEY, 'true');
      onSuperAdminLogin();
    } catch {
      setAdminError('Login failed. Please try again.');
    } finally {
      setAdminLoading(false);
    }
  }, [username, password, onSuperAdminLogin]);

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', bgcolor: 'background.default' }}>
      <Grid container sx={{ flex: 1 }}>

        {/* ── Left decorative panel ── */}
        <Grid
          size={{ xs: 0, md: 7 }}
          sx={{
            display: { xs: 'none', md: 'flex' },
            flexDirection: 'column',
            justifyContent: 'center',
            px: 10, py: 8, gap: 4,
            background: 'linear-gradient(135deg, var(--oxygen-palette-primary-dark) 0%, var(--oxygen-palette-primary-main) 100%)',
            color: '#fff',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          {[
            { size: 400, top: -100, right: -100, opacity: 0.05 },
            { size: 250, bottom: -60, left: -60, opacity: 0.08 },
          ].map((c, i) => (
            <Box key={i} sx={{
              position: 'absolute', width: c.size, height: c.size, borderRadius: '50%',
              bgcolor: `rgba(255,255,255,${c.opacity})`,
              ...(c.top    !== undefined && { top:    c.top }),
              ...(c.right  !== undefined && { right:  c.right }),
              ...(c.bottom !== undefined && { bottom: c.bottom }),
              ...(c.left   !== undefined && { left:   c.left }),
            }} />
          ))}

          <Box sx={{ position: 'relative' }}><Logo height={52} /></Box>

          <Stack spacing={2} sx={{ position: 'relative' }}>
            <Typography variant="h3" fontWeight="bold" sx={{ color: '#fff' }}>
              Welcome to AI Platform
            </Typography>
            <Typography variant="body1" sx={{ color: 'rgba(255,255,255,0.8)', maxWidth: 420 }}>
              Manage AI gateways, LLM providers, and intelligent proxies — all in one place.
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
                <Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: 'rgba(255,255,255,0.7)', flexShrink: 0 }} />
                <Typography variant="body2" sx={{ color: 'rgba(255,255,255,0.85)' }}>{item}</Typography>
              </Stack>
            ))}
          </Stack>
        </Grid>

        {/* ── Right panel ── */}
        <Grid size={{ xs: 12, md: 5 }}>
          <Paper square elevation={0} sx={{
            display: 'flex', flexDirection: 'column',
            minHeight: '100vh', px: { xs: 3, sm: 6 }, py: 4,
          }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
              <Box sx={{ display: { md: 'none' } }}><Logo height={40} /></Box>
              <Box sx={{ ml: 'auto' }}><ColorSchemeToggle /></Box>
            </Box>

            <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Box sx={{ width: '100%', maxWidth: 420 }}>
                <Stack spacing={3}>

                  <Stack spacing={0.5}>
                    <Typography variant="h5" fontWeight="bold">Sign in</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Choose how you want to continue.
                    </Typography>
                  </Stack>

                  {/* ── Option 1: Login to Organization ── */}
                  <Paper variant="outlined" sx={{ borderRadius: 2, overflow: 'hidden' }}>
                    <Box
                      onClick={() => togglePanel('org')}
                      sx={{
                        px: 2.5, py: 2,
                        display: 'flex', alignItems: 'center', gap: 2,
                        cursor: 'pointer',
                        bgcolor: active === 'org' ? 'action.selected' : 'transparent',
                        '&:hover': { bgcolor: 'action.hover' },
                        transition: 'background 0.15s',
                      }}
                    >
                      <Box sx={{
                        width: 40, height: 40, borderRadius: '50%',
                        bgcolor: 'primary.main', display: 'flex',
                        alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                      }}>
                        <Building2 size={18} color="#fff" />
                      </Box>
                      <Stack spacing={0} sx={{ flex: 1 }}>
                        <Typography variant="body1" fontWeight={600}>Login to Organization</Typography>
                        <Typography variant="caption" color="text.secondary">
                          Sign in with your organization's identity provider
                        </Typography>
                      </Stack>
                      <ChevronRight
                        size={18}
                        style={{
                          transition: 'transform 0.2s',
                          transform: active === 'org' ? 'rotate(90deg)' : 'rotate(0deg)',
                          flexShrink: 0,
                        }}
                      />
                    </Box>

                    <Collapse in={active === 'org'}>
                      <Box sx={{ px: 2.5, pb: 2.5, pt: 1 }}>
                        {externalError && (
                          <Alert severity="error" sx={{ mb: 2 }}>{externalError}</Alert>
                        )}
                        <Box component="form" onSubmit={handleOrgSubmit} noValidate>
                          <Stack spacing={2}>
                            <FormControl fullWidth required disabled={isFetching}>
                              <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>Organization Handle</FormLabel>
                              <TextField
                                placeholder="e.g. acme"
                                value={handle}
                                onChange={(e) => { setHandle(e.target.value.toLowerCase()); setHandleError(undefined); }}
                                error={!!handleError}
                                fullWidth
                                autoFocus={active === 'org'}
                                disabled={isFetching}
                                inputProps={{ pattern: '[a-z0-9][-a-z0-9]*' }}
                              />
                              <FormHelperText error={!!handleError}>{handleError}</FormHelperText>
                            </FormControl>
                            <Button
                              type="submit"
                              variant="contained"
                              fullWidth
                              disabled={isFetching}
                              startIcon={isFetching ? <CircularProgress size={15} color="inherit" /> : undefined}
                            >
                              {isFetching ? 'Loading…' : 'Continue'}
                            </Button>
                          </Stack>
                        </Box>
                      </Box>
                    </Collapse>
                  </Paper>

                  {/* ── Option 2: Login as Super Admin ── */}
                  {SUPER_ADMIN_PASSWORD_HASH && (
                    <Paper variant="outlined" sx={{ borderRadius: 2, overflow: 'hidden' }}>
                      <Box
                        onClick={() => togglePanel('superadmin')}
                        sx={{
                          px: 2.5, py: 2,
                          display: 'flex', alignItems: 'center', gap: 2,
                          cursor: 'pointer',
                          bgcolor: active === 'superadmin' ? 'action.selected' : 'transparent',
                          '&:hover': { bgcolor: 'action.hover' },
                          transition: 'background 0.15s',
                        }}
                      >
                        <Box sx={{
                          width: 40, height: 40, borderRadius: '50%',
                          bgcolor: 'warning.main', display: 'flex',
                          alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                        }}>
                          <ShieldCheck size={18} color="#fff" />
                        </Box>
                        <Stack spacing={0} sx={{ flex: 1 }}>
                          <Typography variant="body1" fontWeight={600}>Login as Super Admin</Typography>
                          <Typography variant="caption" color="text.secondary">
                            Register and manage organizations
                          </Typography>
                        </Stack>
                        <ChevronRight
                          size={18}
                          style={{
                            transition: 'transform 0.2s',
                            transform: active === 'superadmin' ? 'rotate(90deg)' : 'rotate(0deg)',
                            flexShrink: 0,
                          }}
                        />
                      </Box>

                      <Collapse in={active === 'superadmin'}>
                        <Box sx={{ px: 2.5, pb: 2.5, pt: 1 }}>
                          {adminError && (
                            <Alert severity="error" onClose={() => setAdminError(null)} sx={{ mb: 2 }}>
                              {adminError}
                            </Alert>
                          )}
                          <Box component="form" onSubmit={handleAdminSubmit} noValidate>
                            <Stack spacing={2}>
                              <FormControl fullWidth required disabled={adminLoading}>
                                <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>Username</FormLabel>
                                <TextField
                                  placeholder="admin"
                                  value={username}
                                  onChange={(e) => setUsername(e.target.value)}
                                  fullWidth
                                  autoFocus={active === 'superadmin'}
                                  disabled={adminLoading}
                                />
                              </FormControl>
                              <FormControl fullWidth required disabled={adminLoading}>
                                <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>Password</FormLabel>
                                <TextField
                                  type={showPassword ? 'text' : 'password'}
                                  placeholder="••••••••"
                                  value={password}
                                  onChange={(e) => setPassword(e.target.value)}
                                  fullWidth
                                  disabled={adminLoading}
                                  InputProps={{
                                    endAdornment: (
                                      <InputAdornment position="end">
                                        <IconButton
                                          onClick={() => setShowPassword((v) => !v)}
                                          edge="end"
                                          size="small"
                                          aria-label={showPassword ? 'Hide password' : 'Show password'}
                                        >
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
                                disabled={adminLoading}
                                startIcon={adminLoading ? <CircularProgress size={15} color="inherit" /> : <ShieldCheck size={15} />}
                              >
                                {adminLoading ? 'Signing in…' : 'Sign In as Super Admin'}
                              </Button>
                            </Stack>
                          </Box>
                        </Box>
                      </Collapse>
                    </Paper>
                  )}

                </Stack>
              </Box>
            </Box>

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
