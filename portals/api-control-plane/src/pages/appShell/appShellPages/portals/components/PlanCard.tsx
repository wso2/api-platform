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

import { alpha, Box, Chip, Form, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Ban, Check, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl } from 'react-intl';

import { focusRingSx, selectableCardSx } from '@/theme/receipes';
import type { PlanLimitDisplay } from '../utils/subscriptionPlanLimit';

const messages = defineMessages({
  active: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.active',
    defaultMessage: 'Active',
    description: 'Status pill on a plan card.',
  },
  inactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactive',
    defaultMessage: 'Inactive',
    description: 'Status pill on a plan card.',
  },
  inactiveBlocked: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.inactiveBlocked',
    defaultMessage: 'Inactive. Activate it in Settings.',
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
  /** Replaces `onToggle` for an inactive plan that is not selected. */
  onBlocked?: () => void;
  readOnly?: boolean;
  selected: boolean;
};

/** Side of the leading checkbox square. */
const CHECK_SIZE = 20;

/**
 * One plan in the grid: a single `Form.CardButton` wearing the `checkbox` role, so the whole card
 * is the hit target. The square at the left is drawn, not a real checkbox, which cannot nest in a button.
 *
 * An inactive plan that is not selected is *blocked*: dimmed (not the pill), dashed, and `aria-disabled`
 * rather than `disabled` so it keeps focus and its tooltip. An inactive plan that is still selected keeps
 * full strength, because the user has to clear it.
 *
 * `readOnly` (the published view) never sets `disabled`, which would grey the card out; `pointerEvents`
 * blocks interaction instead.
 */
export function PlanCard({
  disabled,
  displayName,
  inactive = false,
  limit,
  onBlocked,
  onToggle,
  readOnly,
  selected,
}: PlanCardProps) {
  const intl = useIntl();
  const blocked = inactive && !selected && !readOnly;
  const needsClearing = inactive && selected && !readOnly;
  const accent = readOnly ? 'success' : inactive && selected ? 'warning' : 'primary';
  const tooltip = blocked
    ? intl.formatMessage(messages.inactiveBlocked)
    : needsClearing
      ? intl.formatMessage(messages.inactiveSelected)
      : '';
  const dimmed = blocked ? { opacity: 0.6 } : undefined;
  const statusColor = inactive ? 'warning' : 'success';

  return (
    // describeChild keeps the plan's name as the card's accessible name.
    <Tooltip describeChild title={tooltip}>
      <Form.CardButton
        alignItems="flex-start"
        aria-checked={selected}
        aria-disabled={readOnly || blocked}
        disabled={!readOnly && disabled}
        onClick={readOnly ? undefined : blocked ? onBlocked : onToggle}
        role="checkbox"
        selected={selected}
        sx={(theme) => ({
          ...selectableCardSx(theme, { selected }, accent),
          ...focusRingSx(theme),
          display: 'flex',
          flexDirection: 'column',
          gap: 2.5,
          p: 3,
          width: '100%',
          ...(blocked && { borderStyle: 'dashed', cursor: 'not-allowed' }),
          ...(readOnly && { cursor: 'default', pointerEvents: 'none' }),
        })}
        tabIndex={readOnly ? -1 : 0}
      >
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5, justifyContent: 'space-between', width: '100%' }}>
          <Box sx={{ alignItems: 'center', display: 'flex', flex: 1, gap: 1.5, minWidth: 0, ...dimmed }}>
            {blocked ? (
              <Ban aria-hidden color="currentColor" size={CHECK_SIZE} style={{ flexShrink: 0 }} />
            ) : (
              <Box
                sx={(theme) => ({
                  alignItems: 'center',
                  bgcolor: selected ? theme.palette[accent].main : 'transparent',
                  border: 1.5,
                  borderColor: selected ? theme.palette[accent].main : 'text.disabled',
                  borderRadius: '6px', // explicit: the theme's base radius would make a 20px square nearly round
                  display: 'flex',
                  flexShrink: 0,
                  height: CHECK_SIZE,
                  justifyContent: 'center',
                  width: CHECK_SIZE,
                })}
              >
                {selected && <Check color="#fff" size={14} strokeWidth={3} />}
              </Box>
            )}
            <Typography noWrap sx={{ fontSize: '1rem', fontWeight: 600, minWidth: 0 }} variant="subtitle1">
              {displayName}
            </Typography>
          </Box>
          <Chip
            color={statusColor}
            icon={
              inactive ? (
                <TriangleAlert aria-hidden size={14} />
              ) : (
                <Box aria-hidden sx={{ bgcolor: 'currentColor', borderRadius: '50%', height: 8, ml: 1, width: 8 }} />
              )
            }
            label={<FormattedMessage {...(inactive ? messages.inactive : messages.active)} />}
            size="small"
            sx={(theme) => ({
              bgcolor: alpha(theme.palette[statusColor].main, 0.16),
              color: `${statusColor}.main`,
              flexShrink: 0,
              fontSize: 12,
              fontWeight: 600,
              letterSpacing: '0.01em',
              '& .MuiChip-icon': { color: 'inherit' },
            })}
            variant="filled"
          />
        </Box>

        <Box sx={{ alignItems: 'baseline', columnGap: 1, display: 'flex', ...dimmed }}>
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
