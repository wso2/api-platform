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

import { useEffect, useState, type ReactNode } from 'react';
import { Link as RouterLink, Navigate, useNavigate } from 'react-router-dom';
import {
  Alert,
  AlertTitle,
  Box,
  Breadcrumbs,
  Button,
  Card,
  CardContent,
  Chip,
  CodeBlock,
  Collapse,
  IconButton,
  Link,
  Tooltip,
  Divider,
  LinearProgress,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Check, ChevronDown, Copy, Eye, EyeOff, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ErrorCode, isErrorCode } from '@/api/core/errors';
import { useApiScope } from '@/api/core/scope';
import {
  rolesOf,
  scopesOfRoles,
  useIssueServiceAccountToken,
  useRoles,
  type ServiceAccountTokenResponse,
} from '@/api/resources/serviceAccounts';
import { SecondaryText } from '@/components/common/SecondaryText';
import { useTrackFeature } from '@/featureUsage';
import { Can } from '@/permissions';
import { routes } from '@/routes/paths';
import { useCopy } from './components/useCopy';
import { clearCredentials, peekCredentials } from './createdCredentials';
import { tokenRequestCurl, tokenUseCurl } from './serviceAccountCurl';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.title',
    defaultMessage: 'Service account created',
  },
  warningTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.warningTitle',
    defaultMessage: 'The secret will not be shown again',
  },
  warningBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.warningBody',
    defaultMessage:
      'Store it in your secret manager. If it is lost, regenerate it — which breaks anything still using the old one.',
  },
  clientId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.clientId',
    defaultMessage: 'Client ID',
  },
  clientSecret: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.clientSecret',
    defaultMessage: 'Client secret',
  },
  copied: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.copied',
    defaultMessage: 'Copied',
  },
  copyLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.copyLabel',
    defaultMessage: 'Copy {label}',
  },
  showLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.showLabel',
    defaultMessage: 'Show {label}',
  },
  hideLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.hideLabel',
    defaultMessage: 'Hide {label}',
  },
  copyEnv: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.copyEnv',
    defaultMessage: 'Copy both as .env',
  },
  exampleTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.exampleTitle',
    defaultMessage: 'Get an access token',
  },
  exampleHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.exampleHint',
    defaultMessage:
      'Set PLATFORM_API_URL to the API Platform address and CLIENT_SECRET from your secret store. The secret is never written into this example.',
    description: 'PLATFORM_API_URL and CLIENT_SECRET are shell variable names — do not translate.',
  },
  testTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.testTitle',
    defaultMessage: 'Get a test token',
  },
  testBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.testBody',
    defaultMessage:
      'Check the credentials work. Uses the secret above, so there’s nothing to paste. Getting a token updates “Last token issued” and is logged.',
  },
  crumbList: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.crumbList',
    defaultMessage: 'Service accounts',
  },
  crumbGetToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.crumbGetToken',
    defaultMessage: 'Get token',
  },
  crumbLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.crumbLabel',
    defaultMessage: 'Breadcrumb',
  },
  retry: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.retry',
    defaultMessage: 'Retry',
  },
  getToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.getToken',
    defaultMessage: 'Get token',
  },
  getAnother: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.getAnother',
    defaultMessage: 'Get another',
  },
  getting: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.getting',
    defaultMessage: 'Getting token…',
  },
  works: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.works',
    defaultMessage:
      'The credentials work. Token issued just now, and “Last token issued” is updated.',
  },
  accessToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.accessToken',
    defaultMessage: 'Access token',
  },
  expiresIn: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.expiresIn',
    defaultMessage: 'Expires in {minutes}:{seconds}',
    description:
      'Countdown to token expiry, e.g. "Expires in 14:52". {seconds} is always two digits.',
  },
  expired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.expired',
    defaultMessage: 'Expired',
  },
  scopesInToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.scopesInToken',
    defaultMessage: '{count, plural, one {scope in this token} other {scopes in this token}}',
  },
  useIt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.useIt',
    defaultMessage: 'Use it',
  },
  revokeNote: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.revokeNote',
    defaultMessage:
      'This token cannot be revoked on its own. Disabling the account or regenerating its secret stops it.',
  },
  rejected: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.rejected',
    defaultMessage: 'Could not get a token. Check the secret, and that the account is enabled.',
  },
  failed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.failed',
    defaultMessage: 'Could not get a token. Try again in a moment.',
  },
  scopeFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.scopeFailed',
    defaultMessage:
      "The account's roles do not grant the requested scopes. Save the secret before you leave this page, then check the role-to-scope mapping.",
  },
  rolesFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.rolesFailed',
    defaultMessage:
      "Could not load the roles, so the token's scopes are unknown. Retry without leaving this page, so the secret stays on screen.",
  },
  done: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.ServiceAccountCreatedPage.done',
    defaultMessage: 'Done',
  },
});

