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

import { useCallback, useMemo, useState } from 'react';
import { Card, PageTitle, Stack } from '@wso2/oxygen-ui';
import { Rocket } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { parseSpecContent, useRestApi, useRestApiOpenApi } from '@/api/resources/restApis';
import { useDeployments } from '@/api/resources/restApis/deployments';
import { useRestApiGateways } from '@/api/resources/restApis/apiGateways/apiGateways.hooks';
import { isTestKeyExpired, testKeyRemainingMs } from './utils/testApiKey';
import { useTestApiKey } from './utils/useTestApiKey';
import type { Gateway } from '@/api/resources/gateways';
import { isApiError } from '@/api/core/errors';
import { ApiDesignerCanvasIllustration } from '@/components/illustrations/ApiDesignerCanvasIllustration';
import { GatewayIllustration } from '@/components/illustrations/GatewayIllustration';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { useNow } from '@/hooks/useNow';
import { ScopeGate } from '@/scope/ScopeGate';
import { buildInvokeUrl } from '../apis/overview/InvokeUrlPanel';
import { gatewayEndpoint } from '../gateways/utils/gatewayDisplay';
import { MOCK_ENVIRONMENTS } from '../gateways/utils/gatewayEnvironments';
import { CurlBuilder } from './curl/CurlBuilder';
import { deployedGateways as deployedGatewaysOf } from './utils/deployedGateways';
import { apiKeyAuthOf } from './utils/apiKeyAuth';
import { GatewaySection } from './components/GatewaySection';
import { buildConsoleRequest, firstOperationOf } from './utils/operationRequest';
import { TestKeySection } from './components/TestKeySection';
import { emptyRequest, withTarget, type ConsoleRequest, type KeyValueRow } from './utils/types';

const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.apiNotFound',
    defaultMessage: 'API not found',
  },
  consoleView: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.consoleView',
    defaultMessage: 'Console view',
    description: 'Toggle option showing the interactive spec console.',
  },
  curlView: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.curlView',
    defaultMessage: 'cURL view',
    description: 'Toggle option showing the command builder.',
  },
  deployAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.deployAction',
    defaultMessage: 'Deploy API',
    description: 'Button in the not-deployed banner that opens the Deploy page. A command.',
  },
  notDeployedTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.notDeployedTitle',
    defaultMessage: 'You must deploy the API to start testing.',
    description:
      'Heading of the empty state shown when the API is not deployed anywhere, so there is no endpoint to send requests to.',
  },
  noDefinitionTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.noDefinitionTitle',
    defaultMessage: 'You must add an API definition to start testing.',
    description:
      'Heading of the empty state shown when the API has no stored OpenAPI definition, so there are no resources to test.',
  },
  deploymentUnavailable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.deploymentUnavailable',
    defaultMessage: 'This API’s deployments could not be loaded.',
    description:
      'Shown when fetching the gateways or deployments failed — distinct from the API genuinely not being deployed anywhere.',
  },
  definitionUnavailable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.definitionUnavailable',
    defaultMessage: 'This API’s definition could not be loaded.',
    description:
      "Shown when fetching or parsing the API's OpenAPI definition failed — distinct from the API simply not having one.",
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.loading',
    defaultMessage: 'Loading test console',
    description: 'Shown while the API and its definition are being fetched.',
  },
  scopePrompt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.scopePrompt',
    defaultMessage: 'The test console runs against a single API.',
    description: 'Explains why an API must be picked before this page can render.',
  },
  subtitleConsoleView: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.subtitle.console',
    defaultMessage: 'Send requests to a deployed gateway.',
  },
  subtitleCurlView: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.subtitle.curl',
    defaultMessage: 'Copy a command to run in a terminal.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.title',
    defaultMessage: 'Test',
    description: 'Page heading.',
  },
  viewLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.viewLabel',
    defaultMessage: 'Console or cURL view',
    description: 'Accessible label for the toggle between the two views.',
  },
});

/** Refresh the test key countdown every 30 seconds. */
const KEY_COUNTDOWN_TICK_MS = 30_000;

/** Gateway label as "Name — Environment", matching the Invoke URL panel. */
const gatewayOptionLabel = (gateway: Gateway): string => {
  const name = gateway.displayName || gateway.id || '';
  const environment = MOCK_ENVIRONMENTS.find(
    (candidate) => candidate.id === gateway.properties?.environment,
  )?.name;
  return environment ? `${name} — ${environment}` : name;
};

