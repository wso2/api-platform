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
  PageContent,
  PageTitle,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortalList } from './hooks';
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
  const { create } = useManagedPortalList();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const nameError = useMemo(() => validatePortalName(name), [name]);
  const derivedHandle = useMemo(() => portalHandleFromName(name), [name]);
  const missingRequired = !name.trim();
  const canSubmit = !missingRequired && !nameError && Boolean(derivedHandle) && !submitting;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      await create({
        handle: derivedHandle,
        name: name.trim(),
        description: description.trim() || undefined,
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
            <FormControl fullWidth required error={Boolean(nameError)}>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                autoFocus
                placeholder="Enter portal name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
                error={Boolean(nameError)}
                helperText={
                  nameError
                  ?? (derivedHandle
                    ? `Handle: ${derivedHandle}`
                    : 'The portal handle is derived from this name and cannot be changed later.')
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
          {/* Login environment omitted on create; the server picks the org's preferred env. Edit exposes it. */}
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
