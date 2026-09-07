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

import { Box, Button, Link, Stack, Typography } from '@wso2/oxygen-ui';
import { ExternalLink } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';

import { ApiDesignerCanvasIllustration } from '@/components/illustrations/ApiDesignerCanvasIllustration';
import { runtimeConfig } from '@/config/runtime';

const messages = defineMessages({
  action: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.DesignWithAiPanel.action',
    defaultMessage: 'Open API Designer',
    description:
      'Button opening the API Designer VS Code extension listing in a new tab. "API Designer" is a product name — leave it untranslated.',
  },
  body: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.DesignWithAiPanel.body',
    defaultMessage:
      'Draw operations on a canvas, check them against governance rules, and design with AI in VS Code.',
  },
  docs: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.DesignWithAiPanel.docs',
    defaultMessage: 'How to get started',
  },
  skeletonHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.DesignWithAiPanel.skeletonHint',
    defaultMessage:
      'Or carry on here - the skeleton on the right is a starting point you can edit by hand.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.create.components.DesignWithAiPanel.title',
    defaultMessage: 'Design with AI',
  },
});

/**
 * Left-hand panel of the "design from scratch" approach.
 *
 * The visual designer is not hosted in the console; it ships as the API
 * Designer VS Code extension. So rather than promising a canvas here, this
 * shows what that canvas looks like and hands the user straight to it — while
 * the skeleton in the pane beside it stays editable for anyone who would
 * rather not leave the wizard.
 */
export const DesignWithAiPanel = () => (
  <Box>
    <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
      <FormattedMessage {...messages.title} />
    </Typography>

    <Stack alignItems="center" spacing={2} sx={{ pt: 3, textAlign: 'center' }}>
      {/* The illustration is the backdrop and the button sits on top of it, so
          the canvas sets the scene without competing with the call to action. */}
      <Box sx={{ position: 'relative', width: '100%' }}>
        <Box sx={{ opacity: 0.5 }}>
          <ApiDesignerCanvasIllustration />
        </Box>
        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            inset: 0,
            justifyContent: 'center',
            position: 'absolute',
          }}
        >
          <Button
            component="a"
            endIcon={<ExternalLink size={16} />}
            href={runtimeConfig.apiDesignerVsCodeUrl}
            // `noopener` keeps the opened tab from reaching back through
            // `window.opener`; `noreferrer` withholds the console URL from it.
            rel="noopener noreferrer"
            target="_blank"
            variant="contained"
          >
            <FormattedMessage {...messages.action} />
          </Button>
        </Box>
      </Box>

      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.body} />
      </Typography>

      <Link
        href={runtimeConfig.apiDesignerDocsUrl}
        rel="noopener noreferrer"
        target="_blank"
        variant="body2"
      >
        <FormattedMessage {...messages.docs} />
      </Link>

      <Typography color="text.secondary" sx={{ pt: 1 }} variant="body2">
        <FormattedMessage {...messages.skeletonHint} />
      </Typography>
    </Stack>
  </Box>
);
