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

import { Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useApiDocumentContent } from '@/api/resources/apiDocuments';
import { MarkdownView } from '@/components/MarkdownView';
import { LoadingState } from '@/components/StateViews';
import { isTextContent } from './documentContent';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentBody.loading',
    defaultMessage: 'Loading content',
  },
  error: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentBody.error',
    defaultMessage: 'Unable to load the content of this document.',
  },
  unsupported: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentBody.unsupported',
    defaultMessage: 'This document’s format can’t be previewed here.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentBody.empty',
    defaultMessage: 'This document has no content.',
  },
});

type DocumentBodyProps = {
  apiHandle: string;
  /** The `{apiType}` path segment of the API that owns the document. */
  apiType: string;
  docId: string;
};

/** A document's body, read-only: its Markdown rendered, or why it can't be shown. */
export function DocumentBody({ apiHandle, apiType, docId }: DocumentBodyProps) {
  const intl = useIntl();
  const contentQuery = useApiDocumentContent(apiType, apiHandle, docId);

  if (contentQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (contentQuery.error) {
    return (
      <Typography color="error" variant="body2">
        <FormattedMessage {...messages.error} />
      </Typography>
    );
  }
  if (!isTextContent(contentQuery.data.contentType)) {
    return (
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.unsupported} />
      </Typography>
    );
  }
  return (
    <MarkdownView
      emptyFallback={
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.empty} />
        </Typography>
      }
      source={contentQuery.data.text}
    />
  );
}
