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

import { render } from '@testing-library/react';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import { useEffect } from 'react';
import { beforeEach, describe, expect, it } from 'vitest';
import TrackingGate from './TrackingGate';
import { resetTrackingForTests } from './loader';

const win = window as unknown as { __RUNTIME_CONFIG__?: Record<string, unknown>; dataLayer?: Record<string, unknown>[] };
const patterns = [
  '/organizations/:orgHandle/home',
  '/organizations/:orgHandle/gateways',
  '/organizations/:orgHandle/landing',
];

function Go({ to }: { to: string }) {
  const navigate = useNavigate();
  useEffect(() => {
    navigate(to);
  }, [navigate, to]);
  return null;
}

describe('TrackingGate', () => {
  beforeEach(() => {
    resetTrackingForTests();
    document.head.innerHTML = '';
    win.dataLayer = [];
  });

  it('does nothing when the deployment configures nothing', () => {
    win.__RUNTIME_CONFIG__ = {};
    render(<MemoryRouter initialEntries={['/organizations/acme/home']}><TrackingGate routePatterns={patterns} /></MemoryRouter>);
    expect(document.head.querySelectorAll('script')).toHaveLength(0);
    expect(win.dataLayer).toHaveLength(0);
  });

  it('loads the scripts and pushes one pattern view per route change', () => {
    win.__RUNTIME_CONFIG__ = {
      cookieProDomainScript: '01a1a5f4-f908-7fef-aa9a-e3d16219021b',
      gtmContainerId: 'GTM-ABC123',
    };
    render(
      <MemoryRouter initialEntries={['/organizations/acme/home']}>
        <TrackingGate routePatterns={patterns} />
        <Go to="/organizations/acme/gateways" />
      </MemoryRouter>
    );
    expect(document.head.querySelectorAll('script')).toHaveLength(2);
    const views = win.dataLayer!.filter((e) => e.event === 'virtual_page_view').map((e) => e.page_path);
    expect(views).toEqual(['/organizations/:orgHandle/home', '/organizations/:orgHandle/gateways']);
  });

  it('does not count redirect-only routes as pages', () => {
    win.__RUNTIME_CONFIG__ = {
      cookieProDomainScript: '01a1a5f4-f908-7fef-aa9a-e3d16219021b',
      gtmContainerId: 'GTM-ABC123',
    };
    render(
      <MemoryRouter initialEntries={['/organizations/acme/landing']}>
        <TrackingGate routePatterns={patterns} nonPagePatterns={['/organizations/:orgHandle/landing']} />
        <Go to="/organizations/acme/home" />
      </MemoryRouter>
    );
    const views = win.dataLayer!.filter((e) => e.event === 'virtual_page_view').map((e) => e.page_path);
    expect(views).toEqual(['/organizations/:orgHandle/home']);
  });
});
