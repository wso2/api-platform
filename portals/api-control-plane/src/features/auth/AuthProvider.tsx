import { ReactNode, useContext } from 'react';

import { runtimeConfig } from '../../config/runtime';
import { AsgardeoAuthAdapter } from './adapters/AsgardeoAuthAdapter';
import { LocalFileAuthAdapter } from './adapters/LocalFileAuthAdapter';
import { ThunderAuthAdapter } from './adapters/ThunderAuthAdapter';
import { AuthStateContext } from './AuthStateContext';
import { useAsgardeoAvailability } from './AsgardeoProvider';

export function AuthProvider({ children }: { children: ReactNode }) {
  const { isConfigured } = useAsgardeoAvailability();

  // Thunder OIDC (oidc-client-ts) — independent of the Asgardeo SDK provider.
  if (runtimeConfig.authMode === 'thunder') {
    return <ThunderAuthAdapter>{children}</ThunderAuthAdapter>;
  }

  // Asgardeo unless explicitly local-file, or when the Asgardeo SDK is not
  // configured (no runtime SDK config available).
  const useLocalFileAuth =
    runtimeConfig.authMode === 'local-file' || !isConfigured;

  if (useLocalFileAuth) {
    return <LocalFileAuthAdapter>{children}</LocalFileAuthAdapter>;
  }

  return <AsgardeoAuthAdapter>{children}</AsgardeoAuthAdapter>;
}

export const useAuth = () => {
  const context = useContext(AuthStateContext);
  if (!context) throw new Error('useAuth must be used within AuthProvider');
  return context;
};

export type { AuthState, AuthStatus, AuthUser, LoginProvider } from './authTypes';
