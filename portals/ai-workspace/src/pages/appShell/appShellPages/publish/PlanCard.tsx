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

import { defineMessages, FormattedMessage, FormattedNumber, useIntl, type MessageDescriptor } from 'react-intl';
import { alpha, Box, Chip, Form, Tooltip, Typography, type Theme } from '@wso2/oxygen-ui';
import { Check } from '@wso2/oxygen-ui-icons-react';

import type { PlanLimitDisplay } from './subscriptionPlanLimit';

const messages = defineMessages({
  inactive: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.inactive',
    defaultMessage: 'Inactive',
  },
  edited: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.edited',
    defaultMessage: 'Edited',
  },
  editedPublishedHint: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.editedPublishedHint',
    defaultMessage: 'Plan was edited after publishing. Republish to apply changes.',
  },
  inactivePublishedHint: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.inactivePublishedHint',
    defaultMessage: 'Plan is inactive. Republish to remove it from the listing.',
  },
  deletedPublishedHint: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.deletedPublishedHint',
    defaultMessage: 'This plan was deleted. Republish to remove it from the listing.',
  },
  editedSincePublish: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.editedSincePublish',
    defaultMessage: 'Edited since last publish',
  },
  deletedSincePublish: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.deletedSincePublish',
    defaultMessage: 'Deleted since last publish',
  },
  unlimitedValue: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planCard.unlimitedValue',
    defaultMessage: 'Unlimited',
  },
});

type PlanCardProps = {
  /** Published view only: the plan no longer exists, so its name is struck through. */
  deleted?: boolean;
  disabled?: boolean;
  displayName: string;
  /** Changed since the listing was published; only the published view shows it. */
  edited?: boolean;
  /** Published view only: the draft never offers an inactive plan. */
  inactive?: boolean;
  /** Absent for a deleted plan, whose limit is no longer known. */
  limit?: PlanLimitDisplay;
  onToggle?: () => void;
  readOnly?: boolean;
  selected: boolean;
};

/** Primary in the draft, success in the published view; state is carried by the chips. */
type Accent = 'primary' | 'success';

/** The states of a published card that get a chip. Deleted has none: its name is struck through. */
type Chipped = 'inactive' | 'edited';

const CHECK_SIZE = 20;

/** Room for the limit, so cards with and without one stay level. */
const BODY_MIN_HEIGHT = 42;

/** A drawn checkbox: a real one cannot nest inside a button. */
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

/** `Form.CardButton` paints its hover border primary, so a selected card restates it in the accent. */
const planCardSx = (
  theme: Theme,
  {
    accent,
    disabled,
    readOnly,
    selected,
    unavailable,
  }: { accent: Accent; disabled?: boolean; readOnly?: boolean; selected: boolean; unavailable: boolean }
) => ({
  borderColor: selected ? `${accent}.main` : 'divider',
  ...(unavailable && { '&:hover': { borderColor: 'divider' } }),
  ...(selected && {
    backgroundColor: alpha(theme.palette[accent].main, 0.06),
    boxShadow: theme.shadows[1],
    ...(!disabled && { '&:hover': { borderColor: `${accent}.main` } }),
  }),
  '&:focus-visible': { outline: `2px solid ${theme.palette.primary.main}`, outlineOffset: '-2px' },
  alignItems: 'flex-start',
  display: 'flex',
  flexDirection: 'column',
  gap: 2.5,
  p: 3,
  width: '100%',
  ...(readOnly && { cursor: 'default' }),
});

/**
 * One plan in the grid; the whole card is the checkbox. The published card shows what became of the
 * plan: deleted or inactive goes grey, deleted is struck through, inactive or edited gets a chip, and an
 * edited plan hides its limit, which would be the current one. `readOnly` never sets `disabled`, so the
 * tooltip still shows.
 */
export function PlanCard({
  deleted = false,
  disabled,
  displayName,
  edited = false,
  inactive = false,
  limit,
  onToggle,
  readOnly,
  selected,
}: PlanCardProps) {
  const intl = useIntl();
  const accent: Accent = readOnly ? 'success' : 'primary';
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
        aria-checked={selected}
        aria-disabled={readOnly || disabled || undefined}
        disabled={!readOnly && disabled}
        // `Form.CardButton` does not turn `disabled` into a disabled button, so a click is dropped here too.
        onClick={readOnly || disabled ? undefined : onToggle}
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
      </Form.CardButton>
    </Tooltip>
  );
}
