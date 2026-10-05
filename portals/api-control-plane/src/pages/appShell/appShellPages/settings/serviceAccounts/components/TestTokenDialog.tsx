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

import { useEffect, useState, type FormEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CodeBlock,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Info } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ErrorCode, isErrorCode } from '@/api/core/errors';
import {
  useIssueServiceAccountToken,
  type ServiceAccount,
  type ServiceAccountTokenResponse,
} from '@/api/resources/serviceAccounts';
import { tokenUseCurl } from '../serviceAccountCurl';
import { SecretField } from './SecretField';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.title',
    defaultMessage: 'Get token — {name}',
    description: '{name} is the service account display name — never translated.',
  },
  clientId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.clientId',
    defaultMessage: 'Client ID',
  },
  clientSecret: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.clientSecret',
    defaultMessage: 'Client secret',
  },
  secretHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.secretHelper',
    defaultMessage: 'The server does not keep the secret, so paste it here. It is not stored.',
  },
  usageNote: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.usageNote',
    defaultMessage: 'Getting a token counts as using the account: it updates "Last token issued" and is logged.',
  },
  getToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.getToken',
    defaultMessage: 'Get token',
  },
  getAnother: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.getAnother',
    defaultMessage: 'Get another',
  },
  getting: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.getting',
    defaultMessage: 'Getting token…',
  },
  rejected: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.rejected',
    defaultMessage: 'Could not get a token. Check the secret, and that the account is enabled.',
    description: 'Shown when the server rejects the client ID or secret. It never says which, on purpose.',
  },
  failed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.failed',
    defaultMessage: 'Could not get a token. Try again in a moment.',
    description: 'Shown for any failure other than rejected credentials or scopes: timeouts, network or server errors.',
  },
  scopeFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.scopeFailed',
    defaultMessage: "The account's roles do not grant the requested scopes. Reload the page and try again.",
  },
  accessToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.accessToken',
    defaultMessage: 'Access token',
  },
  expiresIn: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.expiresIn',
    defaultMessage: 'Expires in {minutes}:{seconds}',
    description: 'Countdown to token expiry, e.g. "Expires in 14:52". {seconds} is always two digits.',
  },
  expired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.expired',
    defaultMessage: 'Expired',
  },
  scopes: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.scopes',
    defaultMessage: 'Scopes',
  },
  useIt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.useIt',
    defaultMessage: 'Use it',
  },
  revokeNote: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.revokeNote',
    defaultMessage:
      'This token cannot be revoked on its own. Disabling the account or regenerating its secret stops it.',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.TestTokenDialog.close',
    defaultMessage: 'Close',
  },
});

const SECRET_FIELD = 'service-account-test-secret';

type Issued = ServiceAccountTokenResponse & { expiresAt: number };

export type TestTokenDialogProps = {
  account: ServiceAccount | null;
  /** Prefilled when the admin comes straight from the credentials dialog. */
  initialSecret?: string;
  /** Requested as `scope`; required by a server in scope mode, ignored in role mode. */
  scopes: string[];
  onClose: () => void;
};

/**
 * Exchanges the account's client ID and a pasted secret for a short-lived token.
 * Mount it per account (keyed) so nothing carries over between accounts.
 */
