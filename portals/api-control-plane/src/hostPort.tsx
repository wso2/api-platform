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
 *
 * The "Port": the small set of host capabilities an extension's `render`
 * function receives, so a feature component never imports this portal's
 * own hooks (`useConsoleScope`, `useNotifications`) directly — that's what
 * would make it impossible to reuse the same component in another host.
 *
 * This is a real React context, but — unlike `slots/index.tsx` — it is NOT
 * meant to be imported across the api-platform/apim-saas repo boundary.
 * apim-saas's host file (e.g. `hosts/api-control-plane.tsx`) hand-mirrors
 * this exact `CloudHostPort` type (same convention as `ApiControlPlaneExtension`)
 * and receives a value of that shape as a plain function argument via
 * `extension.render(port)` — never by importing `PortContext`/`usePort`
 * itself. `PortProvider` is built and mounted once, here in core, from real
 * hooks; only the small type crosses the boundary, never the context object.
 */

import { createContext, useContext, type ReactNode } from 'react';

export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
};

const PortContext = createContext<CloudHostPort | null>(null);

export function PortProvider({
  value,
  children,
}: {
  value: CloudHostPort;
  children: ReactNode;
}) {
  return <PortContext.Provider value={value}>{children}</PortContext.Provider>;
}

export function usePort(): CloudHostPort {
  const port = useContext(PortContext);
  if (!port) {
    throw new Error('usePort must be used within a PortProvider');
  }
  return port;
}
