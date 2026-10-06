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
import {
  issueServiceAccountToken,
  SERVICE_ACCOUNT_INVALID_SCOPE,
  type ServiceAccount,
  type ServiceAccountTokenResponse,
} from '../../../../../apis/serviceAccountApis';
import { getErrorCode, getHttpStatus } from '../../../../../utils/apiError';
import SecretField from './SecretField';
import { tokenUseCurl } from './serviceAccountUtils';

const P = 'aiWorkspace.pages.appShell.appShellPages.settings.serviceAccounts.TestTokenDialog';
const messages = defineMessages({
  title: { id: `${P}.title`, defaultMessage: 'Get token — {name}' },
  clientId: { id: `${P}.clientId`, defaultMessage: 'Client ID' },
  clientSecret: { id: `${P}.clientSecret`, defaultMessage: 'Client secret' },
  secretHelper: {
    id: `${P}.secretHelper`,
    defaultMessage: 'The server does not keep the secret, so paste it here. It is not stored.',
  },
  usageNote: {
    id: `${P}.usageNote`,
    defaultMessage: 'Getting a token counts as using the account: it updates "Last token issued" and is logged.',
  },
  getToken: { id: `${P}.getToken`, defaultMessage: 'Get token' },
  getAnother: { id: `${P}.getAnother`, defaultMessage: 'Get another' },
  getting: { id: `${P}.getting`, defaultMessage: 'Getting token…' },
  rejected: {
    id: `${P}.rejected`,
    defaultMessage: 'Could not get a token. Check the secret, and that the account is enabled.',
  },
  failed: { id: `${P}.failed`, defaultMessage: 'Could not get a token. Try again in a moment.' },
  scopeFailed: {
    id: `${P}.scopeFailed`,
    defaultMessage: "The account's roles do not grant the requested scopes. Reload the page and try again.",
  },
  accessToken: { id: `${P}.accessToken`, defaultMessage: 'Access token' },
  expiresIn: { id: `${P}.expiresIn`, defaultMessage: 'Expires in {minutes}:{seconds}' },
  expired: { id: `${P}.expired`, defaultMessage: 'Expired' },
  scopes: { id: `${P}.scopes`, defaultMessage: 'Scopes' },
  useIt: { id: `${P}.useIt`, defaultMessage: 'Use it' },
  revokeNote: {
    id: `${P}.revokeNote`,
    defaultMessage:
      'This token cannot be revoked on its own. Disabling the account or regenerating its secret stops it.',
  },
  close: { id: `${P}.close`, defaultMessage: 'Close' },
});

const SECRET_FIELD = 'service-account-test-secret';

type Issued = ServiceAccountTokenResponse & { expiresAt: number };

interface TestTokenDialogProps {
  account: ServiceAccount;
  /** Prefilled when the admin comes straight from the credentials dialog. */
  initialSecret?: string;
  /** Requested as `scope`; required by a server in scope mode, ignored in role mode. */
  scopes: string[];
  onClose: () => void;
  /** Called after a token is issued, since that updates "Last token issued". */
  onIssued: () => void;
}

export default function TestTokenDialog({
  account,
  initialSecret = '',
  scopes,
  onClose,
  onIssued,
}: TestTokenDialogProps) {
  const intl = useIntl();
  const [secret, setSecret] = useState(initialSecret);
  const [issued, setIssued] = useState<Issued | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!issued) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [issued]);

  const remaining = issued ? Math.max(0, Math.round((issued.expiresAt - now) / 1000)) : 0;
  const expired = issued !== null && remaining === 0;

  const errorMessage = !error
    ? null
    : getErrorCode(error) === SERVICE_ACCOUNT_INVALID_SCOPE
      ? intl.formatMessage(messages.scopeFailed)
      : getHttpStatus(error) === 401
        ? intl.formatMessage(messages.rejected)
        : intl.formatMessage(messages.failed);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!secret || pending) return;
    // A failed retry must not leave the previous token on screen.
    setIssued(null);
    setError(null);
    setPending(true);
    try {
      const response = await issueServiceAccountToken(account.clientId, secret, scopes);
      setNow(Date.now());
      setIssued({ ...response, expiresAt: Date.now() + response.expires_in * 1000 });
      onIssued();
    } catch (err) {
      setError(err);
    } finally {
      setPending(false);
    }
  };

  const close = () => {
    setSecret('');
    setIssued(null);
    onClose();
  };

  return (
    // A shown token is lost on close, so only the Close button dismisses it then.
    <Dialog fullWidth maxWidth="sm" onClose={issued || pending ? undefined : close} open>
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
          <Button disabled={pending} onClick={close} variant="outlined">
            <FormattedMessage {...messages.close} />
          </Button>
          <Button disabled={!secret || pending} type="submit" variant="contained">
            <FormattedMessage {...(pending ? messages.getting : issued ? messages.getAnother : messages.getToken)} />
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
