import { Alert, Box, Button, Link, Typography } from '@wso2/oxygen-ui';
import {
  Activity,
  ArrowRight,
  PencilRuler,
  Rocket,
  ShieldCheck,
  Sparkles,
} from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { ChangeEvent, KeyboardEvent, useMemo, useRef, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { runtimeConfig } from '../../config/runtime';
import { useAuth } from './AuthProvider';

type LoginLocationState = {
  confirmationKey?: string;
  confirmationOrg?: string;
  displayOrgName?: string;
  from?: { pathname?: string };
  returnToUrl?: string;
};

const FEATURES: { icon: ReactNode; label: string }[] = [
  { icon: <PencilRuler size={18} />, label: 'Design API proxies or MCP servers' },
  { icon: <Rocket size={18} />, label: 'Deploy and manage with zero friction' },
  { icon: <ShieldCheck size={18} />, label: 'Secure and govern traffic' },
  { icon: <Activity size={18} />, label: 'Monitor, test, and analyze performance' },
  { icon: <Sparkles size={18} />, label: 'Publish faster with AI-powered capabilities' },
];

const ORANGE_GRADIENT = 'linear-gradient(90deg,#F47B20,#EF4223)';

const isSupportedBrowser = () => {
  if (typeof navigator === 'undefined') return true;
  const userAgent = navigator.userAgent;
  return /Chrome|Chromium|Firefox/.test(userAgent) && !/Edg\//.test(userAgent);
};

// The BFF's OIDC callback redirects back here with ?error=<code> on failure
// (state/nonce mismatch, code exchange failure, or an error the IdP itself
// returned) — there is no in-app exception to catch, since login() is a full
// page navigation to the BFF.
const OIDC_ERROR_MESSAGES: Record<string, string> = {
  auth_failed: 'Unable to complete sign in. Please try again.',
  session_failed: 'Unable to establish a session. Please try again.',
  access_denied: 'Access was denied.',
};

const fieldSx = {
  bgcolor: 'rgba(255,255,255,0.04)',
  border: '1px solid rgba(255,255,255,0.14)',
  borderRadius: '10px',
  color: '#fff',
  fontFamily: 'inherit',
  fontSize: 14,
  fontWeight: 500,
  height: 44,
  outline: 'none',
  px: '14px',
  width: '100%',
  '&::placeholder': { color: 'rgba(255,255,255,0.4)' },
  '&:focus': {
    borderColor: '#FF7300',
    boxShadow: '0 0 0 3px rgba(255,115,0,0.22)',
  },
};

const primaryCtaSx = {
  background: ORANGE_GRADIENT,
  borderRadius: '23px',
  boxShadow: '0 8px 22px rgba(239,66,35,0.32)',
  color: '#fff',
  fontSize: 14,
  fontWeight: 600,
  gap: 1,
  height: 46,
  textTransform: 'none',
  width: '100%',
  '&:hover': { background: ORANGE_GRADIENT, opacity: 0.92 },
  '&.Mui-disabled': { background: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.4)' },
};

export function LoginPage() {
  const auth = useAuth();
  const location = useLocation();
  const state = (location.state || {}) as LoginLocationState;
  const queryParams = useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );
  const from = state.returnToUrl || state.from?.pathname || '/';
  const isInvitation = Boolean(state.confirmationOrg && state.confirmationKey);
  const isBrowserSupported = isSupportedBrowser();
  const isOidcMode = runtimeConfig.authMode === 'oidc';
  const autoRedirectStarted = useRef(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const oidcErrorCode = queryParams.get('error');
  const oidcError = oidcErrorCode
    ? OIDC_ERROR_MESSAGES[oidcErrorCode] || 'Unable to complete sign in.'
    : undefined;

  if (auth.isAuthenticated) return <Navigate to={from} replace />;

  // In OIDC mode this page is just a redirect step to the IdP, so skip it and
  // go straight there — unless there's an invitation to show or a failed
  // attempt just redirected back here (retrying immediately would loop).
  if (
    isOidcMode &&
    !isInvitation &&
    !oidcError &&
    !autoRedirectStarted.current
  ) {
    autoRedirectStarted.current = true;
    auth.login(from);
  }

  const message = auth.status === 'expired'
    ? 'Your session has expired. Sign in again to continue.'
    : auth.status === 'forbidden'
      ? 'You do not have access to this console.'
      : 'Continue to your API Platform console.';

  const handleKeyDown = (
    event: KeyboardEvent<HTMLInputElement>,
    action: () => void
  ) => {
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
        background:
          'radial-gradient(900px 600px at 85% 8%, rgba(92,209,255,0.16), transparent 60%), radial-gradient(800px 700px at 0% 70%, rgba(255,115,0,0.12), transparent 55%), linear-gradient(135deg,#0B1220 0%,#0E1726 55%,#111A2B 100%)',
        color: '#fff',
        display: 'flex',
        flexWrap: { xs: 'wrap', md: 'nowrap' },
        minHeight: '100vh',
        width: '100%',
      }}
    >
      {/* LEFT — marketing */}
      <Box
        sx={{
          display: { xs: 'none', md: 'flex' },
          flex: '1 1 56%',
          flexDirection: 'column',
          gap: 4.25,
          justifyContent: 'center',
          minWidth: 0,
          p: '64px 72px',
        }}
      >
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 2 }}>
          <Box
            sx={{
              alignItems: 'center',
              bgcolor: 'rgba(255,255,255,0.08)',
              border: '1px solid rgba(255,255,255,0.16)',
              borderRadius: '50%',
              display: 'inline-flex',
              height: 70,
              justifyContent: 'center',
              width: 70,
            }}
          >
            <Activity color="#ffffff" size={40} />
          </Box>
          <Box sx={{ lineHeight: 1 }}>
            <Typography sx={{ fontSize: 30, fontWeight: 700, letterSpacing: '-.5px' }}>
              WSO2
            </Typography>
            <Typography sx={{ color: 'rgba(255,255,255,0.7)', fontSize: 18, mt: 0.5 }}>
              <b style={{ color: '#fff', fontWeight: 600 }}>API</b> Platform
            </Typography>
          </Box>
        </Box>

        <Box sx={{ maxWidth: 560 }}>
          <Typography
            sx={{
              color: '#5CD1FF',
              fontSize: 11,
              fontWeight: 600,
              letterSpacing: '1.5px',
              mb: 1.75,
            }}
          >
            AI-NATIVE · SCALABLE SAAS
          </Typography>
          <Typography
            component="h1"
            sx={{ fontSize: 38, fontWeight: 400, letterSpacing: '-.5px', lineHeight: 1.18, m: 0, mb: 2.25 }}
          >
            Manage, secure, and scale your{' '}
            <Box
              component="span"
              sx={{
                background: ORANGE_GRADIENT,
                backgroundClip: 'text',
                color: 'transparent',
                fontWeight: 600,
                WebkitBackgroundClip: 'text',
              }}
            >
              APIs &amp; MCP servers
            </Box>
          </Typography>
          <Typography sx={{ color: 'rgba(255,255,255,0.62)', fontSize: 15, lineHeight: 1.6, maxWidth: 480 }}>
            A comprehensive platform for designing, deploying, governing, and
            optimizing APIs and MCP servers — end to end.
          </Typography>
        </Box>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.75, maxWidth: 520 }}>
          {FEATURES.map((feature) => (
            <Box key={feature.label} sx={{ alignItems: 'center', display: 'flex', gap: 1.75 }}>
              <Box
                sx={{
                  alignItems: 'center',
                  bgcolor: 'rgba(255,255,255,0.06)',
                  border: '1px solid rgba(255,255,255,0.10)',
                  borderRadius: '9px',
                  color: '#FF8A33',
                  display: 'inline-flex',
                  flex: 'none',
                  height: 34,
                  justifyContent: 'center',
                  width: 34,
                }}
              >
                {feature.icon}
              </Box>
              <Typography sx={{ color: 'rgba(255,255,255,0.86)', fontSize: 14.5 }}>
                {feature.label}
              </Typography>
            </Box>
          ))}
        </Box>
      </Box>

      {/* RIGHT — sign-in card */}
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          flex: { xs: '1 1 100%', md: '0 0 480px' },
          justifyContent: 'center',
          maxWidth: { xs: '100%', md: 480 },
          p: { xs: 3, md: '48px 40px' },
        }}
      >
        <Box
          sx={{
            backdropFilter: 'blur(16px)',
            bgcolor: 'rgba(255,255,255,0.05)',
            border: '1px solid rgba(255,255,255,0.12)',
            borderRadius: '18px',
            boxShadow: '0 24px 60px rgba(0,0,0,0.35)',
            maxWidth: 392,
            p: { xs: '28px 22px', sm: '34px 32px' },
            width: '100%',
          }}
        >
          {/* mobile brand lockup */}
          <Box sx={{ alignItems: 'center', display: { md: 'none' }, gap: 1.25, mb: 2.5 }}>
            <Activity color="#ffffff" size={24} style={{ verticalAlign: 'middle' }} />{' '}
            <Box component="span" sx={{ fontWeight: 700 }}>
              WSO2
            </Box>{' '}
            <Box component="span" sx={{ color: 'rgba(255,255,255,0.7)' }}>
              API Platform
            </Box>
          </Box>

          {/* header */}
          <Typography sx={{ fontSize: 24, fontWeight: 500, letterSpacing: '-.3px' }}>
            Sign in
          </Typography>
          <Typography sx={{ color: 'rgba(255,255,255,0.55)', fontSize: 13, mt: 0.75 }}>
            {message}
          </Typography>

          {/* alerts */}
          <Box sx={{ '& > *': { mt: 2.5 } }}>
            {!isBrowserSupported && (
              <Alert severity="warning">
                This console is optimized for Google Chrome and Mozilla Firefox.
              </Alert>
            )}
            {isInvitation && state.displayOrgName && (
              <Alert severity="success">
                Invitation verified for {state.displayOrgName}. Sign in to accept it.
              </Alert>
            )}
            {oidcError && <Alert severity="error">{oidcError}</Alert>}
            {auth.error && <Alert severity="error">{auth.error}</Alert>}
          </Box>

          {isOidcMode ? (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 2.75 }}>
              <Button onClick={() => auth.login(from)} sx={primaryCtaSx}>
                Sign in
                <ArrowRight size={17} />
              </Button>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 2.75 }}>
              <Box
                autoFocus
                component="input"
                onChange={(event: ChangeEvent<HTMLInputElement>) =>
                  setUsername(event.target.value)
                }
                onKeyDown={(event: KeyboardEvent<HTMLInputElement>) =>
                  handleKeyDown(event, () => void startBasicLogin())
                }
                placeholder="Username"
                sx={fieldSx}
                type="text"
                value={username}
              />
              <Box
                component="input"
                onChange={(event: ChangeEvent<HTMLInputElement>) =>
                  setPassword(event.target.value)
                }
                onKeyDown={(event: KeyboardEvent<HTMLInputElement>) =>
                  handleKeyDown(event, () => void startBasicLogin())
                }
                placeholder="Password"
                sx={fieldSx}
                type="password"
                value={password}
              />
              <Button
                disabled={submitting || !username.trim() || !password}
                onClick={() => void startBasicLogin()}
                sx={primaryCtaSx}
              >
                Sign in
                <ArrowRight size={17} />
              </Button>
            </Box>
          )}

          {/* footer */}
          <Box
            sx={{
              borderTop: '1px solid rgba(255,255,255,0.10)',
              mt: 2.75,
              pt: 2.25,
              textAlign: 'center',
            }}
          >
            <Typography sx={{ color: 'rgba(255,255,255,0.45)', fontSize: 12, mb: 1.25 }}>
              More details at{' '}
              <Link href={runtimeConfig.apiPlatformHomePage} sx={{ color: '#FF7300' }} target="_blank">
                wso2.com/api-platform
              </Link>
            </Typography>
            <Box
              sx={{
                alignItems: 'center',
                color: 'rgba(255,255,255,0.4)',
                display: 'flex',
                fontSize: 11,
                gap: 1.75,
                justifyContent: 'center',
              }}
            >
              <Link
                href={runtimeConfig.privacyPolicyLink}
                sx={{ color: 'rgba(255,255,255,0.5)' }}
                target="_blank"
              >
                Privacy policy
              </Link>
              <Box component="span" sx={{ opacity: 0.4 }}>
                ·
              </Box>
              <Link
                href={runtimeConfig.termsOfUseLink}
                sx={{ color: 'rgba(255,255,255,0.5)' }}
                target="_blank"
              >
                Terms of use
              </Link>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
