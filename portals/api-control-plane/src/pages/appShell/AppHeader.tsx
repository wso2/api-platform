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
  Badge,
  ColorSchemeToggle,
  Header,
  IconButton,
  Tooltip,
  UserMenu,
  useAppShell,
} from '@wso2/oxygen-ui';
import { Bell, LogOut, WSO2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import { useLocation } from 'react-router-dom';

import { ErrorBoundary } from '../../components/errors/ErrorBoundary';
import { HeaderSwitchersErrorFallback } from '../../components/errors/ErrorFallback';
import { useAuth } from '../../contexts/auth/AuthProvider';
import { HeaderScopeSwitchers } from './HeaderScopeSwitchers';

export function AppHeader() {
  const intl = useIntl();
  const location = useLocation();
  const { actions } = useAppShell();
  const auth = useAuth();

  const userName = auth.user?.name || 'User';
  const userEmail = auth.user?.email || '';

  return (
    <Header>
      <Header.Toggle />
      <Header.Brand>
        <Header.BrandLogo>
          <WSO2 style={{ height: 26, width: 26 }} />
        </Header.BrandLogo>
        <Header.BrandTitle>
          <FormattedMessage
            id="appShell.header.title"
            defaultMessage="API Platform"
          />
        </Header.BrandTitle>
      </Header.Brand>

      {/* Guard only the switchers: their data (orgs, projects, APIs, etc.) may be
        missing or malformed. Keep the brand/actions outside so logout stays
        available. Use `resetKeys` with pathname so a broken switcher recovers
        after navigation. */}
      <ErrorBoundary
        fallback={() => <HeaderSwitchersErrorFallback />}
        resetKeys={[location.pathname]}
      >
        <HeaderScopeSwitchers />
      </ErrorBoundary>

      <Header.Spacer />

      <Header.Actions>
        <ColorSchemeToggle />
        <Tooltip title={intl.formatMessage({ id: 'appShell.header.notifications', defaultMessage: 'Notifications' })}>
          <IconButton
            aria-label={intl.formatMessage({ id: 'appShell.header.notifications', defaultMessage: 'Notifications' })}
            onClick={actions.toggleNotificationPanel}
            size="small"
          >
            <Badge color="primary" variant="dot" invisible>
              <Bell size={20} />
            </Badge>
          </IconButton>
        </Tooltip>
        <UserMenu>
          <UserMenu.Trigger name={userName} />
          <UserMenu.Header name={userName} email={userEmail} />
          <UserMenu.Divider />
          <UserMenu.Logout icon={<LogOut size={18} />} onClick={auth.logout} />
        </UserMenu>
      </Header.Actions>
    </Header>
  );
}
