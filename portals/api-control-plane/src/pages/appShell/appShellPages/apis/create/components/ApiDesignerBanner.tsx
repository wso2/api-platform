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

import { Box, Button, Card, Stack, Typography } from '@wso2/oxygen-ui';
import { ExternalLink, FileText } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';

import { ApiDesignerCanvasIllustration } from '@/components/illustrations/ApiDesignerCanvasIllustration';
import { runtimeConfig } from '@/config/runtime';

const messages = defineMessages({
  action: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.ApiDesignerBanner.action',
    defaultMessage: 'Open API Designer',
    description:
      'Button opening the API Designer VS Code extension listing in a new tab. "API Designer" is a product name — leave it untranslated.',
  },
  body: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.ApiDesignerBanner.body',
    defaultMessage:
      'Design, edit, and validate OpenAPI 3.x APIs directly in VS Code. Includes Spectral governance checks and AI-readiness assessment.',
  },
  docs: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.ApiDesignerBanner.docs',
    defaultMessage: 'How to get started',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.ApiDesignerBanner.title',
    defaultMessage: 'API Designer',
  },
});

/** API Designer promotion shown after the API-type choices. */
export const ApiDesignerBanner = () => (
  <Card
    sx={{
      display: 'flex',
      flexDirection: { md: 'row', xs: 'column' },
      gap: 2,
      p: 2,
      width: '100%',
    }}
  >
    <Stack spacing={1.25} sx={{ flex: 1, justifyContent: 'center', minWidth: 0 }}>
      <Box>
        <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 0.5, opacity: 0.65 }} variant="body2">
          <FormattedMessage {...messages.body} />
        </Typography>
      </Box>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        <Button
          component="a"
          endIcon={<ExternalLink size={16} />}
          href={runtimeConfig.apiDesignerVsCodeUrl}
          rel="noopener noreferrer"
          size="small"
          target="_blank"
          variant="outlined"
        >
          <FormattedMessage {...messages.action} />
        </Button>
        <Button
          component="a"
          endIcon={<FileText size={16} />}
          href={runtimeConfig.apiDesignerDocsUrl}
          rel="noopener noreferrer"
          size="small"
          target="_blank"
          variant="text"
        >
          <FormattedMessage {...messages.docs} />
        </Button>
      </Stack>
    </Stack>

    <Box
      sx={{
        flex: 1,
        height: { md: 128, xs: 150 },
        minWidth: 0,
        display: 'flex',
        justifyContent: { md: 'flex-end', xs: 'center' },
        '& svg': { height: '100%', maxWidth: '100%', objectFit: 'contain', width: 'auto' },
      }}
    >
      <ApiDesignerCanvasIllustration />
    </Box>
  </Card>
);
