// src/App.tsx
import React from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import MainLayout from "./layout/MainLayout";

// Pages
import Overview from "./pages/Overview";
import Gateways from "./pages/Gateway";
import Portals from "./pages/PortalManagement";

// API Proxies
import APIs from "./pages/APIs";
import ApiTest from "./pages/apis/ApiTest";
import ApiDeploy from "./pages/apis/ApiDeploy";
import ApiPolicies from "./pages/apis/ApiPolicies";
import ApiOverview from "./pages/apis/ApiOverview";
import ApiDevelop from "./pages/apis/ApiDevelop"; // <-- ADD
import ApiPublish from "./pages/apis/ApiPublish";

// MCP Servers
import McpServers from "./pages/McpServers";
import McpTest from "./pages/mcp/McpTest";
import McpDeploy from "./pages/mcp/McpDeploy";
import McpPolicies from "./pages/mcp/McpPolicies";

// API Products
import ApiProducts from "./pages/ApiProducts";
import ProductsTest from "./pages/products/ProductsTest";
import ProductsDeploy from "./pages/products/ProductsDeploy";
import ProductsPolicies from "./pages/products/ProductsPolicies";

import Admin from "./pages/Admin";
import Policies from "./pages/Policies";
import { useOrganization } from "./context/OrganizationContext";
import { useProjects } from "./context/ProjectContext";
import { projectSlugFromName } from "./utils/projectSlug";
import ProjectOverview from "./pages/ProjectOverview";

const OverviewEntry: React.FC = () => {
  const { organization, loading } = useOrganization();
  const { selectedProject } = useProjects();

  if (loading || !organization) return null;

  const projectSegment = selectedProject
    ? `/${projectSlugFromName(selectedProject.name, selectedProject.id)}`
    : "";

  return <Navigate to={`/${organization.handle}${projectSegment}/overview`} replace />;
};

const App: React.FC = () => {
  return (
    <BrowserRouter>
      <MainLayout>
        <Routes>
          <Route path="/" element={<OverviewEntry />} />
          <Route path="/overview" element={<OverviewEntry />} />
          <Route path="/policies" element={<Policies />} />

          <Route path="/:orgHandle">
            <Route index element={<Navigate to="overview" replace />} />
            <Route path="overview" element={<Overview />} />
            <Route path="gateways" element={<Gateways />} />
            <Route path="portals" element={<Portals />} />
            <Route path="portals/create" element={<Portals />} />
            <Route path="portals/:portalId/edit" element={<Portals />} />
            <Route path="portals/:portalId/theme" element={<Portals />} />
            <Route path="policies" element={<Policies />} />
            {/* API Proxies (org scope) */}
            <Route path="apis" element={<APIs />} />
            <Route path="apis/develop" element={<ApiDevelop />} />   {/* <-- ADD */}
            <Route path="apis/test" element={<ApiTest />} />
            <Route path="apis/deploy" element={<ApiDeploy />} />
            <Route path="apis/policies" element={<ApiPolicies />} />
            <Route path="apis/:apiId/overview" element={<ApiOverview />} />
            <Route path="apis/:apiId/publish" element={<ApiPublish />} />
            <Route path=":apiSlug/apioverview" element={<ApiOverview />} />

            {/* MCP + Products + Admin */}
            <Route path="mcp" element={<McpServers />} />
            <Route path="mcp/test" element={<McpTest />} />
            <Route path="mcp/deploy" element={<McpDeploy />} />
            <Route path="mcp/policies" element={<McpPolicies />} />
            <Route path="products" element={<ApiProducts />} />
            <Route path="products/test" element={<ProductsTest />} />
            <Route path="products/deploy" element={<ProductsDeploy />} />
            <Route path="products/policies" element={<ProductsPolicies />} />
            <Route path="admin" element={<Admin />} />

            {/* Project scope */}
            <Route path=":projectHandle">
              <Route index element={<Navigate to="overview" replace />} />
              <Route path="overview" element={<ProjectOverview />} />
              <Route path="gateways" element={<Gateways />} />
              <Route path="portals" element={<Portals />} />
              <Route path="portals/create" element={<Portals />} />
              <Route path="portals/:portalId/edit" element={<Portals />} />
              <Route path="portals/:portalId/theme" element={<Portals />} />
              <Route path="policies" element={<Policies />} />
              {/* API Proxies (project scope) */}
              <Route path="apis" element={<APIs />} />
              <Route path="apis/develop" element={<ApiDevelop />} /> {/* <-- ADD */}
              <Route path="apis/test" element={<ApiTest />} />
              <Route path="apis/deploy" element={<ApiDeploy />} />
              <Route path="apis/policies" element={<ApiPolicies />} />
              <Route path="apis/:apiId/overview" element={<ApiOverview />} />
              <Route path="apis/:apiId/publish" element={<ApiPublish />} />
              <Route path=":apiSlug/apioverview" element={<ApiOverview />} />
              {/* MCP + Products + Admin */}
              <Route path="mcp" element={<McpServers />} />
              <Route path="mcp/test" element={<McpTest />} />
              <Route path="mcp/deploy" element={<McpDeploy />} />
              <Route path="mcp/policies" element={<McpPolicies />} />
              <Route path="products" element={<ApiProducts />} />
              <Route path="products/test" element={<ProductsTest />} />
              <Route path="products/deploy" element={<ProductsDeploy />} />
              <Route path="products/policies" element={<ProductsPolicies />} />
              <Route path="admin" element={<Admin />} />
            </Route>
          </Route>

          <Route path="*" element={<OverviewEntry />} />
        </Routes>
      </MainLayout>
    </BrowserRouter>
  );
};

export default App;
