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
import { defineMessages, useIntl, type IntlShape } from 'react-intl';

import { isBuiltInType, type ConditionLimits, type PlanCondition } from './planConditions';

/** The plans table shows a plan's first five conditions as chips; a "+N" chip lists the rest. */
export const MAX_CONDITION_CHIPS = 5;

const messages = defineMessages({
  allowed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.allowed',
    defaultMessage: 'Allowed past the limit',
    description: 'Tooltip line of a condition that lets requests through once a limit is used up.',
  },
  label: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.label',
    defaultMessage:
      '{type, select, requests {Requests} events {Events} toolCalls {Tool calls} total {Tokens} prompt {Prompt tokens} completion {Completion tokens} other {Custom}}',
    description: 'Name on a condition chip: what the condition counts, or Custom.',
  },
  moreAria: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.moreAria',
    defaultMessage: '{count, plural, one {# more condition} other {# more conditions}}',
  },
  moreTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionChips.moreTitle',
    defaultMessage: 'More conditions',
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
  'event-count': 'events',
  'request-count': 'requests',
  'tool-call-count': 'toolCalls',
};

/** One chip per condition: its first limit and how many more it has, with every limit (or a custom
 * condition's values) for the tooltip. As the API Portal's plans table builds them. */
export const conditionChips = (intl: IntlShape, conditions: PlanCondition[]): ConditionChip[] =>
  conditions.map((condition) => {
    const params = (condition.params ?? {}) as Record<string, unknown>;
    if (!isBuiltInType(condition.type)) {
      const values = Object.entries(params).map(([name, value]) =>
        intl.formatMessage(messages.pair, {
          name,
          value: typeof value === 'string' ? value : JSON.stringify(value),
        }),
      );
      return {
        custom: true,
        details: values.length > 0 ? values : [intl.formatMessage(messages.noValues)],
        first: condition.id ?? '',
        label: intl.formatMessage(messages.label, { type: 'custom' }),
        more: 0,
      };
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
    return {
      custom: false,
      details: [
        ...windows,
        ...(limits.onExceed === 'allow' ? [intl.formatMessage(messages.allowed)] : []),
      ],
      first: windows[0] ?? intl.formatMessage(messages.noLimit),
      label: intl.formatMessage(messages.label, { type: labelKey }),
      more: Math.max(windows.length - 1, 0),
    };
  });

const chipSx = {
  '& .MuiChip-label': { display: 'flex', gap: 0.75, alignItems: 'center' },
  height: 24,
  typography: 'caption',
} as const;

const TipLines = ({ lines }: { lines: string[] }) => (
  <>
    {lines.map((line, index) => (
      <Typography component="div" key={index} sx={{ fontFamily: 'monospace' }} variant="caption">
        {line}
      </Typography>
    ))}
  </>
);

const chipTitle = (chip: ConditionChip) =>
  chip.custom ? `${chip.label} · ${chip.first}` : chip.label;

/** The Conditions cell of the plans table: one chip per condition, at most five, and a "+N" chip
 * for the rest; a plan with none is Unlimited. Hovering or focusing a chip lists every limit. */
export function PlanConditionChips({ conditions }: { conditions: PlanCondition[] | undefined }) {
  const intl = useIntl();
  const chips = conditionChips(intl, conditions ?? []);
  if (chips.length === 0) {
    return (
      <Typography color="text.secondary" variant="body2">
        {intl.formatMessage(messages.unlimited)}
      </Typography>
    );
  }
  const shown = chips.slice(0, MAX_CONDITION_CHIPS);
  const rest = chips.slice(MAX_CONDITION_CHIPS);
  return (
    <Stack direction="row" flexWrap="wrap" gap={0.75}>
      {shown.map((chip, index) => (
        <Tooltip
          key={index}
          title={
            <Box>
              <Typography component="div" sx={{ fontWeight: 600 }} variant="caption">
                {chipTitle(chip)}
              </Typography>
              <TipLines lines={chip.details} />
            </Box>
          }
        >
          <Chip
            label={
              <>
                <Box component="span" sx={{ fontWeight: 600 }}>
                  {chip.label}
                </Box>
                <Box component="span" sx={{ fontFamily: 'monospace' }}>
                  {chip.first}
                </Box>
                {chip.more > 0 && (
                  <Box component="span" sx={{ color: 'text.secondary' }}>
                    {intl.formatMessage(messages.moreWindows, { count: chip.more })}
                  </Box>
                )}
              </>
            }
            size="small"
            sx={chipSx}
            tabIndex={0}
            variant="outlined"
          />
        </Tooltip>
      ))}
      {rest.length > 0 && (
        <Tooltip
          title={
            <Stack spacing={0.75}>
              <Typography component="div" sx={{ fontWeight: 600 }} variant="caption">
                {intl.formatMessage(messages.moreTitle)}
              </Typography>
              {rest.map((chip, index) => (
                <Box key={index}>
                  <Typography component="div" sx={{ fontWeight: 600 }} variant="caption">
                    {chipTitle(chip)}
                  </Typography>
                  <TipLines lines={chip.details} />
                </Box>
              ))}
            </Stack>
          }
        >
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
