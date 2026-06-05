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
  ColorSchemeToggle,
  Divider,
  Grid,
  Paper,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { AlertTriangle, ArrowLeft, Settings } from 'lucide-react';
import Logo from '../../Components/Logo';

interface Props {
  orgHandle: string;
  onBack: () => void;
}

export default function IDPNotConfiguredPage({ orgHandle, onBack }: Props) {
  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', bgcolor: 'background.default' }}>
      <Grid container sx={{ flex: 1 }}>

        {/* Left decorative panel */}
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
              'linear-gradient(135deg, var(--oxygen-palette-warning-dark) 0%, var(--oxygen-palette-warning-main) 100%)',
            color: '#fff',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
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
              Setup Required
            </Typography>
            <Typography variant="body1" sx={{ color: 'rgba(255,255,255,0.85)', maxWidth: 420 }}>
              The organization <strong>{orgHandle}</strong> needs an identity provider
              configured before users can sign in.
            </Typography>
          </Stack>

          <Stack spacing={2} sx={{ position: 'relative' }}>
            {[
              'Contact your platform administrator',
              'Provide the organization handle to your admin',
              'Wait for IDP configuration to be completed',
              'Try signing in again once configured',
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

        {/* Right panel */}
        <Grid size={{ xs: 12, md: 5 }}>
          <Paper
            square elevation={0}
            sx={{
              display: 'flex', flexDirection: 'column',
              minHeight: '100vh', px: { xs: 3, sm: 6 }, py: 4,
            }}
          >
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
              <Box sx={{ display: { md: 'none' } }}>
                <Logo height={40} />
              </Box>
              <Box sx={{ ml: 'auto' }}>
                <ColorSchemeToggle />
              </Box>
            </Box>

            <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Box sx={{ width: '100%', maxWidth: 420 }}>
                <Stack spacing={3}>
                  <Stack spacing={0.5}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <AlertTriangle size={20} />
                      <Typography variant="h5" fontWeight="bold">
                        IDP Not Configured
                      </Typography>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      Organization <strong>{orgHandle}</strong> does not have an identity
                      provider set up yet.
                    </Typography>
                  </Stack>

                  <Divider />

                  <Alert severity="warning" icon={<Settings size={18} />}>
                    This organization has not been configured with a sign-in provider.
                    A platform administrator needs to complete setup before you can log in.
                  </Alert>

                  {/* TODO: Replace this section with actual setup instructions */}
                  <Stack spacing={2}>
                    <Typography variant="subtitle2" fontWeight={600}>
                      Next Steps
                    </Typography>
                    <Stack spacing={1.5}>
                      {[
                        {
                          step: '1',
                          title: 'Contact your administrator',
                          description: 'Reach out to your platform administrator with the organization handle.',
                        },
                        {
                          step: '2',
                          title: 'Administrator configures IDP',
                          description: 'The admin registers the organization and configures an identity provider.',
                        },
                        {
                          step: '3',
                          title: 'Sign in once ready',
                          description: 'Return to this page and try signing in after setup is complete.',
                        },
                      ].map(({ step, title, description }) => (
                        <Stack key={step} direction="row" spacing={2} alignItems="flex-start">
                          <Box
                            sx={{
                              width: 28, height: 28,
                              borderRadius: '50%',
                              bgcolor: 'warning.light',
                              color: 'warning.contrastText',
                              display: 'flex', alignItems: 'center', justifyContent: 'center',
                              flexShrink: 0,
                              fontSize: '0.75rem',
                              fontWeight: 700,
                            }}
                          >
                            {step}
                          </Box>
                          <Stack spacing={0.25}>
                            <Typography variant="body2" fontWeight={600}>{title}</Typography>
                            <Typography variant="caption" color="text.secondary">{description}</Typography>
                          </Stack>
                        </Stack>
                      ))}
                    </Stack>
                  </Stack>

                  <Button
                    variant="outlined"
                    startIcon={<ArrowLeft size={16} />}
                    onClick={onBack}
                    fullWidth
                  >
                    Try a Different Organization
                  </Button>
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
