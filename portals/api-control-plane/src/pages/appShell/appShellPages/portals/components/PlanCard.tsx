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

import { alpha, Box, Chip, Form, Tooltip, Typography, type Theme } from '@wso2/oxygen-ui';
import { Check } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl, type MessageDescriptor } from 'react-intl';

import { focusRingSx } from '@/theme/receipes';
import type { PlanLimitDisplay } from '../utils/subscriptionPlanLimit';

const messages = defineMessages({
  inactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactive',
    defaultMessage: 'Inactive',
    description: 'Chip after the name of a published plan whose subscription plan is inactive.',
  },
  edited: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.edited',
    defaultMessage: 'Edited',
    description: 'Chip after the name of a published plan that was changed after it was last published.',
  },
  editedPublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.editedPublishedHint',
    defaultMessage: 'Plan was edited after publishing. Republish to apply changes.',
    description: 'Tooltip on a published plan card whose plan was edited.',
  },
  inactivePublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactivePublishedHint',
    defaultMessage: 'Plan is inactive. Republish to remove it from the listing.',
    description: 'Tooltip on a published plan card whose plan is inactive.',
  },
  deletedPublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.deletedPublishedHint',
    defaultMessage: 'This plan was deleted. Republish to remove it from the listing.',
    description: 'Tooltip on a published plan card whose plan no longer exists.',
  },
  editedSincePublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.editedSincePublish',
    defaultMessage: 'Edited since last publish',
    description: 'Helper text on a published plan card whose plan was changed after it was last published.',
  },
  deletedSincePublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.deletedSincePublish',
    defaultMessage: 'Deleted since last publish',
    description: 'Helper text on a published plan card whose plan was deleted after it was last published.',
  },
  unlimitedValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.unlimitedValue',
    defaultMessage: 'Unlimited',
    description: 'Plan card value when the plan has no configured limit.',
  },
});

type PlanCardProps = {
  /** Published view only: the plan no longer exists, so its name is struck through. */
  deleted?: boolean;
  disabled?: boolean;
  displayName: string;
  /** The plan was changed after the listing was published. Only the published view shows it; the draft always reads the live plan. */
  edited?: boolean;
  /** Published view only: the draft never offers an inactive plan. */
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

/** The states of a published card that get a chip. Deleted has none: its name is struck through. */
type Chipped = 'inactive' | 'edited';

/** Side of the leading checkbox square. */
const CHECK_SIZE = 20;

/** Room for the limit, so cards with and without one stay level. */
const BODY_MIN_HEIGHT = 42;

/** The drawn checkbox at the card's left; a real checkbox cannot nest inside a button. A plan no longer available is drawn grey. */
function SelectionSquare({ accent, selected, unavailable }: { accent: Accent; selected: boolean; unavailable: boolean }) {
  return (
    <Box
      sx={(theme) => {
        const color = unavailable ? theme.palette.text.disabled : theme.palette[accent].main;
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
 * a `disabled` card keeps the component's own (suppressed) hover. An unavailable card is plain grey:
 * no tint or shadow, and its hover border stays the divider's.
 */
const planCardSx = (
  theme: Theme,
  {
    accent,
    disabled,
    readOnly,
    selected,
    unavailable,
  }: { accent: Accent; disabled?: boolean; readOnly?: boolean; selected: boolean; unavailable: boolean },
) => ({
  borderColor: selected ? `${accent}.main` : 'divider',
  ...(unavailable && { '&:hover': { borderColor: 'divider' } }),
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
 * One plan in the grid: a single `Form.CardButton` with the `checkbox` role, so the whole card is the hit target
 * (the square is drawn; a real checkbox cannot nest in a button).
 * The published card (`readOnly`) shows what became of the plan since publish: a deleted or inactive plan goes grey
 * and keeps its tick; deleted is struck through; inactive or edited gets an outlined chip (inactive wins). An edited
 * plan hides its limit, which would be the current one, not the published one.
 * `readOnly` never sets `disabled`, so hovering still shows the tooltip.
 */
export function PlanCard({
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
  // A deleted or inactive plan is still held by the listing, but is no longer available.
  const unavailable = Boolean(readOnly) && (deleted || inactive);

  let chip: Chipped | undefined;
  if (readOnly && !deleted && inactive) chip = 'inactive';
  else if (readOnly && !deleted && edited) chip = 'edited';

  let hint: MessageDescriptor | undefined;
  if (readOnly && deleted) hint = messages.deletedPublishedHint;
  else if (chip === 'inactive') hint = messages.inactivePublishedHint;
  else if (chip === 'edited') hint = messages.editedPublishedHint;
  const tooltip = hint ? intl.formatMessage(hint) : '';

  // Where the limit would sit: an edited or deleted plan has none to show, even when the chip says Inactive.
  const editedSincePublish = Boolean(readOnly) && !deleted && edited;
  let helper: MessageDescriptor | undefined;
  if (readOnly && deleted) helper = messages.deletedSincePublish;
  else if (editedSincePublish) helper = messages.editedSincePublish;

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
        selected={selected && !unavailable}
        sx={(theme) =>
          planCardSx(theme, { accent, disabled: !readOnly && disabled, readOnly, selected: selected && !unavailable, unavailable })
        }
        tabIndex={readOnly ? -1 : 0}
      >
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5, minWidth: 0, width: '100%' }}>
          <SelectionSquare accent={accent} selected={selected} unavailable={unavailable} />
          <Typography
            component={readOnly && deleted ? 'del' : 'span'}
            noWrap
            sx={{ color: unavailable ? 'text.secondary' : undefined, fontSize: '1rem', fontWeight: 600, minWidth: 0 }}
            variant="subtitle1"
          >
            {displayName}
          </Typography>
          {chip && (
            <Chip
              color={chip === 'inactive' ? 'default' : 'warning'}
              label={<FormattedMessage {...messages[chip]} />}
              size="small"
              sx={{ flexShrink: 0 }}
              variant="outlined"
            />
          )}
        </Box>

        <Box sx={{ minHeight: BODY_MIN_HEIGHT, textAlign: 'start', width: '100%' }}>
          {helper && (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...helper} />
            </Typography>
          )}
          {limit && !editedSincePublish && (
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
