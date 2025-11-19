// src/routes.tsx
import React from "react";
import { Routes, Route, Navigate } from "react-router-dom";

// Pages
import MainOverview from "./pages/Overview";
import Gateways from "./pages/Gateway";
import Portals from "./pages/PortalManagement";

// API Proxies
import APIs from "./pages/APIs";
import ApiTest from "./pages/apis/ApiTest";
import ApiDeploy from "./pages/apis/ApiDeploy";
import ApiOverview from "./pages/apis/ApiOverview";
import ApiDevelop from "./pages/apis/ApiDevelop";
import ApiPublish from "./pages/apis/ApiPublish";

// MCP Servers
import McpServers from "./pages/McpServers";
import McpTest from "./pages/mcp/McpTest";
import McpDeploy from "./pages/mcp/McpDeploy";

// API Products
import ApiProducts from "./pages/ApiProducts";
import ProductsTest from "./pages/products/ProductsTest";
import ProductsDeploy from "./pages/products/ProductsDeploy";
import ProductsPolicies from "./pages/products/ProductsPolicies";

// Policies
import OrgPolicies from "./pages/policies/orgPolicies/PolicyLandingPage";
import ApiPolicies from "./pages/policies/apiPolicies/PolicyLandingPage";

import Admin from "./pages/Admin";
import ProjectOverview from "./pages/ProjectOverview";

import { useOrganization } from "./context/OrganizationContext";
import { useProjects } from "./context/ProjectContext";
import { projectSlugFromName } from "./utils/projectSlug";

/** Redirects / and /overview to /:org/:project?/overview once org is known */
const OverviewEntry: React.FC = () => {
  const { organization, loading } = useOrganization();
  const { selectedProject } = useProjects();

  if (loading || !organization) return null;

  const projectSegment = selectedProject
    ? `/${projectSlugFromName(selectedProject.name, selectedProject.id)}`
    : "";

  return <Navigate to={`/${organization.handle}${projectSegment}/overview`} replace />;
};

const AppRoutes: React.FC = () => {
  return (
    <Routes>
      <Route path="/" element={<OverviewEntry />} />
      <Route path="/overview" element={<OverviewEntry />} />

      <Route path="/:orgHandle">
        <Route index element={<Navigate to="overview" replace />} />
        <Route path="overview" element={<MainOverview />} />
        <Route path="gateways" element={<Gateways />} />
        <Route path="portals" element={<Portals />} />
        <Route path="portals/create" element={<Portals />} />
        <Route path="portals/:portalId/edit" element={<Portals />} />
        <Route path="portals/:portalId/theme" element={<Portals />} />
        <Route path="policies" element={<OrgPolicies />} />

        {/* API Proxies (org scope) */}
        <Route path="apis" element={<APIs />} />
        <Route path="apis/develop" element={<ApiDevelop />} />
        <Route path="apis/test" element={<ApiTest />} />
        <Route path="apis/deploy" element={<ApiDeploy />} />
        <Route path="apis/:apiId/overview" element={<ApiOverview />} />
        <Route path="apis/:apiId/publish" element={<ApiPublish />} />
        <Route path=":apiSlug/apioverview" element={<ApiOverview />} />

        {/* MCP + Products + Admin */}
        <Route path="mcp" element={<McpServers />} />
        <Route path="mcp/test" element={<McpTest />} />
        <Route path="mcp/deploy" element={<McpDeploy />} />
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
          <Route path="policies" element={<OrgPolicies />} />

          {/* API Proxies (project scope) */}
          <Route path="apis" element={<APIs />} />
          <Route path="apis/develop" element={<ApiDevelop />} />
          <Route path="apis/test" element={<ApiTest />} />
          <Route path="apis/deploy" element={<ApiDeploy />} />
          <Route path="apis/:apiId/overview" element={<ApiOverview />} />
          <Route path="apis/:apiId/publish" element={<ApiPublish />} />
          <Route path=":apiSlug/apioverview" element={<ApiOverview />} />

          {/* MCP + Products + Admin */}
          <Route path="mcp" element={<McpServers />} />
          <Route path="mcp/test" element={<McpTest />} />
          <Route path="mcp/deploy" element={<McpDeploy />} />
          <Route path="products" element={<ApiProducts />} />
          <Route path="products/test" element={<ProductsTest />} />
          <Route path="products/deploy" element={<ProductsDeploy />} />
          <Route path="products/policies" element={<ProductsPolicies />} />
          <Route path="admin" element={<Admin />} />
        </Route>
      </Route>

      <Route path="*" element={<OverviewEntry />} />
    </Routes>
  );
};

export default AppRoutes;
