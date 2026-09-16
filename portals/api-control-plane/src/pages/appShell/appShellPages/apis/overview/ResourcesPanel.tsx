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

import { useMemo } from 'react';
import yaml from 'js-yaml';
import { Box, Card, Divider, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ApiError } from '@/api/core/errors';
import type { RestApi } from '@/api/resources/restApis';
import { useRestApiOpenApi } from '@/api/resources/restApis';
import SwaggerSpecViewer from '@/components/SwaggerSpecViewer';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { ResourcePreviewPlaceholder } from '../components/ResourcePreviewPlaceholder';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.loading',
    defaultMessage: 'Loading API definition',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.loadError',
    defaultMessage: 'Unable to load the API definition.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.empty',
    defaultMessage: 'No available resources.',
    description: 'Shown in place of the operation list when the API definition exposes none.',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.emptyDescription',
    defaultMessage: 'This API’s definition does not expose any operations yet.',
    description:
      'Sits under "No available resources." and explains why the list is empty — the definition itself has no operations, as opposed to anything having failed.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.title',
    defaultMessage: 'Resources',
    description:
      "Heading of the panel listing the API's operations (its OpenAPI paths). A noun, not a command.",
  },
});

function parseSpecContent(content: string): Record<string, unknown> | null {
  try {
    const parsed = yaml.load(content);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    // Not valid YAML/JSON.
  }
  return null;
}

/**
 * Left panel of the Overview tab: the full API definition loaded from
 * GET /openapi, rendered by the shared spec viewer in a scrollable bordered
 * box. Schemas (components/definitions) from the stored spec are visible at
 * depth 1. When no spec has been uploaded yet the placeholder is shown instead.
 */
export function ResourcesPanel({ api }: { api: RestApi }) {
  const intl = useIntl();
  const openApiQuery = useRestApiOpenApi(api.id);
  const openApiError = openApiQuery.error as ApiError | null;

  const spec = useMemo(
    () => (openApiQuery.data?.content ? parseSpecContent(openApiQuery.data.content) : null),
    [openApiQuery.data?.content],
  );

  if (openApiQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  if (openApiError && openApiError.status !== 404) {
    return <ErrorState title={intl.formatMessage(messages.loadError)} />;
  }

  if (!spec) {
    return (
      <ResourcePreviewPlaceholder
        description={intl.formatMessage(messages.emptyDescription)}
        testId="resources-panel-empty"
        title={intl.formatMessage(messages.empty)}
      />
    );
  }

  return (
    <Box sx={{ minWidth: 0 }}>
      <Card
        sx={{
          '& .swagger-ui': { bgcolor: 'transparent' },
        }}
      >
        <Box sx={{ px: 2, py: 1.5 }}>
          <Typography sx={{ fontWeight: 600 }} variant="h6">
            <FormattedMessage {...messages.title} />
          </Typography>
        </Box>
        <Divider />
        <Box sx={{ maxHeight: { md: 720, xs: 420 }, overflowY: 'auto', px: 2, py: 1 }}>
          <SwaggerSpecViewer
            defaultModelsExpandDepth={1}
            disableTryOutBtn
            displayRequestDuration={false}
            enableResourceSearch
            hideAuthorizeButton
            hideInfoSection
            hideServers
            hideTagHeaders
            spec={spec}
          />
        </Box>
      </Card>
    </Box>
  );
}
