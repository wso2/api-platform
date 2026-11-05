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

import { illustrationCardSx } from './illustrationCardSx';
import './MeterIllustration.css';

/** Decorative meter animation for the Meters empty state: requests are counted, failures are skipped. */
export function MeterIllustration() {
  return (
    <Card
      className="api-meter-animation"
      aria-hidden="true"
      sx={(theme) => illustrationCardSx('meter', theme)}
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
        <div className="api-meter-head">
          <span className="api-meter-icon">
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--meter-accent)"
              strokeWidth="2.2"
              strokeLinecap="round"
            >
              <path d="M4 20a8 8 0 1 1 16 0M12 20l4-6" />
            </svg>
          </span>
          <span className="api-meter-name mono">successful_requests</span>
          <div className="api-meter-odo">
            <div className="api-meter-odo-inner">
              <span>0</span>
              <span>1</span>
              <span>2</span>
              <span>3</span>
            </div>
          </div>
        </div>
        <div className="api-meter-rows mono">
          <div className="api-meter-row api-meter-row-first">
            <span className="api-meter-code api-meter-ok">200</span>
            <span className="api-meter-path">GET /orders</span>
            <span className="api-meter-tag api-meter-plus">+1</span>
          </div>
          <div className="api-meter-row api-meter-row-second">
            <span className="api-meter-code api-meter-ok">200</span>
            <span className="api-meter-path">POST /orders</span>
            <span className="api-meter-tag api-meter-plus">+1</span>
          </div>
          <div className="api-meter-row api-meter-row-third">
            <span className="api-meter-code api-meter-err">500</span>
            <span className="api-meter-path">{'GET /orders/{id}'}</span>
            <span className="api-meter-tag api-meter-skip">
              <FormattedMessage
                id="apiControlPlane.components.MeterIllustration.skipped"
                defaultMessage="skipped"
              />
            </span>
          </div>
          <div className="api-meter-row api-meter-row-fourth">
            <span className="api-meter-code api-meter-ok">201</span>
            <span className="api-meter-path">POST /orders</span>
            <span className="api-meter-tag api-meter-plus">+1</span>
          </div>
        </div>
        <div className="api-meter-foot">
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="var(--meter-accent)"
            strokeWidth="2.2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M20.6 13.4l-7.2 7.2a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8zM7.5 7.5h.01" />
          </svg>
          <FormattedMessage
            id="apiControlPlane.components.MeterIllustration.charged"
            defaultMessage="Charged in Gold plan"
          />
          <span className="api-meter-price mono">$0.002 / req</span>
        </div>
      </CardContent>
    </Card>
  );
}
