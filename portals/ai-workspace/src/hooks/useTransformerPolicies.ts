/**
 * The translators a proxy can attach to a provider.
 *
 * Two sources feed one list: the shared catalogue, narrowed to translators, and
 * the policies a gateway has of its own. Gateway policies are included whatever
 * the catalogue filter says — they are not in the catalogue to be filtered, and
 * leaving them out would make a policy that exists unreachable from the screen
 * that attaches it.
 *
 * The same list decides what a provider resolves to and what the picker offers,
 * so the two cannot disagree about which translators exist.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { getGuardrails } from '../apis/policyHubApis';
import { getGatewayCustomPolicies } from '../apis/gatewayPolicyApis';
import type { GatewayCustomPolicy } from '../apis/gatewayPolicyApis';
import { logger } from '../utils/logger';
import type {
  PolicyHubPolicy,
  PolicyParameterDefinition,
  SelectablePolicy,
} from '../utils/types';

/** The catalogue category translators are published under. */
export const TRANSFORMER_POLICY_CATEGORY = 'LLM Transformation';

/**
 * What the picker searches for when it opens.
 *
 * Narrows a broad fetch to translators without hiding anything: the filter is
 * editable, so clearing it shows everything that was fetched.
 */
export const DEFAULT_TRANSFORMER_SEARCH = 'transformer';

/**
 * The category translators sit under in catalogues published before the
 * dedicated one existed, and the one the picker opens on for that reason.
 *
 * Both are fetched, as separate requests: the catalogue treats several
 * categories in one request as "carries all of them", so asking for both at
 * once matches nothing until every translator carries both labels.
 */
export const FALLBACK_POLICY_CATEGORY = 'AI';

/**
 * Reads a policy's own parameter declarations, whatever shape it publishes
 * them in.
 *
 * The configuration form is built from these rather than from a list held here,
 * so a translator that gains a parameter becomes configurable without a change
 * to this application. A policy that declares none yields an empty list rather
 * than undefined, so callers need no special case.
 */
const readParameterDefinitions = (
  source: Record<string, unknown> | undefined
): PolicyParameterDefinition[] => {
  if (!source) {
    return [];
  }
  const parameters = source.parameters ?? source.params;
  if (Array.isArray(parameters)) {
    return parameters.filter(
      (entry): entry is PolicyParameterDefinition =>
        Boolean(entry) &&
        typeof (entry as PolicyParameterDefinition).name === 'string'
    );
  }
  // A map of name → definition is the other shape in use.
  if (parameters && typeof parameters === 'object') {
    return Object.entries(
      parameters as Record<string, Record<string, unknown>>
    ).map(([name, definition]) => ({
      name,
      type: (definition?.type as string) ?? 'string',
      displayName: definition?.displayName as string | undefined,
      description: definition?.description as string | undefined,
      required: Boolean(definition?.required),
      default: definition?.default,
    }));
  }
  return [];
};

const fromCataloguePolicy = (policy: PolicyHubPolicy): SelectablePolicy => ({
  name: policy.name,
  displayName: policy.displayName || policy.name,
  version: policy.version,
  source: 'catalogue',
  description: policy.description,
  categories: policy.categories,
  parameters: readParameterDefinitions(policy as Record<string, unknown>),
});

const fromGatewayPolicy = (policy: GatewayCustomPolicy): SelectablePolicy => ({
  name: policy.name,
  displayName: policy.displayName || policy.name,
  version: policy.version,
  source: 'custom',
  description: policy.description,
  parameters: readParameterDefinitions(policy.policyDefinition),
  definition: policy.policyDefinition,
});

export type TransformerPoliciesState = {
  policies: SelectablePolicy[];
  /** False until both sources have answered, one way or the other. */
  isLoaded: boolean;
  isLoading: boolean;
  error: Error | null;
  reload: () => Promise<void>;
};

export default function useTransformerPolicies(): TransformerPoliciesState {
  const [cataloguePolicies, setCataloguePolicies] = useState<
    SelectablePolicy[]
  >([]);
  const [gatewayPolicies, setGatewayPolicies] = useState<SelectablePolicy[]>(
    []
  );
  const [isLoading, setIsLoading] = useState(true);
  const [isLoaded, setIsLoaded] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const load = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    // Settled rather than sequential: one source failing should not hide what
    // the other returned, and a gateway with no policies of its own is normal.
    const [labelledResult, legacyResult, gatewayResult] =
      await Promise.allSettled([
        getGuardrails(TRANSFORMER_POLICY_CATEGORY),
        getGuardrails(FALLBACK_POLICY_CATEGORY),
        getGatewayCustomPolicies(),
      ]);

    const catalogueRows: PolicyHubPolicy[] = [];
    if (labelledResult.status === 'fulfilled') {
      catalogueRows.push(...(labelledResult.value.data ?? []));
    }
    if (legacyResult.status === 'fulfilled') {
      catalogueRows.push(...(legacyResult.value.data ?? []));
    }

    if (
      labelledResult.status === 'rejected' &&
      legacyResult.status === 'rejected'
    ) {
      logger.error(
        'Failed to load transformer policies:',
        labelledResult.reason
      );
      setError(
        labelledResult.reason instanceof Error
          ? labelledResult.reason
          : new Error('Failed to load transformer policies')
      );
    }

    // The same policy can answer both requests once it carries both labels.
    const seen = new Set<string>();
    setCataloguePolicies(
      catalogueRows
        .filter((policy) => {
          const key = `${policy.name}@${policy.version}`;
          if (seen.has(key)) {
            return false;
          }
          seen.add(key);
          return true;
        })
        .map(fromCataloguePolicy)
    );

    if (gatewayResult.status === 'fulfilled') {
      setGatewayPolicies(
        (gatewayResult.value.list ?? []).map(fromGatewayPolicy)
      );
    } else {
      logger.error('Failed to load gateway policies:', gatewayResult.reason);
      setGatewayPolicies([]);
    }

    setIsLoading(false);
    setIsLoaded(true);
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const policies = useMemo(
    () =>
      [...cataloguePolicies, ...gatewayPolicies].sort((a, b) =>
        a.displayName.localeCompare(b.displayName)
      ),
    [cataloguePolicies, gatewayPolicies]
  );

  return { policies, isLoaded, isLoading, error, reload: load };
}
