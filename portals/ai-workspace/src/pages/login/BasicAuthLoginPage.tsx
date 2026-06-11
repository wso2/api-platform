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
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  ColorSchemeImage,
  ColorSchemeToggle,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  Link,
  Paper,
  ParticleBackground,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff, LogIn, Cloud, AppWindow, Cog, FlaskConical } from 'lucide-react';
import Logo from '../../Components/Logo';
import loginImage from '../../assets/images/login.svg';
import loginImageInverted from '../../assets/images/login-inverted.svg';
import { basicAuthLogin } from '../../contexts/BasicAuthProvider';

interface Props {
  onSuccess: () => void;
}

const sloganItems = [
  { icon: <Cloud size={20} />, title: 'Connect multiple LLM providers and models' },
  { icon: <AppWindow size={20} />, title: 'Build, run, and iterate AI workflows' },
  { icon: <Cog size={20} />, title: 'Configure prompts, tools, and runtime settings' },
  { icon: <FlaskConical size={20} />, title: 'Test, evaluate, and compare model responses' },
];

export default function BasicAuthLoginPage({ onSuccess }: Props) {
  const [username, setUsername]         = useState('');
  const [password, setPassword]         = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError]               = useState<string | null>(null);
  const [loading, setLoading]           = useState(false);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!username.trim() || !password) {
      setError('Username and password are required.');
      return;
    }

    setLoading(true);
    try {
      await basicAuthLogin(username.trim(), password);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed. Please try again.');
    } finally {
      setLoading(false);
    }
  }, [username, password, onSuccess]);

  return (
    <Box sx={{ height: '100vh', display: 'flex' }}>
      <ParticleBackground opacity={0.5} />
      <Grid container sx={{ flex: 1 }}>

        {/* ── Left panel — branding ──────────────────────────────────────────── */}
        <Grid
          size={{ xs: 12, md: 8 }}
          sx={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'flex-start',
            padding: 18,
            textAlign: 'left',
            position: 'relative',
          }}
        >
          <Stack
            direction="column"
            alignItems="flex-start"
            gap={2}
            maxWidth={580}
            display={{ xs: 'none', md: 'flex' }}
          >
            <Box sx={{ my: 1 }}>
              <Logo height={60} />
            </Box>

            <Typography variant="h3" sx={{ fontWeight: 'bold', mb: 0 }}>
              AI Workspace for Building and Running Intelligent Applications
            </Typography>

            <Typography variant="body1" sx={{ color: 'text.secondary' }}>
              A unified workspace to experiment with LLMs, manage providers, and build AI-powered solutions
            </Typography>

            <Stack sx={{ gap: 2 }}>
              {sloganItems.map((item) => (
                <Stack key={item.title} direction="row" sx={{ gap: 2, alignItems: 'flex-start' }}>
                  <Box sx={{ color: 'text.secondary', mt: '2px' }}>{item.icon}</Box>
                  <Typography sx={{ fontWeight: 'medium' }}>{item.title}</Typography>
                </Stack>
              ))}
            </Stack>
          </Stack>

          <ColorSchemeImage
            src={{ light: loginImage, dark: loginImageInverted }}
            alt={{ light: 'Login Screen Image (Light)', dark: 'Login Screen Image (Dark)' }}
            height={450}
            width="auto"
            sx={{ position: 'absolute', bottom: 50, right: -100 }}
          />
        </Grid>

        {/* ── Right panel — form ─────────────────────────────────────────────── */}
        <Grid size={{ xs: 12, md: 4 }}>
          <Paper
            sx={{
              display: 'flex',
              padding: 4,
              width: '100%',
              height: '100%',
              flexDirection: 'column',
              position: 'relative',
              textAlign: 'left',
            }}
          >
            <Box display="flex" justifyContent="flex-end">
              <ColorSchemeToggle />
            </Box>

            <Box
              sx={{
                alignItems: 'center',
                justifyContent: 'center',
                padding: 4,
                width: '100%',
                maxWidth: 500,
                margin: 'auto',
              }}
            >
              <Stack spacing={0.5} sx={{ mb: 3 }}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <LogIn size={20} />
                  <Typography variant="h6" fontWeight={700}>Sign In</Typography>
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  Enter your credentials to access the workspace.
                </Typography>
              </Stack>

              {error && (
                <Alert severity="error" onClose={() => setError(null)} sx={{ mb: 2 }}>
                  {error}
                </Alert>
              )}

              <Box component="form" onSubmit={handleSubmit} noValidate>
                <Stack spacing={2.5}>
                  <FormControl fullWidth required disabled={loading}>
                    <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>Username</FormLabel>
                    <TextField
                      placeholder="username"
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
                            <IconButton
                              onClick={() => setShowPassword((v) => !v)}
                              edge="end"
                              size="small"
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
                    size="large"
                    fullWidth
                    disabled={loading}
                    startIcon={loading ? <CircularProgress size={15} color="inherit" /> : <LogIn size={15} />}
                  >
                    {loading ? 'Signing in…' : 'Sign In'}
                  </Button>
                </Stack>
              </Box>

              <Box component="footer" sx={{ mt: 8 }}>
                <Typography sx={{ textAlign: 'center' }}>
                  © Copyright {new Date().getFullYear()}
                </Typography>
                <Stack direction="row" justifyContent="center" sx={{ mt: 2 }} spacing={1}>
                  <Link href="https://wso2.com/bijira/privacy-policy" target="_blank" rel="noopener noreferrer">
                    Privacy Policy
                  </Link>
                  <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />
                  <Link href="https://wso2.com/bijira/terms-of-use" target="_blank" rel="noopener noreferrer">
                    Terms of Use
                  </Link>
                </Stack>
              </Box>
            </Box>
          </Paper>
        </Grid>

      </Grid>
    </Box>
  );
}
