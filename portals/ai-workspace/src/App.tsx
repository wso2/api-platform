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
  Routes,
  Route,
  Navigate,
  useLocation,
  useNavigate,
} from 'react-router-dom';
import { useAuth } from 'react-oidc-context';
import Login from './pages/login/login';
import AppShellMain from './pages/appShell/appShellMain';
import { AppShellProvider } from './contexts/AppShellContext';
import { RoleProvider } from './contexts/RoleContext';
import PageErrorBoundary from './Components/common/PageErrorBoundary';
import { AIWorkspaceSnackbarProvider } from './contexts/AIWorkspaceSnackbarContext';
import { MoesifProvider } from './contexts/MoesifContext';

// App Shell Pages
import Overview from './pages/appShell/appShellPages/overview/Overview';
import ApplicationsLayout from './pages/appShell/appShellPages/applications/ApplicationsLayout';
import ApplicationsList from './pages/appShell/appShellPages/applications/ApplicationsList';
import ApplicationNew from './pages/appShell/appShellPages/applications/ApplicationNew';
import ApplicationOverview from './pages/appShell/appShellPages/applications/ApplicationOverview';
import EditApplication from './pages/appShell/appShellPages/applications/EditApplication';
import ProjectPage from './pages/appShell/appShellPages/projects/Main';
import ProjectsList from './pages/appShell/appShellPages/projects/ProjectsList';
import ProjectListView from './pages/appShell/appShellPages/projects/ProjectListView';
import AddNewProject from './pages/appShell/appShellPages/projects/AddNewProject';
import EditProject from './pages/appShell/appShellPages/projects/EditProject';
import ProjectShell from './pages/appShell/appShellPages/projects/ProjectShell';
import OrgShell from './pages/appShell/appShellPages/projects/OrgShell';
import OrgRedirect from './pages/appShell/appShellPages/projects/OrgRedirect';
import LLMProxyLayout from './pages/appShell/appShellPages/proxies/LLMProxyLayout';
import LLMProxiesList from './pages/appShell/appShellPages/proxies/LLMProxiesList';
import LLMProxyNew from './pages/appShell/appShellPages/proxies/LLMProxyNew';
import LLMProxyOverview from './pages/appShell/appShellPages/proxies/LLMProxyOverview';
import LLMProxyDeploy from './pages/appShell/appShellPages/proxies/LLMProxyDeploy';
import EditLLMProxy from './pages/appShell/appShellPages/proxies/EditLLMProxy';
import ServiceProviderLayout from './pages/appShell/appShellPages/serviceProvider/ServiceProviderLayout';
import ServiceProviders from './pages/appShell/appShellPages/serviceProvider/ProvidersList';
import ServiceProviderNew from './pages/appShell/appShellPages/serviceProvider/ServiceProviderNew';
import ServiceProviderOverview from './pages/appShell/appShellPages/serviceProvider/ServiceProviderOverview';
import ServiceProviderDeploy from './pages/appShell/appShellPages/serviceProvider/ServiceProviderDeploy';
import EditServiceProvider from './pages/appShell/appShellPages/serviceProvider/EditServiceProvider';

import GatewaysLayout from './pages/appShell/appShellPages/gateways/GatewaysLayout.tsx';
import OrgRegisterPage from './pages/register/OrgRegisterPage';
import OrgSelectPage from './pages/select/OrgSelectPage';
import Insights from './pages/appShell/appShellPages/insights/Main';
import QuickStart from './pages/appShell/appShellPages/quickStart/Main';
import Settings from './pages/appShell/appShellPages/settings/Main';
import ExternalServersList from './pages/appShell/appShellPages/externalServers/ExternalServersList';
import ExternalServersNew from './pages/appShell/appShellPages/externalServers/ExternalServersNew';
import ExternalServersOverview from './pages/appShell/appShellPages/externalServers/ExternalServersOverview';
import ExternalServersDeploy from './pages/appShell/appShellPages/externalServers/ExternalServersDeploy';
import EditExternalServer from './pages/appShell/appShellPages/externalServers/EditExternalServer';
import { MCPServerValidationProvider } from './contexts/MCP';
import { LLMProvidersProvider } from './contexts/llmProvider';
import { ChoreoUserProvider, useChoreoUser } from './contexts/ChoreoUserContext';
import { useAppAuth } from './contexts/AppAuthContext';
import { ORG_HANDLE } from './config.env';
import { Box, CircularProgress } from '@wso2/oxygen-ui';

