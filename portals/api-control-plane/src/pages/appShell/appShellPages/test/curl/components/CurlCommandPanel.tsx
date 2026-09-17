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

import { useMemo, useState } from 'react';
import { Box, Button, CodeBlock, Stack, Typography } from '@wso2/oxygen-ui';
import { Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { CopyButton } from './CopyButton';
import { curlSummary, toCurl } from '../utils/toCurl';
import type { ConsoleRequest } from '../../utils/types';

const messages = defineMessages({
  copyCommand: {
    id: 'apiControlPlane.pages.test.console.CurlCommandPanel.copyCommand',
    defaultMessage: 'Copy command',
    description: 'Button that copies the whole curl command. A command.',
  },
  hideKey: {
    id: 'apiControlPlane.pages.test.console.CurlCommandPanel.hideKey',
    defaultMessage: 'Key hidden',
    description:
      'State of the toggle when the credential is masked in the displayed command. An adjective, not a command.',
  },
  showKey: {
    id: 'apiControlPlane.pages.test.console.CurlCommandPanel.showKey',
    defaultMessage: 'Key shown',
    description:
      'State of the toggle when the credential is visible in the displayed command. An adjective, not a command.',
  },
  summary: {
    id: 'apiControlPlane.pages.test.console.CurlCommandPanel.summary',
    defaultMessage:
      '{method} · {headerCount, plural, =0 {no headers} one {# header} other {# headers}}{bodyKind, select, json { · JSON body} xml { · XML body} text { · text body} formData { · form data} urlEncoded { · URL-encoded body} other {}}',
    description:
      'One-line description of the command. {method} is an HTTP verb and is not translated. {bodyKind} selects how the body is encoded, or "none" for no body.',
  },
  title: {
    id: 'apiControlPlane.pages.test.console.CurlCommandPanel.title',
    defaultMessage: 'cURL command',
    description: 'Heading of the generated command block. A noun.',
  },
});

/**
 * Body kinds as ICU `select` selectors.
 *
 * ICU selector fragments must be identifiers, so the model's real mode names —
 * `form-data` and `url-encoded` — cannot appear in a message; a hyphen there
 * fails to parse. Mapped here rather than renamed in the model, because those
 * are the names of the encodings everywhere else.
 */
const ICU_BODY_KIND: Record<string, string> = {
  'form-data': 'formData',
  'url-encoded': 'urlEncoded',
};

/** Height at which the wrapped command scrolls instead of growing further. */
const CODE_MAX_HEIGHT = 260;

type CurlCommandPanelProps = {
  request: ConsoleRequest;
};

/**
 * The generated command, with copy and script export.
 *
 * The reveal toggle governs **display only**. Copy always takes the real
 * credential, because that is the only version that runs — a command pasted
 * into a terminal with `<redacted>` in it fails with an authentication error
 * that looks like a gateway problem. The notice under the block says so
 * outright rather than leaving the user to discover it.
 */
export function CurlCommandPanel({ request }: CurlCommandPanelProps) {
  const intl = useIntl();
  const [revealed, setRevealed] = useState(false);

  const summary = useMemo(() => curlSummary(request), [request]);
  const displayed = useMemo(
    () => toCurl(request, { revealSecrets: revealed }),
    [request, revealed],
  );

  // API keys may be in headers or query params; keep masking and reveal in sync.
  const hasSecret = [...request.headers, ...request.queryParams].some(
    (row) => row.secret && row.enabled,
  );

  return (
    <Box sx={{ borderTop: '1px solid', borderColor: 'divider', px: 2, py: 2 }}>
      <Stack
        alignItems="center"
        direction="row"
        flexWrap="wrap"
        justifyContent="space-between"
        spacing={1}
        sx={{ mb: 1.5 }}
        useFlexGap
      >
        <Stack alignItems="baseline" direction="row" spacing={1}>
          <Typography variant="subtitle2">
            <FormattedMessage {...messages.title} />
          </Typography>
          <Typography color="text.secondary" variant="caption">
            <FormattedMessage
              {...messages.summary}
              values={{
                bodyKind: ICU_BODY_KIND[summary.bodyKind] ?? summary.bodyKind,
                headerCount: summary.headerCount,
                method: summary.method,
              }}
            />
          </Typography>
        </Stack>

        {hasSecret && (
          <Button
            onClick={() => setRevealed((current) => !current)}
            size="small"
            startIcon={revealed ? <Eye size={16} /> : <EyeOff size={16} />}
            variant="outlined"
          >
            <FormattedMessage {...(revealed ? messages.showKey : messages.hideKey)} />
          </Button>
        )}
      </Stack>

      <Box
        sx={{
          position: 'relative',
          '&& pre': {
            maxHeight: CODE_MAX_HEIGHT,
            overflowX: 'hidden',
            overflowY: 'auto',
            overflowWrap: 'anywhere',
            pr: 6,
            whiteSpace: 'pre-wrap',
          },
        }}
      >
        <CodeBlock code={displayed} language="bash" />
        <Box sx={{ position: 'absolute', right: 8, top: 8, zIndex: 1 }}>
          <CopyButton
            getValue={() => toCurl(request, { revealSecrets: true })}
            label={intl.formatMessage(messages.copyCommand)}
          />
        </Box>
      </Box>
    </Box>
  );
}
