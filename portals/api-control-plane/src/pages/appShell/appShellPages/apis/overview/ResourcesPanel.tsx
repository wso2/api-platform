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
import { Box, Card, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { RestApi } from '@/api/resources/restApis';
import SwaggerSpecViewer from '@/components/SwaggerSpecViewer';
import { ResourcePreviewPlaceholder } from '../components/ResourcePreviewPlaceholder';
import { restApiToOpenApiSpec } from '../utils/operationsToSpec';

const messages = defineMessages({
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.empty',
    defaultMessage: 'No available resources.',
    description: 'Shown in place of the operation list when the API definition exposes none.',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.emptyDescription',
    defaultMessage: 'This API’s definition does not expose any operations yet.',
    description:
      'Sits under “No available resources.” and explains why the list is empty — the definition itself has no operations, as opposed to anything having failed.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ResourcesPanel.title',
    defaultMessage: 'Resources',
    description:
      "Heading of the panel listing the API's operations (its OpenAPI paths). A noun, not a command.",
  },
});

/**
 * Left panel of the Overview tab (ai-workspace "OpenAPI Resources"): the API's
 * operations rendered by the shared spec viewer in a scrollable bordered box,
 * or — when the definition exposes none — a placeholder showing the shape the
 * list would take.
 *
 * The document the viewer draws is rebuilt from `api.operations`, not fetched:
 * the platform never stores the definition an API was created from, so method,
 * path and summary are the whole of what there is to show. See
 * `utils/operationsToSpec`.
 */
export function ResourcesPanel({ api }: { api: RestApi }) {
  const intl = useIntl();
  // `operations` is optional on the spec's `RESTAPI`.
  const operations = api.operations ?? [];
  const spec = useMemo(() => restApiToOpenApiSpec(api), [api]);

  return (
    <Box sx={{ flex: 1, minWidth: 0 }}>
      <Typography sx={{ fontWeight: 600, mb: 0.5 }} variant="h6">
        <FormattedMessage {...messages.title} />
      </Typography>
      {operations.length === 0 ? (
        // The placeholder brings its own bordered surface, so it stands in for
        // the scroll box rather than sitting inside it — a hairline drawn
        // inside a hairline reads as a mistake rather than a frame.
        <ResourcePreviewPlaceholder
          description={intl.formatMessage(messages.emptyDescription)}
          testId="resources-panel-empty"
          title={intl.formatMessage(messages.empty)}
        />
      ) : (
        <Card
          sx={{
            maxHeight: { md: 520, xs: 320 },
            overflowY: 'auto',
            px: 2,
            py: 1,
            // Swagger UI ships its own canvas; keep it from fighting the
            // panel's surface.
            '& .swagger-ui': { bgcolor: 'transparent' },
          }}
        >
          {/* Read-only, and stripped down to what a rebuilt document can
              honestly show. The info block would repeat the page header; the
              servers/authorize strip and try-it-out belong to a console, which
              this panel is not; the responses section would be an empty table,
              because the platform kept no responses to put in it. Operations
              carry no tags either, so the lone "default" group header is
              hidden and the operations read as one flat list. */}
          <SwaggerSpecViewer
            disableResponseSection
            disableTryOutBtn
            displayRequestDuration={false}
            enableResourceSearch
            hideAuthorizeButton
            hideInfoSection
            hideServers
            hideTagHeaders
            spec={spec}
          />
        </Card>
      )}
    </Box>
  );
}
