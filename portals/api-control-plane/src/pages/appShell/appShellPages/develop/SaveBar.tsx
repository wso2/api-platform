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

import { Box, Button, Card, CardContent, Typography } from '@wso2/oxygen-ui';
import { Circle } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { APP_FOOTER_ID } from '../../appLayoutConstants';

const messages = defineMessages({
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.cancel',
    defaultMessage: 'Discard',
    description: 'Throws away the unsaved edits and restores the last saved state. Verb.',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.save',
    defaultMessage: 'Save changes',
    description: 'Default label on the save bar button. Verb phrase.',
  },
  saving: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.saving',
    defaultMessage: 'Saving\u2026',
    description: 'Label on the save button while the request is in flight.',
  },
  unsaved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.unsaved',
    defaultMessage: 'You have unsaved changes',
    description: 'Shown on the save bar while the panel has edits that have not been saved yet.',
  },
  unsavedCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.unsavedCount',
    defaultMessage: '{count, plural, one {# unsaved change} other {# unsaved changes}}',
    description: 'Shown on the save bar. {count} is how many edits have not been saved yet.',
  },
});

type SaveBarProps = {
  /** Whether the panel's edited state differs from what was last saved. Gates the button independently of validity. */
  dirty: boolean;
  /** Disables the save button (invalid form or in-flight save). */
  disabled?: boolean;
  /** Renders the in-flight label and is implied disabled. */
  saving?: boolean;
  /** How many edits are unsaved; without it the bar only says there are some. */
  changeCount?: number;
  onCancel: () => void;
  onSave: () => void;
  /** Overrides the default "Save changes"; already-translated text. */
  label?: string;
};

/** Gap between the bar and whatever sits beneath it (the app footer, or the viewport edge). */
const BAR_GAP = 3;

/** Room kept at the end of the page so the floating bar never covers its last row. */
const SPACER_HEIGHT = 96;

/**
 * Height of the app footer, so the bar can float above it instead of over it. The
 * footer is measured because it changes with the viewport width.
 */
function useFooterHeight(active: boolean): number {
  const [height, setHeight] = useState(0);
  useEffect(() => {
    if (!active) return undefined;
    const footer = document.getElementById(APP_FOOTER_ID);
    if (!footer) return undefined;
    const measure = () => setHeight(footer.offsetHeight);
    measure();
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [active]);
  return height;
}

/**
 * Floating action bar for a develop tab: it appears, pinned to the bottom of the
 * window and centred, only while there are unsaved edits, and says how many. It is
 * fixed to the viewport, so it stays put as the page scrolls beneath it; a spacer
 * at the end of the page keeps the last row reachable.
 */
export function SaveBar({
  dirty,
  disabled,
  saving,
  changeCount,
  onCancel,
  onSave,
  label,
}: SaveBarProps) {
  const intl = useIntl();
  const visible = dirty || Boolean(saving);
  const footerHeight = useFooterHeight(visible);
  if (!visible) return null;

  // Defaulted here rather than in the signature: a default parameter is
  // evaluated before `useIntl` exists, so the fallback could not be translated.
  const saveLabel = label ?? intl.formatMessage(messages.save);
  return (
    <>
      <Box aria-hidden sx={{ height: SPACER_HEIGHT }} />
      <Card
        role="region"
        sx={(theme) => ({
          bottom: `calc(${footerHeight}px + ${theme.spacing(BAR_GAP)})`,
          left: '50%',
          maxWidth: `calc(100vw - ${theme.spacing(4)})`,
          position: 'fixed',
          transform: 'translateX(-50%)',
          zIndex: theme.zIndex.appBar - 1,
        })}
      >
        <CardContent
          sx={{
            alignItems: 'center',
            display: 'flex',
            flexWrap: 'wrap',
            gap: 2,
            p: 1.5,
            '&:last-child': { pb: 1.5 },
          }}
        >
          <Box sx={{ alignItems: 'center', color: 'primary.main', display: 'flex', gap: 1 }}>
            <Circle fill="currentColor" size={8} />
            <Typography color="text.primary" variant="body2">
              {!dirty ? (
                // Saving, with the edits already reverted: there is nothing left to count.
                <FormattedMessage {...messages.saving} />
              ) : changeCount === undefined ? (
                <FormattedMessage {...messages.unsaved} />
              ) : (
                <FormattedMessage {...messages.unsavedCount} values={{ count: changeCount }} />
              )}
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button color="secondary" disabled={saving} onClick={onCancel} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button disabled={disabled || saving} onClick={onSave} variant="contained">
              {saving ? intl.formatMessage(messages.saving) : saveLabel}
            </Button>
          </Box>
        </CardContent>
      </Card>
    </>
  );
}
