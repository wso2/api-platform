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

/**
 * Secondary text: descriptions, helper lines and status. The theme's
 * `text.secondary` is close to `text.primary` in dark mode, so this plugin mutes
 * it the way its hosting tabs already do, a little further to match the design.
 */
export const secondaryTextSx = { color: 'text.secondary', opacity: 0.55 } as const;

/**
 * The accent for work still in progress (a change being applied). The theme has
 * no such colour (`info` is white in dark mode), so it is the one literal here,
 * as the deploy plugin's provisioning card has its own.
 */
export const PROGRESS_ACCENT = '#8B7CF6';

/**
 * Outlined inputs in the drawer: the primary colour when focused, as the design
 * has it, and when `modified`, so an unsaved edit stays marked after the field
 * loses focus. The selectors repeat the Choreo Labs theme's own focus rule
 * (`.Mui-focused:not(.Mui-error)`, cobalt) so they outrank it; an error keeps
 * its red. `&` covers a Select, whose root is the input; `& .MuiOutlinedInput-root`
 * a TextField, whose input is a child.
 */
export const inputSx = (modified: boolean) =>
  ({
    ...(modified
      ? {
          '&:not(.Mui-error) .MuiOutlinedInput-notchedOutline, & .MuiOutlinedInput-root:not(.Mui-error) .MuiOutlinedInput-notchedOutline':
            { borderColor: 'primary.main' },
        }
      : {}),
    '&.Mui-focused:not(.Mui-error) .MuiOutlinedInput-notchedOutline, & .MuiOutlinedInput-root.Mui-focused:not(.Mui-error) .MuiOutlinedInput-notchedOutline':
      { borderColor: 'primary.main' },
  }) as const;
