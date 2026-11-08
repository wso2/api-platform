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

import { AppSwitcher } from "@wso2/oxygen-ui";
import { House, Settings } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

import {
  CURRENT_PLATFORM_ID,
  resolveManageLinks,
  resolvePlatforms,
  type CloudShellConfig,
} from "./platforms";

const MANAGE_ICONS: Record<string, ReactNode> = {
  home: <House size={18} />,
  settings: <Settings size={18} />,
};

export type AppSwitcherButtonProps = {
  /** The deployment's `cloudShell` runtime config. */
  cloudShell?: CloudShellConfig;
  /** The WSO2 Cloud console's origin, for the Manage links. */
  cloudConsoleUrl?: string;
};

/**
 * The header's platform switcher: a grid trigger opening a popover of WSO2
 * Cloud platforms, with a Manage section (Cloud home, Settings).
 */
export default function AppSwitcherButton({
  cloudShell,
  cloudConsoleUrl,
}: AppSwitcherButtonProps) {
  const platforms = resolvePlatforms(cloudShell, window.location.origin);
  const manage = resolveManageLinks(cloudConsoleUrl);

  return (
    <AppSwitcher currentAppId={CURRENT_PLATFORM_ID}>
      <AppSwitcher.Trigger />
      <AppSwitcher.Section label="Categories">
        {platforms.map((p) => (
          <AppSwitcher.App
            key={p.id}
            id={p.id}
            name={p.name}
            url={p.url}
            disabled={p.disabled}
            tooltip={p.disabled ? "Coming soon" : undefined}
          />
        ))}
      </AppSwitcher.Section>
      {manage.length > 0 && (
        <AppSwitcher.Footer label="Manage">
          {manage.map((l) => (
            <AppSwitcher.App
              key={l.key}
              name={l.name}
              url={l.url}
              icon={MANAGE_ICONS[l.key]}
            />
          ))}
        </AppSwitcher.Footer>
      )}
    </AppSwitcher>
  );
}
