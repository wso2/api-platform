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

import type { SxProps, Theme } from '@mui/material/styles';

export type ColorScheme = 'light' | 'dark';

const getElementColorScheme = (element: Element | null): ColorScheme | null => {
  if (!element) return null;

  const attrScheme = [
    element.getAttribute('data-mui-color-scheme'),
    element.getAttribute('data-color-scheme'),
    element.getAttribute('data-theme'),
  ].find((value) => value === 'light' || value === 'dark');

  if (attrScheme === 'light' || attrScheme === 'dark') {
    return attrScheme;
  }

  if (element.classList.contains('dark')) {
    return 'dark';
  }

  return null;
};

export const getActiveColorScheme = (): ColorScheme => {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return 'light';
  }

  return (
    getElementColorScheme(document.documentElement) ??
    getElementColorScheme(document.body) ??
    (window.matchMedia?.('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light')
  );
};

export const subscribeToColorSchemeChanges = (
  onChange: (scheme: ColorScheme) => void
): (() => void) => {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return () => {};
  }

  const updateScheme = () => {
    onChange(getActiveColorScheme());
  };

  updateScheme();

  const observer = new MutationObserver(updateScheme);
  const observerOptions: MutationObserverInit = {
    attributes: true,
    attributeFilter: [
      'class',
      'data-mui-color-scheme',
      'data-color-scheme',
      'data-theme',
    ],
  };

  observer.observe(document.documentElement, observerOptions);
  if (document.body) {
    observer.observe(document.body, observerOptions);
  }

  const mediaQuery = window.matchMedia?.('(prefers-color-scheme: dark)');
  mediaQuery?.addEventListener?.('change', updateScheme);

  return () => {
    observer.disconnect();
    mediaQuery?.removeEventListener?.('change', updateScheme);
  };
};

export const getCommandTextFieldSx = (
  colorScheme: ColorScheme
): SxProps<Theme> => ({
  '& .css-1fbx0xo-MuiInputBase-root-MuiOutlinedInput-root, & .MuiOutlinedInput-root':
    {
      bgcolor: (theme: Theme) =>
        `${
          colorScheme === 'dark'
            ? theme.palette.background.paper
            : theme.palette.common.black
        } !important`,
    },
  '& .MuiOutlinedInput-input': {
    color: (theme: Theme) =>
      colorScheme === 'dark'
        ? theme.palette.text.primary
        : theme.palette.common.white,
    WebkitTextFillColor: (theme: Theme) =>
      colorScheme === 'dark'
        ? theme.palette.text.primary
        : theme.palette.common.white,
  },
  '& .MuiIconButton-root': {
    color: (theme: Theme) =>
      colorScheme === 'dark'
        ? theme.palette.text.primary
        : theme.palette.common.white,
  },
});
