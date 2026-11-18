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

import type { ReactNode } from 'react';
import {
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  type ToggleButtonGroupProps,
} from '@wso2/oxygen-ui';

export type SegmentedControlOption<T extends string> = {
  value: T;
  label: ReactNode;
  icon?: ReactNode;
  disabled?: boolean;
};

export type SegmentedControlProps<T extends string> = Omit<
  ToggleButtonGroupProps,
  'children' | 'exclusive' | 'onChange' | 'value'
> & {
  options: SegmentedControlOption<T>[];
  value: T;
  onChange: (value: T) => void;
  /** Accessible name of the group. */
  'aria-label': string;
};

/**
 * A rounded, pill-shaped switch between a few mutually exclusive choices. The
 * chosen segment is filled with the strong text colour so it reads in both the
 * light and the dark theme. Built on Oxygen's `ToggleButtonGroup`, so it keeps
 * its keyboard and `aria-pressed` behaviour; extra props go to the group.
 * Pressing the selected segment never clears the choice.
 */
export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  sx,
  ...rest
}: SegmentedControlProps<T>) {
  return (
    <ToggleButtonGroup
      exclusive
      onChange={(_event, next: T | null) => {
        if (next !== null) onChange(next);
      }}
      size="small"
      sx={[
        {
          bgcolor: 'transparent',
          border: 1,
          borderColor: 'divider',
          borderRadius: 999,
          gap: 0.5,
          p: 0.5,
          '& .MuiToggleButtonGroup-grouped': {
            border: 0,
            borderRadius: 999,
            color: 'text.primary',
            fontWeight: 600,
            m: 0,
            px: 2,
            py: 0.5,
            textTransform: 'none',
            '&:hover': { bgcolor: 'action.hover', color: 'primary.main' },
            '&.Mui-selected': {
              bgcolor: 'text.primary',
              color: 'background.default',
              '&:hover': { bgcolor: 'text.primary', color: 'background.default' },
            },
          },
        },
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
      value={value}
      {...rest}
    >
      {options.map((option) => (
        <ToggleButton disabled={option.disabled} key={option.value} value={option.value}>
          {option.icon ? (
            <Stack alignItems="center" direction="row" spacing={1}>
              {option.icon}
              <span>{option.label}</span>
            </Stack>
          ) : (
            option.label
          )}
        </ToggleButton>
      ))}
    </ToggleButtonGroup>
  );
}
