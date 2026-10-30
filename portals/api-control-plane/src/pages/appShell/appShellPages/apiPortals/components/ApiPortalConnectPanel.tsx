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
import { Alert, Box, Button, Card, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useUpdateApiPortal } from '@/api/resources/apiPortals';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { Can } from '@/permissions';
import { CopyableCommand } from '../../gateways/components/CopyableCommand';
import {
  generateSharedKey,
  HASH_PLACEHOLDER,
  internalAuthConfig,
  sharedKeyHash,
} from '../utils/apiPortalSetup';

const messages = defineMessages({
  confirmBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.confirm.body',
    defaultMessage:
      'This replaces the shared key the control plane uses for this portal. Publishing to the portal fails until it is configured with the new hash.',
  },
  confirmButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.confirm.button',
    defaultMessage: 'Generate new key',
  },
  confirmCancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.confirm.cancel',
    defaultMessage: 'Cancel',
  },
  confirmTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.confirm.title',
    defaultMessage: 'Generate a new shared key?',
  },
  generate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.generate',
    defaultMessage: 'Generate new key',
    description: 'Replaces the shared key, so the portal needs the new hash.',
  },
  generating: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.generating',
    defaultMessage: 'Generating…',
  },
  hashOnce: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.hashOnce',
    defaultMessage:
      'Copy this now. The console does not keep the hash; to see one again, generate a new key.',
  },
  hashPending: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.hashPending',
    defaultMessage:
      'The hash is shown only right after the key is set. If you no longer have it, generate a new key.',
  },
  intro: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.intro',
    defaultMessage:
      'The control plane authenticates to the portal with a shared key. The portal checks it against the key’s SHA-256 hash, which you set in the portal’s configuration.',
  },
  quickstart: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.quickstart',
    defaultMessage:
      'Using the quickstart distribution? Write the hash to {file} instead.',
    description: '{file} is a file path, rendered as code. Keep it as is.',
  },
  restart: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.restart',
    defaultMessage: 'Restart the portal so it picks up the new hash.',
  },
  stepConfigure: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.step.configure',
    defaultMessage: 'Step 1: Add the hash to the portal configuration',
  },
  stepConfigureIntro: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.step.configure.intro',
    defaultMessage: 'Set this in the portal’s config.toml:',
    description: 'config.toml is a file name. Keep it as is.',
  },
  stepRestart: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.step.restart',
    defaultMessage: 'Step 2: Restart the portal',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.title',
    defaultMessage: 'Connect the portal',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalConnectPanel.updated',
    defaultMessage: 'New shared key set. Configure the portal with the new hash.',
  },
});

/** Where the quickstart's portal reads the hash from, relative to the distribution. */
const QUICKSTART_HASH_FILE = 'resources/keys/api-portal-internal-key-hash';

const stepTitleSx = { fontWeight: 600 } as const;

export type ApiPortalConnectPanelProps = {
  apiPortalId: string;
  /** Hash of a key set earlier in this session, such as by the register page. */
  initialHash?: string;
};

/**
 * How to configure a portal with the hash of its shared key, and a way to
 * replace the key.
 *
 * The key itself is never shown or kept: it is generated here, sent once on
 * the update, and dropped. The hash lives in component state only, so leaving
 * the page discards it.
 */
export function ApiPortalConnectPanel({ apiPortalId, initialHash }: ApiPortalConnectPanelProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const updatePortal = useUpdateApiPortal();

  const [hash, setHash] = useState(initialHash);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [preparing, setPreparing] = useState(false);
  const busy = preparing || updatePortal.isPending;

  const rotate = async () => {
    setConfirmOpen(false);
    const sharedKey = generateSharedKey();
    setPreparing(true);
    let next: string;
    try {
      next = await sharedKeyHash(sharedKey);
    } finally {
      setPreparing(false);
    }
    updatePortal.mutate(
      { apiPortalId, body: { sharedKey } },
      {
        onSuccess: () => {
          setHash(next);
          notify(intl.formatMessage(messages.updated), 'success');
        },
      },
    );
  };

  return (
    <Card sx={{ p: 3 }}>
      <Stack spacing={3}>
        <Stack spacing={1}>
          <Typography sx={{ fontWeight: 700 }} variant="h5">
            <FormattedMessage {...messages.title} />
          </Typography>
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.intro} />
          </Typography>
        </Stack>

        <Stack spacing={1}>
          <Typography color="primary.main" sx={stepTitleSx} variant="subtitle1">
            <FormattedMessage {...messages.stepConfigure} />
          </Typography>
          {hash ? (
            <Alert severity="warning">
              <FormattedMessage {...messages.hashOnce} />
            </Alert>
          ) : (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.hashPending} />
            </Typography>
          )}
          <Box>
            <Can do="UpdateApiPortal" denied="disable">
              <Button disabled={busy} onClick={() => setConfirmOpen(true)} variant="outlined">
                <FormattedMessage {...(busy ? messages.generating : messages.generate)} />
              </Button>
            </Can>
          </Box>
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.stepConfigureIntro} />
          </Typography>
          <CopyableCommand code={internalAuthConfig(hash ?? HASH_PLACEHOLDER)} />
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage
              {...messages.quickstart}
              values={{ file: <code key="file">{QUICKSTART_HASH_FILE}</code> }}
            />
          </Typography>
        </Stack>

        <Stack spacing={1}>
          <Typography color="primary.main" sx={stepTitleSx} variant="subtitle1">
            <FormattedMessage {...messages.stepRestart} />
          </Typography>
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.restart} />
          </Typography>
        </Stack>
      </Stack>

      <ConfirmDialog
        cancelLabel={intl.formatMessage(messages.confirmCancel)}
        confirmLabel={intl.formatMessage(messages.confirmButton)}
        destructive
        loading={busy}
        message={intl.formatMessage(messages.confirmBody)}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={() => void rotate()}
        open={confirmOpen}
        title={intl.formatMessage(messages.confirmTitle)}
      />
    </Card>
  );
}
