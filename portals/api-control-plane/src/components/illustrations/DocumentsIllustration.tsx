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
import { Eye, FileText } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

import './DocumentsIllustration.css';
import { illustrationCardSx } from './illustrationCardSx';

/** A sample path, not UI copy, so it is not translated. */
const SAMPLE_PATH = 'docs / untitled-api';

/**
 * Decorative documents animation for the Develop › Documents empty state: a docs
 * window appears, a new guide drops into the list and is marked added, and the
 * footer counts the documents visible to developers. Colours come from the theme
 * through `--docs-*` variables; purely decorative, so it is hidden from assistive
 * technology.
 */
export function DocumentsIllustration() {
  return (
    <Card
      className="api-docs-animation"
      aria-hidden="true"
      sx={(theme) => illustrationCardSx('docs', theme)}
    >
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        <div className="api-docs-bar">
          <i />
          <i />
          <i />
          <span className="api-docs-url mono">{SAMPLE_PATH}</span>
        </div>
        <div className="api-docs-body">
          <div className="api-docs-title">
            <FormattedMessage
              id="apiControlPlane.components.DocumentsIllustration.title"
              defaultMessage="API documents"
              description="Heading of the animated documents preview."
            />
          </div>
          <div className="api-docs-sub">
            <FormattedMessage
              id="apiControlPlane.components.DocumentsIllustration.subtitle"
              defaultMessage="Guides, samples and support"
              description="Subheading of the animated documents preview."
            />
          </div>
          <div className="api-docs-list">
            <div className="api-docs-row">
              <FileText size={16} color="var(--docs-muted)" />
              <div className="api-docs-lines">
                <span className="api-docs-ln" style={{ width: '70%' }} />
                <span className="api-docs-ln dim" style={{ width: '45%' }} />
              </div>
            </div>
            <div className="api-docs-slot">
              <div className="api-docs-slot-empty" />
              <div className="api-docs-card">
                <FileText size={18} color="var(--docs-accent)" />
                <div className="api-docs-lines">
                  <div className="api-docs-card-top">
                    <span className="api-docs-card-title">
                      <FormattedMessage
                        id="apiControlPlane.components.DocumentsIllustration.sampleTitle"
                        defaultMessage="Getting started"
                        description="Name of the sample document that drops into the animated documents preview."
                      />
                    </span>
                    <span className="api-docs-badge">
                      <FormattedMessage
                        id="apiControlPlane.components.DocumentsIllustration.added"
                        defaultMessage="Added"
                        description="Badge on the sample document in the animated documents preview."
                      />
                    </span>
                  </div>
                  <span className="api-docs-grow l1" />
                  <span className="api-docs-grow l2" />
                </div>
              </div>
            </div>
          </div>
          <div className="api-docs-foot">
            <Eye size={12} color="var(--docs-accent)" />
            <FormattedMessage
              id="apiControlPlane.components.DocumentsIllustration.visible"
              defaultMessage="Visible to developers"
              description="Footer of the animated documents preview."
            />
            <span className="api-docs-count mono">
              <FormattedMessage
                id="apiControlPlane.components.DocumentsIllustration.count"
                defaultMessage="2 documents"
                description="Footer count in the animated documents preview."
              />
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