/** Must match the key used in main.tsx's OIDCBootstrap. */
const ORG_HANDLE_STORAGE_KEY = 'ai_workspace_org_handle';

function resolveOrgHandle(): string {
  return ORG_HANDLE || localStorage.getItem(ORG_HANDLE_STORAGE_KEY) || '';
}
import React from 'react';

/**
 * Only allow same-origin relative paths as return URLs to prevent open redirects.
 * Rejects protocol-relative URLs (//evil.com) and absolute URLs.
 */
function sanitizeReturnUrl(url: string): string {
  if (typeof url !== 'string') return '/';
  if (!url.startsWith('/') || url.startsWith('//')) return '/';
  // Strip any embedded newlines that could be used for header injection
  const clean = url.replace(/[\r\n]/g, '');
  return clean || '/';
}

function PublicOnlyRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAppAuth();
  if (isLoading) return null;
  if (isAuthenticated) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAppAuth();
  const location = useLocation();

  if (isLoading) {
    return null;
  }

  if (!isAuthenticated) {
    const returnUrl = sanitizeReturnUrl(location.pathname + location.search);
    const skip = ['/', '/login', '/signin', '/getting-started'];
    if (!skip.includes(returnUrl)) {
      sessionStorage.setItem('ai_workspace_return_url', returnUrl);
    }
    return <Navigate to="/getting-started" replace />;
  }

  return <>{children}</>;
}

// Super admin can only access /register-org. Redirect everything else there.
function SuperAdminRoute({ children }: { children: React.ReactNode }) {
  const { isSuperAdmin } = useAppAuth();
  const location = useLocation();

  if (isSuperAdmin && location.pathname !== '/register-org') {
    return <Navigate to="/register-org" replace />;
  }

  return <>{children}</>;
}

// Shows a spinner while react-oidc-context automatically processes the ?code= param.
// Navigation to the org home is handled by onSigninCallback in OIDCWrapper (main.tsx).
// If the callback fails, auth.error is set and we redirect to /getting-started.
function SigninCallbackRoute() {
  const auth = useAuth();
  const navigate = useNavigate();

  React.useEffect(() => {
    if (auth.error) {
      navigate('/getting-started', { replace: true });
    }
  }, [auth.error, navigate]);

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <CircularProgress />
    </Box>
  );
}

// Redirects to the org home when an authenticated user lands on a non-org path
// (e.g. navigating directly to '/').
function PostSignInInit({ children }: { children: React.ReactNode }) {
  const { isTokenExchanged, setIsTokenExchanged } = useChoreoUser();
  const navigate = useNavigate();
  const location = useLocation();

  React.useEffect(() => {
    if (isTokenExchanged) return;
    setIsTokenExchanged(true);
    // Only redirect if we're at a root-level path with no org context yet.
    if (!location.pathname.startsWith('/organizations')) {
      const handle = resolveOrgHandle();
      if (handle) navigate(`/organizations/${handle}/home`, { replace: true });
    }
  }, [isTokenExchanged, navigate, location.pathname]);

  return <>{children}</>;
}

function ProtectedAppShell() {
  const { user } = useAppAuth();
  const userName = user?.name ?? undefined;
  const userEmail = user?.email ?? undefined;

  return (
    <PostSignInInit>
      <RoleProvider>
        <AIWorkspaceSnackbarProvider>
          <AppShellProvider userName={userName} userEmail={userEmail}>
            <MoesifProvider>
              <AppShellMain />
            </MoesifProvider>
          </AppShellProvider>
        </AIWorkspaceSnackbarProvider>
      </RoleProvider>
    </PostSignInInit>
  );
}

function RoutePageBoundary({ children }: { children: React.ReactNode }) {
  const location = useLocation();

  return (
    <PageErrorBoundary fullWidth key={location.pathname}>
      {children}
    </PageErrorBoundary>
  );
}

function WithPageBoundary({ children }: { children: React.ReactNode }) {
  return <RoutePageBoundary>{children}</RoutePageBoundary>;
}

