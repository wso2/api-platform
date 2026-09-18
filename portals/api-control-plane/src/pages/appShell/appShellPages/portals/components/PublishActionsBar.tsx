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
import { Box, Button, ButtonGroup, Menu, MenuItem } from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { stickyBottomBarSx } from '@/theme/receipes';

const messages = defineMessages({
  saveDraft: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.saveDraft',
    defaultMessage: 'Save Draft',
  },
  publish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.publish',
    defaultMessage: 'Publish',
  },
  unpublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.unpublish',
    defaultMessage: 'Unpublish',
  },
  moreActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.moreActions',
    defaultMessage: 'More publish actions',
  },
});

export type PublishActionsBarProps = {
  /** Whether this API is currently live on this portal — what enables Unpublish. */
  isPublished: boolean;
  onPublish: () => void;
  onSaveDraft: () => void;
  onUnpublish: () => void;
  publishing: boolean;
  savingDraft: boolean;
  unpublishing: boolean;
};

/**
 * The page's sticky footer: Save Draft, and a Publish action with Unpublish as
 * its one alternative — "the publish button will only have publish and
 * unpublish actions" (no Deprecate; that release action isn't built yet, see
 * `Implementation_Plan.md` Slice 8). Publish is always the primary action —
 * REST_Design.md's lifecycle table allows publishing (republishing) from every
 * state a draft can exist in; Unpublish is only valid once actually live, so
 * it's disabled rather than hidden the rest of the time.
 *
 * Field validation isn't gated here: both actions stay clickable and reveal
 * any problem on submit, the same way `GeneralCreateApiForm`'s Create button
 * does — a proactively-disabled button would never get the chance to.
 */
type PrimaryAction = 'publish' | 'unpublish';

export function PublishActionsBar({
  isPublished,
  onPublish,
  onSaveDraft,
  onUnpublish,
  publishing,
  savingDraft,
  unpublishing,
}: PublishActionsBarProps) {
  const intl = useIntl();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  // Which action the split button's primary side currently performs — chosen
  // from the dropdown, mirroring the "Merge pull request"-style split button:
  // picking an alternative from the menu re-arms the primary button with that
  // action (and its own color) rather than firing it immediately. Falls back
  // to 'publish' if Unpublish was armed and the API is no longer live.
  const [primaryAction, setPrimaryAction] = useState<PrimaryAction>('publish');
  const busy = savingDraft || publishing || unpublishing;
  const effectiveAction: PrimaryAction = primaryAction === 'unpublish' && isPublished ? 'unpublish' : 'publish';

  return (
    <Box
      sx={(theme) => ({
        ...stickyBottomBarSx(theme),
        bottom: 0,
        display: 'flex',
        gap: 2,
        justifyContent: 'flex-end',
        mx: -3,
        px: 3,
        py: 2,
      })}
    >
      <Button disabled={busy} onClick={onSaveDraft} variant="outlined">
        <FormattedMessage {...messages.saveDraft} />
      </Button>

      <ButtonGroup color={effectiveAction === 'unpublish' ? 'error' : 'primary'} disabled={busy} variant="contained">
        <Button onClick={effectiveAction === 'unpublish' ? onUnpublish : onPublish}>
          <FormattedMessage {...(effectiveAction === 'unpublish' ? messages.unpublish : messages.publish)} />
        </Button>
        <Button
          aria-label={intl.formatMessage(messages.moreActions)}
          onClick={(event) => setMenuAnchor(event.currentTarget)}
          sx={{ px: 0.5 }}
        >
          <ChevronDown size={16} />
        </Button>
      </ButtonGroup>

      <Menu
        anchorEl={menuAnchor}
        anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
        onClose={() => setMenuAnchor(null)}
        open={Boolean(menuAnchor)}
        transformOrigin={{ horizontal: 'right', vertical: 'top' }}
      >

        {effectiveAction === 'unpublish' ? (
          <MenuItem
            onClick={() => {
              setMenuAnchor(null);
              setPrimaryAction('publish');
            }}
          >
            <FormattedMessage {...messages.publish} />
          </MenuItem>
        ) : (
          <MenuItem
            disabled={!isPublished || busy}
            onClick={() => {
              setMenuAnchor(null);
              setPrimaryAction('unpublish');
            }}
          >
            <FormattedMessage {...messages.unpublish} />
          </MenuItem>
        )}
      </Menu>
    </Box>
  );
}
