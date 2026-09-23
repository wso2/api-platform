/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { Boxes, Network, Workflow } from "@wso2/oxygen-ui-icons-react";

import { EnvironmentsFeature } from "@wso2-enterprise/apip-cloud-ui-environments-new";
import { GatewaysFeature } from "@wso2-enterprise/apip-cloud-ui-gateways";
import type { GatewayType } from "@wso2-enterprise/apip-cloud-ui-gateways";
import {
  InsightsFeature,
  isInsightsMoesifConfigured,
} from "@wso2-enterprise/apip-cloud-ui-insights";
import {
  PipelinesFeature,
  ProjectPipelinesFeature,
} from "@wso2-enterprise/apip-cloud-ui-pipelines";
import type { BrandLogo } from "../../../../ai-workspace/src/branding/BrandLogoProvider";
import {
  DeployFeature,
  ProviderDeployFeature,
} from "@wso2-enterprise/apip-cloud-ui-deploy";
import { TrialStatusFeature } from "@wso2-enterprise/apip-cloud-ui-trial-status";
import {
  AI_WORKSPACE_HEADER_ACTIONS_SLOT,
  AI_WORKSPACE_GATEWAYS_NAV_REGION,
  AI_WORKSPACE_GATEWAYS_SLOT,
  AI_WORKSPACE_INSIGHTS_SLOT,
  AI_WORKSPACE_LLM_PROVIDER_DEPLOY_SLOT,
  AI_WORKSPACE_LLM_PROXY_DEPLOY_SLOT,
  AI_WORKSPACE_MCP_DEPLOY_SLOT,
  type AIWorkspaceCloudEntry,
  type AIWorkspaceExtension,
} from "../../../../ai-workspace/src/extensions";
import type { AIWorkspaceHostPort } from "../../../../ai-workspace/src/hostPort";
import {
  defineCloudPlugin,
  getCloudExtensions,
  type CloudPluginFeature,
} from "../plugin";
import cloudLogoDark from "../assets/logos/aiworkspace_white.svg";
import cloudLogoLight from "../assets/logos/aiworkspace_black.svg";

/** Cloud AI Workspace logo supplied to the host's `Logo` component. */
export const cloudBrandLogo: BrandLogo = {
  dark: cloudLogoDark,
  light: cloudLogoLight,
};

/**
 * The kinds of gateway this host manages. Module scope, not a literal at the
 * use site: the gateways feature asks the server for exactly these kinds, and a
 * fresh array on every render would make that request repeat.
 */
const AI_GATEWAY_TYPES: GatewayType[] = ["ai"];

/**
 * Cloud features registered for the AI Workspace host. The host owns routing,
 * organization/project scope, navigation and notifications; each plugin only
 * renders against the small host port passed to it.
 *
 * Most entries are `sidebar.main` items (new nav entry + route). `gateways`
 * is different: it registers against `AI_WORKSPACE_GATEWAYS_SLOT` to replace
 * what renders at the host's existing, built-in `gateways` route/sidebar item
 * — see `GatewaysRoute` in `ai-workspace/src/App.tsx` — rather than adding a
 * new one. Nothing under `ai-workspace/src/pages/appShell/appShellPages/gateways`
 * is touched by this. Every gateway in this workspace is an AI gateway, so it
 * registers `ai` as the only type and the create form shows no type picker.
 * It also carries nav placement so the entry sits between Environments and
 * Pipelines, suppressing the built-in item via `hides`.
 *
 * `insights` registers against `AI_WORKSPACE_INSIGHTS_SLOT` the same way —
 * see `InsightsRoute` in `ai-workspace/src/App.tsx` — so the built-in Insights
 * nav stays and only the page body is replaced when Moesif is configured.
 * Registration is gated by `isInsightsMoesifConfigured` (single reader in the
 * insights package) so App.tsx needs no Moesif config knowledge: no override
 * means InsightsRoute keeps the built-in page.
 */
