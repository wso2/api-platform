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

import React, { useState, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  ColorSchemeToggle,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Building2, CheckCircle2, RefreshCw } from 'lucide-react';
import Logo from '../../Components/Logo';
import UserMenu from '../../Components/UserMenu';
import { useAppAuth } from '../../contexts/AppAuthContext';
import { CheckCircle2 as SuccessIcon } from 'lucide-react';
import {
  registerOrganization,
  type RegisterOrganizationRequest,
} from '../../apis/platformApis';
// ─── Constants ───────────────────────────────────────────────────────────────

const REGIONS = [
  { value: 'us',           label: 'United States (us)' },
  { value: 'eu',           label: 'Europe (eu)' },
  { value: 'ap',           label: 'Asia Pacific (ap)' },
  { value: 'us-east-1',    label: 'US East 1 (us-east-1)' },
  { value: 'us-west-2',    label: 'US West 2 (us-west-2)' },
  { value: 'eu-west-1',    label: 'EU West 1 (eu-west-1)' },
  { value: 'ap-southeast-1', label: 'AP Southeast 1 (ap-southeast-1)' },
] as const;

const HANDLE_PATTERN = /^[a-z0-9-]+$/;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
/** ms to show the success banner before navigating to the workspace */
const REDIRECT_DELAY_MS = 1500;

// ─── Helpers ─────────────────────────────────────────────────────────────────

const generateUUID = (): string => crypto.randomUUID();

const toHandle = (name: string): string =>
  name.toLowerCase().trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

// ─── Types ────────────────────────────────────────────────────────────────────

interface FormState {
  id: string;
  name: string;
  handle: string;
  region: string;
}

interface FormErrors {
  name?: string;
  handle?: string;
  region?: string;
  id?: string;
}

// ─── Redirecting banner ───────────────────────────────────────────────────────

