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

import type {
  CreateLLMProviderRequest,
  ProviderTemplate,
  UpdateLLMProviderRequest,
} from '../../../../../utils/types';
import { PLATFORM_GATEWAY_VERSIONS } from '../../../../../config.env';
import type {
  FormState,
  GuardrailSelection,
} from '../../serviceProvider/AddNewProvider/serviceProviderTypes';
import type { GatewayFormState } from './types';

export const VERSION_PATTERN = /^v\d+\.\d+$/;
export const CONTEXT_PATTERN = /^\/([a-zA-Z0-9_\-/]*[^/])?$/;
export const MAX_GATEWAY_NAME_LENGTH = 255;
export const MAX_GATEWAY_DESCRIPTION_LENGTH = 1023;
/**
 * Gateway releases the wizard offers, read from the same
 * `PLATFORM_GATEWAY_VERSIONS` config the Gateways page uses — the wizard never
 * carries its own hardcoded version. `version` is the short tag stored on a
 * gateway (e.g. "1.2"); the release tag and the zip/folder names it resolves to
 * can carry a suffix the short tag does not, which is why they are resolved
 * separately below (same split as `ViewGateway`'s own resolvers).
 */
export const AVAILABLE_GATEWAY_VERSIONS = PLATFORM_GATEWAY_VERSIONS.map(
  (entry) => entry.version
);
export const GATEWAY_VERSION = AVAILABLE_GATEWAY_VERSIONS[0] ?? '1.2';

const findVersionEntry = (version?: string) =>
  version
    ? PLATFORM_GATEWAY_VERSIONS.find((entry) => entry.version === version)
    : PLATFORM_GATEWAY_VERSIONS[0];

/** Release tag (with leading "v") used in the gateway download URL. */
export function resolveGatewayReleaseTag(version?: string): string {
  const entry = findVersionEntry(version);
  if (!entry) return version ? `v${version}` : `v${GATEWAY_VERSION}`;
  return entry.latestVersion ?? `v${entry.version}`;
}

/** Version segment of the distribution zip name. */
export function resolveGatewayDistVersion(version?: string): string {
  const entry = findVersionEntry(version);
  return (
    entry?.distVersion ?? getGatewayVersionHelm(resolveGatewayReleaseTag(version))
  );
}

/** Version segment of the directory the zip unpacks into. */
export function resolveGatewayDistFolderVersion(version?: string): string {
  return findVersionEntry(version)?.distFolderVersion ?? resolveGatewayDistVersion(version);
}

export const initialProviderFormState: FormState = {
  name: '',
  description: '',
  version: 'v1.0',
  context: '/',
  providerType: '',
  upstreamUrl: '',
  upstreamAuthType: 'api-key',
  upstreamAuthHeader: 'Authorization',
  upstreamAuthValue: '',
  valuePrefix: '',
};

export const initialGatewayFormState: GatewayFormState = {
  displayName: '',
  description: '',
  vhost: normalizeVhost('https://localhost:8443'),
  environment: '',
  version: GATEWAY_VERSION,
};

export function getGatewayVersionHelm(version: string): string {
  return version.startsWith('v') ? version.slice(1) : version;
}

export function getGatewayZipName(version?: string): string {
  return `wso2apip-ai-gateway-${resolveGatewayDistVersion(version)}`;
}

export function getGatewayFolderName(version?: string): string {
  return `wso2apip-ai-gateway-${resolveGatewayDistFolderVersion(version)}`;
}

export function toProviderId(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function buildAutoContext(name: string): string {
  const id = toProviderId(name);
  return id ? `/${id}` : '/';
}

export function buildProviderPolicies(
  selectedTemplateId: string,
  guardrails: GuardrailSelection[]
) {
  return [
    ...(selectedTemplateId !== 'azure-openai' &&
    selectedTemplateId !== 'azureai-foundry'
      ? [
          {
            name: 'llm-cost',
            version: 'v1',
            paths: [{ path: '/*', methods: ['*'], params: {} }],
          },
        ]
      : []),
    ...guardrails.map((guardrail) => ({
      name: guardrail.name,
      version: guardrail.version,
      paths: [
        {
          path: '/*',
          methods: ['*'],
          params: guardrail.settings ?? {},
        },
      ],
    })),
  ];
}

type BuildProviderRequestParams = {
  formState: FormState;
  selectedTemplateId: string;
  template: ProviderTemplate | null;
  openapiSpec: string;
  guardrails: GuardrailSelection[];
};

export function buildProviderCreateRequest({
  formState,
  selectedTemplateId,
  template,
  openapiSpec,
  guardrails,
}: BuildProviderRequestParams): CreateLLMProviderRequest {
  const updateRequest = buildProviderUpdateRequest({
    formState,
    selectedTemplateId,
    template,
    openapiSpec,
    guardrails,
  });

  return {
    id: toProviderId(formState.name),
    displayName: formState.name.trim(),
    description: formState.description.trim(),
    version: formState.version.trim(),
    context: formState.context.trim() || '/',
    template: selectedTemplateId,
    upstream: updateRequest.upstream as CreateLLMProviderRequest['upstream'],
    accessControl:
      updateRequest.accessControl as CreateLLMProviderRequest['accessControl'],
    policies: updateRequest.policies as CreateLLMProviderRequest['policies'],
    openapi: updateRequest.openapi,
  };
}

export function buildProviderUpdateRequest({
  formState,
  selectedTemplateId,
  template,
  openapiSpec,
  guardrails,
}: BuildProviderRequestParams): UpdateLLMProviderRequest {
  return {
    displayName: formState.name.trim(),
    description: formState.description.trim(),
    version: formState.version.trim(),
    context: formState.context.trim() || '/',
    template: selectedTemplateId,
    openapi: openapiSpec,
    upstream: {
      main: {
        url: template?.metadata?.endpointUrl || formState.upstreamUrl,
        ref: '',
        auth: {
          type: template?.metadata?.auth?.type || formState.upstreamAuthType,
          header:
            template?.metadata?.auth?.header || formState.upstreamAuthHeader,
          value: formState.valuePrefix
            ? `${formState.valuePrefix}${formState.upstreamAuthValue}`
            : formState.upstreamAuthValue,
        },
      },
    },
    policies: buildProviderPolicies(selectedTemplateId, guardrails),
    accessControl: {
      mode: 'allow_all',
      exceptions: [],
    },
  };
}

export function normalizeVhost(value: string): string {
  const trimmed = value.trim();
  if (trimmed.startsWith('https://')) return trimmed.slice(8);
  if (trimmed.startsWith('http://')) return trimmed.slice(7);
  return trimmed;
}

export function getDisplayUrl(vhost: string): string {
  if (!vhost || !vhost.trim()) return '';
  const trimmed = vhost.trim();
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
    return trimmed;
  }
  return `https://${trimmed}`;
}

export function generateGatewayName(displayName: string): string {
  if (!displayName || displayName.trim().length === 0) {
    return '';
  }

  return displayName
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/^-+|-+$/g, '')
    .replace(/-+/g, '-')
    .substring(0, 64)
    .replace(/-+$/g, '');
}

export function buildGatewayInvokeUrl(vhost: string, context: string): string {
  const normalizedVhost = vhost.trim();
  if (!normalizedVhost) return '';

  const base = /^https?:\/\//i.test(normalizedVhost)
    ? normalizedVhost.replace(/\/+$/, '')
    : `https://${normalizedVhost.replace(/\/+$/, '')}`;
  const normalizedContext = (context || '/').startsWith('/')
    ? context || '/'
    : `/${context}`;

  return `${base}${normalizedContext}`;
}
