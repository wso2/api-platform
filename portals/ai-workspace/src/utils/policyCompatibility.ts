/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import type { AIEntityType } from '../contexts/AIEntitiesContext';
import type { GatewayManifestPolicy } from '../apis/gatewayPolicyApis';
import type {
  AgentProxy,
  Application,
  LLMProvider,
  MCPServer,
  Proxy,
} from './types';

type PolicyBearingEntity =
  | LLMProvider
  | Proxy
  | MCPServer
  | AgentProxy
  | Application;

const normalizePolicyName = (name: string): string => name.trim().toLowerCase();

const addNamedPolicies = (
  names: Set<string>,
  policies: ReadonlyArray<{ name?: string }> | undefined
): void => {
  for (const policy of policies ?? []) {
    if (policy?.name) names.add(policy.name);
  }
};

/**
 * Names of every policy attached to an AI entity, deduplicated. Shared across
 * LLM Providers, LLM Proxies, MCP Proxies and Agent Proxies so
 * gateway-compatibility checks stay in one place instead of being
 * reimplemented per entity kind.
 *
 * Each entity kind stores its policies in its own shape (see `utils/types.ts`),
 * so this is a switch rather than one shared field access:
 *   - LLM Provider / LLM Proxy: `globalPolicies` + `operationPolicies`, plus
 *     the legacy `policies` list — still actively populated/edited by
 *     ServiceProviderGuardrailsTab/LLMProxyGuardrailsTab alongside
 *     `globalPolicies`, so an entity can carry guardrails there and nowhere
 *     else.
 *   - MCP Proxy: a single `policies` list (typed `unknown[]` at rest, but
 *     always `{ name, version, params? }` on the wire)
 *   - Agent Proxy: `a2a.operationConfigs.policies` (agent-wide),
 *     `a2a.operationConfigs.operations[].policies` (per-operation), and
 *     `a2a.agentCard.public.policies` (public Agent Card — a distinct third
 *     bucket edited separately in AgentProxyGuardrailsTab)
 *   - Application (Gen AI application): no policy attachments of its own —
 *     reports none.
 */
export function getEntityPolicyNames(
  entityType: AIEntityType | null,
  entity: PolicyBearingEntity | null
): string[] {
  if (!entity) return [];

  const names = new Set<string>();

  switch (entityType) {
    case 'llm-provider':
    case 'llm-proxy': {
      const policyEntity = entity as LLMProvider | Proxy;
      addNamedPolicies(names, policyEntity.globalPolicies);
      addNamedPolicies(names, policyEntity.operationPolicies);
      addNamedPolicies(names, policyEntity.policies);
      break;
    }
    case 'mcp-server': {
      const server = entity as MCPServer;
      addNamedPolicies(
        names,
        server.policies as ReadonlyArray<{ name?: string }> | undefined
      );
      break;
    }
    case 'agent-proxy': {
      const agentProxy = entity as AgentProxy;
      const operationConfigs = agentProxy.a2a?.operationConfigs;
      addNamedPolicies(names, operationConfigs?.policies);
      for (const operation of operationConfigs?.operations ?? []) {
        addNamedPolicies(names, operation.policies);
      }
      addNamedPolicies(names, agentProxy.a2a?.agentCard?.public?.policies);
      break;
    }
    case 'application':
    default:
      break;
  }

  return [...names];
}

/** Policy names installed on a gateway, normalized for case-insensitive comparison. */
export function getGatewayManifestPolicyNameSet(
  manifestPolicies: GatewayManifestPolicy[]
): Set<string> {
  return new Set(manifestPolicies.map((policy) => normalizePolicyName(policy.name)));
}

/**
 * Entity policy names that are not present in a gateway's installed policy
 * set. Returns the original (non-normalized) names, in the order the entity
 * referenced them, so they can be displayed as-is.
 */
export function getMissingGatewayPolicies(
  entityPolicyNames: string[],
  gatewayPolicyNameSet: Set<string>
): string[] {
  return entityPolicyNames.filter(
    (name) => !gatewayPolicyNameSet.has(normalizePolicyName(name))
  );
}

/** Human-readable label for an AI entity kind, for use in compatibility messaging. */
export function getAIEntityTypeLabel(entityType: AIEntityType | null): string {
  switch (entityType) {
    case 'llm-provider':
      return 'LLM Provider';
    case 'llm-proxy':
      return 'LLM Proxy';
    case 'mcp-server':
      return 'MCP Proxy';
    case 'agent-proxy':
      return 'Agent Proxy';
    case 'application':
      return 'Application';
    default:
      return 'artifact';
  }
}
