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
import { FormattedMessage } from 'react-intl';

import './ImportApiIllustration.css';

/** Decorative import animation supplied for the API listing empty state. */
export function ImportApiIllustration() {
  return (
    <Card
      className="api-import-animation"
      aria-hidden="true"
      sx={(theme) => {
        const palette = theme.vars?.palette ?? theme.palette;
        return {
          width: 280,
          maxWidth: '100%',
          '--import-accent': palette.primary.dark,
          '--import-accent-base': palette.primary.main,
          '--import-success': palette.success.dark,
          '--import-success-base': palette.success.main,
          '--import-input-bg': palette.background.paper,
          '--import-border': palette.divider,
          '--import-muted': palette.text.secondary,
          '--import-cursor': palette.text.primary,
          '--import-cursor-outline': palette.background.paper,
          ...theme.applyStyles('light', {
            backgroundColor: `color-mix(in srgb, ${palette.primary.main} 12%, ${palette.background.paper})`,
            backgroundImage: `linear-gradient(145deg, ${palette.background.paper} 0%, color-mix(in srgb, ${palette.primary.main} 8%, ${palette.background.paper}) 45%, color-mix(in srgb, ${palette.primary.main} 16%, ${palette.background.paper}) 100%)`,
            border: '1px solid',
            borderColor: `color-mix(in srgb, ${palette.primary.main} 40%, ${palette.background.paper})`,
            boxShadow: `inset 0 1px 0 ${palette.background.paper}, inset 0 -1px 2px color-mix(in srgb, ${palette.primary.main} 10%, transparent), 0 8px 20px color-mix(in srgb, ${palette.primary.dark} 12%, transparent)`,
          }),
          ...theme.applyStyles('dark', {
            '--import-accent': palette.primary.light,
            '--import-success': palette.success.light,
            '--import-input-bg': 'rgba(0, 0, 0, 0.25)',
          }),
        };
      }}
    >
      <CardContent
        sx={{
          display: 'flex',
          flexDirection: 'column',
          gap: 1.25,
          p: 1.5,
          '&:last-child': { pb: 1.5 },
        }}
      >
        <div className="api-import-label">
          <FormattedMessage
            id="apiControlPlane.components.ImportApiIllustration.backendUrl"
            defaultMessage="Backend URL"
          />
        </div>
        <div className="api-import-bar">
          <div className="api-import-input mono">
            <span className="api-import-proto">https://</span>
            <span className="api-import-typed">orders.internal:8080</span>
            <span className="api-import-caret"></span>
          </div>
          <div className="api-import-btn-wrap">
            <span className="api-import-ring"></span>
            <div className="api-import-btn">
              <svg
                width="15"
                height="15"
                viewBox="0 0 24 24"
                fill="none"
                stroke="var(--import-accent)"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M5 12h14M13 6l6 6-6 6" />
              </svg>
            </div>
            <svg className="api-import-cursor" width="14" height="14" viewBox="0 0 24 24">
              <path
                d="M4 3l15 7-6.5 2L10 19z"
                fill="var(--import-cursor)"
                stroke="var(--import-cursor-outline)"
                strokeWidth="1.2"
                strokeLinejoin="round"
              />
            </svg>
          </div>
        </div>
        <div className="api-import-panel">
          <span className="api-import-scan"></span>
          <div className="api-import-head">
            <span className="api-import-title">
              <FormattedMessage
                id="apiControlPlane.components.ImportApiIllustration.endpoints"
                defaultMessage="Discovered endpoints"
              />
            </span>
            <span className="api-import-ready">
              <FormattedMessage
                id="apiControlPlane.components.ImportApiIllustration.ready"
                defaultMessage="API ready"
              />
            </span>
          </div>
          <div className="api-import-row api-import-row-first">
            <span className="api-import-m api-import-get mono">GET</span>
            <span className="api-import-path mono">/orders</span>
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--import-success)"
              strokeWidth="2.6"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M5 12l5 5 9-10" />
            </svg>
          </div>
          <div className="api-import-row api-import-row-second">
            <span className="api-import-m api-import-post mono">POST</span>
            <span className="api-import-path mono">/orders</span>
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--import-success)"
              strokeWidth="2.6"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M5 12l5 5 9-10" />
            </svg>
          </div>
          <div className="api-import-row api-import-row-third">
            <span className="api-import-m api-import-get mono">GET</span>
            <span className="api-import-path mono">{'/orders/{id}'}</span>
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--import-success)"
              strokeWidth="2.6"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M5 12l5 5 9-10" />
            </svg>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