const mono = { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' } as const;

/** Copy to the clipboard; the icon turns into a tick once the value is copied. */
function CopyIconButton({ label, value }: { label: string; value: string }) {
  const intl = useIntl();
  const { copied, copy } = useCopy();
  const text = intl.formatMessage(copied ? messages.copied : messages.copyLabel, { label });
  return (
    <Tooltip arrow title={text}>
      <IconButton aria-label={text} onClick={() => copy(value)} size="small">
        {copied ? <Check size={16} /> : <Copy size={16} />}
      </IconButton>
    </Tooltip>
  );
}

/** A code example with a copy button in its corner. */
function CopyableCode({ code, label }: { code: string; label: string }) {
  return (
    <Box sx={{ position: 'relative' }}>
      <CodeBlock code={code} language="bash" />
      <Box sx={{ position: 'absolute', right: 8, top: 8 }}>
        <CopyIconButton label={label} value={code} />
      </Box>
    </Box>
  );
}

/** A read-only value with eye and copy icon buttons, like the design. */
function ValueField({
  aside,
  id,
  label,
  masked = false,
  value,
}: {
  /** Shown opposite the label, e.g. when a token expires. */
  aside?: ReactNode;
  id: string;
  label: string;
  masked?: boolean;
  value: string;
}) {
  const intl = useIntl();
  const [revealed, setRevealed] = useState(!masked);
  const revealLabel = intl.formatMessage(revealed ? messages.hideLabel : messages.showLabel, {
    label,
  });

  return (
    <Box>
      <Stack alignItems="baseline" direction="row" justifyContent="space-between" sx={{ mb: 0.75 }}>
        <Typography component="label" htmlFor={id} variant="body2">
          {label}
        </Typography>
        {aside}
      </Stack>
      <Stack
        alignItems="center"
        direction="row"
        spacing={0.5}
        sx={{
          bgcolor: 'background.default',
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          px: 2,
          py: 0.5,
        }}
      >
        <Box
          component="output"
          id={id}
          sx={{ ...mono, flex: 1, minWidth: 0, overflowWrap: 'anywhere', py: 0.5 }}
        >
          {revealed ? value : '•'.repeat(Math.min(value.length, 48))}
        </Box>
        {masked && (
          <Tooltip arrow title={revealLabel}>
            <IconButton
              aria-label={revealLabel}
              onClick={() => setRevealed((current) => !current)}
              size="small"
            >
              {revealed ? <EyeOff size={16} /> : <Eye size={16} />}
            </IconButton>
          </Tooltip>
        )}
        <CopyIconButton label={label} value={value} />
      </Stack>
    </Box>
  );
}

type Issued = ServiceAccountTokenResponse & { expiresAt: number };

/** Shown once after a service account is created: its credentials and a test token. */
export function ServiceAccountCreatedPage() {
  const { org } = useApiScope();
  return (
    <Can do="createServiceAccount" fallback={<ForgetAndLeave to={routes.serviceAccounts(org)} />}>
      <CreatedView key={org} />
    </Can>
  );
}

/** Leaves for the list, dropping any held secret first so a later visit cannot show it. */
function ForgetAndLeave({ to }: { to: string }) {
  useEffect(() => clearCredentials(), []);
  return <Navigate replace to={to} />;
}

function CreatedView() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { org, orgId } = useApiScope();
  const listHref = routes.serviceAccounts(org);

  // Read once; the module copy is dropped when the page goes away.
  const [credentials] = useState(() => peekCredentials(orgId ?? ''));
  useEffect(() => clearCredentials, []);

  const rolesQuery = useRoles();
  const mutation = useIssueServiceAccountToken();
  const trackFeature = useTrackFeature();
  const [issued, setIssued] = useState<Issued | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [scopesOpen, setScopesOpen] = useState(false);
  const { copied: envCopied, copy: copyEnv } = useCopy();

  useEffect(() => {
    if (!issued) return;
    const timer = window.setInterval(() => {
      const current = Date.now();
      setNow(current);
      // Nothing left to count down once the token has expired.
      if (current >= issued.expiresAt) window.clearInterval(timer);
    }, 1000);
    return () => window.clearInterval(timer);
  }, [issued]);

  // A reload or a direct visit has no secret to show; the list is the only sensible place.
  if (!credentials) return <ForgetAndLeave to={listHref} />;

  const { clientId, clientSecret, serviceAccount } = credentials;
  const scopes = scopesOfRoles(rolesOf(serviceAccount), rolesQuery.data?.list ?? []);
  const scopesReady = rolesQuery.isSuccess;

  const remaining = issued ? Math.max(0, Math.round((issued.expiresAt - now) / 1000)) : 0;
  const expired = issued !== null && remaining === 0;
  const total = issued?.expires_in ?? 0;

  const errorMessage = !mutation.error
    ? null
    : isErrorCode(mutation.error, 'SERVICE_ACCOUNT_INVALID_SCOPE')
      ? intl.formatMessage(messages.scopeFailed)
      : isErrorCode(mutation.error, ErrorCode.UNAUTHORIZED)
        ? intl.formatMessage(messages.rejected)
        : intl.formatMessage(messages.failed);

  const getToken = () => {
    if (!scopesReady || mutation.isPending) return;
    // A failed retry must not leave the previous token on screen.
    setIssued(null);
    trackFeature('apip.security.service-account-test-token-get-click', {});
    mutation.mutate(
      {
        client_id: clientId,
        client_secret: clientSecret,
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

  const issuedScopes = issued?.scope ? issued.scope.split(' ') : [];

  return (
    <Box sx={{ maxWidth: 840, mx: 'auto', width: '100%' }}>
      <Breadcrumbs aria-label={intl.formatMessage(messages.crumbLabel)} sx={{ mb: 2 }}>
        <Link
          color="inherit"
          component={RouterLink}
          to={listHref}
          underline="hover"
          variant="body2"
        >
          <FormattedMessage {...messages.crumbList} />
        </Link>
        <Typography color="text.secondary" variant="body2">
          {serviceAccount.displayName}
        </Typography>
        <Typography color="text.primary" variant="body2">
          <FormattedMessage {...messages.crumbGetToken} />
        </Typography>
      </Breadcrumbs>
      <Stack alignItems="center" direction="row" spacing={2} sx={{ mb: 3 }}>
        <Box
          aria-hidden
          sx={(theme) => ({
            alignItems: 'center',
            bgcolor: `color-mix(in srgb, ${theme.palette.success.main} 18%, transparent)`,
            borderRadius: '50%',
            color: 'success.main',
            display: 'flex',
            flexShrink: 0,
            height: 40,
            justifyContent: 'center',
            width: 40,
          })}
        >
          <Check size={20} />
        </Box>
        <Typography component="h1" sx={{ fontWeight: 400 }} variant="h4">
          <Box component="span" sx={{ fontWeight: 700, mr: 1 }}>
            {serviceAccount.displayName}
          </Box>
          <FormattedMessage {...messages.title} />
        </Typography>
      </Stack>

      <Stack spacing={3}>
        <Card>
          <CardContent>
            <Stack spacing={2.5}>
              <Alert icon={<TriangleAlert size={18} />} severity="warning">
                <AlertTitle sx={{ fontWeight: 600 }}>
                  <FormattedMessage {...messages.warningTitle} />
                </AlertTitle>
                <FormattedMessage {...messages.warningBody} />
              </Alert>
              <ValueField
                id="service-account-client-id"
                label={intl.formatMessage(messages.clientId)}
                value={clientId}
              />
              <ValueField
                id="service-account-client-secret"
                label={intl.formatMessage(messages.clientSecret)}
                masked
                value={clientSecret}
              />
              <Box sx={{ mt: -1.5, textAlign: 'right' }}>
                <Button
                  onClick={() => copyEnv(`CLIENT_ID=${clientId}\nCLIENT_SECRET=${clientSecret}`)}
                  size="small"
                  startIcon={envCopied ? <Check size={14} /> : undefined}
                  variant="outlined"
                >
                  <FormattedMessage {...(envCopied ? messages.copied : messages.copyEnv)} />
                </Button>
              </Box>
              <Divider />
              <Box>
                <Typography sx={{ fontWeight: 600 }} variant="body2">
                  <FormattedMessage {...messages.exampleTitle} />
                </Typography>
                <SecondaryText sx={{ mb: 1.5, mt: 0.5 }} variant="body2">
                  <FormattedMessage {...messages.exampleHint} />
                </SecondaryText>
                <CopyableCode
                  code={tokenRequestCurl(clientId, scopes)}
                  label={intl.formatMessage(messages.exampleTitle)}
                />
              </Box>
            </Stack>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <Stack spacing={2.5}>
              <Stack
                alignItems={{ sm: 'center' }}
                direction={{ sm: 'row', xs: 'column' }}
                justifyContent="space-between"
                spacing={2}
              >
                <Box>
                  <Typography sx={{ fontWeight: 600 }} variant="subtitle1">
                    <FormattedMessage {...messages.testTitle} />
                  </Typography>
                  <SecondaryText sx={{ mt: 0.5 }} variant="body2">
                    <FormattedMessage {...messages.testBody} />
                  </SecondaryText>
                </Box>
                {!issued && (
                  <Button
                    disabled={!scopesReady || mutation.isPending}
                    onClick={getToken}
                    sx={{ flexShrink: 0 }}
                    variant="contained"
                  >
                    <FormattedMessage
                      {...(mutation.isPending ? messages.getting : messages.getToken)}
                    />
                  </Button>
                )}
              </Stack>

              {rolesQuery.isError && (
                <Alert
                  action={
                    <Button
                      color="inherit"
                      disabled={rolesQuery.isFetching}
                      onClick={() => void rolesQuery.refetch()}
                      size="small"
                    >
                      <FormattedMessage {...messages.retry} />
                    </Button>
                  }
                  severity="error"
                >
                  <FormattedMessage {...messages.rolesFailed} />
                </Alert>
              )}
              {errorMessage && <Alert severity="error">{errorMessage}</Alert>}

              {issued && (
                <>
                  <Alert severity="success">
                    <FormattedMessage {...messages.works} />
                  </Alert>
                  <Box>
                    <ValueField
                      aside={
                        <Typography
                          color={expired ? 'error' : 'success.main'}
                          sx={mono}
                          variant="caption"
                        >
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
                      }
                      id="service-account-access-token"
                      label={intl.formatMessage(messages.accessToken)}
                      masked
                      value={issued.access_token}
                    />
                    <LinearProgress
                      color={expired ? 'error' : 'success'}
                      sx={{ borderRadius: 1, mt: 1 }}
                      value={total > 0 ? (remaining / total) * 100 : 0}
                      variant="determinate"
                    />
                  </Box>
                  {issuedScopes.length > 0 && (
                    <Box>
                      <Button
                        aria-expanded={scopesOpen}
                        color="inherit"
                        endIcon={
                          <ChevronDown
                            size={14}
                            style={{ transform: scopesOpen ? 'rotate(180deg)' : undefined }}
                          />
                        }
                        onClick={() => setScopesOpen((open) => !open)}
                        size="small"
                        sx={{ fontWeight: 400, ml: -1 }}
                      >
                        <Typography
                          component="span"
                          sx={{ fontWeight: 600, mr: 0.75 }}
                          variant="body2"
                        >
                          {issuedScopes.length}
                        </Typography>
                        <FormattedMessage
                          {...messages.scopesInToken}
                          values={{ count: issuedScopes.length }}
                        />
                      </Button>
                      <Collapse in={scopesOpen} unmountOnExit>
                        <Stack direction="row" flexWrap="wrap" gap={0.5} sx={{ pt: 1 }}>
                          {issuedScopes.map((scope) => (
                            <Chip
                              key={scope}
                              label={scope}
                              size="small"
                              sx={{ ...mono, borderRadius: '4px' }}
                              variant="outlined"
                            />
                          ))}
                        </Stack>
                      </Collapse>
                    </Box>
                  )}
                  <Box>
                    <Typography sx={{ fontWeight: 600, mb: 1 }} variant="body2">
                      <FormattedMessage {...messages.useIt} />
                    </Typography>
                    <CopyableCode
                      code={tokenUseCurl()}
                      label={intl.formatMessage(messages.useIt)}
                    />
                  </Box>
                  <Alert icon={<TriangleAlert size={18} />} severity="warning" sx={{ mt: -1 }}>
                    <FormattedMessage {...messages.revokeNote} />
                  </Alert>
                  <Box>
                    <Button
                      disabled={!scopesReady || mutation.isPending}
                      onClick={getToken}
                      variant="outlined"
                    >
                      <FormattedMessage
                        {...(mutation.isPending ? messages.getting : messages.getAnother)}
                      />
                    </Button>
                  </Box>
                </>
              )}
            </Stack>
          </CardContent>
        </Card>

        <Stack direction="row" justifyContent="flex-end">
          <Button
            onClick={() => {
              clearCredentials();
              navigate(listHref);
            }}
            variant="outlined"
          >
            <FormattedMessage {...messages.done} />
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}
