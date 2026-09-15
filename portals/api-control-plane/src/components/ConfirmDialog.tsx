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

import { useEffect, useId, useState, type FormEvent } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  type DialogProps,
  DialogTitle,
  Form,
  OutlinedInput,
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
  maxWidth?: DialogProps['maxWidth'];
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
  maxWidth = 'xs',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState('');
  // Generated rather than a constant: this dialog is rendered by several pages,
  // and two mounted at once would otherwise share one id, pointing both labels
  // at whichever input the DOM saw first.
  const fieldId = useId();

  // Reset the typed phrase whenever the dialog opens/closes.
  useEffect(() => {
    if (!open) setTyped('');
  }, [open]);

  const matched = !confirmPhrase || typed === confirmPhrase;
  const canConfirm = matched && !loading;

  // A real `form` element, so Enter in the phrase field confirms the way a form
  // is expected to — no key handler duplicating the browser's own behaviour.
  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (canConfirm) onConfirm();
  };

  return (
    <Dialog fullWidth maxWidth={maxWidth} onClose={onCancel} open={open}>
      <DialogTitle>{title}</DialogTitle>
      <Box component="form" noValidate onSubmit={handleSubmit}>
        <DialogContent>
          <Form.Stack spacing={2}>
            <DialogContentText>{message}</DialogContentText>
            {confirmPhrase && (
              // `ElementWrapper` is the Oxygen form primitive for exactly this:
              // a full-width `FormControl` with its `FormLabel` bound to the
              // control by id. It carries no `required`/`error` of its own, which
              // is why it suits this field — the phrase is validated by matching,
              // not by field state, so there is nothing to propagate.
              <Form.ElementWrapper
                label={confirmInputLabel ?? `Type "${confirmPhrase}" to confirm`}
                name={fieldId}
              >
                <OutlinedInput
                  autoFocus
                  id={fieldId}
                  onChange={(event) => setTyped(event.target.value)}
                  placeholder={confirmPhrase}
                  size="small"
                  value={typed}
                />
              </Form.ElementWrapper>
            )}
          </Form.Stack>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" disabled={loading} onClick={onCancel}>
            {cancelLabel}
          </Button>
          <Button
            color={destructive ? 'error' : 'primary'}
            disabled={!canConfirm}
            type="submit"
            variant="contained"
          >
            {confirmLabel}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
