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

import { useEffect, useRef, useState } from 'react';
import { Box, CodeBlock, IconButton, Tooltip } from '@wso2/oxygen-ui';
import { Check, Copy } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  copied: {
    id: 'gateways.CopyableCommand.copied',
    defaultMessage: 'Copied',
    description: 'Tooltip confirming the command was copied to the clipboard.',
  },
  copy: {
    id: 'gateways.CopyableCommand.copy',
    defaultMessage: 'Copy command',
    description: 'Accessible label and tooltip for the button that copies a terminal command.',
  },
});

/** How long the button stays in its confirmed state after a copy. */
const COPIED_FEEDBACK_MS = 1500;

/** A shell command block with a copy-to-clipboard button in its corner. */
export function CopyableCommand({ code }: { code: string }) {
  const intl = useIntl();
  const [copied, setCopied] = useState(false);

  // Held in a ref so a copy that lands just before the block unmounts (a tab
  // change, a dialog closing) does not leave a timer setting state afterwards.
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(resetTimer.current), []);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.clearTimeout(resetTimer.current);
      resetTimer.current = window.setTimeout(() => setCopied(false), COPIED_FEEDBACK_MS);
    } catch {
      // No clipboard permission (or no clipboard at all, over plain HTTP). The
      // command is still on screen and selectable, so there is nothing useful
      // to report; an error toast here would only interrupt.
    }
  };

  return (
    <Box sx={{ position: 'relative' }}>
      <Tooltip title={intl.formatMessage(copied ? messages.copied : messages.copy)}>
        <IconButton
          aria-label={intl.formatMessage(messages.copy)}
          onClick={copy}
          size="small"
          sx={{ position: 'absolute', right: 8, top: 8, zIndex: 1 }}
        >
          {copied ? <Check size={16} /> : <Copy size={16} />}
        </IconButton>
      </Tooltip>
      <CodeBlock code={code} language="bash" />
    </Box>
  );
}
