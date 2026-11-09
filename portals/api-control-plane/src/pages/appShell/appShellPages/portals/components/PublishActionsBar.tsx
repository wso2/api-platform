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
import {
  Box,
  Button,
  ButtonGroup,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
} from '@wso2/oxygen-ui';
import {
  ChevronDown,
  CircleX,
  Megaphone,
  RefreshCw,
  TriangleAlert,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

const messages = defineMessages({
  saveDraft: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.saveDraft',
    defaultMessage: 'Save Draft',
  },
  publish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.publish',
    defaultMessage: 'Publish',
  },
  republish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.republish',
    defaultMessage: 'Republish',
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
  publishing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.publishing',
    defaultMessage: 'Publishing\u2026',
    description: 'Label of the Publish button while the publish is in progress.',
  },
  republishing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.republishing',
    defaultMessage: 'Republishing\u2026',
    description: 'Label of the Republish button while the republish is in progress.',
  },
  unpublishing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.unpublishing',
    defaultMessage: 'Unpublishing\u2026',
    description: 'Label of the Unpublish button while the unpublish is in progress.',
  },
  deprecating: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishActionsBar.deprecating',
    defaultMessage: 'Deprecating\u2026',
    description: 'Label of the Deprecate button while the deprecation is in progress.',
  },
});

export type PublishActionsBarProps = {
  /** Whether this API is currently live on this portal (published or deprecated) — what enables Unpublish. */
  isPublished: boolean;
  /** Whether the live listing is in the published state — what enables Deprecate. */
  canDeprecate: boolean;
  /** Whether there are edits to save; Save Draft is off until there are. */
  canSaveDraft: boolean;
  deprecating: boolean;
  onDeprecate: () => void;
  onPublish: () => void;
  /** Holds Publish back (not Save Draft), e.g. while the documents it should carry are still loading. */
  publishDisabled?: boolean;
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

/** Every label the main button can show, idle or running. */
const ACTION_LABELS = [
  messages.publish,
  messages.republish,
  messages.deprecate,
  messages.unpublish,
  messages.publishing,
  messages.republishing,
  messages.deprecating,
  messages.unpublishing,
];

/** What a running action's button says. */
const RUNNING_LABEL = {
  deprecate: messages.deprecating,
  publish: messages.publishing,
  republish: messages.republishing,
  unpublish: messages.unpublishing,
} as const;

/** The label shown for an action, and kept as wide as the widest of `labels`, so the button never resizes. */
function ActionLabel({
  current,
  labels,
}: {
  current: MessageDescriptor;
  labels: MessageDescriptor[];
}) {
  return (
    <Box component="span" sx={{ display: 'inline-grid', textAlign: 'center' }}>
      {labels.map((label) => (
        <Box
          component="span"
          key={label.id}
          sx={{ gridArea: '1 / 1', visibility: label === current ? 'visible' : 'hidden' }}
        >
          <FormattedMessage {...label} />
        </Box>
      ))}
    </Box>
  );
}

const ACTION_COLOR = { publish: 'primary', unpublish: 'error', deprecate: 'warning' } as const;

/**
 * The page's action row: Save Draft, and a split button whose primary side is
 * Publish, with Deprecate and Unpublish as its alternatives. Publish is always
 * available; Unpublish needs a live listing (published or deprecated) and
 * Deprecate a published one, so each is disabled rather than hidden otherwise.
 *
 * Field validation isn't gated here: every action stays clickable and reveals
 * any problem on submit.
 */
export function PublishActionsBar({
  canDeprecate,
  canSaveDraft,
  deprecating,
  isPublished,
  onDeprecate,
  onPublish,
  publishDisabled = false,
  onSaveDraft,
  onUnpublish,
  publishing,
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
  const running: Record<PrimaryAction, boolean> = {
    publish: publishing,
    unpublish: unpublishing,
    deprecate: deprecating,
  };

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
  const alternatives = (['publish', 'deprecate', 'unpublish'] as const).filter(
    (action) => action !== effectiveAction,
  );
  // Publish reads as "Republish" once the listing is already published — canDeprecate
  // is only true in that exact state — so a re-push isn't mistaken for the first one.
  const isRepublish = (action: PrimaryAction) => action === 'publish' && canDeprecate;
  const actionLabel = (action: PrimaryAction) =>
    isRepublish(action) ? messages.republish : messages[action];
  const runningLabel = (action: PrimaryAction) =>
    RUNNING_LABEL[isRepublish(action) ? 'republish' : action];
  const actionIcon = (action: PrimaryAction) =>
    ACTION_ICON[isRepublish(action) ? 'republish' : action];

  return (
    <Stack direction="row" spacing={2} sx={{ alignItems: 'stretch', justifyContent: 'flex-end' }}>
      <Button disabled={busy || !canSaveDraft} onClick={onSaveDraft} variant="outlined">
        <FormattedMessage {...messages.saveDraft} />
      </Button>

      <ButtonGroup color={ACTION_COLOR[effectiveAction]} disabled={busy} variant="contained">
        <Button
          disabled={effectiveAction === 'publish' && publishDisabled}
          loading={running[effectiveAction]}
          loadingPosition="start"
          onClick={run[effectiveAction]}
          startIcon={actionIcon(effectiveAction)}
        >
          <ActionLabel
            current={
              running[effectiveAction]
                ? runningLabel(effectiveAction)
                : actionLabel(effectiveAction)
            }
            labels={ACTION_LABELS}
          />
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
            <ListItemIcon>{actionIcon(action)}</ListItemIcon>
            <ListItemText>
              <FormattedMessage {...actionLabel(action)} />
            </ListItemText>
          </MenuItem>
        ))}
      </Menu>
    </Stack>
  );
}

