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

import { useEffect, useState } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from '@wso2/oxygen-ui';

export type ConfirmDialogProps = {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Use the error color for the confirm button (destructive actions). */
  destructive?: boolean;
  /**
   * When set, the user must type this exact phrase before confirm is enabled
   * (legacy "type the name to confirm" pattern for irreversible deletes).
   */
  confirmPhrase?: string;
  /** Label for the type-to-confirm field; defaults to a generic prompt. */
  confirmInputLabel?: string;
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

/** A small controlled confirmation dialog for destructive/irreversible actions. */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  destructive,
  confirmPhrase,
  confirmInputLabel,
  loading,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState('');

  // Reset the typed phrase whenever the dialog opens/closes.
  useEffect(() => {
    if (!open) setTyped('');
  }, [open]);

  const matched = !confirmPhrase || typed === confirmPhrase;
  const canConfirm = matched && !loading;

  return (
    <Dialog fullWidth maxWidth="xs" onClose={onCancel} open={open}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <DialogContentText>{message}</DialogContentText>
        {confirmPhrase && (
          <TextField
            autoFocus
            fullWidth
            label={confirmInputLabel ?? `Type "${confirmPhrase}" to confirm`}
            onChange={(event) => setTyped(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && canConfirm) onConfirm();
            }}
            placeholder={confirmPhrase}
            size="small"
            sx={{ mt: 2 }}
            value={typed}
          />
        )}
      </DialogContent>
      <DialogActions>
        <Button color="inherit" disabled={loading} onClick={onCancel}>
          {cancelLabel}
        </Button>
        <Button
          color={destructive ? 'error' : 'primary'}
          disabled={!canConfirm}
          onClick={onConfirm}
          variant="contained"
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
