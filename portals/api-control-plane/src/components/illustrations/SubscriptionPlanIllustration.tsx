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
import './SubscriptionPlanIllustration.css';

/** Decorative plan animation for the Subscription Plans empty state: a quota fills, the limit is hit, the plan is enabled on APIs. */
export function SubscriptionPlanIllustration() {
  return (
    <Card
      className="api-plan-animation"
      aria-hidden="true"
      sx={(theme) => illustrationCardSx('plan', theme)}
    >
      <CardContent
        sx={{
          display: 'flex',
          flexDirection: 'column',
          gap: 1.5,
          p: 1.5,
          '&:last-child': { pb: 1.5 },
        }}
      >
        <div className="api-plan-head">
          <span className="api-plan-icon">
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--plan-accent)"
              strokeWidth="2.2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M12 2l3 6.5 7 .8-5.2 4.8 1.4 7L12 17.6 5.8 21.1l1.4-7L2 9.3l7-.8z" />
            </svg>
          </span>
          <span className="api-plan-name">Gold</span>
          <span className="api-plan-rate mono">100 req / min</span>
        </div>
        <div className="api-plan-usage">
          <div className="api-plan-usage-top">
            <span>
              <FormattedMessage
                id="apiControlPlane.components.SubscriptionPlanIllustration.usage"
                defaultMessage="Mobile app · this minute"
              />
            </span>
            <span className="api-plan-count mono">
              <span className="api-plan-odo">
                <span className="api-plan-odo-inner">
                  <span>0</span>
                  <span>25</span>
                  <span>50</span>
                  <span>75</span>
                  <span>100</span>
                </span>
              </span>{' '}
              / 100
            </span>
          </div>
          <div className="api-plan-bar">
            <div className="api-plan-fill"></div>
            <div className="api-plan-ticks">
              {Array.from({ length: 9 }, (_, index) => (
                <span key={index}></span>
              ))}
            </div>
          </div>
        </div>
        <div className="api-plan-slot">
          <div className="api-plan-blocked mono">
            <span className="api-plan-code">429</span>
            <span className="api-plan-msg">
              <FormattedMessage
                id="apiControlPlane.components.SubscriptionPlanIllustration.tooManyRequests"
                defaultMessage="Too many requests"
              />
            </span>
          </div>
        </div>
        <div className="api-plan-foot">
          <span>
            <FormattedMessage
              id="apiControlPlane.components.SubscriptionPlanIllustration.enabledOn"
              defaultMessage="Enabled on"
            />
          </span>
          <span className="api-plan-chip api-plan-chip-first">Orders API</span>
          <span className="api-plan-chip api-plan-chip-second">Payments API</span>
        </div>
      </CardContent>
    </Card>
  );
}
