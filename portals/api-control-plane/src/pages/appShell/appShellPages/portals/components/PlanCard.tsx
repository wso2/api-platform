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
import { Check, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl } from 'react-intl';

import { focusRingSx } from '@/theme/receipes';
import type { PlanLimitDisplay } from '../utils/subscriptionPlanLimit';

const messages = defineMessages({
  inactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactive',
    defaultMessage: 'Inactive',
    description: 'Status pill on a plan card.',
  },
  inactiveSelected: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactiveSelected',
    defaultMessage: 'Clear inactive plan to save or publish.',
  },
  unlimitedValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.unlimitedValue',
    defaultMessage: 'Unlimited',
    description: 'Plan card value when the plan has no configured limit.',
  },
});

type PlanCardProps = {
  disabled?: boolean;
  displayName: string;
  inactive?: boolean;
  limit: PlanLimitDisplay;
  onToggle?: () => void;
  readOnly?: boolean;
  selected: boolean;
};

/** Palette colour a card is drawn in: success when live, warning when it must be cleared. */
type Accent = 'primary' | 'success' | 'warning';

/** Side of the leading checkbox square. */
const CHECK_SIZE = 20;

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
  ...(readOnly && { cursor: 'default', pointerEvents: 'none' }),
});

/**
 * One plan in the grid: a single `Form.CardButton` wearing the `checkbox` role, so the whole card
 * is the hit target. The square at the left is drawn, not a real checkbox, which cannot nest in a button.
 *
 * An inactive plan only appears while it is selected, in the warning accent with a tooltip, because the
 * user has to clear it.
 *
 * `readOnly` (the published view) never sets `disabled`, which would grey the card out; `pointerEvents`
 * blocks interaction instead.
 */
export function PlanCard({
  disabled,
  displayName,
  inactive = false,
  limit,
  onToggle,
  readOnly,
  selected,
}: PlanCardProps) {
  const intl = useIntl();
  const needsClearing = inactive && selected && !readOnly;
  const accent: Accent = readOnly ? 'success' : needsClearing ? 'warning' : 'primary';
  const tooltip = needsClearing ? intl.formatMessage(messages.inactiveSelected) : '';

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
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5, justifyContent: 'space-between', width: '100%' }}>
          <Box sx={{ alignItems: 'center', display: 'flex', flex: 1, gap: 1.5, minWidth: 0 }}>
            <SelectionSquare accent={accent} selected={selected} />
            <Typography noWrap sx={{ fontSize: '1rem', fontWeight: 600, minWidth: 0 }} variant="subtitle1">
              {displayName}
            </Typography>
          </Box>
          {inactive && (
            <Chip
              color="warning"
              icon={<TriangleAlert aria-hidden size={14} />}
              label={<FormattedMessage {...messages.inactive} />}
              size="small"
              sx={(theme) => ({
                bgcolor: alpha(theme.palette.warning.main, 0.16),
                color: 'warning.main',
                flexShrink: 0,
                fontSize: 12,
                fontWeight: 600,
                letterSpacing: '0.01em',
                '& .MuiChip-icon': { color: 'inherit' },
              })}
              variant="filled"
            />
          )}
        </Box>

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
      </Form.CardButton>
    </Tooltip>
  );
}
