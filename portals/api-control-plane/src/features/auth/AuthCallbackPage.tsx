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

import { Navigate } from 'react-router-dom';

// The BFF's own /api/auth/callback performs the whole OIDC code exchange
// server-side and redirects straight to the sanitized return URL with the
// session cookie already set — the SPA is never actually navigated here in
// the normal flow. This route exists only as a safe fallback for a stray
// deep link to /signin.
export function AuthCallbackPage() {
  return <Navigate to="/" replace />;
}
