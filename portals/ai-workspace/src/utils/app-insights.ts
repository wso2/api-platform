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

// @ts-expect-error: moesif-browser-js does not have types
import moesif from 'moesif-browser-js';
import { MOESIF_APP_API_KEY } from '../config.env';
import logger from './logger';

// Get Moesif app key from config
const moesifAppKey = MOESIF_APP_API_KEY;

let mo: ReturnType<typeof moesif.init> | null = null;

// Module-level user cache — populated after OIDC login via setCurrentUser()
let currentUser: { sub?: string; email?: string } | null = null;

export function setCurrentUser(user: { sub?: string; email?: string } | null) {
  currentUser = user;
}

/**
 * Initialize and get Moesif client
 */
function getMoesifClient() {
  if (!mo && typeof window !== 'undefined' && moesifAppKey) {
    mo = moesif.init({
      applicationId: moesifAppKey,
      batchEnabled: true,
      batchSize: 20,
      batchMaxTime: 5000,
      // eslint-disable-next-line prefer-arrow-callback
      getMetadata: function () {
        // Not implemented
      },
    });
    mo.start();
  }
  return mo;
}

/**
 * Identify user in Moesif
 */
export function identifyUser(
  userId: string,
  metadata: Record<string, any> = {}
) {
  const moClient = getMoesifClient();
  if (moClient) {
    moClient.identifyUser(userId, metadata);
  }
}

/**
 * Identify company/organization in Moesif
 */
export function identifyCompany(
  companyId: string,
  metadata: Record<string, any> = {}
) {
  const moClient = getMoesifClient();
  if (moClient) {
    moClient.identifyCompany(companyId, metadata);
  }
}

/**
 * Reset Moesif tracking
 */
export function resetMoesif() {
  const moClient = getMoesifClient();
  if (moClient) {
    moClient.reset();
  }
}

/**
 * Track event to Moesif analytics
 */
export const trackEvent = async (name: string, customProperties?: {}) => {
  const orgUuid = sessionStorage.getItem('orgUuid');
  const windowWidth = window.innerWidth;
  const windowHeight = window.innerHeight;

  const properties: Record<string, any> = {
    ...customProperties,
    ...(orgUuid ? { org: orgUuid } : {}),
    context: 'ai-workspace',
    origin: 'ai-workspace',
    windowWidth: `${windowWidth}`,
    windowHeight: `${windowHeight}`,
  };

  let userId: string | undefined;

  // Enrich with user info stored after login (set via setCurrentUser)
  if (currentUser) {
    if (currentUser.sub) {
      userId = currentUser.sub;
      properties.idpId = currentUser.sub;
    }
    if (currentUser.email) {
      properties.email = currentUser.email;
      properties.isWSO2User = String(currentUser.email.endsWith('@wso2.com'));
    }
  }

  // Publish to Moesif
  if (moesifAppKey) {
    const moClient = getMoesifClient();
    if (moClient) {
      moClient.track(name, properties);

      if (userId) {
        identifyUser(userId, {
          email: properties.email,
          isWSO2User: properties.isWSO2User,
        });
      }

      if (orgUuid) {
        identifyCompany(orgUuid, {});
      }
    }
  }
};

// Hybrid Gateway (Self-Hosted Gateway) events
export const trackHybridGatewayCreate = (
  orgUuid: string | undefined,
  gatewayId: string,
  functionalityType: string,
  environmentId?: string
) => {
  trackEvent('hybrid-gateway-create', {
    org: orgUuid,
    gatewayId,
    functionalityType,
    environmentId,
  });
};

export const trackHybridGatewayUpdate = (
  orgUuid: string | undefined,
  gatewayId: string,
  functionalityType: string,
  environmentId?: string
) => {
  trackEvent('hybrid-gateway-update', {
    org: orgUuid,
    gatewayId,
    functionalityType,
    environmentId,
  });
};

export const trackHybridGatewayDelete = (
  orgUuid: string | undefined,
  gatewayId: string
) => {
  trackEvent('hybrid-gateway-delete', {
    org: orgUuid,
    gatewayId,
  });
};

// API deployments to Platform Gateway (Hybrid Gateway deployments)
export const trackHybridGatewayDeploymentCreate = (params: {
  orgUuid?: string;
  gatewayId: string;
  gatewayName?: string;
  apiId?: string;
  deploymentId?: string;
  base?: string;
  hasBuildId?: boolean;
  hasEndpointOverride?: boolean;
  resourceType?: 'provider' | 'proxy' | 'mcp-server';
}) => {
  const {
    orgUuid,
    gatewayId,
    gatewayName,
    apiId,
    deploymentId,
    base,
    hasBuildId,
    hasEndpointOverride,
    resourceType,
  } = params;

  trackEvent('hybrid-gateway-deployment-create', {
    org: orgUuid,
    gatewayId,
    gatewayName,
    apiId,
    deploymentId,
    base,
    hasBuildId: hasBuildId ? 'true' : 'false',
    hasEndpointOverride: hasEndpointOverride ? 'true' : 'false',
    resourceType,
  });
};

