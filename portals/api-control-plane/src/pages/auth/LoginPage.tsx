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
  alpha,
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  FormLabel,
  InputAdornment,
  Link,
  List,
  ListItem,
  ListItemAvatar,
  ListItemText,
  Stack,
  TextField,
  type Theme,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Activity,
  ArrowRight,
  Lock,
  PencilRuler,
  Rocket,
  ShieldCheck,
  Sparkles,
  User,
} from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { ChangeEvent, KeyboardEvent, useEffect, useMemo, useRef, useState } from 'react';
import { defineMessages, FormattedMessage, type MessageDescriptor, useIntl } from 'react-intl';
import { Navigate, useLocation } from 'react-router-dom';

import { runtimeConfig } from '../../config/runtime';
import { useAuth } from '../../contexts/auth/AuthProvider';
import { ambientGlowSx, hairline } from '../../theme/receipes';

const messages = defineMessages({
  brandLockup: {
    id: 'apiControlPlane.pages.auth.LoginPage.brandLockup',
    defaultMessage: '<brand>WSO2</brand> API Platform',
    description:
      'Compact brand lockup shown above the sign-in card on narrow screens. WSO2 is the company name and stays as-is; the brand tag only styles it.',
  },
  brandName: {
    id: 'apiControlPlane.pages.auth.LoginPage.brandName',
    defaultMessage: 'WSO2',
    description: 'Company name in the brand lockup. Do not translate.',
  },
  browserUnsupported: {
    id: 'apiControlPlane.pages.auth.LoginPage.browserUnsupported',
    defaultMessage: 'This console is optimized for Google Chrome and Mozilla Firefox.',
  },
  continueToConsole: {
    id: 'apiControlPlane.pages.auth.LoginPage.continueToConsole',
    defaultMessage: 'Continue to your API Platform console.',
  },
  eyebrow: {
    id: 'apiControlPlane.pages.auth.LoginPage.eyebrow',
    defaultMessage: 'AI-NATIVE · SCALABLE SAAS',
    description:
      'Marketing label above the headline, set in upper case. Keep it short — it renders inside a small pill.',
  },
  featureDeploy: {
    id: 'apiControlPlane.pages.auth.LoginPage.featureDeploy',
    defaultMessage: 'Deploy and manage with zero friction',
  },
  featureDesign: {
    id: 'apiControlPlane.pages.auth.LoginPage.featureDesign',
    defaultMessage: 'Design API proxies or MCP servers',
  },
  featureMonitor: {
    id: 'apiControlPlane.pages.auth.LoginPage.featureMonitor',
    defaultMessage: 'Monitor, test, and analyze performance',
  },
  featurePublish: {
    id: 'apiControlPlane.pages.auth.LoginPage.featurePublish',
    defaultMessage: 'Publish faster with AI-powered capabilities',
  },
  featureSecure: {
    id: 'apiControlPlane.pages.auth.LoginPage.featureSecure',
    defaultMessage: 'Secure and govern traffic',
  },
  headline: {
    id: 'apiControlPlane.pages.auth.LoginPage.headline',
    defaultMessage: 'Manage, secure, and scale your <accent>APIs & MCP servers</accent>',
    description:
      'Marketing headline. The accent tag marks the phrase painted in the brand gradient — keep it around the equivalent phrase in the translation.',
  },
  homePageLink: {
    id: 'apiControlPlane.pages.auth.LoginPage.homePageLink',
    defaultMessage: 'More details at <link>wso2.com/api-platform</link>',
    description:
      'Footer line under the sign-in form. The link tag wraps the product home page address, which stays as-is.',
  },
  invitationVerified: {
    id: 'apiControlPlane.pages.auth.LoginPage.invitationVerified',
    defaultMessage: 'Invitation verified for {organization}. Sign in to accept it.',
  },
  oidcAccessDenied: {
    id: 'apiControlPlane.pages.auth.LoginPage.oidcAccessDenied',
    defaultMessage: 'Access was denied.',
  },
  oidcAuthFailed: {
    id: 'apiControlPlane.pages.auth.LoginPage.oidcAuthFailed',
    defaultMessage: 'Unable to complete sign in. Please try again.',
  },
  oidcGenericError: {
    id: 'apiControlPlane.pages.auth.LoginPage.oidcGenericError',
    defaultMessage: 'Unable to complete sign in.',
    description:
      'Shown when the identity provider returned a failure code this console does not recognise.',
  },
  oidcSessionFailed: {
    id: 'apiControlPlane.pages.auth.LoginPage.oidcSessionFailed',
    defaultMessage: 'Unable to establish a session. Please try again.',
  },
  passwordHide: {
    id: 'apiControlPlane.pages.auth.LoginPage.passwordHide',
    defaultMessage: 'Hide',
    description: 'Toggle that masks the password again. Command, not a noun.',
  },
  passwordLabel: {
    id: 'apiControlPlane.pages.auth.LoginPage.passwordLabel',
    defaultMessage: 'Password',
  },
  passwordPlaceholder: {
    id: 'apiControlPlane.pages.auth.LoginPage.passwordPlaceholder',
    defaultMessage: 'Enter your password',
  },
  passwordShow: {
    id: 'apiControlPlane.pages.auth.LoginPage.passwordShow',
    defaultMessage: 'Show',
    description: 'Toggle that reveals the typed password. Command, not a noun.',
  },
  privacyPolicy: {
    id: 'apiControlPlane.pages.auth.LoginPage.privacyPolicy',
    defaultMessage: 'Privacy policy',
  },
  productName: {
    id: 'apiControlPlane.pages.auth.LoginPage.productName',
    defaultMessage: '<emphasis>API</emphasis> Platform',
    description:
      'Product name under the WSO2 wordmark. The emphasis tag marks the part set in the stronger weight.',
  },
  sessionExpired: {
    id: 'apiControlPlane.pages.auth.LoginPage.sessionExpired',
    defaultMessage: 'Your session has expired. Sign in again to continue.',
  },
  signInAction: {
    id: 'apiControlPlane.pages.auth.LoginPage.signInAction',
    defaultMessage: 'Sign in',
    description: 'Label of the button that submits the sign-in form. A command.',
  },
  tagline: {
    id: 'apiControlPlane.pages.auth.LoginPage.tagline',
    defaultMessage:
      'A comprehensive platform for designing, deploying, governing, and optimizing APIs and MCP servers — end to end.',
  },
  termsOfUse: {
    id: 'apiControlPlane.pages.auth.LoginPage.termsOfUse',
    defaultMessage: 'Terms of use',
  },
  title: {
    id: 'apiControlPlane.pages.auth.LoginPage.title',
    defaultMessage: 'Sign in',
    description: 'Heading of the sign-in card. A noun phrase naming the page.',
  },
  usernameLabel: {
    id: 'apiControlPlane.pages.auth.LoginPage.usernameLabel',
    defaultMessage: 'Username',
  },
  usernamePlaceholder: {
    id: 'apiControlPlane.pages.auth.LoginPage.usernamePlaceholder',
    defaultMessage: 'Enter your username',
  },
});

