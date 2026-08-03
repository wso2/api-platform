import {
  AuthProvider as AsgardeoAuthProvider,
  type AuthReactConfig,
} from '@asgardeo/auth-react';
import { createContext, type ReactNode, useContext, useMemo } from 'react';

import { LoadingState } from '../../components/StateViews';
import { runtimeConfig } from '../../config/runtime';

type AsgardeoAvailability = {
  isConfigured: boolean;
};

const AsgardeoAvailabilityContext = createContext<AsgardeoAvailability>({
  isConfigured: false,
});

const getRedirectUrl = (path: string) =>
  `${window.location.origin}${runtimeConfig.appBasePath || ''}${path}`;

const buildSdkConfig = (): AuthReactConfig | undefined => {
  if (runtimeConfig.authMode === 'local-file') return undefined;
  if (!runtimeConfig.asgardeoSdkConfig) return undefined;

  return {
    ...runtimeConfig.asgardeoSdkConfig,
    clientHost:
      runtimeConfig.asgardeoSdkConfig.clientHost || window.location.origin,
    disableAutoSignIn: true,
    disableTrySignInSilently: true,
    resourceServerURLs: runtimeConfig.asgardeoSdkResourceServerUrls,
    scope: runtimeConfig.asgardeoSdkScopes,
    signInRedirectURL:
      runtimeConfig.asgardeoSdkConfig.signInRedirectURL ||
      getRedirectUrl('/signin'),
    signOutRedirectURL:
      runtimeConfig.asgardeoSdkConfig.signOutRedirectURL ||
      getRedirectUrl('/login'),
  } as AuthReactConfig;
};

export function ApiPlatformAsgardeoProvider({ children }: { children: ReactNode }) {
  const sdkConfig = useMemo(buildSdkConfig, []);
  const availability = useMemo(
    () => ({ isConfigured: Boolean(sdkConfig) }),
    [sdkConfig]
  );

  if (!sdkConfig) {
    return (
      <AsgardeoAvailabilityContext.Provider value={availability}>
        {children}
      </AsgardeoAvailabilityContext.Provider>
    );
  }

  return (
    <AsgardeoAvailabilityContext.Provider value={availability}>
      <AsgardeoAuthProvider
        config={sdkConfig}
        fallback={<LoadingState fullScreen label="Initializing sign in" />}
      >
        {children}
      </AsgardeoAuthProvider>
    </AsgardeoAvailabilityContext.Provider>
  );
}

export const useAsgardeoAvailability = () =>
  useContext(AsgardeoAvailabilityContext);
