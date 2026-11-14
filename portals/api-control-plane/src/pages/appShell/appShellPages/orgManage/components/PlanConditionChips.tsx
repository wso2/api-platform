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

import { Box, Chip, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Fragment } from 'react';
import { defineMessages, useIntl, type IntlShape } from 'react-intl';

import { isBuiltInType, type ConditionLimits, type PlanCondition } from './planConditions';

/** The plans table shows a plan's first five conditions as chips; a "+N" chip lists the rest. */
export const MAX_CONDITION_CHIPS = 3;

const messages = defineMessages({
  allowed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.allowed',
    defaultMessage: 'Allowed past the limit',
    description: 'Tooltip line of a condition that lets requests through once a limit is used up.',
  },
  label: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.label',
    defaultMessage:
      '{type, select, requests {Requests} toolCalls {Tool calls} total {Tokens} prompt {Prompt tokens} completion {Completion tokens} other {Custom}}',
    description: 'Name on a condition chip: what the condition counts, or Custom.',
  },
  moreAria: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.moreAria',
    defaultMessage: '{count, plural, one {# more condition} other {# more conditions}}',
  },
  moreWindows: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.moreWindows',
    defaultMessage: '+{count, number}',
    description: 'How many more limits (or conditions) there are than the chip shows.',
  },
  noLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.noLimit',
    defaultMessage: 'No limit',
  },
  noValues: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.noValues',
    defaultMessage: 'No values',
  },
  pair: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.pair',
    defaultMessage: '{name} = {value}',
    description: 'One key-value pair of a custom condition.',
  },
  unlimited: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.unlimited',
    defaultMessage: 'Unlimited',
    description: 'Shown for a plan with no conditions.',
  },
  window: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.window',
    defaultMessage:
      '{count} / {amount, plural, one {} other {# }}{unit, select, MINUTE {min} HOUR {hr} DAY {day} MONTH {mo} other {}}',
    description: 'One limit of a condition, e.g. "1,000 / min" or "5,000 / 2 hr".',
  },
});

/** What one chip shows, and its tooltip. */
export type ConditionChip = {
  label: string;
  first: string;
  more: number;
  details: string[];
  custom: boolean;
};

const LABEL_KEYS: Record<string, string> = {
  'request-count': 'requests',
  'tool-call-count': 'toolCalls',
};

/** One chip per condition window (a custom condition: one chip, its values for the tooltip). */
export const conditionChips = (intl: IntlShape, conditions: PlanCondition[]): ConditionChip[] =>
  conditions.flatMap((condition): ConditionChip[] => {
    const params = (condition.params ?? {}) as Record<string, unknown>;
    if (!isBuiltInType(condition.type)) {
      const values = Object.entries(params).map(([name, value]) =>
        intl.formatMessage(messages.pair, {
          name,
          value: typeof value === 'string' ? value : JSON.stringify(value),
        }),
      );
      return [
        {
          custom: true,
          details: values.length > 0 ? values : [intl.formatMessage(messages.noValues)],
          first: condition.id ?? '',
          label: intl.formatMessage(messages.label, { type: 'custom' }),
          more: 0,
        },
      ];
    }
    const limits = params as Partial<ConditionLimits>;
    const labelKey =
      condition.type === 'token-count' ? (limits.tokenType ?? 'total') : LABEL_KEYS[condition.type];
    const windows = (limits.limits ?? []).map((window) =>
      intl.formatMessage(messages.window, {
        amount: window.timeAmount ?? 1,
        count: intl.formatNumber(window.count),
        unit: window.timeUnit,
      }),
    );
    const label = intl.formatMessage(messages.label, { type: labelKey });
    const allowed = limits.onExceed === 'allow' ? [intl.formatMessage(messages.allowed)] : [];
    if (windows.length === 0) {
      const none = intl.formatMessage(messages.noLimit);
      return [{ custom: false, details: [none], first: none, label, more: 0 }];
    }
    // Each window is its own chip: two request windows read as two Requests chips.
    return windows.map((window) => ({
      custom: false,
      details: [window, ...allowed],
      first: window,
      label,
      more: 0,
    }));
  });

const chipSx = {
  '& .MuiChip-label': { display: 'flex', gap: 0.75, alignItems: 'center' },
  height: 24,
  typography: 'caption',
} as const;

const chipTitle = (chip: ConditionChip) =>
  chip.custom ? `${chip.label} · ${chip.first}` : chip.label;

/** The tooltip surface: the app's paper, not the default grey, so it reads like a small card. */
export const tipSlotProps = {
  tooltip: {
    sx: {
      bgcolor: 'background.paper',
      border: 1,
      borderColor: 'divider',
      boxShadow: 3,
      color: 'text.primary',
      maxWidth: 360,
      px: 1.5,
      py: 1,
    },
  },
} as const;

/** Conditions as a compact two-column list: the condition, then its limits (or values). */
const ConditionTip = ({ chips }: { chips: ConditionChip[] }) => (
  <Box
    sx={{
      alignItems: 'baseline',
      columnGap: 1.5,
      display: 'grid',
      gridTemplateColumns: 'auto 1fr',
      rowGap: 0.5,
    }}
  >
    {chips.map((chip, index) => (
      <Fragment key={index}>
        <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="caption">
          {chipTitle(chip)}
        </Typography>
        <Typography
          color="text.secondary"
          component="div"
          sx={{ fontFamily: 'monospace' }}
          variant="caption"
        >
          {chip.details.join(', ')}
        </Typography>
      </Fragment>
    ))}
  </Box>
);

/** The Conditions cell of a plans table: one chip per condition window (a condition with two
 * windows shows two chips), at most `max` (three by default), and a "+N" chip whose tooltip lists the
 * rest; a plan with none is Unlimited. */
export function PlanConditionChips({
  conditions,
  max = MAX_CONDITION_CHIPS,
  singleLine = false,
}: {
  conditions: PlanCondition[] | undefined;
  max?: number;
  /** Keep the chips on one line (a fixed-height table row), never wrapping. */
  singleLine?: boolean;
}) {
  const intl = useIntl();
  const chips = conditionChips(intl, conditions ?? []);
  if (chips.length === 0) {
    return (
      <Typography color="text.secondary" variant="body2">
        {intl.formatMessage(messages.unlimited)}
      </Typography>
    );
  }
  const shown = chips.slice(0, max);
  const rest = chips.slice(max);
  return (
    <Stack direction="row" flexWrap={singleLine ? 'nowrap' : 'wrap'} gap={0.75} sx={singleLine ? { minWidth: 0, overflow: 'hidden' } : undefined}>
      {shown.map((chip, index) => (
        // The chip says it all; only "+N" has a tooltip, for what it hides.
        <Chip
          key={index}
          label={
            <>
              <Box component="span" sx={{ fontWeight: 600 }}>
                {chip.label}
              </Box>
              <Box component="span" sx={{ fontFamily: 'monospace' }}>
                {chip.first}
              </Box>
            </>
          }
          size="small"
          sx={chipSx}
          variant="outlined"
        />
      ))}
      {rest.length > 0 && (
        <Tooltip slotProps={tipSlotProps} title={<ConditionTip chips={rest} />}>
          <Chip
            aria-label={intl.formatMessage(messages.moreAria, { count: rest.length })}
            label={intl.formatMessage(messages.moreWindows, { count: rest.length })}
            size="small"
            sx={chipSx}
            tabIndex={0}
            variant="outlined"
          />
        </Tooltip>
      )}
    </Stack>
  );
}
