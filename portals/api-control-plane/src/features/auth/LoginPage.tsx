import { Alert, Box, Button, Link, Typography } from '@wso2/oxygen-ui';
import {
  Activity,
  ArrowRight,
  Building2,
  PencilRuler,
  Rocket,
  ShieldCheck,
  Sparkles,
} from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { ChangeEvent, KeyboardEvent, useEffect, useMemo, useRef, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { runtimeConfig } from '../../config/runtime';
import { useAuth } from './AuthProvider';

type LoginLocationState = {
  confirmationKey?: string;
  confirmationOrg?: string;
  displayOrgName?: string;
  from?: { pathname?: string };
  returnToOrg?: string;
  returnToUrl?: string;
};

type LoginRegion = {
  label: string;
  url: string;
};

const AUTH_METHOD_BASIC = 'basic';

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

const parseLoginRegion = (value: string): LoginRegion | undefined => {
  const [label, url] = value.split('::');
  if (!label || !url) return undefined;
  return { label, url };
};

const getLoginRegions = () =>
  runtimeConfig.availableLoginRegions
    .map(parseLoginRegion)
    .filter(Boolean) as LoginRegion[];

const isValidEmail = (value: string) =>
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());

// --- dark field + button styles (standalone branded sign-in page) ---
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

