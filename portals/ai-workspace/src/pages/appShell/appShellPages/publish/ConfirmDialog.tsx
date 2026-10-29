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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useEffect, useId, useState, type FormEvent } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormLabel,
  OutlinedInput,
  Stack,
} from '@wso2/oxygen-ui';

const messages = defineMessages({
  typeToConfirm: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.confirmDialog.typeToConfirm',
    defaultMessage: 'Type "{phrase}" to confirm',
  },
  cancel: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.confirmDialog.cancel',
    defaultMessage: 'Cancel',
  },
  confirm: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.confirmDialog.confirm',
    defaultMessage: 'Confirm',
  },
});

export type ConfirmDialogProps = {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Use the error color for the confirm button (destructive actions). */
  destructive?: boolean;
  /** Overrides the confirm button's colour, e.g. `warning`. */
  confirmColor?: 'primary' | 'error' | 'warning';
  /** When set, the user must type this exact phrase before confirm is enabled. */
  confirmPhrase?: string;
  confirmInputLabel?: string;
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

/** A small controlled confirmation dialog, with an optional type-the-name-to-confirm field. */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  cancelLabel,
  destructive,
  confirmColor,
  confirmPhrase,
  confirmInputLabel,
  loading,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const intl = useIntl();
  const [typed, setTyped] = useState('');
  const fieldId = useId();

  useEffect(() => {
    if (!open) setTyped('');
  }, [open]);

  const canConfirm = (!confirmPhrase || typed === confirmPhrase) && !loading;

  // A real `form`, so Enter in the phrase field confirms the way a form is expected to.
  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (canConfirm) onConfirm();
  };

  return (
    <Dialog fullWidth maxWidth="xs" onClose={onCancel} open={open}>
      <DialogTitle>{title}</DialogTitle>
      <Box component="form" noValidate onSubmit={handleSubmit}>
        <DialogContent>
          <Stack spacing={2}>
            <DialogContentText>{message}</DialogContentText>
            {confirmPhrase && (
              <FormControl fullWidth>
                <FormLabel htmlFor={fieldId}>
                  {confirmInputLabel ?? intl.formatMessage(messages.typeToConfirm, { phrase: confirmPhrase })}
                </FormLabel>
                <OutlinedInput
                  autoFocus
                  id={fieldId}
                  onChange={(event) => setTyped(event.target.value)}
                  placeholder={confirmPhrase}
                  value={typed}
                />
              </FormControl>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" disabled={loading} onClick={onCancel}>
            {cancelLabel ?? intl.formatMessage(messages.cancel)}
          </Button>
          <Button
            color={confirmColor ?? (destructive ? 'error' : 'primary')}
            disabled={!canConfirm}
            type="submit"
            variant="contained"
          >
            {confirmLabel ?? intl.formatMessage(messages.confirm)}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
