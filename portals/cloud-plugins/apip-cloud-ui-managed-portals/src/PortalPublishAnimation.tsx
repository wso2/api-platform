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

import { Card, CardContent, type Theme } from '@wso2/oxygen-ui';
import { Eye } from '@wso2/oxygen-ui-icons-react';

import './PortalPublishAnimation.css';

/** Card chrome: theme colours exposed as `--portal-*` CSS variables. */
const illustrationCardSx = (prefix: string, theme: Theme) => {
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


/**
 * Decorative portal animation for the Publish page's empty state: a developer
 * portal window appears, goes live, an API card drops into the catalog and is
 * marked published. Colours come from the theme through `--portal-*` variables.
 */
export function PortalPublishAnimation() {
  return (
    <Card
      className="api-portal-animation"
      aria-hidden="true"
      sx={(theme) => illustrationCardSx('portal', theme)}
    >
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        <div className="api-portal-bar">
          <i />
          <i />
          <i />
          <span className="api-portal-url mono">developer.example.com</span>
          <span className="api-portal-live">
            Live
          </span>
        </div>
        <div className="api-portal-body">
          <div className="api-portal-title">
            API catalog
          </div>
          <div className="api-portal-sub">
            Discover and subscribe to APIs
          </div>
          <div className="api-portal-grid">
            <div className="api-portal-card api-portal-other">
              <span className="api-portal-name">Payments API</span>
              <span className="api-portal-meta mono">v2 · REST</span>
            </div>
            <div className="api-portal-slot">
              <div className="api-portal-slot-empty" />
              <div className="api-portal-card api-portal-api">
                <div className="api-portal-api-top">
                  <span className="api-portal-name">
                    Untitled API
                  </span>
                  <span className="api-portal-badge">
                    Published
                  </span>
                </div>
                <span className="api-portal-meta mono">v1 · REST</span>
              </div>
            </div>
          </div>
          <div className="api-portal-foot">
            <Eye size={12} color="var(--portal-accent)" />
            Visible to developers
            <span className="api-portal-count mono">
              1 of 1 portals
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
