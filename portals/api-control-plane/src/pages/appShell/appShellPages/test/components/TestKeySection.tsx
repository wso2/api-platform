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

import { Box, Button, CircularProgress, Stack, Typography } from '@wso2/oxygen-ui';
import { LucideKeyRound, RefreshCw } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { ApiKeyLocation } from '../utils/apiKeyAuth';
import { SecretValue } from './SecretValue';

const messages = defineMessages({
  expired: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.expired',
    defaultMessage: 'The key has expired.',
    description:
      'Replaces the countdown once the test key is no longer valid. A whole sentence, because it follows another one in the same line.',
  },
  expiresIn: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.expiresIn',
    defaultMessage:
      'Expires in {minutes, plural, =0 {less than a minute} one {# min} other {# min}}.',
    description:
      'Countdown to the test key expiring, following the sentence about how the key is sent. {minutes} is the whole minutes remaining.',
  },
  failed: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.failed',
    defaultMessage: 'Could not create a test key.',
    description: 'Shown when minting a test key failed.',
  },
  newKey: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.newKey',
    defaultMessage: 'Get Test API Key',
    description: 'Button that issues a replacement test key. A command.',
  },
  placeholder: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.placeholder',
    defaultMessage: 'No key yet',
    description: 'Stand-in shown where the key value would be, before one exists.',
  },
  scopedToHeader: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.scopedToHeader',
    defaultMessage: 'Sent as the {name} header on requests to this gateway.',
    description:
      'Explains how the key travels when the policy sends it in a header. {name} is the HTTP header name, editable in the cURL view.',
  },
  scopedToQuery: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.scopedToQuery',
    defaultMessage: 'Sent as the {name} query parameter on requests to this gateway.',
    description:
      'Explains how the key travels when the policy sends it in the query string. {name} is the query parameter name, editable in the cURL view.',
  },
  title: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.title',
    defaultMessage: 'Test key',
    description: 'Heading of the section holding the credential the console sends. A noun.',
  },
});

type TestKeySectionProps = {
  error?: boolean;
  /** Name of the header or query parameter carrying the key. */
  keyName: string;
  loading?: boolean;
  /** Where the policy places the credential. */
  location: ApiKeyLocation;
  onRegenerate: () => void;
  regenerating?: boolean;
  /**
    * Milliseconds until the key expires; zero once expired.
    * Counted down by the owning page.
   */
  remainingMs: number;
  value?: string;
};

/**
 * Displays the session-cached test key, its expiry countdown, and replacement action.
 * The key is minted on first render; see `utils/useTestApiKey`.
 */
export function TestKeySection({
  error,
  keyName,
  loading,
  location,
  onRegenerate,
  regenerating,
  remainingMs,
  value,
}: TestKeySectionProps) {
  const intl = useIntl();

  const expired = remainingMs === 0;
  const minutes = Math.floor(remainingMs / 60_000);

  return (
    <Box sx={{ px: 2, py: 2 }}>
      <Stack spacing={1.5}>
        <Stack spacing={0.5}>
          <Stack alignItems="center" direction="row" spacing={1}>
            <Box sx={{ color: 'primary.main', display: 'flex' }}>
              <LucideKeyRound size={18} />
            </Box>
            <Typography variant="subtitle2">
              <FormattedMessage {...messages.title} />
            </Typography>
          </Stack>

          {/* Keep the description and countdown as separate sentences for
              clearer pluralization and expiry handling. */}
          <Typography
            color={error ? 'error' : 'text.secondary'}
            sx={{ display: 'block' }}
            variant="caption"
          >
            {error ? (
              <FormattedMessage {...messages.failed} />
            ) : (
              <>
                <FormattedMessage
                  {...(location === 'query' ? messages.scopedToQuery : messages.scopedToHeader)}
                  values={{
                    name: <Box component="code">{keyName}</Box>,
                  }}
                />
                {/* No countdown before a key exists — there is nothing yet to
                    expire, and "Expires in 60 min" beside an empty well would
                    describe a credential that has not been issued. */}
                {value && (
                  <>
                    {' '}
                    {expired ? (
                      // Only the expired clause turns red. Repainting the whole
                      // line would read as a failure, and the sentence next to
                      // it is still true.
                      <Box component="span" sx={{ color: 'error.main' }}>
                        <FormattedMessage {...messages.expired} />
                      </Box>
                    ) : (
                      <FormattedMessage {...messages.expiresIn} values={{ minutes }} />
                    )}
                  </>
                )}
              </>
            )}
          </Typography>
        </Stack>

        {loading ? (
          <Stack alignItems="center" direction="row" spacing={1} sx={{ py: 1 }}>
            <CircularProgress size={16} />
          </Stack>
        ) : (
          <Stack direction="row" spacing={1} sx={{ width: '100%' }}>
            <SecretValue
              actions={
                <Button
                  disabled={regenerating}
                  onClick={onRegenerate}
                  size="small"
                  startIcon={
                    regenerating ? <CircularProgress size={14} /> : <RefreshCw size={16} />
                  }
                  sx={{ flexShrink: 0, ml: 0.5 }}
                  variant="contained"
                >
                  <FormattedMessage {...messages.newKey} />
                </Button>
              }
              placeholder={intl.formatMessage(messages.placeholder)}
              // An expired key is presented as absent: its value is still in
              // memory, but showing it invites pasting a credential that no
              // longer works and reads as a gateway fault.
              value={expired ? '' : (value ?? '')}
            />
          </Stack>
        )}
      </Stack>
    </Box>
  );
}
