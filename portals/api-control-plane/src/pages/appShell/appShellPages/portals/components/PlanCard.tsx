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

import { alpha, Box, Form, Tooltip, Typography, type Theme } from '@wso2/oxygen-ui';
import { Check } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl, type MessageDescriptor } from 'react-intl';

import { focusRingSx } from '@/theme/receipes';
import type { PlanLimitDisplay } from '../utils/subscriptionPlanLimit';
import { StatusChip, type Status } from './StatusChip';

const messages = defineMessages({
  cleared: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.cleared',
    defaultMessage: 'Will be removed when you save.',
    description: 'Shown on a draft plan card the user cleared that is no longer offered once saved: a deleted or inactive plan.',
  },
  inactiveSelected: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactiveSelected',
    defaultMessage: 'Plan is inactive. Clear it to save or publish.',
  },
  editedPublished: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.editedPublished',
    defaultMessage: 'Edited after last publish.',
    description:
      'Shown in place of the limit on a published plan card whose plan was edited. Any edit counts (name, limit or status), so it does not say which.',
  },
  editedPublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.editedPublishedHint',
    defaultMessage: 'Plan was edited after publishing. Republish to apply changes.',
    description: 'Tooltip on the Edited chip of a published plan card.',
  },
  deletedDraft: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.deletedDraft',
    defaultMessage: 'Deleted after last save.',
    description: 'Shown on a draft plan card whose subscription plan no longer exists.',
  },
  deletedDraftHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.deletedDraftHint',
    defaultMessage: 'This plan was deleted. Clear it to save or publish.',
    description: 'Tooltip on the Deleted chip of a draft plan card.',
  },
  deletedPublished: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.deletedPublished',
    defaultMessage: 'Deleted after last publish.',
    description: 'Shown on a published plan card whose subscription plan no longer exists.',
  },
  inactivePublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactivePublishedHint',
    defaultMessage: 'Plan is inactive. Clear it in draft, then republish.',
    description: 'Tooltip on the Inactive chip of a published plan card.',
  },
  deletedPublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.deletedPublishedHint',
    defaultMessage: 'This plan was deleted. Clear it in draft, then republish.',
    description: 'Tooltip on the Deleted chip of a published plan card.',
  },
  unlimitedValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.unlimitedValue',
    defaultMessage: 'Unlimited',
    description: 'Plan card value when the plan has no configured limit.',
  },
});

type PlanCardProps = {
  /** The user cleared a deleted or inactive plan; it stays on screen, unticked, until the draft is saved. */
  cleared?: boolean;
  deleted?: boolean;
  disabled?: boolean;
  displayName: string;
  /** The plan was changed after the listing was published. Only the published view shows it; the draft always reads the live plan. */
  edited?: boolean;
  inactive?: boolean;
  /** Absent for a deleted plan, whose limit is no longer known. */
  limit?: PlanLimitDisplay;
  onToggle?: () => void;
  pricing?: string;
  readOnly?: boolean;
  selected: boolean;
};

/** The card's own colour: primary in the draft, success in the published view. State is carried by the chips, never the accent. */
type Accent = 'primary' | 'success';

/** Most blocking first. A card shows only the first state that applies: Deleted cannot be fixed, Inactive blocks save and publish, Edited is informational. */
const STATUS_PRIORITY: readonly Status[] = ['deleted', 'inactive', 'edited'];

/** Side of the leading checkbox square. */
const CHECK_SIZE = 20;

/** Room for the limit or a two-line notice, so cards with and without one stay level. */
const BODY_MIN_HEIGHT = 42;

/** The drawn checkbox at the card's left; a real checkbox cannot nest inside a button. */
function SelectionSquare({ accent, selected }: { accent: Accent; selected: boolean }) {
  return (
    <Box
      sx={(theme) => {
        const color = theme.palette[accent].main;
        return {
          alignItems: 'center',
          bgcolor: selected ? color : 'transparent',
          border: 1.5,
          borderColor: selected ? color : 'text.disabled',
          borderRadius: '6px', // explicit: the theme's base radius would make a 20px square nearly round
          display: 'flex',
          flexShrink: 0,
          height: CHECK_SIZE,
          justifyContent: 'center',
          width: CHECK_SIZE,
        };
      }}
    >
      {selected && <Check color="#fff" size={14} strokeWidth={3} />}
    </Box>
  );
}

/**
 * The card surface: selected border and tint in the accent, then layout, then the read-only block.
 * `Form.CardButton` paints its hover border primary, so a selected card restates it in the accent;
 * a `disabled` card keeps the component's own (suppressed) hover.
 */
