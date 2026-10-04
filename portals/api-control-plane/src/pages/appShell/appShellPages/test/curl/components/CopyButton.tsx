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
import { Check, Copy, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
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
  failed: {
    id: 'apiControlPlane.pages.test.console.CopyButton.failed',
    defaultMessage: 'Copy failed',
    description:
      'Tooltip and button label shown when the browser refused the clipboard write. A statement, not a command.',
  },
});

/** How long the button stays in its confirmed state after a copy. */
const COPIED_FEEDBACK_MS = 1500;

/** Resting, just-copied, or refused by the browser. */
type CopyStatus = 'idle' | 'copied' | 'failed';

type CopyButtonProps = {
  /** Produces the text to copy, which may differ from the displayed value. */
  getValue: () => string;
  /** Accessible label. Defaults to a generic "Copy". */
  label?: string;
  /** Renders as a labelled button rather than an icon-only one. */
  variant?: 'icon' | 'button';
};

/**
 * Copies a value to the clipboard and reports the result inline.
 *
 * Success clears itself; failure remains visible, which is important when the
 * displayed value is masked and cannot be selected as a fallback.
 */
export function CopyButton({ getValue, label, variant = 'icon' }: CopyButtonProps) {
  const intl = useIntl();
  const [status, setStatus] = useState<CopyStatus>('idle');

  // Held in a ref so a copy landing just before unmount (a view toggle, a tab
  // change) does not leave a timer setting state on a gone component.
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(resetTimer.current), []);

  const accessibleLabel = label ?? intl.formatMessage(messages.copy);
  const failed = status === 'failed';

  const title = {
    copied: intl.formatMessage(messages.copied),
    failed: intl.formatMessage(messages.failed),
    idle: accessibleLabel,
  }[status];

  const icon = {
    copied: <Check size={16} />,
    failed: <TriangleAlert size={16} />,
    idle: <Copy size={16} />,
  }[status];

  const copy = async () => {
    // A retry starts from the resting state, so the previous outcome never
    // outlives the attempt that produced it.
    window.clearTimeout(resetTimer.current);
    try {
      await navigator.clipboard.writeText(getValue());
      setStatus('copied');
      resetTimer.current = window.setTimeout(() => setStatus('idle'), COPIED_FEEDBACK_MS);
    } catch {
      setStatus('failed');
    }
  };

  if (variant === 'button') {
    return (
      <Button
        color={failed ? 'error' : 'primary'}
        onClick={copy}
        size="small"
        startIcon={icon}
        variant="contained"
      >
        {title}
      </Button>
    );
  }

  return (
    <Tooltip title={title}>
      <IconButton
        aria-label={title}
        color={failed ? 'error' : undefined}
        onClick={copy}
        size="small"
      >
        {icon}
      </IconButton>
    </Tooltip>
  );
}
