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
import { Button, IconButton, Tooltip } from '@wso2/oxygen-ui';
import { Check, Copy } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  copied: {
    id: 'apiControlPlane.pages.test.console.CopyButton.copied',
    defaultMessage: 'Copied',
    description: 'Tooltip and button label confirming the value reached the clipboard.',
  },
  copy: {
    id: 'apiControlPlane.pages.test.console.CopyButton.copy',
    defaultMessage: 'Copy',
    description: 'Default accessible label for a button that copies a value to the clipboard.',
  },
});

/** How long the button stays in its confirmed state after a copy. */
const COPIED_FEEDBACK_MS = 1500;

type CopyButtonProps = {
  /** Produces the text to copy, which may differ from the displayed value. */
  getValue: () => string;
  /** Accessible label. Defaults to a generic "Copy". */
  label?: string;
  /** Renders as a labelled button rather than an icon-only one. */
  variant?: 'icon' | 'button';
};

/**
 * Copies a value to the clipboard, confirming in place.
 *
 * Confirmation is inline rather than a toast: the console has several of these
 * within a screen of each other, and a toast would not say which one fired.
 */
export function CopyButton({ getValue, label, variant = 'icon' }: CopyButtonProps) {
  const intl = useIntl();
  const [copied, setCopied] = useState(false);

  // Held in a ref so a copy landing just before unmount (a view toggle, a tab
  // change) does not leave a timer setting state on a gone component.
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(resetTimer.current), []);

  const accessibleLabel = label ?? intl.formatMessage(messages.copy);
  const title = copied ? intl.formatMessage(messages.copied) : accessibleLabel;

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(getValue());
      setCopied(true);
      window.clearTimeout(resetTimer.current);
      resetTimer.current = window.setTimeout(() => setCopied(false), COPIED_FEEDBACK_MS);
    } catch {
      // Clipboard unavailable; the value remains visible and selectable.
    }
  };

  if (variant === 'button') {
    return (
      <Button
        onClick={copy}
        size="small"
        startIcon={copied ? <Check size={16} /> : <Copy size={16} />}
        variant="contained"
      >
        {title}
      </Button>
    );
  }

  return (
    <Tooltip title={title}>
      <IconButton aria-label={accessibleLabel} onClick={copy} size="small">
        {copied ? <Check size={16} /> : <Copy size={16} />}
      </IconButton>
    </Tooltip>
  );
}
