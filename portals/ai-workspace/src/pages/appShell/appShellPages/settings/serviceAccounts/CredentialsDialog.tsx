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

import {
  Alert,
  AlertTitle,
  Box,
  Button,
  CodeBlock,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import type { ServiceAccountCredentials } from '../../../../../apis/serviceAccountApis';
import SecretField from './SecretField';
import { tokenRequestCurl } from './serviceAccountUtils';

const P = 'aiWorkspace.pages.appShell.appShellPages.settings.serviceAccounts.CredentialsDialog';
const messages = defineMessages({
  title: { id: `${P}.title`, defaultMessage: 'Copy the secret now' },
  subtitle: { id: `${P}.subtitle`, defaultMessage: 'Credentials for {name}.' },
  warningTitle: { id: `${P}.warningTitle`, defaultMessage: 'The secret will not be shown again' },
  warningBody: {
    id: `${P}.warningBody`,
    defaultMessage:
      'Store it in your secret manager. If it is lost, regenerate it — which breaks anything still using the old one.',
  },
  clientId: { id: `${P}.clientId`, defaultMessage: 'Client ID' },
  clientSecret: { id: `${P}.clientSecret`, defaultMessage: 'Client secret' },
  exampleTitle: { id: `${P}.exampleTitle`, defaultMessage: 'Get an access token' },
  exampleHint: {
    id: `${P}.exampleHint`,
    defaultMessage:
      'Set PLATFORM_API_URL to the API Platform address and CLIENT_SECRET from your secret store. The secret is never written into this example.',
  },
  testToken: { id: `${P}.testToken`, defaultMessage: 'Get token' },
  done: { id: `${P}.done`, defaultMessage: 'Done' },
});

interface CredentialsDialogProps {
  credentials: ServiceAccountCredentials | null;
  /** The scopes the account's roles grant; sent as `scope` in the example. */
  scopes: string[];
  onTestToken: (credentials: ServiceAccountCredentials) => void;
  onDone: () => void;
}

export default function CredentialsDialog({ credentials, scopes, onTestToken, onDone }: CredentialsDialogProps) {
  const intl = useIntl();

  return (
    // No onClose: the secret cannot be recovered, so only Done dismisses it.
    <Dialog fullWidth maxWidth="sm" open={credentials !== null}>
      {credentials && (
        <>
          <DialogTitle>
            <Typography component="span" sx={{ display: 'block', fontWeight: 600 }} variant="h6">
              <FormattedMessage {...messages.title} />
            </Typography>
            <Typography color="text.secondary" component="span" sx={{ display: 'block' }} variant="body2">
              <FormattedMessage {...messages.subtitle} values={{ name: credentials.serviceAccount.displayName }} />
            </Typography>
          </DialogTitle>
          <DialogContent>
            <Stack spacing={2.5}>
              <Alert icon={<TriangleAlert size={18} />} severity="warning">
                <AlertTitle sx={{ fontWeight: 600 }}>
                  <FormattedMessage {...messages.warningTitle} />
                </AlertTitle>
                <FormattedMessage {...messages.warningBody} />
              </Alert>
              <SecretField
                id="service-account-client-id"
                label={intl.formatMessage(messages.clientId)}
                value={credentials.clientId}
              />
              <SecretField
                id="service-account-client-secret"
                label={intl.formatMessage(messages.clientSecret)}
                masked
                value={credentials.clientSecret}
              />
              <Box>
                <Typography sx={{ fontWeight: 600, mb: 0.5 }} variant="body2">
                  <FormattedMessage {...messages.exampleTitle} />
                </Typography>
                <Typography color="text.secondary" sx={{ mb: 1 }} variant="body2">
                  <FormattedMessage {...messages.exampleHint} />
                </Typography>
                <CodeBlock code={tokenRequestCurl(credentials.clientId, scopes)} language="bash" />
              </Box>
            </Stack>
          </DialogContent>
          <Divider />
          <DialogActions>
            <Button onClick={() => onTestToken(credentials)} variant="outlined">
              <FormattedMessage {...messages.testToken} />
            </Button>
            <Button onClick={onDone} variant="contained">
              <FormattedMessage {...messages.done} />
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}
