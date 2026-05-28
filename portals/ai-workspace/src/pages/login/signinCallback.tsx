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

import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthContext } from '@asgardeo/auth-react';
import { Box, Stack, Typography } from '@wso2/oxygen-ui';
import { logger } from '../../utils/logger';
import { useChoreoUser } from '../../contexts/ChoreoUserContext';

import { storeUserInfo } from '../../auth/login';
import AILoader from '../../Components/AILoader';
import { FormattedMessage } from 'react-intl';

// Session storage key for the return URL (must match useSignInSilent.ts)
const RETURN_URL_SESSION_KEY = 'ai_workspace_return_url';

const getOrgHandleFromUrl = (): string | null => {
  const returnUrl = sessionStorage.getItem(RETURN_URL_SESSION_KEY);
  if (!returnUrl) return null;
  const match = returnUrl.match(/^\/organizations\/([^/]+)/);
  return match ? match[1] : null;
};

export default function SigninCallback() {
  const navigate = useNavigate();
  const { state, getBasicUserInfo } = useAuthContext();
  const { isAuthenticated, isLoading } = state;
  const {
    validateUser,
    exchangeOrgToken,
    setIsTokenExchanged,
    setOrganizations,
    setIsOrgAdmin,
  } = useChoreoUser();

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }

    const validateAndInitiate = async () => {
      try {
        // Store user info for display
        const userInfo = await getBasicUserInfo();
        storeUserInfo(userInfo);

        // Validate user and get organizations
        const response = await validateUser();
        const orgs = response.organizations;
        sessionStorage.setItem('idpId', response.idpId);
        setOrganizations(orgs);

        if (orgs.length > 0) {
          const urlOrgHandle = getOrgHandleFromUrl();
          const storedOrgHandle = sessionStorage.getItem('currentOrgHandle');
          let targetOrg = orgs[0];

          if (urlOrgHandle) {
            const found = orgs.find((o) => o.handle === urlOrgHandle);
            if (found) targetOrg = found;
          } else if (storedOrgHandle) {
            const found = orgs.find((o) => o.handle === storedOrgHandle);
            if (found) targetOrg = found;
          }

          sessionStorage.setItem('currentOrgHandle', targetOrg.handle);

          // Exchange org token
          const isAdmin = await exchangeOrgToken(targetOrg.handle);
          setIsOrgAdmin(isAdmin);
          setIsTokenExchanged(true);

          // Navigate to the protected app
          const returnUrl = sessionStorage.getItem(RETURN_URL_SESSION_KEY);
          sessionStorage.removeItem(RETURN_URL_SESSION_KEY);
          navigate(returnUrl || '/', { replace: true });
        } else {
          logger.error('No organizations found for user');
          navigate('/', { replace: true });
        }
      } catch (error) {
        logger.error('SigninCallback: initialization failed:', error);
        const returnUrl = sessionStorage.getItem(RETURN_URL_SESSION_KEY);
        sessionStorage.removeItem(RETURN_URL_SESSION_KEY);
        navigate(returnUrl || '/', { replace: true });
      }
    };

    validateAndInitiate();
  }, [isAuthenticated]);

  // Edge case: not authenticated and not loading -> go back to login
  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      navigate('/login', { replace: true });
    }
  }, [isLoading, isAuthenticated, navigate]);

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        px: 2,
      }}
    >
      <Box
        sx={{
          width: 'min(520px, 100%)',
          textAlign: 'center',
        }}
      >
        <Stack spacing={2.5} alignItems="center">
          <Box>
            <AILoader />
          </Box>

          <Typography variant="body2">
            <FormattedMessage
              id="aiWorkspace.pages.login.signinCallback.completing.sign.in"
              defaultMessage={'Completing sign in...'}
            />
          </Typography>
          {/* 
          <Typography variant="body2" color="text.secondary">
            Just a moment while we set things up for you.
          </Typography> */}
        </Stack>
      </Box>
    </Box>
  );
}
