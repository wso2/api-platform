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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Box, Button, ButtonGroup, ListItemIcon, ListItemText, Menu, MenuItem, Stack } from '@wso2/oxygen-ui';
import { ChevronDown, CircleX, Megaphone, RefreshCw, TriangleAlert } from '@wso2/oxygen-ui-icons-react';

const messages = defineMessages({
  more: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.actions.more',
    defaultMessage: 'More publish actions',
  },
  saveDraft: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.actions.saveDraft',
    defaultMessage: 'Save Draft',
  },
});

export type PublishActionsBarProps = {
  /** Whether this proxy is currently live on this hub (published or deprecated) — what enables Unpublish. */
  isPublished: boolean;
  /** Whether the live listing is in the published state — what enables Deprecate. */
  canDeprecate: boolean;
  /** Whether there are edits to save; Save Draft is off until there are. */
  canSaveDraft: boolean;
  /** Save Draft and Publish are off: saving now would lose a part of the listing the page could not read. */
  saveBlocked?: boolean;
  /** What the signed-in user may do; an action they lack is disabled like one that does not apply. */
  permissions: { publish: boolean; unpublish: boolean; deprecate: boolean };
  deprecating: boolean;
  onDeprecate: () => void;
  onPublish: () => void;
  onSaveDraft: () => void;
  onUnpublish: () => void;
  publishing: boolean;
  savingDraft: boolean;
  unpublishing: boolean;
};

type PrimaryAction = 'publish' | 'unpublish' | 'deprecate';

const ICON_SIZE = 16;

const ACTION_ICON = {
  deprecate: <TriangleAlert size={ICON_SIZE} />,
  publish: <Megaphone size={ICON_SIZE} />,
  republish: <RefreshCw size={ICON_SIZE} />,
  unpublish: <CircleX size={ICON_SIZE} />,
} as const;

const LABEL = defineMessages({
  publish: { id: 'aiWorkspace.pages.appShell.appShellPages.publish.actions.publish', defaultMessage: 'Publish' },
  republish: { id: 'aiWorkspace.pages.appShell.appShellPages.publish.actions.republish', defaultMessage: 'Republish' },
  unpublish: { id: 'aiWorkspace.pages.appShell.appShellPages.publish.actions.unpublish', defaultMessage: 'Unpublish' },
  deprecate: { id: 'aiWorkspace.pages.appShell.appShellPages.publish.actions.deprecate', defaultMessage: 'Deprecate' },
});

const ACTION_LABELS = [LABEL.publish, LABEL.republish, LABEL.deprecate, LABEL.unpublish];

const ACTION_COLOR = { publish: 'primary', unpublish: 'error', deprecate: 'warning' } as const;

/**
 * Save Draft and a split button: Publish, with Deprecate and Unpublish as alternatives. An action
 * that doesn't apply is disabled, not hidden; form errors are reported on submit, not here.
 */
export function PublishActionsBar({
  canDeprecate,
  canSaveDraft,
  deprecating,
  isPublished,
  onDeprecate,
  onPublish,
  onSaveDraft,
  onUnpublish,
  permissions,
  publishing,
  saveBlocked = false,
  savingDraft,
  unpublishing,
}: PublishActionsBarProps) {
  const intl = useIntl();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  // The action the primary button performs. Picking one from the menu arms it
  // rather than firing it; it resets when the live state changes, so cancelling
  // a confirm dialog keeps it armed.
  const liveState = `${isPublished}:${canDeprecate}`;
  const [seenLiveState, setSeenLiveState] = useState(liveState);
  const [armedAction, setArmedAction] = useState<PrimaryAction>('publish');
  if (seenLiveState !== liveState) {
    setSeenLiveState(liveState);
    setArmedAction('publish');
  }

  const busy = savingDraft || publishing || unpublishing || deprecating;
  const available: Record<PrimaryAction, boolean> = {
    publish: permissions.publish && !saveBlocked,
    unpublish: isPublished && permissions.unpublish,
    deprecate: canDeprecate && permissions.deprecate,
  };
  const run: Record<PrimaryAction, () => void> = {
    publish: onPublish,
    unpublish: onUnpublish,
    deprecate: onDeprecate,
  };

  const effectiveAction: PrimaryAction = armedAction === 'publish' || available[armedAction] ? armedAction : 'publish';
  const alternatives = (['publish', 'deprecate', 'unpublish'] as const).filter((action) => action !== effectiveAction);

  // "Republish" once the listing is published, which is exactly when canDeprecate is true.
  const isRepublish = (action: PrimaryAction) => action === 'publish' && canDeprecate;
  const actionLabel = (action: PrimaryAction) => intl.formatMessage(isRepublish(action) ? LABEL.republish : LABEL[action]);
  const actionIcon = (action: PrimaryAction) => ACTION_ICON[isRepublish(action) ? 'republish' : action];

  return (
    <Stack direction="row" spacing={2} sx={{ alignItems: 'stretch', justifyContent: 'flex-end' }}>
      <Button disabled={busy || !canSaveDraft || saveBlocked} onClick={onSaveDraft} variant="outlined">
        <FormattedMessage {...messages.saveDraft} />
      </Button>
      <ButtonGroup color={ACTION_COLOR[effectiveAction]} variant="contained">
        <Button
          disabled={busy || !available[effectiveAction]}
          onClick={run[effectiveAction]}
          startIcon={actionIcon(effectiveAction)}
        >
          {/* Every label shares one cell and only the current one shows, so the button is always as wide as the longest. */}
          <Box component="span" sx={{ display: 'inline-grid', textAlign: 'center' }}>
            {ACTION_LABELS.map((label) => (
              <Box
                aria-hidden={intl.formatMessage(label) !== actionLabel(effectiveAction)}
                component="span"
                key={label.id}
                sx={{
                  gridArea: '1 / 1',
                  visibility: intl.formatMessage(label) === actionLabel(effectiveAction) ? 'visible' : 'hidden',
                }}
              >
                <FormattedMessage {...label} />
              </Box>
            ))}
          </Box>
        </Button>
        <Button
          aria-label={intl.formatMessage(messages.more)}
          disabled={busy}
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
            <ListItemIcon>{actionIcon(action)}</ListItemIcon>
            <ListItemText>{actionLabel(action)}</ListItemText>
          </MenuItem>
        ))}
      </Menu>
    </Stack>
  );
}
