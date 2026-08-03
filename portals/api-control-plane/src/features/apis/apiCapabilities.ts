import type { Api } from '../../types/domain';

export type ApiTestMode = 'curl' | 'none';

export type ApiCapabilities = {
  canBuild: boolean;
  canDeploy: boolean;
  canDevelop: boolean;
  canManage: boolean;
  canTest: boolean;
  hasRuntimeLogs: boolean;
  hasUsageInsights: boolean;
  testMode: ApiTestMode;
};

export const getApiCapabilities = (component?: Api): ApiCapabilities => {
  if (!component) {
    return {
      canBuild: false,
      canDeploy: false,
      canDevelop: false,
      canManage: false,
      canTest: false,
      hasRuntimeLogs: false,
      hasUsageInsights: false,
      testMode: 'none',
    };
  }

  const isApiProxy = component.kind === 'API_PROXY';
  const isService = component.kind === 'SERVICE';
  const isWebApp = component.kind === 'WEB_APP';
  const isHttpReachable = component.httpBased !== false;

  return {
    canBuild: isService || isWebApp,
    canDeploy: isApiProxy || isService || isWebApp,
    canDevelop: isApiProxy || isService,
    canManage: isApiProxy || isService || isWebApp,
    canTest: isHttpReachable && (isApiProxy || isService),
    hasRuntimeLogs: isService || isWebApp,
    hasUsageInsights: isApiProxy || isService,
    testMode: isHttpReachable ? 'curl' : 'none',
  };
};