export function TestPage() {
  const intl = useIntl();
  const { params } = useConsoleScope();

  return (
    <ScopeGate prompt={intl.formatMessage(messages.scopePrompt)} requires="api" to={routes.apiTest}>
      <TestConsole key={params.apiHandler ?? ''} />
    </ScopeGate>
  );
}

function TestConsole() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);
  const restApiId = apiQuery.data?.id;

  const definitionQuery = useRestApiOpenApi(restApiId);
  const gatewaysQuery = useRestApiGateways(restApiId);
  const deploymentsQuery = useDeployments(restApiId);

  /** API `api-key-auth` configuration, if present. */
  const apiKeyAuth = useMemo(() => apiKeyAuthOf(apiQuery.data), [apiQuery.data]);
  const needsApiKey = apiKeyAuth !== undefined;

  const gateways = useMemo(
    () => deployedGatewaysOf(gatewaysQuery.data?.list ?? [], deploymentsQuery.data?.list ?? []),
    [deploymentsQuery.data, gatewaysQuery.data],
  );

  /** True while we still do not know whether the API is deployed. */
  const deploymentUnknown = gatewaysQuery.isPending || deploymentsQuery.isPending;

  /** True when either deployment lookup failed. */
  const deploymentFailed = Boolean(gatewaysQuery.error || deploymentsQuery.error);

  /** True when a request can be sent. */
  const deploymentReady = !deploymentUnknown && !deploymentFailed && gateways.length > 0;

  /** Mint only when a required key has a gateway to use. */
  const testApiKey = useTestApiKey(restApiId, needsApiKey && deploymentReady);

  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [request, setRequest] = useState<ConsoleRequest | undefined>(undefined);

  /**
   * Optional user override for the credential name. Preserved across Console
   * view synchronizations, which rebuild the injected rows.
   */
  const [keyHeaderName, setKeyHeaderName] = useState<string | undefined>(undefined);

  const selectedGateway =
    gateways.find((gateway) => gateway.id === selectedGatewayId) ?? gateways[0];

  const resolvedBaseUrl = selectedGateway
    ? buildInvokeUrl(gatewayEndpoint(selectedGateway), apiQuery.data?.context)
    : '';

  const baseUrl = resolvedBaseUrl;

  /** True when both deployment queries have settled and no gateway exists. */
  const notDeployed = !deploymentUnknown && !deploymentFailed && gateways.length === 0;

  /** Destination for the banner, using the Overview page's deployment route. */
  const deployPath = routes.apiDeploy(
    params.orgHandle ?? '',
    params.projectHandler ?? null,
    params.apiHandler ?? null,
  );

  /** Credential sent by the console. Absent until one has been minted. */
  const testKey = testApiKey.key;

  /** State-driven minute tick for the key countdown and expiry. */
  const now = useNow(testKey ? KEY_COUNTDOWN_TICK_MS : 0);

  // The policy is the source of the name; page state only holds a user's edit.
  const headerName = keyHeaderName ?? apiKeyAuth?.name ?? '';
  const keyExpired = isTestKeyExpired(testKey, now);

  /** Injectable credential row, assigned exclusively to the policy-selected list. */
  const credentialRows = useMemo<KeyValueRow[]>(
    () =>
      needsApiKey && testKey && !keyExpired && headerName.trim() !== ''
        ? [
            {
              auto: true,
              enabled: true,
              id: 'test-key',
              name: headerName,
              secret: true,
              value: testKey.value,
            },
          ]
        : [],
    [headerName, keyExpired, needsApiKey, testKey],
  );

  /** Which list the credential goes in — the two are mutually exclusive. */
  const credentialIn = apiKeyAuth?.in ?? 'header';
  // Keep request-builder inputs stable across renders.
  const extraHeaders = useMemo(
    () => (credentialIn === 'header' ? credentialRows : []),
    [credentialIn, credentialRows],
  );
  const extraQueryParams = useMemo(
    () => (credentialIn === 'query' ? credentialRows : []),
    [credentialIn, credentialRows],
  );

  /** True when the API has no stored definition (`404`). */
  const definitionMissing =
    !definitionQuery.isPending &&
    isApiError(definitionQuery.error) &&
    definitionQuery.error.isNotFound;

  const spec = useMemo(() => {
    const content = definitionQuery.data?.content;
    if (definitionMissing || !content) return undefined;
    try {
      return parseSpecContent(content);
    } catch {
      return undefined;
    }
  }, [definitionMissing, definitionQuery.data?.content]);

  /**
   * Derives the cURL request from the latest request, the document's first
   * operation, or an empty request, then applies the current gateway and key.
   * Keeping this derived avoids synchronization conflicts with the Console view.
   */
  const seeded = useMemo(() => {
    if (request) return request;
    if (!spec) return undefined;

    const first = firstOperationOf(spec);
    if (!first) return undefined;

    // Prevent duplicate credential rows; `withTarget` re-applies them below.
    return buildConsoleRequest({
      baseUrl,
      extraHeaders,
      extraQueryParams,
      method: first.method,
      path: first.path,
      spec,
    });
  }, [baseUrl, extraHeaders, extraQueryParams, request, spec]);

  const effectiveRequest = useMemo(
    () => withTarget(seeded ?? emptyRequest(baseUrl), baseUrl, credentialRows, credentialIn),
    [baseUrl, credentialIn, credentialRows, seeded],
  );

  /** Edits from the builder, lifting a rename of the key header into state. */
  const handleBuilderChange = useCallback(
    (next: ConsoleRequest) => {
      // The credential row lives in whichever tab the policy placed it in, so
      // a rename can arrive from either.
      const autoRow =
        next.headers.find((row) => row.auto) ?? next.queryParams.find((row) => row.auto);
      if (autoRow && autoRow.name !== headerName) setKeyHeaderName(autoRow.name);
      setRequest(next);
    },
    [headerName],
  );

  // Wait for deployment and API state to avoid a console flash without an endpoint.
  if (apiQuery.isPending || deploymentUnknown) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (apiQuery.error || !apiQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }

  const api = apiQuery.data;

  /** Shared by the two empty states and the page itself, so they cannot drift. */
  const heading = (
    <PageTitle.Header>
      <Stack alignItems="baseline" direction="row" spacing={1.5}>
        <FormattedMessage {...messages.title} /> {api.displayName}
      </Stack>
    </PageTitle.Header>
  );

  if (deploymentFailed) {
    return (
      <>
        <PageTitle>{heading}</PageTitle>
        <ErrorState title={intl.formatMessage(messages.deploymentUnavailable)} />
      </>
    );
  }

  /** Render the deployment-required empty state when no gateway exists. */
  if (notDeployed) {
    return (
      <>
        <PageTitle>{heading}</PageTitle>
        <EmptyState
          actionIcon={<Rocket size={18} />}
          actionLabel={intl.formatMessage(messages.deployAction)}
          illustration={<GatewayIllustration />}
          onAction={() => navigate(deployPath)}
          title={intl.formatMessage(messages.notDeployedTitle)}
        />
      </>
    );
  }

  // Wait for the definition to avoid swapping the console for an empty state.
  // Check after deployment so undeployed APIs need no definition yet.
  if (definitionQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  /** Nothing to test without a contract; no existing-API upload page exists. */
  if (definitionMissing) {
    return (
      <>
        <PageTitle>{heading}</PageTitle>
        <EmptyState
          illustration={<ApiDesignerCanvasIllustration />}
          title={intl.formatMessage(messages.noDefinitionTitle)}
        />
      </>
    );
  }

  return (
    <>
      <PageTitle>
        {heading}
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitleCurlView} />
        </PageTitle.SubHeader>
      </PageTitle>

      <Stack spacing={2}>
        <Card variant="outlined">
          <GatewaySection
            endpoint={baseUrl}
            gateways={gateways}
            onSelect={setSelectedGatewayId}
            optionLabel={gatewayOptionLabel}
            selectedGatewayId={selectedGateway?.id ?? ''}
          />
          {needsApiKey && (
            <TestKeySection
              error={Boolean(testApiKey.error)}
              keyName={headerName}
              loading={testApiKey.isPending}
              location={credentialIn}
              onRegenerate={testApiKey.regenerate}
              regenerating={testApiKey.isRegenerating}
              remainingMs={testKeyRemainingMs(testKey, now)}
              value={testKey?.value}
            />
          )}
        </Card>

        {!spec ? (
          <ErrorState title={intl.formatMessage(messages.definitionUnavailable)} />
        ) : (
          <CurlBuilder
            onChange={handleBuilderChange}
            onRegenerateSecret={testApiKey.regenerate}
            regenerating={testApiKey.isRegenerating}
            request={effectiveRequest}
            spec={spec}
          />
        )}
      </Stack>
    </>
  );
}
