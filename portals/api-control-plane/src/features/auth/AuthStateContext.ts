import { createContext } from 'react';

import type { AuthState } from './authTypes';

export const AuthStateContext = createContext<AuthState | null>(null);