export const trackHybridGatewayDeploymentRedeploy = (params: {
  orgUuid?: string;
  gatewayId: string;
  gatewayName?: string;
  apiId?: string;
  deploymentId?: string;
  hasBuildId?: boolean;
  resourceType?: 'provider' | 'proxy' | 'mcp-server';
}) => {
  const {
    orgUuid,
    gatewayId,
    gatewayName,
    apiId,
    deploymentId,
    hasBuildId,
    resourceType,
  } = params;

  trackEvent('hybrid-gateway-deployment-redeploy', {
    org: orgUuid,
    gatewayId,
    gatewayName,
    apiId,
    deploymentId,
    hasBuildId: hasBuildId ? 'true' : 'false',
    resourceType,
  });
};

export const trackHybridGatewayDeploymentUndeploy = (params: {
  orgUuid?: string;
  gatewayId: string;
  gatewayName?: string;
  apiId?: string;
  deploymentId?: string;
  resourceType?: 'provider' | 'proxy' | 'mcp-server';
}) => {
  const { orgUuid, gatewayId, gatewayName, apiId, deploymentId, resourceType } =
    params;

  trackEvent('hybrid-gateway-deployment-undeploy', {
    org: orgUuid,
    gatewayId,
    gatewayName,
    apiId,
    deploymentId,
    resourceType,
  });
};

export const trackHybridGatewayDeploymentDelete = (params: {
  orgUuid?: string;
  gatewayId: string;
  gatewayName?: string;
  apiId?: string;
  deploymentId?: string;
  resourceType?: 'provider' | 'proxy' | 'mcp-server';
}) => {
  const { orgUuid, gatewayId, gatewayName, apiId, deploymentId, resourceType } =
    params;

  trackEvent('hybrid-gateway-deployment-delete', {
    org: orgUuid,
    gatewayId,
    gatewayName,
    apiId,
    deploymentId,
    resourceType,
  });
};

// Service Provider events
export const trackServiceProviderCreate = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string,
  providerName?: string
) => {
  trackEvent('service-provider-create', {
    org: orgUuid,
    providerId,
    providerType,
    providerName,
  });
};

export const trackServiceProviderUpdate = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string,
  providerName?: string
) => {
  trackEvent('service-provider-update', {
    org: orgUuid,
    providerId,
    providerType,
    providerName,
  });
};

export const trackServiceProviderDelete = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string
) => {
  trackEvent('service-provider-delete', {
    org: orgUuid,
    providerId,
    providerType,
  });
};

// LLM Provider events
export const trackLLMProviderCreate = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string,
  modelName?: string
) => {
  trackEvent('llm-provider-create', {
    org: orgUuid,
    providerId,
    providerType,
    modelName,
  });
};

export const trackLLMProviderUpdate = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string,
  modelName?: string
) => {
  trackEvent('llm-provider-update', {
    org: orgUuid,
    providerId,
    providerType,
    modelName,
  });
};

export const trackLLMProviderDelete = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string
) => {
  trackEvent('llm-provider-delete', {
    org: orgUuid,
    providerId,
    providerType,
  });
};

export const trackLLMProviderTest = (
  orgUuid: string | undefined,
  providerId: string,
  providerType: string,
  success: boolean
) => {
  trackEvent('llm-provider-test', {
    org: orgUuid,
    providerId,
    providerType,
    success: success ? 'true' : 'false',
  });
};

// LLM Proxy events
export const trackLLMProxyCreate = (
  orgUuid: string | undefined,
  proxyId: string,
  providers: string[]
) => {
  trackEvent('llm-proxy-create', {
    org: orgUuid,
    proxyId,
    providers: providers.join(','),
    providerCount: providers.length,
  });
};

export const trackLLMProxyUpdate = (
  orgUuid: string | undefined,
  proxyId: string,
  providers?: string[]
) => {
  trackEvent('llm-proxy-update', {
    org: orgUuid,
    proxyId,
    ...(providers && {
      providers: providers.join(','),
      providerCount: providers.length,
    }),
  });
};

export const trackLLMProxyDelete = (
  orgUuid: string | undefined,
  proxyId: string
) => {
  trackEvent('llm-proxy-delete', {
    org: orgUuid,
    proxyId,
  });
};

// User session events
export const trackOverviewPageView = (
  orgUuid: string | undefined,
  userId: string,
  email?: string
) => {
  trackEvent('overview-page-view', {
    org: orgUuid,
    userId,
    email,
    viewTime: new Date().toISOString(),
  });
};