export default function App() {
  return (
    <ChoreoUserProvider>
      <Routes>
        {/* OAuth callback */}
        <Route path="/signin" element={<SigninCallbackRoute />} />

        {/* Getting started — handled by AppRoot before App renders, but
            catch it here in case somehow reached inside App */}
        <Route path="/getting-started" element={<Navigate to="/" replace />} />

        {/* Login page — kept for backward compatibility */}
        <Route path="/login" element={<PublicOnlyRoute><Login /></PublicOnlyRoute>} />

        {/* Organization registration — requires auth, shown only when no org exists */}
        <Route path="/register-org" element={
          <ProtectedRoute>
            <OrgRegisterPage />
          </ProtectedRoute>
        } />

        {/* Organization selection — requires auth, lets user navigate to any org by handle */}
        <Route path="/select-org" element={
          <ProtectedRoute>
            <OrgSelectPage />
          </ProtectedRoute>
        } />

        {/* Protected Routes */}
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <SuperAdminRoute>
                <ProtectedAppShell />
              </SuperAdminRoute>
            </ProtectedRoute>
          }
        >
          <Route index element={<OrgRedirect />} />
          <Route path="organizations" element={<OrgRedirect />} />
          <Route path="organizations/:orgSlug" element={<OrgShell />}>
            <Route index element={<Navigate to="home" replace />} />
            <Route
              path="home"
              element={
                <WithPageBoundary>
                  <Overview />
                </WithPageBoundary>
              }
            />
            <Route
              path="projects"
              element={
                <WithPageBoundary>
                  <ProjectsList />
                </WithPageBoundary>
              }
            />
            <Route
              path="projects/list"
              element={
                <WithPageBoundary>
                  <ProjectListView />
                </WithPageBoundary>
              }
            />
            <Route
              path="projects/new"
              element={
                <WithPageBoundary>
                  <AddNewProject />
                </WithPageBoundary>
              }
            />
            <Route
              path="projects/:projectId/edit"
              element={
                <WithPageBoundary>
                  <EditProject />
                </WithPageBoundary>
              }
            />
            <Route
              path="projects/:projectId"
              element={
                <WithPageBoundary>
                  <ProjectPage />
                </WithPageBoundary>
              }
            />
            <Route path="applications" element={<ApplicationsLayout />}>
              <Route
                index
                element={
                  <WithPageBoundary>
                    <ApplicationsList />
                  </WithPageBoundary>
                }
              />
              <Route
                path="new"
                element={
                  <WithPageBoundary>
                    <ApplicationNew />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":applicationId"
                element={
                  <WithPageBoundary>
                    <ApplicationOverview />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":applicationId/edit"
                element={
                  <WithPageBoundary>
                    <EditApplication />
                  </WithPageBoundary>
                }
              />
            </Route>
            <Route path="proxies" element={<LLMProxyLayout />}>
              <Route
                index
                element={
                  <WithPageBoundary>
                    <LLMProxiesList />
                  </WithPageBoundary>
                }
              />
              <Route
                path="new"
                element={
                  <WithPageBoundary>
                    <LLMProxyNew />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":proxyId"
                element={
                  <WithPageBoundary>
                    <LLMProxyOverview />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":proxyId/deploy"
                element={
                  <WithPageBoundary>
                    <LLMProxyDeploy />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":proxyId/edit"
                element={
                  <WithPageBoundary>
                    <EditLLMProxy />
                  </WithPageBoundary>
                }
              />
            </Route>
            <Route path="service-provider" element={<ServiceProviderLayout />}>
              <Route
                index
                element={
                  <WithPageBoundary>
                    <ServiceProviders />
                  </WithPageBoundary>
                }
              />
              <Route
                path="new"
                element={
                  <WithPageBoundary>
                    <ServiceProviderNew />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":providerId"
                element={
                  <WithPageBoundary>
                    <ServiceProviderOverview />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":providerId/deploy"
                element={
                  <WithPageBoundary>
                    <ServiceProviderDeploy />
                  </WithPageBoundary>
                }
              />
              <Route
                path=":providerId/edit"
                element={
                  <WithPageBoundary>
                    <EditServiceProvider />
                  </WithPageBoundary>
                }
              />
            </Route>
            <Route
              path="mcp-proxy"
              element={
                <WithPageBoundary>
                  <ExternalServersList />
                </WithPageBoundary>
              }
            />
            <Route
              path="mcp-proxy/new"
              element={
                <WithPageBoundary>
                  <MCPServerValidationProvider>
                    <ExternalServersNew />
                  </MCPServerValidationProvider>
                </WithPageBoundary>
              }
            />
            <Route
              path="mcp-proxy/:serverId"
              element={
                <WithPageBoundary>
                  <ExternalServersOverview />
                </WithPageBoundary>
              }
            />
            <Route
              path="mcp-proxy/:serverId/deploy"
              element={
                <WithPageBoundary>
                  <ExternalServersDeploy />
                </WithPageBoundary>
              }
            />
            <Route
              path="mcp-proxy/:serverId/edit"
              element={
                <WithPageBoundary>
                  <EditExternalServer />
                </WithPageBoundary>
              }
            />
            <Route
              path="gateways/*"
              element={
                <WithPageBoundary>
                  <GatewaysLayout />
                </WithPageBoundary>
              }
            />
            <Route
              path="quick-start"
              element={
                <WithPageBoundary>
                  <LLMProvidersProvider>
                    <QuickStart />
                  </LLMProvidersProvider>
                </WithPageBoundary>
              }
            />
            <Route
              path="insights"
              element={
                <WithPageBoundary>
                  <Insights />
                </WithPageBoundary>
              }
            />
            <Route
              path="settings"
              element={
                <WithPageBoundary>
                  <Settings />
                </WithPageBoundary>
              }
            />
            <Route path="projects/:projectSlug" element={<ProjectShell />}>
              <Route index element={<Navigate to="home" replace />} />
              <Route
                path="home"
                element={
                  <WithPageBoundary>
                    <Overview />
                  </WithPageBoundary>
                }
              />
              <Route path="applications" element={<ApplicationsLayout />}>
                <Route
                  index
                  element={
                    <WithPageBoundary>
                      <ApplicationsList />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="new"
                  element={
                    <WithPageBoundary>
                      <ApplicationNew />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":applicationId"
                  element={
                    <WithPageBoundary>
                      <ApplicationOverview />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":applicationId/edit"
                  element={
                    <WithPageBoundary>
                      <EditApplication />
                    </WithPageBoundary>
                  }
                />
              </Route>
              <Route path="proxies" element={<LLMProxyLayout />}>
                <Route
                  index
                  element={
                    <WithPageBoundary>
                      <LLMProxiesList />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="new"
                  element={
                    <WithPageBoundary>
                      <LLMProxyNew />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":proxyId"
                  element={
                    <WithPageBoundary>
                      <LLMProxyOverview />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":proxyId/deploy"
                  element={
                    <WithPageBoundary>
                      <LLMProxyDeploy />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":proxyId/edit"
                  element={
                    <WithPageBoundary>
                      <EditLLMProxy />
                    </WithPageBoundary>
                  }
                />
              </Route>
              <Route
                path="service-provider"
                element={<ServiceProviderLayout />}
              >
                <Route
                  index
                  element={
                    <WithPageBoundary>
                      <ServiceProviders />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="new"
                  element={
                    <WithPageBoundary>
                      <ServiceProviderNew />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":providerId"
                  element={
                    <WithPageBoundary>
                      <ServiceProviderOverview />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":providerId/deploy"
                  element={
                    <WithPageBoundary>
                      <ServiceProviderDeploy />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path=":providerId/edit"
                  element={
                    <WithPageBoundary>
                      <EditServiceProvider />
                    </WithPageBoundary>
                  }
                />
              </Route>
              <Route
                path="gateways/*"
                element={
                  <WithPageBoundary>
                    <GatewaysLayout />
                  </WithPageBoundary>
                }
              />
              <Route
                path="mcp-proxy"
                element={
                  <WithPageBoundary>
                    <ExternalServersList />
                  </WithPageBoundary>
                }
              />
              <Route
                path="mcp-proxy/new"
                element={
                  <WithPageBoundary>
                    <MCPServerValidationProvider>
                      <ExternalServersNew />
                    </MCPServerValidationProvider>
                  </WithPageBoundary>
                }
              />
              <Route
                path="mcp-proxy/:serverId"
                element={
                  <WithPageBoundary>
                    <ExternalServersOverview />
                  </WithPageBoundary>
                }
              />
              <Route
                path="mcp-proxy/:serverId/deploy"
                element={
                  <WithPageBoundary>
                    <ExternalServersDeploy />
                  </WithPageBoundary>
                }
              />
              <Route
                path="mcp-proxy/:serverId/edit"
                element={
                  <WithPageBoundary>
                    <EditExternalServer />
                  </WithPageBoundary>
                }
              />
              <Route
                path="quick-start"
                element={
                  <WithPageBoundary>
                    <LLMProvidersProvider>
                      <QuickStart />
                    </LLMProvidersProvider>
                  </WithPageBoundary>
                }
              />
              <Route
                path="insights"
                element={
                  <WithPageBoundary>
                    <Insights />
                  </WithPageBoundary>
                }
              />
              <Route
                path="settings"
                element={
                  <WithPageBoundary>
                    <Settings />
                  </WithPageBoundary>
                }
              />
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ChoreoUserProvider>
  );
}
