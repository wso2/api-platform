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
import { Button, ButtonGroup, Menu, MenuItem, Stack } from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';


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
  deprecate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.deprecate',
    defaultMessage: 'Deprecate',
  },
  moreActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.moreActions',
    defaultMessage: 'More publish actions',
  },
});

export type PublishActionsBarProps = {
  /** Whether this API is currently live on this portal (published or deprecated) — what enables Unpublish. */
  isPublished: boolean;
  /** Whether the live listing is in the published state — what enables Deprecate. */
  canDeprecate: boolean;
  deprecating: boolean;
  onDeprecate: () => void;
  onPublish: () => void;
  onSaveDraft: () => void;
  onUnpublish: () => void;
  publishing: boolean;
  savingDraft: boolean;
  unpublishing: boolean;
};

/**
 * The page's action row, right-aligned under the form like the edit-API form's
 * buttons: Save Draft, and a split button whose primary side
 * is Publish, with Unpublish and Deprecate as its alternatives. Publish is
 * always available — publishing (republishing) is valid from every state a
 * draft can exist in. Unpublish needs a live
 * listing (published or deprecated) and Deprecate needs a published one, so
 * each is disabled rather than hidden the rest of the time.
 *
 * Field validation isn't gated here: every action stays clickable and reveals
 * any problem on submit, the same way `GeneralCreateApiForm`'s Create button
 * does — a proactively-disabled button would never get the chance to.
 */
type PrimaryAction = 'publish' | 'unpublish' | 'deprecate';

const ACTION_COLOR = { publish: 'primary', unpublish: 'error', deprecate: 'warning' } as const;

export function PublishActionsBar({
  canDeprecate,
  deprecating,
  isPublished,
  onDeprecate,
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
  // action (and its own color) rather than firing it immediately. The choice is
  // cleared whenever the server-reported live state changes (unpublished,
  // deprecated, republished), never by a click alone — cancelling a confirm
  // dialog keeps it armed.
  const liveState = `${isPublished}:${canDeprecate}`;
  const [seenLiveState, setSeenLiveState] = useState(liveState);
  const [armedAction, setArmedAction] = useState<PrimaryAction>('publish');
  if (seenLiveState !== liveState) {
    setSeenLiveState(liveState);
    setArmedAction('publish');
  }
  const busy = savingDraft || publishing || unpublishing || deprecating;

  const available: Record<PrimaryAction, boolean> = {
    publish: true,
    unpublish: isPublished,
    deprecate: canDeprecate,
  };
  const run: Record<PrimaryAction, () => void> = {
    publish: onPublish,
    unpublish: onUnpublish,
    deprecate: onDeprecate,
  };
  const effectiveAction: PrimaryAction = available[armedAction] ? armedAction : 'publish';
  const alternatives = (['publish', 'unpublish', 'deprecate'] as const).filter(
    (action) => action !== effectiveAction,
  );

  return (
    <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'flex-end' }}>
      <Button disabled={busy} onClick={onSaveDraft} variant="outlined">
        <FormattedMessage {...messages.saveDraft} />
      </Button>

      <ButtonGroup color={ACTION_COLOR[effectiveAction]} disabled={busy} variant="contained">
        <Button onClick={run[effectiveAction]}>
          <FormattedMessage {...messages[effectiveAction]} />
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
        {alternatives.map((action) => (
          <MenuItem
            disabled={!available[action] || busy}
            key={action}
            onClick={() => {
              setMenuAnchor(null);
              setArmedAction(action);
            }}
          >
            <FormattedMessage {...messages[action]} />
          </MenuItem>
        ))}
      </Menu>
    </Stack>
  );
}
