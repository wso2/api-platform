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

import type { FC, ReactNode } from 'react';
import { Box, Card, Radio, Tooltip, Typography } from '@wso2/oxygen-ui';

export type HostingOptionCardProps = {
  icon: ReactNode;
  title: string;
  description: string;
  selected: boolean;
  /** Roving tab stop: only the selected card (or the first, when none is) is reachable with Tab. */
  tabIndex?: number;
  /** This hosting kind's quota is full: the card cannot be chosen. */
  disabled?: boolean;
  /** Why it cannot be chosen; shown on hover over the disabled card. */
  disabledMessage?: string;
  onClick: () => void;
};

/**
 * One hosting choice on the first step of Add Gateway — bigger and more
 * explanatory than `GatewayTypeCard`'s row (icon, title, two-line description,
 * a radio rather than a plain selected state), since a hosting choice is a
 * one-time, harder-to-reverse decision the admin needs more to go on than a
 * type pick does.
 */
const HostingOptionCard: FC<HostingOptionCardProps> = ({
  icon,
  title,
  description,
  selected,
  tabIndex = 0,
  disabled = false,
  disabledMessage,
  onClick,
}) => {
  const card = (
  <Card
    role="radio"
    aria-checked={selected}
    aria-disabled={disabled || undefined}
    // Out of the roving tab order entirely: a stop that cannot be chosen is
    // just a dead key press.
    tabIndex={disabled ? -1 : tabIndex}
    onClick={disabled ? undefined : onClick}
    onKeyDown={(event) => {
      if (disabled) return;
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        onClick();
      }
    }}
    sx={{
      flex: 1,
      display: 'flex',
      alignItems: 'flex-start',
      gap: 1.5,
      p: 2,
      border: '1px solid',
      borderColor: selected ? 'primary.main' : 'divider',
      cursor: disabled ? 'not-allowed' : 'pointer',
      opacity: disabled ? 0.5 : 1,
      transition: 'border-color 0.15s ease, background-color 0.15s ease',
      '&:hover': selected || disabled ? undefined : { borderColor: 'text.disabled' },
    }}
  >
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 36,
        height: 36,
        borderRadius: '8px',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: 'action.hover',
        color: 'primary.main',
        flexShrink: 0,
      }}
    >
      {icon}
    </Box>
    <Box sx={{ flexGrow: 1, minWidth: 0 }}>
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', opacity: 0.7 }}>
        {description}
      </Typography>
    </Box>
    <Radio checked={selected} disabled={disabled} tabIndex={-1} sx={{ p: 0, mt: 0.25 }} />
  </Card>
  );

  if (!disabled) return card;
  // The card keeps pointer-events (it is not a disabled MUI control), but the
  // Radio inside it does not, so the wrapper is what the Tooltip listens on.
  return (
    <Tooltip title={disabledMessage ?? ''}>
      <Box sx={{ flex: 1, display: 'flex' }}>{card}</Box>
    </Tooltip>
  );
};

export default HostingOptionCard;
