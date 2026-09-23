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

import { ColorSchemeImage, ColorSchemeToggle, Header, UserMenu } from '@wso2/oxygen-ui';
import { LogOut, Menu } from '@wso2/oxygen-ui-icons-react';
import { useIntl } from 'react-intl';
import { useLocation, useNavigate } from 'react-router-dom';

import { useBrandLogo } from '@/branding/BrandLogoProvider';
import { ErrorBoundary } from '@/components/errors/ErrorBoundary';
import { HeaderSwitchersErrorFallback } from '@/components/errors/ErrorFallback';
import { useAuth } from '@/contexts/auth/AuthProvider';
import { routes } from '@/routes/paths';
import { getRouteParamsFromPathname } from '@/scope/consoleRouteParams';
import { HeaderScopeSwitchers } from './HeaderScopeSwitchers';

const BRAND_LOGO_HEIGHT = 32;
/** Matches Oxygen's own Header.Toggle icon size, so swapping the glyph doesn't resize the button. */
const TOGGLE_ICON_SIZE = 20;

export function AppHeader() {
  const intl = useIntl();
  const location = useLocation();
  const navigate = useNavigate();
  // const { actions } = useAppShell();
  const auth = useAuth();
  const brandLogo = useBrandLogo();

  const userName = auth.user?.name || 'User';
  const userEmail = auth.user?.email || '';

  const { orgHandle } = getRouteParamsFromPathname(location.pathname);
  const goToOrganizationHome = () => {
    navigate(orgHandle ? routes.organizationHome(orgHandle) : routes.organizations);
  };

  return (
    <Header>
      <Header.Toggle collapseIcon={<Menu size={TOGGLE_ICON_SIZE} />} />
      <Header.Brand
        aria-label={intl.formatMessage({
          id: 'appShell.header.brand.aria',
          defaultMessage: 'Go to organization overview',
        })}
        onClick={goToOrganizationHome}
      >
        <Header.BrandLogo>
          <ColorSchemeImage
            alt={intl.formatMessage({
              id: 'appShell.header.title',
              defaultMessage: 'API Platform',
            })}
            height={BRAND_LOGO_HEIGHT}
            src={brandLogo}
            width="auto"
          />
        </Header.BrandLogo>
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
        {/* Commenting out the notification bell for now, as it is not yet implemented. */}
        {/* <Tooltip title={intl.formatMessage({ id: 'appShell.header.notifications', defaultMessage: 'Notifications' })}>
          <IconButton
            aria-label={intl.formatMessage({ id: 'appShell.header.notifications', defaultMessage: 'Notifications' })}
            onClick={actions.toggleNotificationPanel}
            size="small"
          >
            <Badge color="primary" variant="dot" invisible>
              <Bell size={20} />
            </Badge>
          </IconButton>
        </Tooltip> */}
        <UserMenu>
          <UserMenu.Trigger name={userName} />
          <UserMenu.Header name={userName} email={userEmail} />
          <UserMenu.Divider />
          <UserMenu.Item
            icon={<LogOut />}
            label={intl.formatMessage({
              id: 'appShell.header.signOut',
              defaultMessage: 'Sign out',
            })}
            onClick={auth.logout}
          />
        </UserMenu>
      </Header.Actions>
    </Header>
  );
}
