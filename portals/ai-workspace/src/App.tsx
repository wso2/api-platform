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
import AutoLoginPage from './pages/login/AutoLoginPage';
import AppShellMain from './pages/appShell/appShellMain';
import { AppShellProvider } from './contexts/AppShellContext';
import { RoleProvider } from './contexts/RoleContext';
import PageErrorBoundary from './Components/common/PageErrorBoundary';
import { AIWorkspaceSnackbarProvider } from './contexts/AIWorkspaceSnackbarContext';
import OrgProvisioningPage from './pages/register/OrgProvisioningPage';
import { checkOrganizationExists, registerOrganization } from './apis/platformApis';
import { forceLogoutAndRedirect } from './auth/logout';
import { DEFAULT_ORG_REGION } from './config.env';

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
import CreateProviderTemplate from './pages/appShell/appShellPages/providerTemplate/CreateProviderTemplate';
import ProviderTemplateOverview from './pages/appShell/appShellPages/providerTemplate/ProviderTemplateOverview';
import EditProviderTemplate from './pages/appShell/appShellPages/providerTemplate/EditProviderTemplate';
import CreateProviderTemplateVersion from './pages/appShell/appShellPages/providerTemplate/CreateProviderTemplateVersion';
import ServiceProviderOverview from './pages/appShell/appShellPages/serviceProvider/ServiceProviderOverview';
import ServiceProviderDeploy from './pages/appShell/appShellPages/serviceProvider/ServiceProviderDeploy';
import EditServiceProvider from './pages/appShell/appShellPages/serviceProvider/EditServiceProvider';

import GatewaysLayout from './pages/appShell/appShellPages/gateways/GatewaysLayout.tsx';
import OrgRegisterPage from './pages/register/OrgRegisterPage';
import Insights from './pages/appShell/appShellPages/insights/Main';
import QuickStart from './pages/appShell/appShellPages/quickStart/Main';
import Settings from './pages/appShell/appShellPages/settings/Main';
import ProviderTemplatesList from './pages/appShell/appShellPages/providerTemplate/ProviderTemplatesList';
import ExternalServersList from './pages/appShell/appShellPages/externalServers/ExternalServersList';
import ExternalServersNew from './pages/appShell/appShellPages/externalServers/ExternalServersNew';
import ExternalServersOverview from './pages/appShell/appShellPages/externalServers/ExternalServersOverview';
import ExternalServersDeploy from './pages/appShell/appShellPages/externalServers/ExternalServersDeploy';
import EditExternalServer from './pages/appShell/appShellPages/externalServers/EditExternalServer';
import { MCPServerValidationProvider } from './contexts/MCP';
import { LLMProvidersProvider } from './contexts/llmProvider';
import React, { useRef, useState } from 'react';
import { ChoreoUserProvider } from './contexts/ChoreoUserContext';
import { useAppAuth } from './contexts/AppAuthContext';
import { Box } from '@wso2/oxygen-ui';

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
    const skip = ['/', '/login', '/signin', '/admin'];
    if (!skip.includes(returnUrl)) {
      sessionStorage.setItem('ai_workspace_return_url', returnUrl);
    }
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

// The OIDC ?code= callback is now handled server-side by the BFF at
// /api/auth/callback (which sets the session cookie and 302s back into the app).
// This SPA route only catches stray hits to the legacy /signin path and bounces
// them home; the gate in main.tsx re-evaluates the session there.
function SigninCallbackRoute() {
  return <Navigate to="/" replace />;
}

type OrgInitState = 'checking' | 'provisioning' | 'done' | 'error';

// After sign-in: checks whether the user's org (from token claims) is registered,
// auto-registers it if not, then navigates to the org home page.
function PostSignInInit({ children }: { children: React.ReactNode }) {
  const { user } = useAppAuth();
  const navigate = useNavigate();
  const initiated = useRef(false);
  const [orgState, setOrgState] = useState<OrgInitState>('checking');
  const [orgError, setOrgError] = useState<string | null>(null);
  const [sessionExpired, setSessionExpired] = useState(false);

  React.useEffect(() => {
    if (initiated.current) return;
    if (!user) return;
    initiated.current = true;

    const org = user.org;
    if (!org?.id || !org?.handle) {
      // No org claims in token — render the app and let AppShellContext handle it.
      setOrgState('done');
      return;
    }

    checkOrganizationExists(org.id)
      .then(async (exists) => {
        if (!exists) {
          setOrgState('provisioning');
          await registerOrganization({
            id: org.id,
            name: org.name || org.handle,
            handle: org.handle,
            region: DEFAULT_ORG_REGION,
          });
        }
        navigate(`/organizations/${org.handle}/home`, { replace: true });
        setOrgState('done');
      })
      .catch((err: unknown) => {
        // A 401 means the session is invalid/expired — retrying won't help.
        // Surface a "session expired" screen with a logout action instead.
        if ((err as { status?: number })?.status === 401) {
          setSessionExpired(true);
          setOrgState('error');
          return;
        }
        setOrgError(err instanceof Error ? err.message : 'Failed to set up workspace');
        setOrgState('error');
      });
  }, [user, navigate]);

  if (orgState === 'checking' || orgState === 'provisioning') {
    return (
      <OrgProvisioningPage
        orgName={user?.org?.name ?? undefined}
        isProvisioning={orgState === 'provisioning'}
      />
    );
  }

  if (orgState === 'error') {
    return (
      <OrgProvisioningPage
        orgName={user?.org?.name ?? undefined}
        error={sessionExpired ? null : orgError}
        isSessionExpired={sessionExpired}
        onLogout={() => { void forceLogoutAndRedirect(); }}
        onRetry={() => { initiated.current = false; setOrgState('checking'); }}
      />
    );
  }

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
            <AppShellMain />
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
        {/* OAuth callback — react-oidc-context processes the ?code= param here */}
        <Route path="/signin" element={<SigninCallbackRoute />} />

        {/* Login — shows full-page loader and auto-redirects to the IDP */}
        <Route path="/login" element={<PublicOnlyRoute><AutoLoginPage /></PublicOnlyRoute>} />

        {/* Organization registration — requires auth */}
        <Route path="/register-org" element={
          <ProtectedRoute>
            <OrgRegisterPage />
          </ProtectedRoute>
        } />

        {/* Protected Routes */}
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <ProtectedAppShell />
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
            <Route path="settings" element={<ServiceProviderLayout />}>
              <Route element={<Settings />}>
                <Route
                  index
                  element={<Navigate to="llm-provider-templates" replace />}
                />
                <Route
                  path="llm-provider-templates"
                  element={
                    <WithPageBoundary>
                      <ProviderTemplatesList />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="llm-provider-templates/new"
                  element={
                    <WithPageBoundary>
                      <CreateProviderTemplate />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="llm-provider-templates/:templateId"
                  element={
                    <WithPageBoundary>
                      <ProviderTemplateOverview />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="llm-provider-templates/:templateId/edit"
                  element={
                    <WithPageBoundary>
                      <EditProviderTemplate />
                    </WithPageBoundary>
                  }
                />
                <Route
                  path="llm-provider-templates/:templateId/new-version"
                  element={
                    <WithPageBoundary>
                      <CreateProviderTemplateVersion />
                    </WithPageBoundary>
                  }
                />
              </Route>
            </Route>
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
              <Route path="settings" element={<ServiceProviderLayout />}>
                <Route element={<Settings />}>
                  <Route
                    index
                    element={
                      <WithPageBoundary>
                        <ProviderTemplatesList />
                      </WithPageBoundary>
                    }
                  />
                </Route>
              </Route>
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ChoreoUserProvider>
  );
}
