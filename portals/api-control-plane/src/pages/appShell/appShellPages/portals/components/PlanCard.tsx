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

import { Box, Form, Typography } from '@wso2/oxygen-ui';
import { Check } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber } from 'react-intl';

import { focusRingSx, selectableCardSx } from '@/theme/receipes';
import type { PlanLimitDisplay } from '../utils/subscriptionPlanLimit';

const messages = defineMessages({
  unlimitedValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PlanCard.unlimitedValue',
    defaultMessage: 'Unlimited',
    description: 'Plan card value when the plan has no configured limit.',
  },
});

type PlanCardProps = {
  disabled?: boolean;
  displayName: string;
  limit: PlanLimitDisplay;
  onToggle?: () => void;
  /** The published view: shows the live selection, but nothing on it can be changed. */
  readOnly?: boolean;
  selected: boolean;
};

/**
 * One plan in the grid: a single `Form.CardButton` wearing the `checkbox`
 * role, so the whole card is the hit target and the native button already
 * gives Enter/Space toggling for free (`CardButton` renders a real
 * `ButtonBase`/`<button>` and forwards unknown props straight to it).
 *
 * `readOnly` never sets MUI's own `disabled` — that greys the card out, which
 * the published view's selected plans must not be: `pointerEvents: 'none'`
 * blocks interaction (and the hover it would otherwise trigger) while leaving
 * the resting colors exactly as `selectableCardSx` renders them.
 */
export function PlanCard({ disabled, displayName, limit, onToggle, readOnly, selected }: PlanCardProps) {
  const accent = readOnly ? 'success' : 'primary';

  return (
    <Form.CardButton
      alignItems="flex-start"
      aria-checked={selected}
      aria-disabled={readOnly}
      disabled={!readOnly && disabled}
      onClick={readOnly ? undefined : onToggle}
      role="checkbox"
      selected={selected}
      sx={(theme) => ({
        ...selectableCardSx(theme, { selected }, accent),
        ...focusRingSx(theme),
        display: 'flex',
        flexDirection: 'column',
        gap: 2.5,
        p: 2.5,
        width: '100%',
        ...(readOnly && { cursor: 'default', pointerEvents: 'none' }),
      })}
      tabIndex={readOnly ? -1 : 0}
    >
      <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'space-between', gap: 1.5, width: '100%' }}>
        <Typography sx={{ fontWeight: 500 }} variant="body1">
          {displayName}
        </Typography>
        <Box
          sx={(theme) => ({
            alignItems: 'center',
            bgcolor: selected ? theme.palette[accent].main : 'transparent',
            border: 1.5,
            borderColor: selected ? theme.palette[accent].main : 'divider',
            borderRadius: 1,
            display: 'flex',
            flexShrink: 0,
            height: 20,
            justifyContent: 'center',
            width: 20,
          })}
        >
          {selected && <Check color="#fff" size={14} strokeWidth={3} />}
        </Box>
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
        <Typography sx={{ lineHeight: 1.2 }} variant="h5">
          {limit.unlimited ? <FormattedMessage {...messages.unlimitedValue} /> : <FormattedNumber value={limit.count} />}
        </Typography>
        <Typography color="text.secondary" variant="caption">
          <FormattedMessage {...limit.unit} values={limit.unlimited ? undefined : limit.unitValues} />
        </Typography>
      </Box>
    </Form.CardButton>
  );
}
