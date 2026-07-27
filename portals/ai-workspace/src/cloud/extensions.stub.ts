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
 * Cloud extension seam for the AI Workspace.
 *
 * The app imports `cloudExtensions` from `@cloud-extensions`. In the OSS/standalone
 * build that alias resolves to THIS file — a no-op default that contributes nothing,
 * so the standalone app renders exactly as before. A cloud build points the alias at
 * a real module (via the APIP_AIW_CLOUD_EXTENSIONS build var) that returns populated
 * `routes` and `navItems`. TypeScript always type-checks against the interface here,
 * so any real module must satisfy this contract.
 */
import type { ComponentType, ReactNode } from 'react';

/** Where a contributed route/nav item is mounted. */
export type CloudScope = 'org' | 'project' | 'both';

/**
 * A route contributed into the app's route tree. `path` is relative to the org or
 * project scope root (e.g. `widgets/*`), mirroring the existing `gateways/*`
 * splat: the element owns its own nested `<Routes>`.
 */
export interface CloudRoute {
  path: string;
  element: ReactNode;
  scope: CloudScope;
}

/**
 * A sidebar nav item. `id` MUST equal the first segment of `path` (e.g. id
 * `widgets` for path `/widgets`) so the shell's active-item sync can match
 * the current URL to this item.
 */
export interface CloudNavItem {
  id: string;
  label: string;
  icon: ComponentType<{ size?: number }>;
  path: string;
  scope: CloudScope;
  category?: string;
  requiredScope?: string;
}

/** Props passed to a cloud-contributed field in the Add Gateway form. */
export interface GatewayCreateFieldProps {
  /** Current field value (opaque to the host). */
  value: string;
  /** Update the field value. */
  onChange: (value: string) => void;
}

/** Context handed to the persist hook after a gateway is created. */
export interface GatewayCreatedContext {
  orgId: string;
  gatewayId: string;
  /** The value collected by the contributed field. */
  value: string;
}

/**
 * A field a cloud module contributes to the Add Gateway form. The host renders
 * `Field` inside its form and, after the gateway is created, calls
 * `onGatewayCreated` so the module can persist any association tied to the new
 * gateway. Absent in the standalone build, so the form is unchanged there.
 */
export interface GatewayCreateExtension {
  Field: ComponentType<GatewayCreateFieldProps>;
  /** When true, the host blocks submission until the field has a value. */
  required?: boolean;
  onGatewayCreated: (ctx: GatewayCreatedContext) => void | Promise<void>;
}

/** The whole contribution surface a cloud module returns. */
export interface CloudExtensions {
  routes: CloudRoute[];
  navItems: CloudNavItem[];
  /** Optional provider(s) wrapped around the authenticated app shell. */
  Providers?: ComponentType<{ children: ReactNode }>;
  /** Optional field injected into the Add Gateway form. */
  gatewayCreate?: GatewayCreateExtension;
}

export const cloudExtensions: CloudExtensions = {
  routes: [],
  navItems: [],
};
