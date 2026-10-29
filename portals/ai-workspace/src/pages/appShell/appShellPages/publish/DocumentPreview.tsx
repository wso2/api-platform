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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Typography } from '@wso2/oxygen-ui';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { MarkdownView } from '../../../../Components/common/MarkdownView';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { logger } from '../../../../utils/logger';
import type { MCPProxyDocumentContent } from '../../../../utils/types';
import { LoadingState } from './PublishStates';

const messages = defineMessages({
  loading: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documentPreview.loading',
    defaultMessage: 'Loading content',
  },
  error: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documentPreview.error',
    defaultMessage: 'Unable to load the content of this document.',
  },
  unsupported: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documentPreview.unsupported',
    defaultMessage: 'This document’s format can’t be previewed here.',
  },
  empty: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documentPreview.empty',
    defaultMessage: 'This document has no content.',
  },
});

/** The content endpoint returns the stored bytes as stored, so only text is rendered; an empty body has no type. */
const isTextContent = (contentType: string): boolean => {
  const mediaType = contentType.split(';')[0].trim().toLowerCase();
  return ['', 'text/markdown', 'text/x-markdown', 'text/plain'].includes(mediaType);
};

type DocumentPreviewProps = {
  docId: string;
  mcpProxyId: string;
};

/** A document's body, read-only: its Markdown rendered, or why it can't be shown. */
export function DocumentPreview({ docId, mcpProxyId }: DocumentPreviewProps) {
  const intl = useIntl();
  const [content, setContent] = useState<MCPProxyDocumentContent>();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setContent(undefined);
    setFailed(false);
    let cancelled = false;
    void (async () => {
      try {
        const body = await mcpProxiesApis.getMcpProxyDocumentContent(mcpProxyId, docId, PLATFORM_API_BASE_URL);
        if (!cancelled) setContent(body);
      } catch (error) {
        logger.error('Failed to load the document content:', error);
        if (!cancelled) setFailed(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [docId, mcpProxyId]);

  if (failed) {
    return (
      <Typography color="error" variant="body2">
        <FormattedMessage {...messages.error} />
      </Typography>
    );
  }
  if (!content) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!isTextContent(content.contentType)) {
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
      source={content.text}
    />
  );
}
