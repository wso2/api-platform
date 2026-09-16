/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import { Box, Card, Divider, Link, Stack, Typography } from '@wso2/oxygen-ui';
import { Globe } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';

const messages = defineMessages({
  notConfigured: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.notConfigured',
    defaultMessage: 'No endpoint configured',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.title',
    defaultMessage: 'Endpoints',
  },
});

type Props = {
  url?: string;
};

export function EndpointsPanel({ url }: Props) {
  return (
    <Card>
      <Box sx={{ px: 2, py: 1.5 }}>
        <Typography sx={{ fontWeight: 600 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
      </Box>
      <Divider />
      <Stack alignItems="center" direction="row" spacing={1.25} sx={{ px: 2, py: 1.5 }}>
        <Box
          sx={{
            alignItems: 'center',
            bgcolor: 'action.hover',
            borderRadius: 1,
            color: 'primary.main',
            display: 'flex',
            flexShrink: 0,
            height: 32,
            justifyContent: 'center',
            width: 32,
          }}
        >
          <Globe size={17} />
        </Box>
        {url ? (
          <Link
            // href={url}
            rel="noopener noreferrer"
            sx={{
              '&:hover': { textDecoration: 'none' },
              minWidth: 0,
              overflow: 'hidden',
              textDecoration: 'none',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
            target="_blank"
            variant="body2"
          >
            {url}
          </Link>
        ) : (
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.notConfigured} />
          </Typography>
        )}
      </Stack>
    </Card>
  );
}
