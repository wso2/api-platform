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

import { Box, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, type MessageDescriptor } from 'react-intl';

import type { SpecIssue, SpecIssueCode } from '../utils/specValidation';

const messages = defineMessages({
  badPathKeys: {
    id: 'api.create.specIssue.badPathKeys',
    defaultMessage: 'Some entries under paths are not paths and were skipped: {detail}',
  },
  externalRefs: {
    id: 'api.create.specIssue.externalRefs',
    defaultMessage: 'This definition references files outside itself, which are not read: {detail}',
  },
  missingTitle: {
    id: 'api.create.specIssue.missingTitle',
    defaultMessage: 'No title in this definition — name the API on the next step.',
  },
  missingVersion: {
    id: 'api.create.specIssue.missingVersion',
    defaultMessage: 'No version in this definition — set one on the next step.',
  },
  noOperations: {
    id: 'api.create.specIssue.noOperations',
    defaultMessage: 'This definition declares no GET, POST, PUT, PATCH or DELETE operation.',
  },
  noPaths: {
    id: 'api.create.specIssue.noPaths',
    defaultMessage: 'This definition has no paths section.',
  },
  noServers: {
    id: 'api.create.specIssue.noServers',
    defaultMessage: 'No server URL in this definition — set the backend on the next step.',
  },
  notASpec: {
    id: 'api.create.specIssue.notASpec',
    defaultMessage:
      'That file is not an OpenAPI definition: it names neither an openapi nor a swagger version.',
  },
  unsupportedDialect: {
    id: 'api.create.specIssue.unsupportedDialect',
    defaultMessage:
      'OpenAPI {detail} is not supported. Import a Swagger 2.0 or OpenAPI 3 definition.',
  },
});

const MESSAGE_FOR: Record<SpecIssueCode, MessageDescriptor> = {
  badPathKeys: messages.badPathKeys,
  externalRefs: messages.externalRefs,
  missingTitle: messages.missingTitle,
  missingVersion: messages.missingVersion,
  noOperations: messages.noOperations,
  noPaths: messages.noPaths,
  noServers: messages.noServers,
  notASpec: messages.notASpec,
  unsupportedDialect: messages.unsupportedDialect,
};

export type SpecIssueListProps = {
  issues: SpecIssue[];
};

/**
 * The validator's findings as sentences.
 *
 * One list, used by every surface that reports on a definition, so a code is
 * translated in exactly one place. The issue's own `detail` is a fragment of
 * the document — it rides in as an ICU value rather than being translated.
 */
export const SpecIssueList = ({ issues }: SpecIssueListProps) => {
  if (issues.length === 0) {
    return null;
  }

  if (issues.length === 1) {
    return (
      <FormattedMessage
        {...MESSAGE_FOR[issues[0].code]}
        values={{ detail: issues[0].detail ?? '' }}
      />
    );
  }

  return (
    <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
      {issues.map((issue) => (
        <Typography component="li" key={issue.code} variant="body2">
          <FormattedMessage {...MESSAGE_FOR[issue.code]} values={{ detail: issue.detail ?? '' }} />
        </Typography>
      ))}
    </Box>
  );
};
