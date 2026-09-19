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

import { lazy, Suspense } from 'react';
import { Alert, Box, Stack } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { LoadingState } from '@/components/StateViews';
import { hairline } from '@/theme/receipes';

/** Same lazy split as `SpecSourceEditor` — Monaco is the heaviest thing this app loads. */
const SpecCodeEditor = lazy(() =>
  import('../../apis/create/components/SpecCodeEditor').then((module) => ({
    default: module.SpecCodeEditor,
  })),
);

const messages = defineMessages({
  editorLoading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.editorLoading',
    defaultMessage: 'Loading editor',
  },
  malformed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.malformed',
    defaultMessage: 'This is not valid JSON: {detail}',
    description: '{detail} is the parser’s own message naming the line.',
  },
});

export type SpecificationTabProps = {
  disabled?: boolean;
  /** The parser's own complaint, when the current buffer doesn't read as JSON. */
  parseError?: string;
  /** Every keystroke, with the buffer's full text. */
  onChange: (text: string) => void;
  /** The definition's raw JSON text. */
  text: string;
};

/**
 * "Specification" — the draft's OpenAPI definition (`.../draft/definition`),
 * as raw JSON text. Alpha scope only supports JSON, not YAML/GraphQL/XML: this
 * console only publishes REST APIs today (see `ApiPortalPublicationsList`),
 * and REST_Design.md's own definition format table lists JSON/YAML as the
 * pair — narrowing to one keeps this tab a plain editor instead of needing
 * `SpecSourceEditor`'s full format-switch/validate-against-creation-rules
 * machinery, which is built for a different job (importing a *new* API).
 */
export function SpecificationTab({ disabled, parseError, onChange, text }: SpecificationTabProps) {
  const intl = useIntl();

  return (
    <Stack spacing={1.5}>
      {parseError && (
        <Alert severity="error">
          <FormattedMessage {...messages.malformed} values={{ detail: parseError }} />
        </Alert>
      )}
      <Box
        sx={(theme) => ({
          border: hairline(theme),
          borderColor: 'divider',
          borderRadius: 1,
          height: 'clamp(480px, calc(100vh - 380px), 800px)',
          overflow: 'hidden',
        })}
      >
        <Suspense fallback={<LoadingState label={intl.formatMessage(messages.editorLoading)} />}>
          <SpecCodeEditor format="json" onChange={onChange} readOnly={disabled} value={text} />
        </Suspense>
      </Box>
    </Stack>
  );
}