const planCardSx = (
  theme: Theme,
  {
    accent,
    disabled,
    readOnly,
    selected,
  }: { accent: Accent; disabled?: boolean; readOnly?: boolean; selected: boolean },
) => ({
  borderColor: selected ? `${accent}.main` : 'divider',
  ...(selected && {
    backgroundColor: alpha(theme.palette[accent].main, 0.06),
    boxShadow: theme.shadows[1],
    ...(!disabled && { '&:hover': { borderColor: `${accent}.main` } }),
  }),
  ...focusRingSx(theme),
  display: 'flex',
  flexDirection: 'column',
  gap: 2.5,
  p: 3,
  width: '100%',
  ...(readOnly && { cursor: 'default' }),
});

/**
 * One plan in the grid: a single `Form.CardButton` wearing the `checkbox` role, so the whole card
 * is the hit target. The square at the left is drawn, not a real checkbox, which cannot nest in a button.
 *
 * The accent never changes with state. One chip beside the name carries it, the highest in
 * `STATUS_PRIORITY`: Deleted in error, Inactive and Edited in warning. The notice and the tooltip
 * follow that same state. The body is the plan's limit, or a notice in its place when the limit
 * cannot be trusted: the plan was deleted, or edited since the listing was published.
 *
 * An inactive plan only appears while the saved selection holds it, because the user has to clear it;
 * a cleared one stays, unticked with a notice, until the draft is saved.
 *
 * `readOnly` (the published view) never sets `disabled`, which would grey the card out; it just has no
 * click handler, so hovering still shows the tooltip. A deleted plan in the draft stays clickable so
 * the user can clear it.
 *
 * `pricing` is the price of a paid plan in words; a free plan shows none.
 */
export function PlanCard({
  cleared = false,
  deleted = false,
  disabled,
  displayName,
  edited = false,
  inactive = false,
  limit,
  onToggle,
  pricing,
  readOnly,
  selected,
}: PlanCardProps) {
  const intl = useIntl();
  const accent: Accent = readOnly ? 'success' : 'primary';

  const flags: Record<Status, boolean> = { deleted, edited: edited && Boolean(readOnly), inactive };
  const status = STATUS_PRIORITY.find((state) => flags[state]);

  let notice: MessageDescriptor | undefined;
  if (cleared && !readOnly) notice = messages.cleared;
  else if (status === 'deleted') notice = readOnly ? messages.deletedPublished : messages.deletedDraft;
  else if (status === 'edited') notice = messages.editedPublished;

  // The notice is the short form; the tooltip says it in full.
  let hint: MessageDescriptor | undefined;
  if (status === 'inactive' && readOnly) hint = messages.inactivePublishedHint;
  else if (status === 'inactive' && !cleared) hint = messages.inactiveSelected;
  else if (status === 'edited') hint = messages.editedPublishedHint;
  else if (status === 'deleted' && readOnly) hint = messages.deletedPublishedHint;
  else if (status === 'deleted' && !cleared) hint = messages.deletedDraftHint;
  const tooltip = hint ? intl.formatMessage(hint) : '';

  return (
    // describeChild keeps the plan's name as the card's accessible name.
    <Tooltip describeChild title={tooltip}>
      <Form.CardButton
        alignItems="flex-start"
        aria-checked={selected}
        aria-disabled={readOnly}
        disabled={!readOnly && disabled}
        onClick={readOnly ? undefined : onToggle}
        role="checkbox"
        selected={selected}
        sx={(theme) => planCardSx(theme, { accent, disabled: !readOnly && disabled, readOnly, selected })}
        tabIndex={readOnly ? -1 : 0}
      >
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5, width: '100%' }}>
          <Box sx={{ alignItems: 'center', display: 'flex', flex: 1, gap: 1.5, minWidth: 0 }}>
            <SelectionSquare accent={accent} selected={selected} />
            <Typography noWrap sx={{ fontSize: '1rem', fontWeight: 600, minWidth: 0 }} variant="subtitle1">
              {displayName}
            </Typography>
          </Box>
          {status && <StatusChip status={status} />}
        </Box>

        <Box sx={{ minHeight: BODY_MIN_HEIGHT, textAlign: 'start', width: '100%' }}>
          {notice && (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...notice} />
            </Typography>
          )}
          {!notice && limit && (
            <Box sx={{ alignItems: 'baseline', columnGap: 1, display: 'flex' }}>
              <Typography
                sx={{ flexShrink: 0, fontSize: '1.75rem', fontVariantNumeric: 'tabular-nums', fontWeight: 600, lineHeight: 1.15 }}
                variant="h4"
              >
                {limit.unlimited ? (
                  <FormattedMessage {...messages.unlimitedValue} />
                ) : (
                  <FormattedNumber value={limit.count} />
                )}
              </Typography>
              <Typography color="text.secondary" noWrap sx={{ minWidth: 0 }} variant="body2">
                <FormattedMessage {...limit.unit} values={limit.unlimited ? undefined : limit.unitValues} />
              </Typography>
            </Box>
          )}
        </Box>
        {pricing && <Typography variant="body2">{pricing}</Typography>}
      </Form.CardButton>
    </Tooltip>
  );
}