export function TestTokenDialog({ account, initialSecret = '', scopes, onClose }: TestTokenDialogProps) {
  const intl = useIntl();
  const mutation = useIssueServiceAccountToken();
  const [secret, setSecret] = useState(initialSecret);
  const [issued, setIssued] = useState<Issued | null>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!issued) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [issued]);

  if (!account) return null;

  const remaining = issued ? Math.max(0, Math.round((issued.expiresAt - now) / 1000)) : 0;
  const expired = issued !== null && remaining === 0;

  const errorMessage = !mutation.error
    ? null
    : isErrorCode(mutation.error, 'SERVICE_ACCOUNT_INVALID_SCOPE')
      ? intl.formatMessage(messages.scopeFailed)
      : isErrorCode(mutation.error, ErrorCode.UNAUTHORIZED)
        ? intl.formatMessage(messages.rejected)
        : intl.formatMessage(messages.failed);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!secret || mutation.isPending) return;
    // A failed retry must not leave the previous token on screen.
    setIssued(null);
    mutation.mutate(
      {
        client_id: account.clientId,
        client_secret: secret,
        grant_type: 'client_credentials',
        scope: scopes.join(' ') || undefined,
      },
      {
        onSuccess: (response) => {
          setNow(Date.now());
          setIssued({ ...response, expiresAt: Date.now() + response.expires_in * 1000 });
        },
      },
    );
  };

  const close = () => {
    setSecret('');
    setIssued(null);
    onClose();
  };

  return (
    <Dialog
      fullWidth
      maxWidth="sm"
      // A shown token is lost on close, so only the Close button dismisses it.
      onClose={issued || mutation.isPending ? undefined : close}
      open
    >
      <DialogTitle>
        <FormattedMessage {...messages.title} values={{ name: account.displayName }} />
      </DialogTitle>
      <Box component="form" noValidate onSubmit={submit}>
        <DialogContent>
          <Stack spacing={2.5}>
            <SecretField
              id="service-account-test-client-id"
              label={intl.formatMessage(messages.clientId)}
              value={account.clientId}
            />
            <FormControl fullWidth required>
              <FormLabel htmlFor={SECRET_FIELD}>
                <FormattedMessage {...messages.clientSecret} />
              </FormLabel>
              <OutlinedInput
                autoFocus={!initialSecret}
                id={SECRET_FIELD}
                inputProps={{ autoComplete: 'off', spellCheck: false }}
                onChange={(event) => setSecret(event.target.value)}
                type="password"
                value={secret}
              />
              <FormHelperText>
                <FormattedMessage {...messages.secretHelper} />
              </FormHelperText>
            </FormControl>
            <Alert icon={<Info size={18} />} severity="info">
              <FormattedMessage {...messages.usageNote} />
            </Alert>
            {errorMessage && <Alert severity="error">{errorMessage}</Alert>}

            {issued && (
              <>
                <Divider />
                <SecretField
                  id="service-account-access-token"
                  label={intl.formatMessage(messages.accessToken)}
                  masked
                  value={issued.access_token}
                />
                <Typography color={expired ? 'error' : 'text.secondary'} variant="body2">
                  {expired ? (
                    <FormattedMessage {...messages.expired} />
                  ) : (
                    <FormattedMessage
                      {...messages.expiresIn}
                      values={{
                        minutes: Math.floor(remaining / 60),
                        seconds: String(remaining % 60).padStart(2, '0'),
                      }}
                    />
                  )}
                </Typography>
                {issued.scope && (
                  <Box>
                    <Typography sx={{ fontWeight: 600, mb: 0.5 }} variant="body2">
                      <FormattedMessage {...messages.scopes} />
                    </Typography>
                    <Stack direction="row" flexWrap="wrap" gap={0.5}>
                      {issued.scope.split(' ').map((scope) => (
                        <Chip key={scope} label={scope} size="small" sx={{ fontFamily: 'monospace' }} />
                      ))}
                    </Stack>
                  </Box>
                )}
                <Box>
                  <Typography sx={{ fontWeight: 600, mb: 0.5 }} variant="body2">
                    <FormattedMessage {...messages.useIt} />
                  </Typography>
                  <CodeBlock code={tokenUseCurl()} language="bash" />
                </Box>
                <Alert severity="warning">
                  <FormattedMessage {...messages.revokeNote} />
                </Alert>
              </>
            )}
          </Stack>
        </DialogContent>
        <Divider />
        <DialogActions>
          <Button disabled={mutation.isPending} onClick={close} variant="outlined">
            <FormattedMessage {...messages.close} />
          </Button>
          <Button disabled={!secret || mutation.isPending} type="submit" variant="contained">
            <FormattedMessage
              {...(mutation.isPending ? messages.getting : issued ? messages.getAnother : messages.getToken)}
            />
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