type LoginLocationState = {
  confirmationKey?: string;
  confirmationOrg?: string;
  displayOrgName?: string;
  from?: { pathname?: string };
  returnToUrl?: string;
};

const FEATURES: { icon: ReactNode; label: MessageDescriptor }[] = [
  { icon: <PencilRuler size={18} />, label: messages.featureDesign },
  { icon: <Rocket size={18} />, label: messages.featureDeploy },
  { icon: <ShieldCheck size={18} />, label: messages.featureSecure },
  { icon: <Activity size={18} />, label: messages.featureMonitor },
  { icon: <Sparkles size={18} />, label: messages.featurePublish },
];

const isSupportedBrowser = () => {
  if (typeof navigator === 'undefined') return true;
  const userAgent = navigator.userAgent;
  return /Chrome|Chromium|Firefox/.test(userAgent) && !/Edg\//.test(userAgent);
};

// The BFF's OIDC callback redirects back here with ?error=<code> on failure
// (state/nonce mismatch, code exchange failure, or an error the IdP itself
// returned) — there is no in-app exception to catch, since login() is a full
// page navigation to the BFF.
const OIDC_ERROR_MESSAGES: Record<string, MessageDescriptor> = {
  auth_failed: messages.oidcAuthFailed,
  session_failed: messages.oidcSessionFailed,
  access_denied: messages.oidcAccessDenied,
};

/**
 * The product's brand gradient painted onto text rather than a surface — the
 * headline's emphasised phrase. `theme.gradient.primary` is the same token the
 * primary call to action uses, so the two always agree.
 */
const gradientTextSx = (theme: Theme) =>
  ({
    backgroundClip: 'text',
    backgroundImage: theme.gradient.primary,
    color: 'transparent',
    WebkitBackgroundClip: 'text',
  }) as const;

/**
 * The single primary action on this page (sign in, in whichever auth mode is
 * configured), lifted above a plain `contained` button with the brand gradient.
 * The disabled state drops back to the theme's own treatment, so a disabled
 * button never reads as available.
 */
