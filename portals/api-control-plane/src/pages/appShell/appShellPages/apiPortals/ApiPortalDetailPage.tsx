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

import { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Link as ExternalLink,
  PageTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { PanelsTopLeft, Pencil, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';

import { useApiPortal, useDeleteApiPortal } from '@/api/resources/apiPortals';
import { AppPage } from '@/components/AppPage';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { Can } from '@/permissions';
import { routes } from '@/routes/paths';
import { ApiPortalConnectPanel } from './components/ApiPortalConnectPanel';
import { ApiPortalEditDialog } from './components/ApiPortalEditDialog';

const messages = defineMessages({
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.back',
    defaultMessage: 'Back to API Portals',
  },
  delete: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.delete',
    defaultMessage: 'Delete',
  },
  deleteBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.delete.body',
    defaultMessage:
      'This removes “{name}” from the control plane, which can then no longer publish to it. The portal itself keeps running, along with the APIs already on it.',
    description: '{name} is the portal name.',
  },
  deleteCancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.delete.cancel',
    defaultMessage: 'Cancel',
  },
  deleted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.deleted',
    defaultMessage: 'API Portal “{name}” deleted.',
    description: '{name} is the portal name.',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.delete.title',
    defaultMessage: 'Delete this API Portal?',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.edit',
    defaultMessage: 'Edit',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.error',
    defaultMessage: 'Unable to load this API Portal.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.loading',
    defaultMessage: 'Loading API Portal',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.notFound',
    defaultMessage: 'API Portal not found',
  },
  registered: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalDetailPage.registered',
    defaultMessage:
      'You registered {name}. Next, configure the portal with the hash below so the control plane can publish to it.',
    description: '{name} is the portal name. Never translated.',
  },
});

/**
 * Router state the register page hands over. The hash is not secret, but the
 * console cannot recover it later, so this is the one chance to show it.
 */
export type ApiPortalDetailLocationState = {
  justRegistered?: boolean;
  sharedKeyHash?: string;
};

export function ApiPortalDetailPage() {
  return (
    <AppPage>
      <ApiPortalDetailPageContent />
    </AppPage>
  );
}

function ApiPortalDetailPageContent() {
  const { orgHandle = '', apiPortalId = '' } = useParams();
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const location = useLocation();
  const handoff = (location.state ?? {}) as ApiPortalDetailLocationState;

  const portalQuery = useApiPortal(apiPortalId);
  const deletePortal = useDeleteApiPortal();

  const [bannerOpen, setBannerOpen] = useState(Boolean(handoff.justRegistered));
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (portalQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (portalQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }
  if (!portalQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }

  const portal = portalQuery.data;

  const confirmDelete = () =>
    deletePortal.mutate(
      { apiPortalId },
      {
        onSuccess: () => {
          setDeleteOpen(false);
          notify(intl.formatMessage(messages.deleted, { name: portal.name }), 'success');
          navigate(routes.orgApiPortals(orgHandle));
        },
      },
    );

  return (
    <>
      <PageTitle>
        <Link to={routes.orgApiPortals(orgHandle)}>
          <PageTitle.BackButton>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
        </Link>
      </PageTitle>

      <Stack spacing={3}>
        {bannerOpen && (
          <Alert onClose={() => setBannerOpen(false)} severity="success">
            <FormattedMessage {...messages.registered} values={{ name: portal.name }} />
          </Alert>
        )}

        <Card sx={{ p: 3 }}>
          <Stack
            direction={{ md: 'row', xs: 'column' }}
            spacing={2}
            sx={{ alignItems: { md: 'flex-start' }, justifyContent: 'space-between' }}
          >
            <Stack direction="row" spacing={2} sx={{ minWidth: 0 }}>
              <Box sx={{ color: 'primary.main', display: 'flex', flexShrink: 0, pt: 0.5 }}>
                <PanelsTopLeft size={32} />
              </Box>
              <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                <Stack alignItems="center" direction="row" spacing={1}>
                  <Typography component="h1" sx={{ fontWeight: 700 }} variant="h4">
                    {portal.name}
                  </Typography>
                  <Chip label={portal.id} size="small" variant="outlined" />
                </Stack>
                <ExternalLink
                  href={portal.url}
                  rel="noopener noreferrer"
                  sx={{ wordBreak: 'break-all' }}
                  target="_blank"
                  variant="body2"
                >
                  {portal.url}
                </ExternalLink>
                {portal.description ? (
                  <Typography color="text.secondary" variant="body2">
                    {portal.description}
                  </Typography>
                ) : null}
              </Stack>
            </Stack>

            <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
              <Can do="UpdateApiPortal" denied="disable">
                <Button onClick={() => setEditOpen(true)} startIcon={<Pencil />} variant="outlined">
                  <FormattedMessage {...messages.edit} />
                </Button>
              </Can>
              <Can do="DeleteApiPortal" denied="disable">
                <Button
                  color="error"
                  onClick={() => setDeleteOpen(true)}
                  startIcon={<Trash2 />}
                  variant="outlined"
                >
                  <FormattedMessage {...messages.delete} />
                </Button>
              </Can>
            </Stack>
          </Stack>
        </Card>

        <ApiPortalConnectPanel apiPortalId={apiPortalId} initialHash={handoff.sharedKeyHash} />
      </Stack>

      {editOpen && <ApiPortalEditDialog onClose={() => setEditOpen(false)} portal={portal} />}

      <ConfirmDialog
        cancelLabel={intl.formatMessage(messages.deleteCancel)}
        confirmLabel={intl.formatMessage(messages.delete)}
        destructive
        loading={deletePortal.isPending}
        message={intl.formatMessage(messages.deleteBody, { name: portal.name })}
        onCancel={() => setDeleteOpen(false)}
        onConfirm={confirmDelete}
        open={deleteOpen}
        title={intl.formatMessage(messages.deleteTitle)}
      />
    </>
  );
}
