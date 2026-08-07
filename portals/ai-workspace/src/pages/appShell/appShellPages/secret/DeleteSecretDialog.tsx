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

import React, { useEffect, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { deleteSecret, SecretConflictError, type SecretMetadata, type SecretReference } from '../../../../apis/secretApis';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import useIsMounted from '../../../../hooks/useIsMounted';
import { getErrorMessage } from '../../../../utils/apiError';

interface DeleteSecretDialogProps {
  secret: SecretMetadata | null;
  onClose: () => void;
  /** Called after the secret has actually been permanently deleted on the server. */
  onDeleted: (handle: string) => void;
}

/**
 * Deleting a secret permanently removes it on the backend and is blocked with a
 * 409 if the secret is still referenced by another resource. This dialog handles
 * both outcomes: a plain confirmation, or — once blocked — the list of
 * referencing resources returned on the conflict.
 */
export default function DeleteSecretDialog({ secret, onClose, onDeleted }: DeleteSecretDialogProps): React.JSX.Element {
  const showSnackbar = useAIWorkspaceSnackbar();
  const isMounted = useIsMounted();
  const [isDeleting, setIsDeleting] = useState(false);
  const [conflictRefs, setConflictRefs] = useState<SecretReference[] | null>(null);

  useEffect(() => {
    setConflictRefs(null);
  }, [secret?.id]);

  const handleConfirm = async () => {
    if (!secret) return;
    setIsDeleting(true);
    try {
      await deleteSecret(secret.id);
      if (!isMounted()) return;
      showSnackbar(`"${secret.displayName}" was deleted.`, 'success');
      onDeleted(secret.id);
      onClose();
    } catch (error) {
      if (!isMounted()) return;
      if (error instanceof SecretConflictError) {
        setConflictRefs(error.conflict.references);
      } else {
        showSnackbar(getErrorMessage(error, 'Failed to delete secret.'), 'error');
        onClose();
      }
    } finally {
      if (isMounted()) setIsDeleting(false);
    }
  };

  return (
    <Dialog open={Boolean(secret)} onClose={onClose}>
      {conflictRefs ? (
        <>
          <DialogTitle>Can&apos;t delete this secret</DialogTitle>
          <DialogContent>
            <DialogContentText sx={{ mb: 2 }}>
              <strong>{secret?.displayName}</strong> is still referenced by {conflictRefs.length}{' '}
              {conflictRefs.length === 1 ? 'resource' : 'resources'}. Remove the reference from each one below before
              deleting it again.
            </DialogContentText>
            <Stack spacing={1}>
              {conflictRefs.map((ref) => (
                <Box
                  key={`${ref.type}-${ref.handle}`}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.5,
                    p: 1.25,
                    borderRadius: 1,
                    border: 1,
                    borderColor: 'divider',
                  }}
                >
                  <Chip label={ref.type} size="small" variant="outlined" />
                  <Box sx={{ minWidth: 0 }}>
                    <Typography variant="body2" sx={{ fontWeight: 600 }} noWrap>
                      {ref.name}
                    </Typography>
                  </Box>
                </Box>
              ))}
            </Stack>
          </DialogContent>
          <DialogActions>
            <Button variant="contained" onClick={onClose}>
              Got it
            </Button>
          </DialogActions>
        </>
      ) : (
        <>
          <DialogTitle>Delete secret</DialogTitle>
          <DialogContent>
            <DialogContentText>
              Delete <strong>{secret?.displayName}</strong>? This permanently deletes the secret and cannot be
              undone. Deletion is blocked while any resource still references it.
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button variant="outlined" color="secondary" onClick={onClose} disabled={isDeleting}>
              Cancel
            </Button>
            <Button color="error" onClick={() => void handleConfirm()} disabled={isDeleting}>
              {isDeleting ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}