const signInButtonSx = (theme: Theme) =>
  ({
    backgroundImage: theme.gradient.primary,
    fontSize: theme.typography.h5.fontSize,
    py: 1.5,
    '&:hover': { backgroundImage: theme.gradient.primary, filter: 'brightness(1.06)' },
    '&.Mui-disabled': { backgroundImage: 'none' },
  }) as const;

/** The label above a field, rather than a floating `TextField` label. */
const fieldLabelSx = { color: 'text.primary', fontWeight: 500 } as const;

/** Tile behind a feature icon in the left panel's list. */
const featureIconSx = (theme: Theme) =>
  ({
    bgcolor: alpha(theme.palette.primary.main, 0.1),
    color: 'primary.main',
    height: 40,
    width: 40,
  }) as const;

export function LoginPage() {
  const auth = useAuth();
  const intl = useIntl();
  const location = useLocation();
  const state = (location.state || {}) as LoginLocationState;
  const queryParams = useMemo(() => new URLSearchParams(location.search), [location.search]);
  const from = state.returnToUrl || state.from?.pathname || '/';
  const isInvitation = Boolean(state.confirmationOrg && state.confirmationKey);
  const isBrowserSupported = isSupportedBrowser();
  const isOidcMode = runtimeConfig.authMode === 'oidc';
  const autoRedirectStarted = useRef(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const oidcErrorCode = queryParams.get('error');
  const oidcError = oidcErrorCode
    ? intl.formatMessage(OIDC_ERROR_MESSAGES[oidcErrorCode] || messages.oidcGenericError)
    : undefined;

  // Whether the card shows an alert block at all. Kept as one flag so the
  // `Stack` wrapping the alerts is skipped entirely when there is nothing to
  // show — an empty child would still take a gap in the card's stack.
  const hasAlerts = Boolean(
    !isBrowserSupported || (isInvitation && state.displayOrgName) || oidcError || auth.error,
  );

  const shouldAutoRedirect = !auth.isAuthenticated && isOidcMode && !isInvitation && !oidcError;

  // In OIDC mode this page is just a redirect step to the IdP, so skip it and
  // go straight there — unless there's an invitation to show or a failed
  // attempt just redirected back here (retrying immediately would loop). Runs
  // in an effect, not the render body: auth.login() navigates the page and
  // mutates a ref, and React 19 requires render to stay pure — under
  // StrictMode's double-render (or a compiler that reorders render output)
  // a render-phase navigation is not reliable.
  useEffect(() => {
    if (!shouldAutoRedirect || autoRedirectStarted.current) return;
    autoRedirectStarted.current = true;
    auth.login(from);
  }, [auth, from, shouldAutoRedirect]);

  if (auth.isAuthenticated) return <Navigate to={from} replace />;

  const message = auth.status === 'expired' ? messages.sessionExpired : messages.continueToConsole;

  const accent = (chunks: ReactNode) => (
    <Box component="span" sx={gradientTextSx}>
      {chunks}
    </Box>
  );

  const brand = (chunks: ReactNode) => (
    <Box component="span" sx={{ color: 'text.primary', fontWeight: 700 }}>
      {chunks}
    </Box>
  );

  const emphasis = (chunks: ReactNode) => (
    <Box component="span" sx={{ color: 'text.primary', fontWeight: 600 }}>
      {chunks}
    </Box>
  );

  const link = (chunks: ReactNode) => (
    <Link href={runtimeConfig.apiPlatformHomePage} target="_blank">
      {chunks}
    </Link>
  );

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>, action: () => void) => {
    if (event.key === 'Enter') action();
  };

  const startBasicLogin = async () => {
    if (!username.trim() || !password) return;
    setSubmitting(true);
    await auth.loginWithCredentials(username.trim(), password);
    setSubmitting(false);
  };

  return (
    <Box
      sx={{
        bgcolor: 'background.default',
        display: 'flex',
        flexWrap: { md: 'nowrap', xs: 'wrap' },
        minHeight: '100vh',
        // The washes below bleed past the viewport edges rather than scrolling it.
        overflow: 'hidden',
        position: 'relative',
        width: '100%',
      }}
    >
      {/* Ambient wash — decorative only, so it stays out of the a11y tree. */}
      <Box
        aria-hidden
        sx={(theme) => ({
          ...ambientGlowSx,
          bgcolor: alpha(theme.palette.info.light, 0.22),
          height: 460,
          right: theme.spacing(-10),
          top: theme.spacing(-14),
          width: 560,
        })}
      />
      <Box
        aria-hidden
        sx={(theme) => ({
          ...ambientGlowSx,
          bgcolor: alpha(theme.palette.primary.light, 0.18),
          bottom: theme.spacing(-16),
          height: 480,
          left: theme.spacing(-12),
          width: 560,
        })}
      />

      {/* LEFT — marketing */}
      <Stack
        spacing={4}
        sx={{
          display: { md: 'flex', xs: 'none' },
          flex: '1 1 50%',
          justifyContent: 'center',
          minWidth: 0,
          p: { lg: 10, md: 6 },
          position: 'relative',
        }}
      >
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Avatar
            sx={(theme) => ({
              bgcolor: alpha(theme.palette.primary.main, 0.1),
              color: 'primary.main',
              height: 56,
              width: 56,
            })}
          >
            <Activity size={26} />
          </Avatar>
          <Box>
            <Typography sx={{ fontWeight: 700, letterSpacing: '-0.5px' }} variant="h2">
              <FormattedMessage {...messages.brandName} />
            </Typography>
            <Typography color="text.secondary" variant="subtitle1">
              <FormattedMessage {...messages.productName} values={{ emphasis }} />
            </Typography>
          </Box>
        </Stack>

        <Stack spacing={2.5} sx={{ maxWidth: 620 }}>
          <Chip
            color="info"
            icon={<Box sx={{ bgcolor: 'info.main', borderRadius: '50%', height: 6, width: 6 }} />}
            label={intl.formatMessage(messages.eyebrow)}
            size="small"
            sx={{
              alignSelf: 'flex-start',
              fontWeight: 600,
              height: 30,
              letterSpacing: '1.5px',
              px: 0.5,
            }}
            variant="outlined"
          />
          <Typography
            component="h1"
            sx={{
              fontSize: { lg: '3.5rem', md: '2.75rem' },
              fontWeight: 500,
              letterSpacing: '-1.5px',
              lineHeight: 1.1,
            }}
            variant="h1"
          >
            <FormattedMessage {...messages.headline} values={{ accent }} />
          </Typography>
          <Typography
            color="text.secondary"
            sx={{ fontSize: '1.125rem', lineHeight: 1.6, maxWidth: 490 }}
            variant="body1"
          >
            <FormattedMessage {...messages.tagline} />
          </Typography>
        </Stack>

        <Divider sx={{ maxWidth: 620 }} />

        <List disablePadding sx={{ maxWidth: 620 }}>
          {FEATURES.map((feature) => (
            <ListItem disableGutters key={feature.label.id} sx={{ py: 1 }}>
              <ListItemAvatar sx={{ minWidth: 0, mr: 2.5 }}>
                <Avatar sx={featureIconSx} variant="rounded">
                  {feature.icon}
                </Avatar>
              </ListItemAvatar>
              <ListItemText
                primary={<FormattedMessage {...feature.label} />}
                slotProps={{ primary: { sx: { fontSize: '1.0625rem' } } }}
              />
            </ListItem>
          ))}
        </List>
      </Stack>

      {/* RIGHT — sign-in card */}
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          flex: { md: '1 1 50%', xs: '1 1 100%' },
          justifyContent: 'center',
          p: { md: 6, xs: 3 },
          position: 'relative',
          width: '100%',
        }}
      >
        <Card
          elevation={0}
          sx={(theme) => ({
            bgcolor: 'background.paper',
            border: hairline(theme),
            borderColor: 'divider',
            borderRadius: 2,
            boxShadow: (theme: Theme) =>
              `0 1px 2px ${alpha(theme.palette.common.black, 0.04)}, 0 24px 56px ${alpha(
                theme.palette.common.black,
                0.1,
              )}`,
            maxWidth: 540,
            width: '100%',
          })}
        >
          <CardContent sx={{ p: { sm: 5, xs: 3 } }}>
            <Stack spacing={3}>
              {/* mobile brand lockup */}
              <Stack
                direction="row"
                spacing={1.25}
                sx={{ alignItems: 'center', display: { md: 'none', xs: 'flex' } }}
              >
                <Activity size={24} />
                <Typography color="text.secondary" variant="subtitle1">
                  <FormattedMessage {...messages.brandLockup} values={{ brand }} />
                </Typography>
              </Stack>

              {/* header */}
              <Stack spacing={0.75}>
                <Typography sx={{ letterSpacing: '-0.5px' }} variant="h1">
                  <FormattedMessage {...messages.title} />
                </Typography>
                <Typography color="text.secondary" variant="subtitle1">
                  <FormattedMessage {...message} />
                </Typography>
              </Stack>

              {/* alerts */}
              {hasAlerts && (
                <Stack spacing={1.5}>
                  {!isBrowserSupported && (
                    <Alert severity="warning">
                      <FormattedMessage {...messages.browserUnsupported} />
                    </Alert>
                  )}
                  {isInvitation && state.displayOrgName && (
                    <Alert severity="success">
                      <FormattedMessage
                        {...messages.invitationVerified}
                        values={{ organization: state.displayOrgName }}
                      />
                    </Alert>
                  )}
                  {oidcError && <Alert severity="error">{oidcError}</Alert>}
                  {auth.error && <Alert severity="error">{auth.error}</Alert>}
                </Stack>
              )}

              {isOidcMode ? (
                <Button
                  endIcon={<ArrowRight size={18} />}
                  fullWidth
                  onClick={() => auth.login(from)}
                  size="large"
                  sx={signInButtonSx}
                  variant="contained"
                >
                  <FormattedMessage {...messages.signInAction} />
                </Button>
              ) : (
                <Stack spacing={2.5}>
                  <Stack spacing={1}>
                    <FormLabel htmlFor="login-username" sx={fieldLabelSx}>
                      <FormattedMessage {...messages.usernameLabel} />
                    </FormLabel>
                    <TextField
                      autoComplete="username"
                      autoFocus
                      fullWidth
                      id="login-username"
                      name="username"
                      onChange={(event: ChangeEvent<HTMLInputElement>) =>
                        setUsername(event.target.value)
                      }
                      onKeyDown={(event: KeyboardEvent<HTMLInputElement>) =>
                        handleKeyDown(event, () => void startBasicLogin())
                      }
                      placeholder={intl.formatMessage(messages.usernamePlaceholder)}
                      slotProps={{
                        input: {
                          startAdornment: (
                            <InputAdornment position="start">
                              <User size={18} />
                            </InputAdornment>
                          ),
                        },
                      }}
                      type="text"
                      value={username}
                    />
                  </Stack>

                  <Stack spacing={1}>
                    <Stack
                      direction="row"
                      sx={{ alignItems: 'center', justifyContent: 'space-between' }}
                    >
                      <FormLabel htmlFor="login-password" sx={fieldLabelSx}>
                        <FormattedMessage {...messages.passwordLabel} />
                      </FormLabel>
                      <Link
                        component="button"
                        onClick={() => setShowPassword((visible) => !visible)}
                        sx={{ color: 'text.secondary', fontWeight: 500 }}
                        type="button"
                        variant="body2"
                      >
                        <FormattedMessage
                          {...(showPassword ? messages.passwordHide : messages.passwordShow)}
                        />
                      </Link>
                    </Stack>
                    <TextField
                      autoComplete="current-password"
                      fullWidth
                      id="login-password"
                      name="password"
                      onChange={(event: ChangeEvent<HTMLInputElement>) =>
                        setPassword(event.target.value)
                      }
                      onKeyDown={(event: KeyboardEvent<HTMLInputElement>) =>
                        handleKeyDown(event, () => void startBasicLogin())
                      }
                      placeholder={intl.formatMessage(messages.passwordPlaceholder)}
                      slotProps={{
                        input: {
                          startAdornment: (
                            <InputAdornment position="start">
                              <Lock size={18} />
                            </InputAdornment>
                          ),
                        },
                      }}
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                    />
                  </Stack>

                  <Button
                    disabled={submitting || !username.trim() || !password}
                    endIcon={<ArrowRight size={18} />}
                    fullWidth
                    onClick={() => void startBasicLogin()}
                    size="large"
                    sx={signInButtonSx}
                    variant="contained"
                  >
                    <FormattedMessage {...messages.signInAction} />
                  </Button>
                </Stack>
              )}

              {/* footer */}
              <Box>
                <Divider sx={{ mb: 2.5 }} />
                <Stack spacing={1.25} sx={{ alignItems: 'center' }}>
                  <Typography color="text.secondary" variant="body2">
                    <FormattedMessage {...messages.homePageLink} values={{ link }} />
                  </Typography>
                  <Stack
                    direction="row"
                    divider={<Divider flexItem orientation="vertical" />}
                    spacing={2}
                    sx={{ alignItems: 'center' }}
                  >
                    <Link
                      color="text.secondary"
                      href={runtimeConfig.privacyPolicyLink}
                      target="_blank"
                      variant="body2"
                    >
                      <FormattedMessage {...messages.privacyPolicy} />
                    </Link>
                    <Link
                      color="text.secondary"
                      href={runtimeConfig.termsOfUseLink}
                      target="_blank"
                      variant="body2"
                    >
                      <FormattedMessage {...messages.termsOfUse} />
                    </Link>
                  </Stack>
                </Stack>
              </Box>
            </Stack>
          </CardContent>
        </Card>
      </Box>
    </Box>
  );
}
