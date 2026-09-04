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
import { Box, Chip, Typography } from '@wso2/oxygen-ui';

export type GatewayTypeCardProps = {
  icon: ReactNode;
  label: string;
  badge?: string;
  selected: boolean;
  /** Shown but not selectable — a type that is announced before it can be created. */
  disabled?: boolean;
  onClick: () => void;
};

const GatewayTypeCard: FC<GatewayTypeCardProps> = ({
  icon,
  label,
  badge,
  selected,
  disabled,
  onClick,
}) => (
  <Box
    aria-disabled={disabled || undefined}
    role="button"
    tabIndex={disabled ? -1 : 0}
    onClick={disabled ? undefined : onClick}
    onKeyDown={(event) => {
      if (disabled) return;
      if (event.key === 'Enter' || event.key === ' ') onClick();
    }}
    sx={{
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      gap: 1.25,
      px: 2,
      py: 1.5,
      border: '1px solid',
      borderColor: selected ? 'primary.main' : 'divider',
      bgcolor: selected ? 'action.selected' : 'background.paper',
      borderRadius: '10px',
      cursor: disabled ? 'not-allowed' : 'pointer',
      opacity: disabled ? 0.6 : 1,
      transition: 'border-color 0.15s ease, background-color 0.15s ease',
      '&:hover': selected || disabled ? undefined : { borderColor: 'text.disabled' },
    }}
  >
    <Box sx={{ display: 'flex', color: selected && !disabled ? 'primary.main' : 'text.secondary' }}>{icon}</Box>
    <Typography
      variant="body2"
      sx={{ fontWeight: 600, flexGrow: 1, color: selected && !disabled ? 'text.primary' : 'text.secondary' }}
    >
      {label}
    </Typography>
    {badge ? (
      <Chip
        color={disabled ? 'default' : 'info'}
        label={badge}
        size="small"
        sx={{ height: 20, fontSize: 11 }}
      />
    ) : null}
  </Box>
);

export default GatewayTypeCard;
