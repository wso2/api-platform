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

import { useCopy } from './useCopy';

const messages = defineMessages({
  copied: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.SecretField.copied',
    defaultMessage: 'Copied',
    description: 'Replaces the copy label once the value is on the clipboard.',
  },
  copy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.SecretField.copy',
    defaultMessage: 'Copy {label}',
    description: 'Accessible name of the copy button. {label} is the field name, e.g. "Client secret".',
  },
  hide: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.SecretField.hide',
    defaultMessage: 'Hide {label}',
  },
  show: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.SecretField.show',
    defaultMessage: 'Show {label}',
  },
});

export type SecretFieldProps = {
  id: string;
  label: string;
  value: string;
  /** Hidden until revealed. Copy works either way. */
  masked?: boolean;
};

/** A read-only value with copy, and show/hide when it is a secret. */
export function SecretField({ id, label, value, masked = false }: SecretFieldProps) {
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
                  <IconButton
                    aria-label={revealLabel}
                    onClick={() => setRevealed((current) => !current)}
                    size="small"
                  >
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
