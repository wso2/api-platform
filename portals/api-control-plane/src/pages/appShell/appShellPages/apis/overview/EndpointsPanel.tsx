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
import { useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useUpdateRestApi, type RestApi } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { isValidUrl } from '../utils/developEdit';
import { useCan } from '@/permissions/useCan';
import type { PermissionOperation } from '@/permissions';

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

export type EndpointCardProps = {
  /** The card heading; defaults to "Endpoints". */
  title?: ReactNode;
  /** The backend URL shown; absent when none is configured. */
  url?: string;
  /** Whether the endpoint may be edited at all (e.g. not a read-only API). */
  editable?: boolean;
  /** The operation the edit button is gated on. */
  editOperation: PermissionOperation;
  /** True while a save is in flight. */
  saving: boolean;
  /** Persists a new URL; the card reports the outcome and closes on success. */
  onSave: (url: string) => Promise<unknown>;
};

/**
 * The backend endpoint card with its edit drawer. Shared by every API kind's
 * overview; each supplies how its API is saved and who may edit it.
 */
export function EndpointCard({
  editable = true,
  editOperation,
  onSave,
  saving,
  title,
  url,
}: EndpointCardProps) {
  const intl = useIntl();
  // Each API kind passes its own update operation, so a GraphQL API is never
  // gated on the REST permission.
  const canUpdate = useCan(editOperation);
  const { notify } = useNotifications();
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
    if (saving) return;
    setDrawerOpen(false);
  };

  const saveEndpoint = () => {
    setTouched(true);
    if (!endpointValid) return;

    onSave(trimmedUrl).then(
      () => {
        notify(intl.formatMessage(messages.saved), 'success');
        setDrawerOpen(false);
      },
      () => notify(intl.formatMessage(messages.saveError), 'error'),
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
            {title ?? <FormattedMessage {...messages.title} />}
          </Typography>
          <Tooltip title={intl.formatMessage(messages.edit)}>
            <IconButton
              aria-label={intl.formatMessage(messages.edit)}
              onClick={openDrawer}
              size="small"
              disabled={!canUpdate || !editable}
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
              disabled={saving}
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
            <Button disabled={saving} onClick={closeDrawer} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button
              disabled={!endpointValid || saving || trimmedUrl === (url ?? '')}
              loading={saving}
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

/** A REST API's backend endpoint: the shared card over REST's update. */
export function EndpointsPanel({ api }: { api: RestApi }) {
  const updateApi = useUpdateRestApi();

  return (
    <EndpointCard
      editOperation="UpdateRESTAPI"
      onSave={(endpointUrl) =>
        updateApi.mutateAsync({
          restApiId: api.id ?? '',
          body: {
            ...api,
            upstream: { ...api.upstream, main: { ...api.upstream?.main, url: endpointUrl } },
          },
        })
      }
      saving={updateApi.isPending}
      url={api.upstream?.main?.url}
    />
  );
}
