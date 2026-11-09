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

import { Card, CardContent } from '@wso2/oxygen-ui';
import { Eye } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

import { illustrationCardSx } from './illustrationCardSx';
import './PortalPublishIllustration.css';

/**
 * Decorative portal animation for the Publish page's empty state: a developer
 * portal window appears, goes live, an API card drops into the catalog and is
 * marked published. Colours come from the theme through `--portal-*` variables.
 */
export function PortalPublishIllustration() {
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
            <FormattedMessage
              id="apiControlPlane.components.PortalPublishIllustration.live"
              defaultMessage="Live"
              description="Status on the animated portal preview once the portal is up."
            />
          </span>
        </div>
        <div className="api-portal-body">
          <div className="api-portal-title">
            <FormattedMessage
              id="apiControlPlane.components.PortalPublishIllustration.catalog"
              defaultMessage="API catalog"
              description="Heading of the animated portal preview."
            />
          </div>
          <div className="api-portal-sub">
            <FormattedMessage
              id="apiControlPlane.components.PortalPublishIllustration.catalogHint"
              defaultMessage="Discover and subscribe to APIs"
              description="Subheading of the animated portal preview."
            />
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
                    <FormattedMessage
                      id="apiControlPlane.components.PortalPublishIllustration.untitledApi"
                      defaultMessage="Untitled API"
                      description="Name of the sample API that drops into the animated portal preview."
                    />
                  </span>
                  <span className="api-portal-badge">
                    <FormattedMessage
                      id="apiControlPlane.components.PortalPublishIllustration.published"
                      defaultMessage="Published"
                      description="Badge on the sample API in the animated portal preview."
                    />
                  </span>
                </div>
                <span className="api-portal-meta mono">v1 · REST</span>
              </div>
            </div>
          </div>
          <div className="api-portal-foot">
            <Eye size={12} color="var(--portal-accent)" />
            <FormattedMessage
              id="apiControlPlane.components.PortalPublishIllustration.visible"
              defaultMessage="Visible to developers"
              description="Footer of the animated portal preview."
            />
            <span className="api-portal-count mono">
              <FormattedMessage
                id="apiControlPlane.components.PortalPublishIllustration.count"
                defaultMessage="1 of 1 portals"
                description="Footer count in the animated portal preview."
              />
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
