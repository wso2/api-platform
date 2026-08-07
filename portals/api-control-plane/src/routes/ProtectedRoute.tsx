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

import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { LoadingState } from '../components/StateViews';
import { useAuth } from '../features/auth/AuthProvider';
import { routes } from './paths';

export function ProtectedRoute() {
  const auth = useAuth();
  const location = useLocation();

  // The BFF's session cookie already tells us, synchronously from
  // GET /api/session on mount, whether there's a session to restore — there
  // is no silent (iframe/prompt=none) restoration step to attempt.
  if (auth.isLoading) {
    return <LoadingState fullScreen label="Checking session" />;
  }

  if (auth.status === 'expired') {
    return <Navigate to={routes.sessionExpired} replace />;
  }

  if (auth.status === 'forbidden') {
    return <Navigate to={routes.unauthorized} replace />;
  }

  if (!auth.isAuthenticated) {
    return <Navigate to={routes.login} replace state={{ from: location }} />;
  }

  return <Outlet />;
}
