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
import { Box, Button, CircularProgress, Stack, Typography } from '@wso2/oxygen-ui';
import { LucideKeyRound, RefreshCw } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

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
  scopedTo: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.scopedTo',
    defaultMessage: 'Sent as the {header} header on requests to this gateway.',
    description:
      'Explains how the key travels. {header} is the HTTP header name, editable in the cURL view.',
  },
  title: {
    id: 'apiControlPlane.pages.test.console.TestKeySection.title',
    defaultMessage: 'Test key',
    description: 'Heading of the section holding the credential the console sends. A noun.',
  },
});

/** How often the countdown re-renders. */
const TICK_MS = 30_000;

type TestKeySectionProps = {
  error?: boolean;
  headerName: string;
  loading?: boolean;
  onRegenerate: () => void;
  regenerating?: boolean;
  /** Milliseconds until the key expires; zero once it has. */
  remainingMs: number;
  value?: string;
};

/**
 * The credential the console sends, with its countdown and a way to replace it.
 *
 * The key is minted on first render and cached for the session, so this usually
 * opens already populated — see `utils/useTestApiKey` for why
 * obtaining a key and creating one are the same operation.
 *
 * "New key" sits inside the value well, next to reveal and copy, rather than on
 * a row of its own: all three act on the value beside them, while the line
 * above the well describes the key rather than offering anything to do to it.
 *
 * That line carries the countdown too, instead of a right-aligned badge in the
 * heading. Both facts answer the same question — what this key is and how long
 * it lasts — so they read as prose together, and the heading stays a heading.
 */
export function TestKeySection({
  error,
  headerName,
  loading,
  onRegenerate,
  regenerating,
  remainingMs,
  value,
}: TestKeySectionProps) {
  const intl = useIntl();

  // The countdown is derived from a timestamp, so nothing re-renders it as time
  // passes. This tick exists only to keep the displayed minutes honest; at
  // 30 seconds it is never more than half a minute stale.
  const [, setTick] = useState(0);
  useEffect(() => {
    const timer = window.setInterval(() => setTick((current) => current + 1), TICK_MS);
    return () => window.clearInterval(timer);
  }, []);

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

          {/* Two whole sentences sharing a line, never one message built from
              fragments: the countdown is plural-inflected and swaps out
              entirely once the key expires, so folding it into the sentence
              beside it would make one message carry every combination. */}
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
                  {...messages.scopedTo}
                  values={{
                    header: <Box component="code">{headerName}</Box>,
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
