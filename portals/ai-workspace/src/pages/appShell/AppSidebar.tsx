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

import React, { useEffect, useRef, useState } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { Box, Sidebar } from '@wso2/oxygen-ui';
import {
  BarChart3,
  Dock,
  Handshake,
  Home,
  Layers,
  Network,
  Rocket,
  Settings,
  Workflow,
} from '@wso2/oxygen-ui-icons-react';
import McpMenuIcon from '../../assets/icons/McpMenuIcon';
import { useAppShell } from '../../contexts/AppShellContext';
import { useAppAuth } from '../../contexts/AppAuthContext';
import { SCOPES } from '../../auth/permissions';
import { buildOrgPath, buildProjectPath } from '../../utils/projectRouting';
import QuickStartIntroPopup, {
  QS_INTRO_STORAGE_KEY,
} from './QuickStartIntroPopup';

const navLinkStyle: React.CSSProperties = {
  textDecoration: 'none',
  color: 'inherit',
  display: 'block',
};

type Props = {
  shellState: any;
  shellActions: any;
  projectCount: number;
};

export default function AppSidebar({
  shellState,
  shellActions,
  projectCount,
}: Props) {
  const { currentProject, currentOrganization } = useAppShell();
  const { hasPermission, isAuthenticated } = useAppAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const quickStartRef = useRef<HTMLDivElement>(null);
  const [showIntro, setShowIntro] = useState(
    () => !localStorage.getItem(QS_INTRO_STORAGE_KEY)
  );

  const handleDismissIntro = () => {
    localStorage.setItem(QS_INTRO_STORAGE_KEY, '1');
    setShowIntro(false);
  };
  const orgHomePath = buildOrgPath(currentOrganization, '/home');
  const orgProjectsListPath = buildOrgPath(
    currentOrganization,
    '/projects/list'
  );
  const orgQuickStartPath = buildOrgPath(currentOrganization, '/quick-start');
  const orgApplicationsPath = buildOrgPath(
    currentOrganization,
    '/applications'
  );
  const orgProxiesPath = buildOrgPath(currentOrganization, '/proxies');
  const orgServiceProviderPath = buildOrgPath(
    currentOrganization,
    '/service-provider'
  );
  const orgExternalServersPath = buildOrgPath(
    currentOrganization,
    '/mcp-proxy'
  );
  const orgGatewaysPath = buildOrgPath(currentOrganization, '/gateways');
  const orgInsightsPath = buildOrgPath(currentOrganization, '/insights');
  const orgSettingsPath = buildOrgPath(currentOrganization, '/settings');
  const projectRoot = buildProjectPath(
    currentOrganization,
    currentProject,
    '/home'
  );
  const quickStartPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/quick-start')
    : orgQuickStartPath;

  useEffect(() => {
    if (showIntro && location.pathname !== quickStartPath) {
      navigate(quickStartPath, { replace: true });
    }
  }, [showIntro, location.pathname, quickStartPath, navigate]);

  const applicationsPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/applications')
    : orgApplicationsPath;
  const proxiesPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/proxies')
    : orgProxiesPath;
  const serviceProviderPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/service-provider')
    : orgServiceProviderPath;
  const externalServersPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/mcp-proxy')
    : orgExternalServersPath;
  const gatewaysPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/gateways')
    : orgGatewaysPath;
  const insightsPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/insights')
    : orgInsightsPath;
  const settingsPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/settings')
    : orgSettingsPath;
  return (
    <Sidebar
      collapsed={shellState.sidebarCollapsed}
      activeItem={shellState.activeMenuItem}
      expandedMenus={shellState.expandedMenus}
      onSelect={shellActions.setActiveMenuItem}
      onToggleExpand={shellActions.toggleMenu}
    >
      <Sidebar.Nav>
        <Sidebar.Category>
          <Box
            ref={quickStartRef}
            style={
              showIntro
                ? {
                    borderRadius: 6,
                    outlineOffset: 1,
                  }
                : undefined
            }
          >
            <NavLink to={quickStartPath} style={navLinkStyle}>
              <Sidebar.Item id="quick-start">
                <Sidebar.ItemIcon>
                  <Rocket size={20} />
                </Sidebar.ItemIcon>
                <Sidebar.ItemLabel>Quick Start</Sidebar.ItemLabel>
                <Sidebar.ItemBadge
                  sx={{
                    background: 'linear-gradient(135deg, #F87644, #e8501a)',
                    borderRadius: 0.3,
                  }}
                >
                  New
                </Sidebar.ItemBadge>
              </Sidebar.Item>
            </NavLink>
          </Box>
          <Box
            style={
              showIntro ? { opacity: 0.25, pointerEvents: 'none' } : undefined
            }
          >
            <NavLink
              to={currentProject ? projectRoot : orgHomePath}
              style={navLinkStyle}
            >
              <Sidebar.Item id="overview">
                <Sidebar.ItemIcon>
                  <Home size={20} />
                </Sidebar.ItemIcon>
                <Sidebar.ItemLabel>Overview</Sidebar.ItemLabel>
              </Sidebar.Item>
            </NavLink>
            {!currentProject && (
              <NavLink
                to={orgProjectsListPath}
                style={navLinkStyle}
                data-cyid="nav-projects"
              >
                <Sidebar.Item id="projects">
                  <Sidebar.ItemIcon>
                    <Layers size={20} />
                  </Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>Projects</Sidebar.ItemLabel>
                </Sidebar.Item>
              </NavLink>
            )}
          </Box>
        </Sidebar.Category>

        {showIntro && (
          <QuickStartIntroPopup
            anchorRef={quickStartRef}
            onDismiss={handleDismissIntro}
          />
        )}

        <Box
          style={
            showIntro ? { opacity: 0.25, pointerEvents: 'none' } : undefined
          }
        >
          {(hasPermission(SCOPES.LLM_PROVIDER_READ) || hasPermission(SCOPES.LLM_PROXY_READ)) && (
            <>
              <Sidebar.Category>
                <Sidebar.CategoryLabel>
                  <span style={{ fontSize: '0.8rem' }}>LLM</span>
                </Sidebar.CategoryLabel>
                {hasPermission(SCOPES.LLM_PROVIDER_READ) && (
                  <NavLink to={serviceProviderPath} style={navLinkStyle} data-cyid="nav-service-provider">
                    <Sidebar.Item id="service-provider">
                      <Sidebar.ItemIcon>
                        <Handshake size={20} />
                      </Sidebar.ItemIcon>
                      <Sidebar.ItemLabel>LLM Providers</Sidebar.ItemLabel>
                    </Sidebar.Item>
                  </NavLink>
                )}
                {hasPermission(SCOPES.LLM_PROXY_READ) && (
                  <NavLink
                    to={proxiesPath}
                    style={navLinkStyle}
                    data-cyid="nav-proxies"
                  >
                    <Sidebar.Item id="proxies">
                      <Sidebar.ItemIcon>
                        <Workflow size={20} />
                      </Sidebar.ItemIcon>
                      <Sidebar.ItemLabel>App LLM Proxies</Sidebar.ItemLabel>
                    </Sidebar.Item>
                  </NavLink>
                )}
              </Sidebar.Category>
              <Box sx={{ borderTop: '1px solid', borderColor: 'divider', mx: 1.5, my: 0.5 }} />
            </>
          )}

          {hasPermission(SCOPES.MCP_PROXY_READ) && (
            <>
              <Sidebar.Category>
                <Sidebar.CategoryLabel>
                  <Box
                    sx={{ display: 'inline-flex', alignItems: 'center', gap: 1 }}
                  >
                    <span style={{ fontSize: '0.8rem' }}>MCP</span>
                  </Box>
                </Sidebar.CategoryLabel>
                <NavLink to={externalServersPath} style={navLinkStyle}>
                  <Sidebar.Item id="external-servers">
                    <Sidebar.ItemIcon>
                      <McpMenuIcon size={20} aria-hidden />
                    </Sidebar.ItemIcon>
                    <Sidebar.ItemLabel>MCP Proxies</Sidebar.ItemLabel>
                  </Sidebar.Item>
                </NavLink>
              </Sidebar.Category>
              <Box sx={{ borderTop: '1px solid', borderColor: 'divider', mx: 1.5, my: 0.5 }} />
            </>
          )}

          <Sidebar.Category>
            {hasPermission(SCOPES.APPLICATION_READ) && (
              <NavLink to={applicationsPath} style={navLinkStyle}>
                <Sidebar.Item id="applications">
                  <Sidebar.ItemIcon>
                    <Dock size={20} />
                  </Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>GenAI Applications</Sidebar.ItemLabel>
                </Sidebar.Item>
              </NavLink>
            )}
            {hasPermission(SCOPES.GATEWAY_READ) && (
              <NavLink to={gatewaysPath} style={navLinkStyle}>
                <Sidebar.Item id="gateways">
                  <Sidebar.ItemIcon>
                    <Network size={20} />
                  </Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>AI Gateways</Sidebar.ItemLabel>
                </Sidebar.Item>
              </NavLink>
            )}
            {isAuthenticated && (
              <NavLink to={insightsPath} style={navLinkStyle}>
                <Sidebar.Item id="insights">
                  <Sidebar.ItemIcon>
                    <BarChart3 size={20} />
                  </Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>Insights</Sidebar.ItemLabel>
                </Sidebar.Item>
              </NavLink>
            )}
          </Sidebar.Category>
        </Box>
      </Sidebar.Nav>

      <Sidebar.Footer>
        <Sidebar.Category>
          {hasPermission(SCOPES.LLM_TEMPLATE_MANAGE) && (
            <NavLink to={settingsPath} style={navLinkStyle}>
              <Sidebar.Item id="settings">
                <Sidebar.ItemIcon>
                  <Settings size={20} />
                </Sidebar.ItemIcon>
                <Sidebar.ItemLabel>Settings</Sidebar.ItemLabel>
              </Sidebar.Item>
            </NavLink>
          )}

          {/* <Sidebar.Item id="help">
            <Sidebar.ItemIcon>
              <HelpCircle size={20} />
            </Sidebar.ItemIcon>
            <Sidebar.ItemLabel>Help & Support</Sidebar.ItemLabel>
          </Sidebar.Item> */}
        </Sidebar.Category>
      </Sidebar.Footer>
    </Sidebar>
  );
}
