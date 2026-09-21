/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo, useState } from 'react';
import {
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortal, useOrgEnvironments } from './hooks';
import type { ManagedPortal, UpdateManagedPortalInput } from './types';
import { validatePortalName } from './utils/name';

export type ManagedPortalEditProps = {
  portalId: string;
  onCancel: () => void;
  onSaved: () => void;
};

/**
 * Fetches the full portal by id before rendering the form: the list projection
 * strips loginEnvironment, and seeding the picker off that stripped record made
 * the current env look unset. The form is a separate inner component so its
 * useState initializers see the fetched values on first render.
 */
export default function ManagedPortalEdit({ portalId, onCancel, onSaved }: ManagedPortalEditProps) {
  const { portal, isLoading, error, update } = useManagedPortal(portalId);

  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Typography variant="body2" color="text.secondary">
          Loading portal…
        </Typography>
      </PageContent>
    );
  }

  // Error and not-found are terminal for this view; without the back button the
  // user has no way back to the list except reloading the whole feature.
  if (error) {
    return (
      <PageContent fullWidth>
        <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onCancel}>
          Back to list
        </Button>
        <Typography variant="body2" color="error" sx={{ mt: 2 }}>
          {error.message}
        </Typography>
      </PageContent>
    );
  }

  if (!portal) {
    return (
      <PageContent fullWidth>
        <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onCancel}>
          Back to list
        </Button>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Portal not found.
        </Typography>
      </PageContent>
    );
  }

  return <EditForm portal={portal} update={update} onCancel={onCancel} onSaved={onSaved} />;
}

type EditFormProps = {
  portal: ManagedPortal;
  update: (input: UpdateManagedPortalInput) => Promise<ManagedPortal>;
  onCancel: () => void;
  onSaved: () => void;
};

function EditForm({ portal, update, onCancel, onSaved }: EditFormProps) {
  const { environments, isLoading: envsLoading, error: envsError } = useOrgEnvironments();

  const [name, setName] = useState(portal.name);
  const [description, setDescription] = useState(portal.description ?? '');
  const [loginEnvironment, setLoginEnvironment] = useState(portal.loginEnvironment ?? '');
  const [submitting, setSubmitting] = useState(false);

  const nameError = useMemo(() => validatePortalName(name), [name]);
  const missingRequired = !name.trim();

  // Compare on trim-normalized values so a whitespace-only edit does not enable
  // Save or restamp updatedAt server-side.
  const trimmedName = name.trim();
  const trimmedDescription = description.trim();
  const trimmedLogin = loginEnvironment.trim();

  const hasChanges = useMemo(() => {
    if (trimmedName !== portal.name) return true;
    if (trimmedDescription !== (portal.description ?? '')) return true;
    if (trimmedLogin !== (portal.loginEnvironment ?? '')) return true;
    return false;
  }, [portal, trimmedName, trimmedDescription, trimmedLogin]);

  const canSubmit = !missingRequired && !nameError && hasChanges && !submitting;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      const patch: UpdateManagedPortalInput = {
        ...(trimmedName !== portal.name ? { name: trimmedName } : {}),
        ...(trimmedDescription !== (portal.description ?? '') ? { description: trimmedDescription } : {}),
        ...(trimmedLogin !== (portal.loginEnvironment ?? '') ? { loginEnvironment: trimmedLogin } : {}),
      };
      if (Object.keys(patch).length === 0) {
        onSaved();
        return;
      }
      await update(patch);
      onSaved();
    } catch {
      // Hook already notified; leave the form in place with user input for retry.
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onCancel} disabled={submitting}>
        Back to list
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>Edit Portal</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Handle</FormLabel>
              <TextField fullWidth value={portal.handle} disabled />
            </FormControl>
          </Grid>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth required error={Boolean(nameError) || missingRequired}>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                autoFocus
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
                error={Boolean(nameError) || missingRequired}
                helperText={nameError ?? (missingRequired ? 'Name is required' : undefined)}
              />
            </FormControl>
          </Grid>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Description</FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={3}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                disabled={submitting}
              />
            </FormControl>
          </Grid>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Login environment</FormLabel>
              {/* Current env kept as a synthetic option when missing from the list, so out-of-band deletions show as a mismatch rather than a silent swap. */}
              <Select
                fullWidth
                value={loginEnvironment}
                onChange={(event) => setLoginEnvironment(String(event.target.value))}
                disabled={submitting || envsLoading}
                displayEmpty
              >
                {loginEnvironment && !environments.some((e) => e.name === loginEnvironment) && (
                  <MenuItem value={loginEnvironment}>
                    {loginEnvironment} (not in current environment list)
                  </MenuItem>
                )}
                {environments.map((env) => (
                  <MenuItem key={env.name} value={env.name}>
                    {env.displayName ? `${env.displayName} (${env.name})` : env.name}
                  </MenuItem>
                ))}
                {environments.length === 0 && !envsLoading && (
                  <MenuItem value="" disabled>
                    No environments - provision one first
                  </MenuItem>
                )}
              </Select>
              {envsError && (
                <Typography variant="caption" color="error" sx={{ mt: 0.5 }}>
                  Failed to load environments: {envsError.message}
                </Typography>
              )}
            </FormControl>
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button variant="outlined" color="secondary" disabled={submitting} onClick={onCancel}>
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={!canSubmit}
            onClick={handleSubmit}
            startIcon={submitting ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {submitting ? 'Saving…' : 'Save Changes'}
          </Button>
        </Box>
      </Box>
    </PageContent>
  );
}
