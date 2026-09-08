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

import { Box, Button, Card, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

const messages = defineMessages({
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.cancel',
    defaultMessage: 'Cancel',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.SaveBar.save',
    defaultMessage: 'Save',
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
});

type SaveBarProps = {
  /** Whether the panel's edited state differs from what was last saved. Gates the button independently of validity. */
  dirty: boolean;
  /** Disables the save button (invalid form or in-flight save). */
  disabled?: boolean;
  /** Renders the in-flight label and is implied disabled. */
  saving?: boolean;
  onCancel: () => void;
  onSave: () => void;
  /** Overrides the default "Save changes"; already-translated text. */
  label?: string;
};

/**
 * Save action bar pinned to the bottom of a develop tab's scroll area
 * (`position: sticky`), offset above the app footer so it is never covered.
 * Rendered as a compact card so it remains distinct from content scrolling
 * underneath without changing appearance based on that content's background.
 */
export function SaveBar({ dirty, disabled, saving, onCancel, onSave, label }: SaveBarProps) {
  const intl = useIntl();
  // Defaulted here rather than in the signature: a default parameter is
  // evaluated before `useIntl` exists, so the fallback could not be translated.
  const saveLabel = label ?? intl.formatMessage(messages.save);
  return (
    <Box sx={{ bottom: 0, position: 'sticky', zIndex: 10 }}>
      <Card>
        <Stack
          alignItems={{ sm: 'center', xs: 'flex-start' }}
          direction={{ sm: 'row', xs: 'column' }}
          justifyContent="space-between"
          spacing={1}
          sx={{ p: 2 }}
        >
          <Typography color={dirty ? 'warning.main' : 'text.secondary'} variant="body2">
            {dirty ? <FormattedMessage {...messages.unsaved} /> : ''}
          </Typography>
          <Stack direction="row" spacing={1}>
            <Button
              color="secondary"
              disabled={disabled || saving || !dirty}
              onClick={onCancel}
              variant="outlined"
            >
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button disabled={disabled || saving || !dirty} onClick={onSave} variant="contained">
              {saving ? intl.formatMessage(messages.saving) : saveLabel}
            </Button>
          </Stack>
        </Stack>
      </Card>
    </Box>
  );
}
