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
import { KeyRound, SquareTerminal } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

import { illustrationCardSx } from './illustrationCardSx';
import './ServiceAccountsIllustration.css';

/** A sample path and account name, not UI copy, so they are not translated. */
const SAMPLE_PATH = 'service-accounts / new';
const SAMPLE_ACCOUNT = 'ci-pipeline';

/**
 * Decorative service-accounts animation for the Service Accounts empty state: a
 * new account drops into the list, its client ID and secret fill in, it is marked
 * active, and the footer shows the short-lived token it can get. Colours come from
 * the theme through `--sa-*` variables; purely decorative, so it is hidden from
 * assistive technology.
 */
export function ServiceAccountsIllustration() {
  return (
    <Card
      className="api-sa-animation"
      aria-hidden="true"
      sx={(theme) => ({ ...illustrationCardSx('sa', theme), width: 320 })}
    >
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        <div className="api-sa-bar">
          <i />
          <i />
          <i />
          <span className="api-sa-url mono">{SAMPLE_PATH}</span>
        </div>
        <div className="api-sa-body">
          <div className="api-sa-title">
            <FormattedMessage
              id="apiControlPlane.components.ServiceAccountsIllustration.title"
              defaultMessage="Service accounts"
              description="Heading of the animated service accounts preview."
            />
          </div>
          <div className="api-sa-sub">
            <FormattedMessage
              id="apiControlPlane.components.ServiceAccountsIllustration.subtitle"
              defaultMessage="Scripts, pipelines and tools"
              description="Subheading of the animated service accounts preview."
            />
          </div>
          <div className="api-sa-slot">
            <div className="api-sa-slot-empty" />
            <div className="api-sa-card">
              <SquareTerminal size={18} color="var(--sa-accent)" />
              <div className="api-sa-card-main">
                <div className="api-sa-card-top">
                  <span className="api-sa-card-title">{SAMPLE_ACCOUNT}</span>
                  <span className="api-sa-badge">
                    <FormattedMessage
                      id="apiControlPlane.components.ServiceAccountsIllustration.active"
                      defaultMessage="Active"
                      description="Badge on the sample service account in the animated preview."
                    />
                  </span>
                </div>
                <div className="api-sa-kv">
                  <span className="api-sa-k mono">
                    <FormattedMessage
                      id="apiControlPlane.components.ServiceAccountsIllustration.clientId"
                      defaultMessage="client id"
                      description="Label of the sample client ID line in the animated preview."
                    />
                  </span>
                  <span className="api-sa-v v1" />
                </div>
                <div className="api-sa-kv">
                  <span className="api-sa-k mono">
                    <FormattedMessage
                      id="apiControlPlane.components.ServiceAccountsIllustration.secret"
                      defaultMessage="secret"
                      description="Label of the sample client secret line in the animated preview."
                    />
                  </span>
                  <span className="api-sa-v v2" />
                </div>
              </div>
            </div>
          </div>
          <div className="api-sa-foot">
            <KeyRound size={12} color="var(--sa-accent)" />
            <FormattedMessage
              id="apiControlPlane.components.ServiceAccountsIllustration.tokenIssued"
              defaultMessage="Token issued"
              description="Footer of the animated service accounts preview."
            />
            <span className="api-sa-exp mono">
              <FormattedMessage
                id="apiControlPlane.components.ServiceAccountsIllustration.expires"
                defaultMessage="expires in 1h"
                description="Token lifetime shown in the animated service accounts preview."
              />
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
