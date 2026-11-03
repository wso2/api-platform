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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, { type ReactElement, type ReactNode } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { IntlProvider } from 'react-intl';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ClassicTheme, OxygenUIThemeProvider } from '@wso2/oxygen-ui';

export type RenderWithProvidersOptions = Omit<RenderOptions, 'wrapper'> & {
  /** Initial router entry, for a component that reads or changes the location. */
  route?: string;
  /** Route pattern to mount the component under, for one that reads path params. */
  path?: string;
};

/**
 * Renders a component under the providers it expects to find above it: theming,
 * message formatting, and a router. Returns a userEvent instance alongside the
 * usual render result so a test can drive the component it just mounted.
 */
export function renderWithProviders(
  ui: ReactElement,
  { route = '/', path, ...options }: RenderWithProvidersOptions = {}
) {
  const routed = path ? <Routes><Route path={path} element={ui} /></Routes> : ui;
  const Wrapper = ({ children }: { children: ReactNode }) => (
    <OxygenUIThemeProvider
      themes={[{ key: 'classic', label: 'Classic Theme', theme: ClassicTheme }]}
      initialTheme="classic"
    >
      <IntlProvider locale="en" defaultLocale="en">
        <MemoryRouter
          initialEntries={[route]}
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          {children}
        </MemoryRouter>
      </IntlProvider>
    </OxygenUIThemeProvider>
  );

  return {
    user: userEvent.setup(),
    ...render(routed, { wrapper: Wrapper, ...options }),
  };
}

export { screen, waitFor, within } from '@testing-library/react';
