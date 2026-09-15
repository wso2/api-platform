/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import { Box, Card, Chip, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { FileText } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';

import documents from './mockDocuments.json';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.title',
    defaultMessage: 'Documents',
  },
  description: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.description',
    defaultMessage: 'Guides, references, and specifications published with this API.',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.updated',
    defaultMessage: '2 days ago',
    description: 'Temporary relative update time for demo document data.',
  },
});

export function DocumentsPanel() {
  return (
    <Card>
      <Box sx={{ px: 2, py: 1.5 }}>
        <Typography sx={{ fontWeight: 600 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Typography color="text.secondary" variant="caption">
          <FormattedMessage {...messages.description} />
        </Typography>
      </Box>
      <Divider />
      <Stack divider={<Divider />}>
        {documents.map((document) => (
          <Stack
            alignItems="center"
            direction="row"
            key={document.id}
            spacing={1.5}
            sx={{ px: 2, py: 1.25 }}
          >
            <FileText color="currentColor" size={16} />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography noWrap variant="body2">
                {document.title}
              </Typography>
              <Typography color="text.secondary" noWrap variant="caption">
                {document.description}
              </Typography>
            </Box>
            <Chip label={document.type} size="small" sx={{ typography: 'caption' }} />
            <Typography
              color="text.secondary"
              sx={{ display: { sm: 'block', xs: 'none' } }}
              variant="caption"
            >
              <FormattedMessage {...messages.updated} />
            </Typography>
          </Stack>
        ))}
      </Stack>
    </Card>
  );
}