const ssoButtonSx = {
  bgcolor: 'rgba(255,255,255,0.04)',
  border: '1px solid rgba(255,255,255,0.14)',
  borderRadius: '22px',
  color: 'rgba(255,255,255,0.92)',
  fontSize: 13.5,
  fontWeight: 500,
  gap: 1.25,
  height: 44,
  textTransform: 'none',
  width: '100%',
  '&:hover': {
    bgcolor: 'rgba(255,115,0,0.10)',
    borderColor: 'rgba(255,115,0,0.45)',
  },
  '&.Mui-disabled': { color: 'rgba(255,255,255,0.4)' },
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

function ProviderButton({
  disabled,
  icon,
  label,
  onClick,
}: {
  disabled: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <Button className="sso" disabled={disabled} onClick={onClick} sx={ssoButtonSx} type="button">
      <Box sx={{ alignItems: 'center', display: 'flex', height: 28, justifyContent: 'center', width: 28 }}>
        {icon}
      </Box>
      {label}
    </Button>
  );
}

const imgIcon = (src: string) => (
  <Box alt="" component="img" src={src} sx={{ height: 28, width: 28 }} />
);

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
  const autoLoginStarted = useRef(false);
  const [enterpriseMode, setEnterpriseMode] = useState(false);
  const [enterpriseEmail, setEnterpriseEmail] = useState('');
  const [enterpriseEmailError, setEnterpriseEmailError] = useState('');
  const [email, setEmail] = useState('');
  const [selectedRegion, setSelectedRegion] = useState(window.location.origin);

  const loginRegions = useMemo(getLoginRegions, []);
  const isThunderLogin = runtimeConfig.authMode === 'thunder';
  const isLoginEnabled =
    runtimeConfig.authMode === 'local-file' ||
    isThunderLogin ||
    Boolean(runtimeConfig.asgardeoSdkConfig) ||
    runtimeConfig.enableLocalAuthFallback;
  const isBrowserSupported = isSupportedBrowser();
  const providerIds = new Set(auth.loginProviders.map((provider) => provider.id));
  const showEmailLogin = providerIds.has(runtimeConfig.fidpEmail);
  const showGoogleLogin = Boolean(
    runtimeConfig.fidpGoogle && providerIds.has(runtimeConfig.fidpGoogle)
  );
  const showGithubLogin = Boolean(
    runtimeConfig.fidpGithub && providerIds.has(runtimeConfig.fidpGithub)
  );
  const showMicrosoftLogin = Boolean(
    runtimeConfig.fidpMicrosoft && providerIds.has(runtimeConfig.fidpMicrosoft)
  );
  const showEnterpriseLogin = Boolean(
    runtimeConfig.fidpEnterprise && providerIds.has(runtimeConfig.fidpEnterprise)
  );
  const hasSso =
    showGoogleLogin || showGithubLogin || showMicrosoftLogin || showEnterpriseLogin;

  useEffect(() => {
    const matchingRegion = loginRegions.find(
      (region) => region.url === window.location.origin
    );
    if (matchingRegion) setSelectedRegion(matchingRegion.url);
  }, [loginRegions]);

  useEffect(() => {
    if (autoLoginStarted.current || !isLoginEnabled || auth.isAuthenticated) return;

    const fidp = queryParams.get('fidp');
    const method = queryParams.get('method');
    const provider =
      fidp || (method === AUTH_METHOD_BASIC ? runtimeConfig.fidpEmail : undefined);
    if (provider) {
      autoLoginStarted.current = true;
      auth.loginWithProvider(provider, from);
      return;
    }

    // In Thunder SSO mode this console's own page is just a redirect step to the
    // IdP (which itself presents Google/GitHub), so skip it and go straight there.
    // Exceptions that must still render the page: an invitation to accept, a
    // multi-region choice, or an access error that would otherwise loop.
    if (
      isThunderLogin &&
      !isInvitation &&
      loginRegions.length <= 1 &&
      auth.status !== 'forbidden' &&
      !auth.error
    ) {
      autoLoginStarted.current = true;
      auth.login(from);
    }
  }, [
    auth,
    from,
    isLoginEnabled,
    isThunderLogin,
    isInvitation,
    loginRegions.length,
    queryParams,
  ]);

  if (auth.isAuthenticated) return <Navigate to={from} replace />;

  const message =
    auth.status === 'expired'
      ? 'Your session has expired. Sign in again to continue.'
      : auth.status === 'forbidden'
        ? 'You do not have access to this console.'
        : 'Continue to your API Platform console.';

  const startProviderLogin = (providerId?: string) => {
    if (!providerId) return;
    auth.loginWithProvider(providerId, from);
  };

  const startEmailLogin = () => {
    auth.loginWithProvider(runtimeConfig.fidpEmail, from, email.trim() || undefined);
  };

  const startEnterpriseLogin = () => {
    if (!isValidEmail(enterpriseEmail)) {
      setEnterpriseEmailError(
        enterpriseEmail.trim()
          ? 'Enter a valid email address in the format email@example.com'
          : 'Enter the email address'
      );
      return;
    }
    auth.loginWithProvider(
      runtimeConfig.fidpEnterprise || 'EnterpriseIDP',
      from,
      enterpriseEmail
    );
  };

  const handleEnterpriseEmailChange = (event: ChangeEvent<HTMLInputElement>) => {
    setEnterpriseEmail(event.target.value);
    setEnterpriseEmailError('');
  };

  const handleKeyDown = (
    event: KeyboardEvent<HTMLInputElement>,
    action: () => void
  ) => {
    if (event.key === 'Enter') action();
  };

  const handleRegionChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const nextRegion = event.target.value;
    setSelectedRegion(nextRegion);
    if (nextRegion && nextRegion !== window.location.origin) {
      window.location.href = `${nextRegion}${window.location.pathname}${window.location.search}`;
    }
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
          <Box sx={{ display: 'flex', gap: 2, justifyContent: 'space-between' }}>
            <Box>
              <Typography sx={{ fontSize: 24, fontWeight: 500, letterSpacing: '-.3px' }}>
                {enterpriseMode ? 'Enterprise sign in' : 'Sign in'}
              </Typography>
              <Typography sx={{ color: 'rgba(255,255,255,0.55)', fontSize: 13, mt: 0.75 }}>
                {message}
              </Typography>
            </Box>
            {loginRegions.length > 1 && !enterpriseMode && (
              <Box sx={{ display: 'flex', flex: 'none', flexDirection: 'column', gap: 0.5 }}>
                <Typography
                  sx={{
                    color: 'rgba(255,255,255,0.5)',
                    fontSize: 10,
                    letterSpacing: '.5px',
                    textTransform: 'uppercase',
                  }}
                >
                  Region
                </Typography>
                <Box
                  component="select"
                  onChange={handleRegionChange}
                  sx={{ ...fieldSx, fontSize: 13, height: 36, px: '8px', width: 90 }}
                  value={selectedRegion}
                >
                  {loginRegions.map((region) => (
                    <option key={region.url} value={region.url}>
                      {region.label}
                    </option>
                  ))}
                </Box>
              </Box>
            )}
          </Box>

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
            {!isLoginEnabled && (
              <Alert severity="error">
                Runtime auth config is missing. Provide ASGARDEO_SDK_CONFIG or set
                VITE_ENABLE_LOCAL_AUTH_FALLBACK=true for local-only development.
              </Alert>
            )}
            {auth.error && <Alert severity="error">{auth.error}</Alert>}
          </Box>

          {enterpriseMode ? (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 2.75 }}>
              <Box
                autoFocus
                component="input"
                onChange={handleEnterpriseEmailChange}
                onKeyDown={(event: KeyboardEvent<HTMLInputElement>) =>
                  handleKeyDown(event, startEnterpriseLogin)
                }
                placeholder="you@company.com"
                sx={{
                  ...fieldSx,
                  ...(enterpriseEmailError
                    ? { borderColor: '#EF4423', '&:focus': { borderColor: '#EF4423' } }
                    : {}),
                }}
                type="email"
                value={enterpriseEmail}
              />
              {enterpriseEmailError && (
                <Typography sx={{ color: '#FF8A33', fontSize: 12 }}>
                  {enterpriseEmailError}
                </Typography>
              )}
              <Button disabled={!isLoginEnabled} onClick={startEnterpriseLogin} sx={primaryCtaSx}>
                Continue
                <ArrowRight size={17} />
              </Button>
              <Button
                onClick={() => setEnterpriseMode(false)}
                sx={{ color: 'rgba(255,255,255,0.7)', textTransform: 'none' }}
              >
                Back
              </Button>
            </Box>
          ) : (
            <>
              {isThunderLogin && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 2.75 }}>
                  <Button
                    disabled={!isLoginEnabled}
                    onClick={() => auth.login(from)}
                    sx={primaryCtaSx}
                  >
                    Sign in
                    <ArrowRight size={17} />
                  </Button>
                </Box>
              )}

              {showEmailLogin && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 2.75 }}>
                  <Box
                    component="input"
                    onChange={(event: ChangeEvent<HTMLInputElement>) =>
                      setEmail(event.target.value)
                    }
                    onKeyDown={(event: KeyboardEvent<HTMLInputElement>) =>
                      handleKeyDown(event, startEmailLogin)
                    }
                    placeholder="you@company.com"
                    sx={fieldSx}
                    type="email"
                    value={email}
                  />
                  <Button disabled={!isLoginEnabled} onClick={startEmailLogin} sx={primaryCtaSx}>
                    Continue with email
                    <ArrowRight size={17} />
                  </Button>
                </Box>
              )}

              {showEmailLogin && hasSso && (
                <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5, my: 2.75 }}>
                  <Box sx={{ bgcolor: 'rgba(255,255,255,0.12)', flex: 1, height: '1px' }} />
                  <Typography sx={{ color: 'rgba(255,255,255,0.4)', fontSize: 11, letterSpacing: '1px' }}>
                    OR
                  </Typography>
                  <Box sx={{ bgcolor: 'rgba(255,255,255,0.12)', flex: 1, height: '1px' }} />
                </Box>
              )}

              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.25, mt: hasSso && !showEmailLogin ? 2.75 : 0 }}>
                {showGoogleLogin && (
                  <ProviderButton
                    disabled={!isLoginEnabled}
                    icon={imgIcon('/images/google-logo.svg')}
                    label="Continue with Google"
                    onClick={() => startProviderLogin(runtimeConfig.fidpGoogle)}
                  />
                )}
                {showGithubLogin && (
                  <ProviderButton
                    disabled={!isLoginEnabled}
                    icon={imgIcon('/images/github.svg')}
                    label="Continue with GitHub"
                    onClick={() => startProviderLogin(runtimeConfig.fidpGithub)}
                  />
                )}
                {showMicrosoftLogin && (
                  <ProviderButton
                    disabled={!isLoginEnabled}
                    icon={imgIcon('/images/microsoft.svg')}
                    label="Continue with Microsoft"
                    onClick={() => startProviderLogin(runtimeConfig.fidpMicrosoft)}
                  />
                )}
                {showEnterpriseLogin && (
                  <ProviderButton
                    disabled={!isLoginEnabled}
                    icon={<Building2 color="#FF8A33" size={28} />}
                    label="Continue with Enterprise ID"
                    onClick={() => setEnterpriseMode(true)}
                  />
                )}
              </Box>

              {showEmailLogin && (
                <Typography sx={{ color: 'rgba(255,255,255,0.6)', fontSize: 13, mt: 3, textAlign: 'center' }}>
                  Don&apos;t have an account?{' '}
                  <Link href={runtimeConfig.signupUrl} sx={{ color: '#FF7300', fontWeight: 600 }}>
                    Sign up
                  </Link>
                </Typography>
              )}
            </>
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