export const cloudPluginFeatures: CloudPluginFeature<AIWorkspaceCloudEntry>[] =
  [
    defineCloudPlugin({
      id: "trial-status",
      version: "0.1.0",
      extensions: [
        {
          id: "trial-status",
          slot: AI_WORKSPACE_HEADER_ACTIONS_SLOT,
          order: 10,
          render: () => <TrialStatusFeature />,
        },
      ],
    }),
    defineCloudPlugin({
      id: "environments",
      version: "0.1.0",
      extensions: [
        {
          id: "environments",
          slot: "sidebar.main",
          order: 50,
          path: "environments",
          label: "Environments",
          icon: <Boxes size={20} />,
          render: (port: AIWorkspaceHostPort) => (
            <EnvironmentsFeature port={port} />
          ),
        },
      ],
    }),
    defineCloudPlugin({
      id: "pipelines",
      version: "0.1.0",
      extensions: [
        {
          id: "pipelines",
          slot: "sidebar.main",
          order: 60,
          path: "pipelines",
          label: "Pipelines",
          icon: <Workflow size={20} />,
          // One scope-adaptive "Pipelines" item: the project binding view when a
          // project is selected, the organization list/create/edit view otherwise.
          render: (port: AIWorkspaceHostPort) =>
            port.projectHandle ? (
              <ProjectPipelinesFeature port={port} />
            ) : (
              <PipelinesFeature port={port} />
            ),
        },
      ],
    }),
    defineCloudPlugin({
      id: "gateways",
      version: "0.1.0",
      extensions: [
        {
          id: "gateways",
          slot: AI_WORKSPACE_GATEWAYS_SLOT,
          // Nav placement: between Environments (50) and Pipelines (60). The
          // override carries it (rather than a second sidebar entry) so the
          // gateways route keeps rendering here, while `hides` suppresses the
          // built-in nav item that would otherwise appear higher up in its own
          // category. Without `hides` both entries would show.
          order: 55,
          path: "gateways",
          label: "AI Gateways",
          icon: <Network size={20} />,
          hides: [AI_WORKSPACE_GATEWAYS_NAV_REGION],
          render: (port: AIWorkspaceHostPort) => (
            <GatewaysFeature gatewayTypes={AI_GATEWAY_TYPES} port={port} />
          ),
        },
      ],
    }),
    defineCloudPlugin({
      id: "insights",
      version: "0.1.0",
      extensions: [
        {
          id: "insights",
          slot: AI_WORKSPACE_INSIGHTS_SLOT,
          order: 0,
          // Same Moesif ai-overview URL at org and project — no project_id filter.
          render: (port: AIWorkspaceHostPort) => (
            <InsightsFeature port={port} embedProfile="ai-workspace" />
          ),
        },
      ],
    }),
    defineCloudPlugin({
      id: "deploy",
      version: "0.1.0",
      // Registered once per artifact kind, each replacing that kind's built-in page
      // at its own route, so environments are what the AI Workspace shows for MCP
      // servers, LLM proxies and LLM providers alike.
      //
      // Two features, because the two kinds of artifact are deployed differently.
      // MCP servers and LLM proxies belong to a PROJECT, so they are deployed through
      // that project's pipeline: environments in promotion order, promoting between
      // them. LLM providers belong to the ORGANIZATION — `llm_providers` has no
      // project — so no pipeline can apply to them, and they are deployed straight to
      // any of the organization's environments.
      //
      // The artifact's handle comes from the route rather than the Port: these pages
      // are scoped to one artifact and the portal reads it off the URL (see
      // ArtifactDeployRoute in the host's App.tsx), which is why render takes it.
      extensions: [
        {
          id: "mcp-deploy",
          // Inert: a page override replaces one route's body, so there is nothing to
          // order it against.
          order: 0,
          slot: AI_WORKSPACE_MCP_DEPLOY_SLOT,
          render: (port, artifactHandle) => (
            <DeployFeature
              port={port}
              kind="Mcp"
              artifactHandle={artifactHandle}
            />
          ),
        },
        {
          id: "llm-proxy-deploy",
          // Inert: a page override replaces one route's body, so there is nothing to
          // order it against.
          order: 0,
          slot: AI_WORKSPACE_LLM_PROXY_DEPLOY_SLOT,
          render: (port, artifactHandle) => (
            <DeployFeature
              port={port}
              kind="LlmProxy"
              artifactHandle={artifactHandle}
            />
          ),
        },
        {
          id: "llm-provider-deploy",
          // Inert: a page override replaces one route's body, so there is nothing to
          // order it against.
          order: 0,
          slot: AI_WORKSPACE_LLM_PROVIDER_DEPLOY_SLOT,
          render: (port, artifactHandle) => (
            <ProviderDeployFeature
              port={port}
              artifactHandle={artifactHandle}
            />
          ),
        },
      ],
    }),
  ];

/** Omit Insights when Moesif is not configured so InsightsRoute keeps the built-in page. */
export const cloudExtensions = getCloudExtensions(
  cloudPluginFeatures.filter(
    (feature) => feature.id !== "insights" || isInsightsMoesifConfigured(),
  ),
);
export type { AIWorkspaceExtension };
