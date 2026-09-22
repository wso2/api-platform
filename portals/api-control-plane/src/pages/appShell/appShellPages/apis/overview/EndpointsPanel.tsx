/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import {
  Box,
  Button,
  Card,
  Divider,
  Drawer,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  Link,
  OutlinedInput,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Globe, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useUpdateRestApi, type RestApi } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { isValidUrl } from '../utils/developEdit';

const messages = defineMessages({
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.cancel',
    defaultMessage: 'Cancel',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.close',
    defaultMessage: 'Close endpoint editor',
  },
  drawerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.drawerTitle',
    defaultMessage: 'Edit endpoint',
  },
  endpointLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.endpointLabel',
    defaultMessage: 'Endpoint URL',
  },
  endpointRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.endpointRequired',
    defaultMessage: 'Enter a valid endpoint URL.',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.edit',
    defaultMessage: 'Edit endpoint',
  },
  notConfigured: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.notConfigured',
    defaultMessage: 'No endpoint configured',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.save',
    defaultMessage: 'Save',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.saved',
    defaultMessage: 'Backend endpoint updated.',
  },
  saveError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.saveError',
    defaultMessage: 'Unable to update the backend endpoint.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.title',
    defaultMessage: 'Endpoints',
  },
});

type Props = {
  api: RestApi;
};

export function EndpointsPanel({ api }: Props) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const updateApi = useUpdateRestApi();
  const url = api.upstream?.main?.url;
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [endpointUrl, setEndpointUrl] = useState('');
  const [touched, setTouched] = useState(false);
  const trimmedUrl = endpointUrl.trim();
  const endpointValid = trimmedUrl !== '' && isValidUrl(trimmedUrl);

  const openDrawer = () => {
    setEndpointUrl(url ?? '');
    setTouched(false);
    setDrawerOpen(true);
  };

  const closeDrawer = () => {
    if (updateApi.isPending) return;
    setDrawerOpen(false);
  };

  const saveEndpoint = () => {
    setTouched(true);
    if (!api.id || !endpointValid) return;

    updateApi.mutate(
      {
        restApiId: api.id,
        body: {
          ...api,
          upstream: {
            ...api.upstream,
            main: { ...api.upstream?.main, url: trimmedUrl },
          },
        },
      },
      {
        onError: () => notify(intl.formatMessage(messages.saveError), 'error'),
        onSuccess: () => {
          notify(intl.formatMessage(messages.saved), 'success');
          setDrawerOpen(false);
        },
      },
    );
  };

  return (
    <>
      <Card>
        <Stack
          alignItems="center"
          direction="row"
          sx={{ justifyContent: 'space-between', px: 2, py: 1.5 }}
        >
          <Typography sx={{ fontWeight: 600 }} variant="h6">
            <FormattedMessage {...messages.title} />
          </Typography>
          <Tooltip title={intl.formatMessage(messages.edit)}>
            <IconButton
              aria-label={intl.formatMessage(messages.edit)}
              onClick={openDrawer}
              size="small"
            >
              <Pencil size={16} />
            </IconButton>
          </Tooltip>
        </Stack>
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
            <Tooltip title={url}>
              <Link
                href={url}
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
            </Tooltip>
          ) : (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.notConfigured} />
            </Typography>
          )}
        </Stack>
      </Card>

      <Drawer
        anchor="right"
        onClose={closeDrawer}
        open={drawerOpen}
        sx={{ '& .MuiDrawer-paper': { width: { md: 480, xs: '100%' } } }}
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
          <Stack
            alignItems="center"
            direction="row"
            spacing={1}
            sx={{ borderBottom: 1, borderColor: 'divider', p: 2 }}
          >
            <IconButton
              aria-label={intl.formatMessage(messages.close)}
              disabled={updateApi.isPending}
              onClick={closeDrawer}
              size="small"
            >
              <ChevronLeft size={20} />
            </IconButton>
            <Typography sx={{ fontWeight: 600 }} variant="h6">
              <FormattedMessage {...messages.drawerTitle} />
            </Typography>
          </Stack>

          <Box sx={{ flex: 1, overflowY: 'auto', p: 3 }}>
            <FormControl error={touched && !endpointValid} fullWidth>
              <FormLabel htmlFor="overview-backend-endpoint">
                <FormattedMessage {...messages.endpointLabel} />
              </FormLabel>
              <OutlinedInput
                aria-describedby={
                  touched && !endpointValid ? 'overview-backend-endpoint-error' : undefined
                }
                autoFocus
                id="overview-backend-endpoint"
                onBlur={() => setTouched(true)}
                onChange={(event) => setEndpointUrl(event.target.value)}
                sx={{ mt: 0.75 }}
                value={endpointUrl}
              />
              {touched && !endpointValid ? (
                <FormHelperText id="overview-backend-endpoint-error">
                  <FormattedMessage {...messages.endpointRequired} />
                </FormHelperText>
              ) : null}
            </FormControl>
          </Box>

          <Stack
            direction="row"
            spacing={1}
            sx={{ borderTop: 1, borderColor: 'divider', justifyContent: 'flex-end', p: 2 }}
          >
            <Button disabled={updateApi.isPending} onClick={closeDrawer} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button
              disabled={!endpointValid || updateApi.isPending || trimmedUrl === (url ?? '')}
              loading={updateApi.isPending}
              onClick={saveEndpoint}
              variant="contained"
            >
              <FormattedMessage {...messages.save} />
            </Button>
          </Stack>
        </Box>
      </Drawer>
    </>
  );
}