function RedirectingToWorkspace({ orgName }: { orgName: string }) {
  return (
    <Stack spacing={3} alignItems="center" sx={{ py: 4 }}>
      <Box
        sx={{
          width: 64,
          height: 64,
          borderRadius: '50%',
          bgcolor: 'success.light',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'success.contrastText',
        }}
      >
        <CheckCircle2 size={32} />
      </Box>

      <Stack spacing={0.5} alignItems="center">
        <Typography variant="h5" fontWeight="bold">
          Organization Created!
        </Typography>
        <Typography variant="body2" color="text.secondary" textAlign="center">
          <strong>{orgName}</strong> is ready. Setting up your workspace…
        </Typography>
      </Stack>

      <CircularProgress size={28} />

      <Typography variant="caption" color="text.secondary">
        Redirecting to workspace…
      </Typography>
    </Stack>
  );
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function OrgRegisterPage() {
  const navigate = useNavigate();
  const { user, logout, isSuperAdmin } = useAppAuth();

  const userForMenu = {
    name: user?.name || user?.email || 'User',
    email: user?.email || '',
    role: user?.role ?? undefined,
  };

  const [form, setForm] = useState<FormState>({
    id: generateUUID(),
    name: '',
    handle: '',
    region: 'us',
  });
  const [errors, setErrors]           = useState<FormErrors>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [apiError, setApiError]         = useState<string | null>(null);
  const [registeredOrgName, setRegisteredOrgName] = useState<string | null>(null);
  const [registeredOrgHandle, setRegisteredOrgHandle] = useState<string | null>(null);

  // ── Auto-redirect after success (not for super admin — they stay to register more) ──

  useEffect(() => {
    if (!registeredOrgName || !registeredOrgHandle || isSuperAdmin) return;
    const timer = setTimeout(() => {
      navigate(`/organizations/${registeredOrgHandle}/home`, { replace: true });
    }, REDIRECT_DELAY_MS);
    return () => clearTimeout(timer);
  }, [registeredOrgName, registeredOrgHandle, navigate, isSuperAdmin]);

  // ── Validation ─────────────────────────────────────────────────────────────

  const validate = (f: FormState): FormErrors => {
    const e: FormErrors = {};
    if (!f.name.trim()) {
      e.name = 'Organization name is required.';
    } else if (f.name.trim().length < 2) {
      e.name = 'Name must be at least 2 characters.';
    }
    if (!f.handle.trim()) {
      e.handle = 'Handle is required.';
    } else if (!HANDLE_PATTERN.test(f.handle)) {
      e.handle = 'Only lowercase letters, numbers, and hyphens allowed.';
    } else if (f.handle.length < 2) {
      e.handle = 'Handle must be at least 2 characters.';
    }
    if (!f.region) {
      e.region = 'Please select a region.';
    }
    if (!f.id.trim()) {
      e.id = 'Organization ID is required.';
    } else if (!UUID_PATTERN.test(f.id.trim())) {
      e.id = 'Must be a valid UUID (e.g. 550e8400-e29b-41d4-a716-446655440000).';
    }
    return e;
  };

  // ── Handlers ───────────────────────────────────────────────────────────────

  const handleNameChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const name = e.target.value;
    setForm((prev) => ({
      ...prev,
      name,
      handle: toHandle(name),
    }));
    setErrors((prev) => ({ ...prev, name: undefined, handle: undefined }));
    setApiError(null);
  }, []);

  const handleHandleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, handle: e.target.value.toLowerCase() }));
    setErrors((prev) => ({ ...prev, handle: undefined }));
    setApiError(null);
  }, []);

  const handleRegionChange = useCallback((value: string) => {
    setForm((prev) => ({ ...prev, region: value }));
    setErrors((prev) => ({ ...prev, region: undefined }));
    setApiError(null);
  }, []);

  const handleIdChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, id: e.target.value }));
    setErrors((prev) => ({ ...prev, id: undefined }));
    setApiError(null);
  }, []);

  const regenerateId = useCallback(() => {
    setForm((prev) => ({ ...prev, id: generateUUID() }));
    setErrors((prev) => ({ ...prev, id: undefined }));
  }, []);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setApiError(null);

    const validationErrors = validate(form);
    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }

    setIsSubmitting(true);
    try {
      const payload: RegisterOrganizationRequest = {
        id: form.id,
        name: form.name.trim(),
        handle: form.handle.trim(),
        region: form.region,
      };

      const org = await registerOrganization(payload);

      setRegisteredOrgName(org.name);
      setRegisteredOrgHandle(org.handle);
    } catch (err: any) {
      setApiError(err?.message ?? 'An unexpected error occurred.');
    } finally {
      setIsSubmitting(false);
    }
  }, [form]);

  // ── Render ─────────────────────────────────────────────────────────────────

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
              Set up your AI workspace
            </Typography>
            <Typography
              variant="body1"
              sx={{ color: 'rgba(255,255,255,0.8)', maxWidth: 420 }}
            >
              Register your organization to start managing AI gateways, LLM
              providers, and intelligent application proxies — all in one place.
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

                {/* ── Success state ── */}
                {registeredOrgName ? (
                  isSuperAdmin ? (
                    <Stack spacing={3} alignItems="center" sx={{ py: 4 }}>
                      <Box
                        sx={{
                          width: 64, height: 64, borderRadius: '50%',
                          bgcolor: 'success.light',
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                          color: 'success.contrastText',
                        }}
                      >
                        <CheckCircle2 size={32} />
                      </Box>
                      <Stack spacing={0.5} alignItems="center">
                        <Typography variant="h5" fontWeight="bold">Organization Registered!</Typography>
                        <Typography variant="body2" color="text.secondary" textAlign="center">
                          <strong>{registeredOrgName}</strong> ({registeredOrgHandle}) has been created successfully.
                        </Typography>
                      </Stack>
                      <Button
                        variant="contained"
                        fullWidth
                        onClick={() => {
                          setRegisteredOrgName(null);
                          setRegisteredOrgHandle(null);
                          setForm({ id: generateUUID(), name: '', handle: '', region: 'us' });
                        }}
                      >
                        Register Another Organization
                      </Button>
                    </Stack>
                  ) : (
                    <RedirectingToWorkspace orgName={registeredOrgName} />
                  )
                ) : (

                  /* ── Registration form ── */
                  <Stack spacing={3}>
                    <Stack spacing={0.5}>
                      <Stack direction="row" spacing={1} alignItems="center">
                        <Building2 size={20} />
                        <Typography variant="h5" fontWeight="bold">
                          Register Organization
                        </Typography>
                      </Stack>
                      <Typography variant="body2" color="text.secondary">
                        Create a new organization on the AI Platform.
                      </Typography>
                    </Stack>

                    <Divider />

                    {apiError && (
                      <Alert severity="error" onClose={() => setApiError(null)}>
                        {apiError}
                      </Alert>
                    )}

                    <Box component="form" onSubmit={handleSubmit} noValidate>
                      <Stack spacing={2.5}>

                        {/* Organization Name */}
                        <FormControl fullWidth required disabled={isSubmitting}>
                          <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>
                            Organization Name
                          </FormLabel>
                          <TextField
                            placeholder="e.g. Acme Corporation"
                            value={form.name}
                            onChange={handleNameChange}
                            error={!!errors.name}
                            fullWidth autoFocus
                            disabled={isSubmitting}
                          />
                          <FormHelperText error={!!errors.name}>
                            {errors.name}
                          </FormHelperText>
                        </FormControl>

                        {/* Handle */}
                        <FormControl fullWidth required disabled={isSubmitting}>
                          <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>
                            Handle
                          </FormLabel>
                          <TextField
                            placeholder="e.g. acme"
                            value={form.handle}
                            onChange={handleHandleChange}
                            error={!!errors.handle}
                            fullWidth
                            disabled={isSubmitting}
                            inputProps={{ pattern: '[a-z0-9][-a-z0-9]*' }}
                          />
                          <FormHelperText error={!!errors.handle}>
                            {errors.handle}
                          </FormHelperText>
                        </FormControl>

                        {/* Region */}
                        <FormControl fullWidth required disabled={isSubmitting}>
                          <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>
                            Region
                          </FormLabel>
                          <Select
                            value={form.region}
                            onChange={(e) => handleRegionChange(e.target.value as string)}
                            error={!!errors.region}
                          >
                            {REGIONS.map(({ value, label }) => (
                              <MenuItem key={value} value={value}>{label}</MenuItem>
                            ))}
                          </Select>
                          {errors.region && (
                            <FormHelperText error>{errors.region}</FormHelperText>
                          )}
                        </FormControl>

                        {/* UUID */}
                        <FormControl fullWidth required disabled={isSubmitting}>
                          <FormLabel sx={{ mb: 0.5, fontWeight: 500 }}>
                            Organization ID (UUID)
                          </FormLabel>
                          <TextField
                            placeholder="e.g. 550e8400-e29b-41d4-a716-446655440000"
                            value={form.id}
                            onChange={handleIdChange}
                            error={!!errors.id}
                            fullWidth
                            disabled={isSubmitting}
                            inputProps={{ style: { fontFamily: 'monospace', fontSize: '0.85rem' } }}
                          />
                          <FormHelperText error={!!errors.id}>
                            {errors.id ?? 'Auto-generated — edit or click ↻ to regenerate'}
                          </FormHelperText>
                        </FormControl>

                        {/* Submit */}
                        <Button
                          type="submit"
                          variant="contained"
                          size="large"
                          fullWidth
                          disabled={isSubmitting}
                          startIcon={isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined}
                        >
                          {isSubmitting ? 'Registering…' : 'Register Organization'}
                        </Button>

                      </Stack>
                    </Box>
                  </Stack>
                )}

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
