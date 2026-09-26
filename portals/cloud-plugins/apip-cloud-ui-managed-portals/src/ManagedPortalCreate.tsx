/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useMemo, useState } from 'react';
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
  Tooltip,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortalList, useOrgEnvironments } from './hooks';
import { portalHandleFromName, validatePortalName } from './utils/name';

export type ManagedPortalCreateProps = {
  onCancel: () => void;
  onCreated: () => void;
};

/**
 * Full-page create form, matching the gateway provision-page layout so add /
 * edit forms share visual conventions across cloud plugins.
 */
export default function ManagedPortalCreate({ onCancel, onCreated }: ManagedPortalCreateProps) {
  const { portals, create } = useManagedPortalList();
  const { environments, isLoading: envsLoading, error: envsError } = useOrgEnvironments();
  // Single-env orgs and env-load failures leave the picker out of the form; the server picks the
  // default (or falls back sensibly) so the user is never shown an empty or single-option dropdown.
  const showEnvPicker = !envsLoading && !envsError && environments.length > 1;

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loginEnvironment, setLoginEnvironment] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // Once the env list arrives, preselect the first production env (isProduction=true);
  // if none is flagged, fall back to the first env. Only fires once — a later manual
  // change from the user stays put. Runs only in the multi-env case since the picker
  // is hidden otherwise (server picks in the hidden case).
  useEffect(() => {
    if (!showEnvPicker) return;
    if (loginEnvironment) return;
    const preferred = environments.find((e) => e.isProduction) ?? environments[0];
    if (preferred) setLoginEnvironment(preferred.name);
  }, [showEnvPicker, environments, loginEnvironment]);

  const nameError = useMemo(() => validatePortalName(name), [name]);
  const derivedHandle = useMemo(() => portalHandleFromName(name), [name]);
  // Client-side handle-collision check against the org's loaded portals. The server is
  // still the source of truth (there's a race where another user creates the same handle
  // between load and submit — a 409 from create() surfaces via the hook), but showing this
  // inline saves the user a failed submit for the common case where the collision is visible.
  const handleTaken = useMemo(
    () => Boolean(derivedHandle) && portals.some((p) => p.handle === derivedHandle),
    [derivedHandle, portals],
  );
  const missingRequired = !name.trim();
  const canSubmit = !missingRequired && !nameError && Boolean(derivedHandle) && !handleTaken && !submitting;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      await create({
        handle: derivedHandle,
        name: name.trim(),
        description: description.trim() || undefined,
        // Only send loginEnvironment when the picker was shown AND the user picked something;
        // otherwise let the server pick its default (single-env orgs, or env-load failure).
        ...(showEnvPicker && loginEnvironment ? { loginEnvironment } : {}),
      });
      onCreated();
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
          <PageTitle.Header>Add Portal</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth required error={Boolean(nameError) || handleTaken}>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                autoFocus
                placeholder="Enter portal name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
                error={Boolean(nameError) || handleTaken}
                helperText={
                  nameError
                  ?? (handleTaken
                    ? `A portal with handle "${derivedHandle}" already exists in this organization.`
                    : (derivedHandle
                      ? `Handle: ${derivedHandle}`
                      : 'The portal handle is derived from this name and cannot be changed later.'))
                }
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
          {/* Login-env picker only when the org has more than one env AND the env list loaded */}
          {/* successfully. Single-env orgs skip the field entirely (server picks it). */}
          {showEnvPicker ? (
            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <Tooltip title="The data-plane environment whose auth server backs portal-user login." arrow placement="top-start">
                  <FormLabel sx={{ width: 'fit-content' }}>Login environment</FormLabel>
                </Tooltip>
                <Select
                  fullWidth
                  value={loginEnvironment}
                  onChange={(event) => setLoginEnvironment(String(event.target.value))}
                  disabled={submitting}
                >
                  {environments.map((env) => (
                    <MenuItem key={env.name} value={env.name}>
                      {env.displayName ? `${env.displayName} (${env.name})` : env.name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          ) : null}
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
            {submitting ? 'Adding…' : 'Add Portal'}
          </Button>
        </Box>
      </Box>
    </PageContent>
  );
}
