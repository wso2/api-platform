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

import { FormControl, InputAdornment, Stack, TextField } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';

import { CopyButton } from '../curl/components/CopyButton';

const messages = defineMessages({
  testKey: {
    id: 'apiControlPlane.pages.test.console.SecretValue.testKey',
    defaultMessage: 'Test key',
    description: 'Accessible label for the field showing the test key.',
  },
  copy: {
    id: 'apiControlPlane.pages.test.console.SecretValue.copy',
    defaultMessage: 'Copy key',
    description: 'Accessible label for the button copying the test key to the clipboard.',
  },
  hide: {
    id: 'apiControlPlane.pages.test.console.SecretValue.hide',
    defaultMessage: 'Hide key',
    description: 'Accessible label for the button that re-masks a revealed test key.',
  },
  reveal: {
    id: 'apiControlPlane.pages.test.console.SecretValue.reveal',
    defaultMessage: 'Show key',
    description: 'Accessible label for the button that reveals the masked test key.',
  },
});

/** Most characters of the value ever left legible when masked. */
const MAX_VISIBLE_PREFIX = 28;

/** Dots standing in for the hidden remainder. */
const MASK_LENGTH = 15;

/** Masks a credential with a visible prefix and fixed-length dots. */
export const maskSecret = (value: string): string => {
  if (value === '') return '';
  const visible = Math.min(MAX_VISIBLE_PREFIX, Math.floor(value.length / 2));
  return `${value.slice(0, visible)}${'•'.repeat(MASK_LENGTH)}`;
};

type SecretValueProps = {
  value: string;
  /** Extra controls rendered after reveal and copy, e.g. regenerate. */
  actions?: React.ReactNode;
  /** Rendered instead of the value when there is nothing to show. */
  placeholder?: string;
};

/**
 * Displays a credential masked, with reveal and copy.
 *
 * Reveal state is deliberately local and starts closed on every mount: a
 * revealed key must not survive a view toggle or a navigation, or it ends up
 * visible in a screen share the user forgot they had opened it in.
 *
 * Copy always takes the *real* value regardless of reveal state — a masked
 * value in the clipboard is silently useless, which is worse than the copy the
 * user asked for.
 */
export function SecretValue({ actions, placeholder, value }: SecretValueProps) {
  const intl = useIntl();

  const hasValue = value.trim() !== '';

  return (
    <Stack alignItems="center" direction="row" sx={{ width: '100%' }}>
      <FormControl fullWidth>
        <TextField
          fullWidth
          size="small"
          slotProps={{
            htmlInput: {
              'aria-label': intl.formatMessage(messages.testKey),
            },
            input: {
              readOnly: true,
              endAdornment: (
                <InputAdornment position="end">
                  <CopyButton
                    getValue={() => value}
                    label={intl.formatMessage(messages.copy)}
                    variant="icon"
                  />
                </InputAdornment>
              ),
            },
          }}
          value={hasValue ? maskSecret(value) : placeholder}
        />
      </FormControl>
      {actions}
    </Stack>
  );
}
