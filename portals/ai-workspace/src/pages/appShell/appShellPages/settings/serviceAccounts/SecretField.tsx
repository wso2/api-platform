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

import { useState } from 'react';
import {
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  OutlinedInput,
  Stack,
  Tooltip,
} from '@wso2/oxygen-ui';
import { Check, Copy, Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';
import { useAIWorkspaceSnackbar } from '../../../../../hooks/aiWorkspaceSnackbar';

const P = 'aiWorkspace.pages.appShell.appShellPages.settings.serviceAccounts.SecretField';
const messages = defineMessages({
  copied: { id: `${P}.copied`, defaultMessage: 'Copied' },
  copy: { id: `${P}.copy`, defaultMessage: 'Copy {label}' },
  copyFailed: { id: `${P}.copyFailed`, defaultMessage: 'Could not copy. Select the value and copy it manually.' },
  hide: { id: `${P}.hide`, defaultMessage: 'Hide {label}' },
  show: { id: `${P}.show`, defaultMessage: 'Show {label}' },
});

/** Copies to the clipboard; `copied` turns true once it worked. */
export function useCopy() {
  const intl = useIntl();
  const showSnackbar = useAIWorkspaceSnackbar();
  const [copied, setCopied] = useState(false);

  const copy = (value: string) => {
    // Clipboard access is missing outside secure contexts.
    const written = navigator.clipboard?.writeText(value);
    if (!written) {
      showSnackbar(intl.formatMessage(messages.copyFailed), 'error');
      return;
    }
    written
      .then(() => setCopied(true))
      .catch(() => showSnackbar(intl.formatMessage(messages.copyFailed), 'error'));
  };

  return { copied, copy };
}

interface SecretFieldProps {
  id: string;
  label: string;
  value: string;
  /** Hidden until revealed. Copy works either way. */
  masked?: boolean;
}

export default function SecretField({ id, label, value, masked = false }: SecretFieldProps) {
  const intl = useIntl();
  const [revealed, setRevealed] = useState(!masked);
  const { copied, copy } = useCopy();

  const copyLabel = copied
    ? intl.formatMessage(messages.copied)
    : intl.formatMessage(messages.copy, { label });
  const revealLabel = intl.formatMessage(revealed ? messages.hide : messages.show, { label });

  return (
    <FormControl fullWidth>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <OutlinedInput
        endAdornment={
          <InputAdornment position="end">
            <Stack alignItems="center" direction="row" spacing={1}>
              {masked && (
                <Tooltip arrow title={revealLabel}>
                  <IconButton aria-label={revealLabel} onClick={() => setRevealed((r) => !r)} size="small">
                    {revealed ? <EyeOff size={16} /> : <Eye size={16} />}
                  </IconButton>
                </Tooltip>
              )}
              <Tooltip arrow title={copyLabel}>
                <IconButton aria-label={copyLabel} onClick={() => copy(value)} size="small">
                  {copied ? <Check size={16} /> : <Copy size={16} />}
                </IconButton>
              </Tooltip>
            </Stack>
          </InputAdornment>
        }
        fullWidth
        id={id}
        inputProps={{ autoComplete: 'off', readOnly: true, spellCheck: false }}
        sx={{ fontFamily: 'monospace' }}
        type={revealed ? 'text' : 'password'}
        value={value}
      />
    </FormControl>
  );
}
