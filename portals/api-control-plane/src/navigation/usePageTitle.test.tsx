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

import { describe, expect, it } from 'vitest';

import { useDocumentTitle } from '../hooks/useDocumentTitle';
import { makeConsoleScope } from '../test/mockScope';
import { aRestApi } from '../test/msw';
import { renderWithProviders } from '../test/utils';
import { usePageTitle } from './usePageTitle';

/** The pair as `AppLayout` wires it, so the assertions read the real tab title. */
function TitleProbe() {
  useDocumentTitle(usePageTitle());
  return null;
}

const ORG = 'acme-org';
const PROJECT = 'retail';
const API = 'pizza-shack';

const titleAt = (route: string, scope = makeConsoleScope()) => {
  renderWithProviders(<TitleProbe />, { route, scope });
  return document.title;
};

describe('usePageTitle', () => {
  it('names a page from its sidebar item', () => {
    expect(titleAt(`/organizations/${ORG}/projects/${PROJECT}/apis/${API}/deploy`)).toBe(
      'Deploy | WSO2 API Platform',
    );
  });

  it('prefers a submenu child over the parent that also matches', () => {
    // `kind: 'RestApi'` is what makes `canDevelop` true, which is what puts the
    // Develop submenu (and so its children) in the sidebar at all.
    const scope = makeConsoleScope({ component: aRestApi({ id: API, kind: 'RestApi' }) });
    expect(
      titleAt(`/organizations/${ORG}/projects/${PROJECT}/apis/${API}/develop/policies`, scope),
    ).toBe('Policies | WSO2 API Platform');
  });

  it('names a page the sidebar does not list', () => {
    expect(titleAt(`/organizations/${ORG}/projects/${PROJECT}/apis`)).toBe(
      'APIs | WSO2 API Platform',
    );
  });

  // `/apis/new` also matches Overview's `.../apis/:apiHandler` pattern, so the
  // explicit route title has to win or the wizard reads "Overview".
  it('names the create wizard, not the Overview item whose pattern it matches', () => {
    expect(titleAt(`/organizations/${ORG}/projects/${PROJECT}/apis/new`)).toBe(
      'Create API | WSO2 API Platform',
    );
  });

  it('names a Settings tab below the bare /settings path', () => {
    expect(titleAt(`/organizations/${ORG}/settings/general`)).toBe('Settings | WSO2 API Platform');
  });

  it('falls back to the bare product name when nothing names the page', () => {
    expect(titleAt(`/organizations/${ORG}/nothing-here`)).toBe('WSO2 API Platform');
  });
});
