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
import { Alert, Box, Button, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  isWebhookSecretMissing,
  useCreateWebhookSecret,
  useRotateWebhookSecret,
  useWebhookSecret,
} from '@/api/resources/webhookSecrets';
import { LoadingState } from '@/components/StateViews';
import { useNotifications } from '@/components/Notifications';
import { useTrackFeature } from '@/featureUsage';
import { Can } from '@/permissions';
import { CopyableCommand } from '../../gateways/components/CopyableCommand';
import {
  WEBHOOK_SECRET_PLACEHOLDER,
  webhookSubscriberBody,
  webhookTargetUrl,
} from '../utils/apiPortalSetup';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.title',
    defaultMessage: 'Step 3: Send portal events to the control plane',
  },
  intro: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.intro',
    defaultMessage:
      'The portal tells the control plane when API keys, subscriptions and applications change, signing each event with a webhook secret. The control plane recognises the portal by that secret.',
  },
  generate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.generate',
    defaultMessage: 'Generate webhook secret',
  },
  rotate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.rotate',
    defaultMessage: 'Rotate webhook secret',
    description: 'Issues a new version of the secret; older versions keep working.',
  },
  working: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.working',
    defaultMessage: 'Generating…',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.loading',
    defaultMessage: 'Checking the portal’s webhook secret',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.loadError',
    defaultMessage: 'Could not check whether this portal has a webhook secret. Try again shortly.',
  },
  exists: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.exists',
    defaultMessage:
      'This portal already has a webhook secret. Its value is shown only when it is issued; rotate it to get a new one. Older versions keep working until you remove them.',
  },
  secretOnce: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.secretOnce',
    defaultMessage:
      'Copy this secret now. The control plane does not show it again; to get another, rotate the secret.',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.created',
    defaultMessage: 'Webhook secret issued. Add it to the portal’s webhook subscriber.',
  },
  settingsIntro: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.settingsIntro',
    defaultMessage:
      'In the portal, open Settings, then Webhooks, and add a subscriber with this target URL, the secret above, and all events:',
    description: 'Settings and Webhooks are pages in the API Portal.',
  },
  addressHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.addressHint',
    defaultMessage:
      'Replace {placeholder} with the address the portal reaches the control plane’s Platform API on.',
    description: '{placeholder} is rendered as code. Keep it as is.',
  },
  apiIntro: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.apiIntro',
    defaultMessage: 'Or create the subscriber through the portal’s API, with this request body:',
  },
  receiverNote: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalWebhookSecretStep.receiverNote',
    defaultMessage:
      'The control plane accepts these events only when its webhook receiver is on: {setting} in its config.toml.',
    description: '{setting} is a configuration line, rendered as code. config.toml is a file name.',
  },
});

const RECEIVER_SETTING = '[platform_api.webhook] enabled = true';
const stepTitleSx = { fontWeight: 600 } as const;

export type ApiPortalWebhookSecretStepProps = {
  apiPortalId: string;
  portalName: string;
};

/**
 * Issues the webhook secret a portal signs its events to the control plane
 * with, and shows how to give it to the portal. The secret is identified by
 * the portal's handle. Its value lives in component state only: shown once,
 * right after it is issued, and dropped when the page is left.
 */
export function ApiPortalWebhookSecretStep({
  apiPortalId,
  portalName,
}: ApiPortalWebhookSecretStepProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const trackFeature = useTrackFeature();
  const secretQuery = useWebhookSecret(apiPortalId);
  const createSecret = useCreateWebhookSecret();
  const rotateSecret = useRotateWebhookSecret();

  const [secret, setSecret] = useState<string>();
  const busy = createSecret.isPending || rotateSecret.isPending;

  const onIssued = (issued: { secret: string }) => {
    setSecret(issued.secret);
    notify(intl.formatMessage(messages.created), 'success');
  };

  const generate = () => {
    trackFeature('apip.lifecycle.api-portal-webhook-secret-click', { operation: 'create' });
    createSecret.mutate(
      { id: apiPortalId, name: portalName, type: 'PORTAL' },
      { onSuccess: onIssued },
    );
  };

  const rotate = () => {
    trackFeature('apip.lifecycle.api-portal-webhook-secret-click', { operation: 'rotate' });
    rotateSecret.mutate({ secretId: apiPortalId }, { onSuccess: onIssued });
  };

  // A 404 is the expected answer for a portal not yet set up to send events.
  const notFound = isWebhookSecretMissing(secretQuery.error);
  const exists = Boolean(secretQuery.data) && !notFound;

  let action;
  if (secretQuery.isPending) {
    action = <LoadingState label={intl.formatMessage(messages.loading)} />;
  } else if (secretQuery.error && !notFound) {
    action = <Alert severity="error">{intl.formatMessage(messages.loadError)}</Alert>;
  } else {
    action = (
      <Box>
        <Can do={exists ? 'rotateWebhookSecret' : 'createWebhookSecret'} denied="disable">
          <Button disabled={busy} onClick={exists ? rotate : generate} variant="outlined">
            <FormattedMessage
              {...(busy ? messages.working : exists ? messages.rotate : messages.generate)}
            />
          </Button>
        </Can>
      </Box>
    );
  }

  return (
    <Stack spacing={1}>
      <Typography color="primary.main" sx={stepTitleSx} variant="subtitle1">
        <FormattedMessage {...messages.title} />
      </Typography>
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.intro} />
      </Typography>
      {exists && !secret && (
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.exists} />
        </Typography>
      )}
      {action}
      {secret && (
        <>
          <Alert severity="warning">
            <FormattedMessage {...messages.secretOnce} />
          </Alert>
          <CopyableCommand code={secret} />
        </>
      )}
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.settingsIntro} />
      </Typography>
      <CopyableCommand code={webhookTargetUrl()} />
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage
          {...messages.addressHint}
          values={{ placeholder: <code key="placeholder">{'<platform-api-url>'}</code> }}
        />
      </Typography>
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.apiIntro} />
      </Typography>
      <CopyableCommand
        code={webhookSubscriberBody(secret ?? WEBHOOK_SECRET_PLACEHOLDER)}
        onCopy={() =>
          trackFeature('apip.lifecycle.api-portal-setup-copy', { setup_step: 'webhook-subscriber' })
        }
      />
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage
          {...messages.receiverNote}
          values={{ setting: <code key="setting">{RECEIVER_SETTING}</code> }}
        />
      </Typography>
    </Stack>
  );
}
