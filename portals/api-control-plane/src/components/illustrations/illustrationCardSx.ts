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

import type { Theme } from '@wso2/oxygen-ui';

/**
 * Card chrome shared by the decorative empty-state animations: the theme's
 * primary/success/error as CSS variables (lighter shades in dark mode), and the
 * tinted gradient card in light mode. Mirrors `ImportApiIllustration`.
 */
export const illustrationCardSx = (prefix: string, theme: Theme) => {
  const palette = theme.vars?.palette ?? theme.palette;
  return {
    width: 280,
    maxWidth: '100%',
    [`--${prefix}-accent`]: palette.primary.dark,
    [`--${prefix}-accent-base`]: palette.primary.main,
    [`--${prefix}-success`]: palette.success.dark,
    [`--${prefix}-success-base`]: palette.success.main,
    [`--${prefix}-error`]: palette.error.dark,
    [`--${prefix}-error-base`]: palette.error.main,
    [`--${prefix}-border`]: palette.divider,
    [`--${prefix}-muted`]: palette.text.secondary,
    [`--${prefix}-text`]: palette.text.primary,
    [`--${prefix}-track`]: palette.action.hover,
    [`--${prefix}-row`]: palette.action.hover,
    ...theme.applyStyles('light', {
      backgroundColor: `color-mix(in srgb, ${palette.primary.main} 12%, ${palette.background.paper})`,
      backgroundImage: `linear-gradient(145deg, ${palette.background.paper} 0%, color-mix(in srgb, ${palette.primary.main} 8%, ${palette.background.paper}) 45%, color-mix(in srgb, ${palette.primary.main} 16%, ${palette.background.paper}) 100%)`,
      border: '1px solid',
      borderColor: `color-mix(in srgb, ${palette.primary.main} 40%, ${palette.background.paper})`,
      boxShadow: `inset 0 1px 0 ${palette.background.paper}, inset 0 -1px 2px color-mix(in srgb, ${palette.primary.main} 10%, transparent), 0 8px 20px color-mix(in srgb, ${palette.primary.dark} 12%, transparent)`,
    }),
    ...theme.applyStyles('dark', {
      [`--${prefix}-accent`]: palette.primary.light,
      [`--${prefix}-success`]: palette.success.light,
      [`--${prefix}-error`]: palette.error.light,
    }),
  };
};
