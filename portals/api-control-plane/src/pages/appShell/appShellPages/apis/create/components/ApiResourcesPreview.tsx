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

import { Box, CodeBlock, FormControlLabel, Stack, Switch, Typography } from '@wso2/oxygen-ui';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

import { ResourcePreviewPlaceholder } from '../../components/ResourcePreviewPlaceholder';

const messages = defineMessages({
  source: {
    id: 'api.create.apiResourcesPreview.source',
    defaultMessage: 'Source',
    description: 'Toggle that swaps the rendered resources for the definition’s own text.',
  },
  title: {
    id: 'api.create.apiResourcesPreview.title',
    defaultMessage: 'API resources',
  },
});

/**
 * How tall the pane is allowed to get. Bounded rather than content-sized: a
 * definition with fifty operations would otherwise run far past the form beside
 * it and take the whole page's scrollbar with it. The clamp keeps it usable on
 * a laptop screen without leaving a stubby box on a tall one.
 */
const PANE_HEIGHT = 'clamp(420px, calc(100vh - 260px), 560px)';

/** Hide Swagger UI chrome we don't use (top bar and info). */
const HideTopBarPlugin = () => ({
  components: { Topbar: () => null },
});

const HideInfoPlugin = () => ({
  components: { info: () => null },
});

export type ApiResourcesPreviewProps = {
  /**
   * The fetched definition, as a parsed object rather than a URL, so Swagger UI
   * never re-downloads the document and the Source view prints the same object
   * the resources are drawn from. It is not a guarantee of no network activity:
   * swagger-client resolves `$ref`s while rendering, and `specValidation`
   * reports an external `$ref` as a warning rather than rejecting it, so a
   * document naming remote refs can have the preview fetch from whatever host
   * they point at. Nothing else here issues a request - try-it-out is off.
   */
  spec?: Record<string, unknown>;
};

/**
 * Right-hand pane of the contract step: the resources of the fetched
 * definition, or an empty state saying that is what will land here.
 */
export const ApiResourcesPreview = ({ spec }: ApiResourcesPreviewProps) => {
  const intl = useIntl();
  const [showSource, setShowSource] = useState(false);
  const hasContract = spec !== undefined;

  // Serialize once per spec; stringify is expensive and unchanged per view.
  // Use JSON since `CodeBlock` highlights it (this is parsed content, not upload bytes).
  const sourceText = useMemo(
    () => (spec === undefined ? '' : JSON.stringify(spec, null, 2)),
    [spec],
  );

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: PANE_HEIGHT,
        // Keep the title fixed; `minHeight: 0` lets the content area shrink.
        minHeight: 0,
        overflow: 'hidden',
      }}
    >
      <Stack
        direction="row"
        spacing={2}
        sx={{
          alignItems: 'center',
          flexShrink: 0,
          justifyContent: 'space-between',
        }}
      >
        <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
          <FormattedMessage {...messages.title} />
        </Typography>
        <FormControlLabel
          control={
            <Switch
              checked={showSource}
              onChange={(event) => setShowSource(event.target.checked)}
              size="small"
              // MUI v9 routes input attributes through slotProps; the older
              // `inputProps` never reaches the element, leaving the control
              // without an accessible name.
              slotProps={{
                input: { 'aria-label': intl.formatMessage(messages.source) },
              }}
            />
          }
          // Nothing to read until something has been fetched.
          disabled={!hasContract}
          label={<FormattedMessage {...messages.source} />}
          labelPlacement="start"
          sx={{ m: 0 }}
        />
      </Stack>

      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          mt: 1,
          overflow: 'auto',
        }}
      >
        {hasContract && showSource ? (
          <CodeBlock code={sourceText} language="json" showLineNumbers />
        ) : null}

        {hasContract && !showSource ? (
          <Box
            sx={{
              // Swagger UI ships its own canvas and gutters; keep both from
              // fighting the pane's background and padding.
              '& .swagger-ui': { bgcolor: 'transparent' },
              '& .swagger-ui .wrapper': { px: 0 },
              // The servers/authorize strip belongs to a try-it-out console,
              // which this preview is not.
              '& .swagger-ui .scheme-container': { display: 'none' },
            }}
          >
            <SwaggerUI
              defaultModelExpandDepth={0}
              displayOperationId
              displayRequestDuration
              filter
              plugins={[HideTopBarPlugin, HideInfoPlugin]}
              requestSnippetsEnabled
              spec={spec}
              supportedSubmitMethods={['get']}
              tryItOutEnabled={false}
            />
          </Box>
        ) : null}

        {hasContract ? null : <ResourcePreviewPlaceholder />}
      </Box>
    </Box>
  );
};