/** Every label the published version's main button can show, idle or running. */
const PUBLISHED_ACTION_LABELS = [
  messages.unpublish,
  messages.deprecate,
  messages.unpublishing,
  messages.deprecating,
];

export type PublishedActionsBarProps = {
  /** Whether the live listing is in the published state — what enables Deprecate. */
  canDeprecate: boolean;
  deprecating: boolean;
  onDeprecate: () => void;
  onUnpublish: () => void;
  unpublishing: boolean;
};

/**
 * The action row under the Published version: a split button for the two things
 * that act on the live listing. Unpublish is the main action; Deprecate sits
 * behind the arrow and, like the draft's split button, is armed by picking it
 * rather than fired. There is nothing to save or publish here, since this
 * version can't be edited. Deprecate is disabled once the listing is already
 * deprecated.
 */
export function PublishedActionsBar({
  canDeprecate,
  deprecating,
  onDeprecate,
  onUnpublish,
  unpublishing,
}: PublishedActionsBarProps) {
  const intl = useIntl();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [armed, setArmed] = useState<'unpublish' | 'deprecate'>('unpublish');
  const busy = deprecating || unpublishing;
  // Deprecate is unavailable once the listing is deprecated, so fall back to Unpublish.
  const action = armed === 'deprecate' && canDeprecate ? 'deprecate' : 'unpublish';
  const other = action === 'unpublish' ? 'deprecate' : 'unpublish';
  const running = action === 'unpublish' ? unpublishing : deprecating;

  return (
    <Stack direction="row" sx={{ alignItems: 'stretch', justifyContent: 'flex-end' }}>
      <ButtonGroup color={ACTION_COLOR[action]} disabled={busy} variant="contained">
        <Button
          loading={running}
          loadingPosition="start"
          onClick={action === 'unpublish' ? onUnpublish : onDeprecate}
          startIcon={ACTION_ICON[action]}
        >
          <ActionLabel
            current={running ? RUNNING_LABEL[action] : messages[action]}
            labels={PUBLISHED_ACTION_LABELS}
          />
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
        <MenuItem
          disabled={busy || (other === 'deprecate' && !canDeprecate)}
          onClick={() => {
            setMenuAnchor(null);
            setArmed(other);
          }}
        >
          <ListItemIcon>{ACTION_ICON[other]}</ListItemIcon>
          <ListItemText>
            <FormattedMessage {...messages[other]} />
          </ListItemText>
        </MenuItem>
      </Menu>
    </Stack>
  );
}
