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

import { useEffect, useState } from 'react';

import { getEnvironments } from '../apis/environmentApis';
import { useAppShell } from '../contexts/AppShellContext';

/**
 * An environment as the pickers consume it: `id` is the environment UUID (what
 * a gateway stores in `properties.environment` and what a create request sends
 * back), `name` is what the user reads. See `apis/environmentApis.ts`.
 */
export interface EnvironmentOption {
  id: string;
  name: string;
}

/**
 * The organization's environments, for the gateway pickers (the Gateways list
 * and the onboarding wizard's Add Gateway step).
 *
 * Note for anyone debugging an empty picker: gateway creation is gated on an
 * environment being selected, so a deployment whose control plane does not
 * serve `/environments` cannot create a gateway at all. The failure is quiet by
 * design — an unreachable list leaves the picker empty rather than breaking the
 * page — so check the network call before suspecting the form.
 */
export function useEnvironments() {
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid;

  const [environments, setEnvironments] = useState<EnvironmentOption[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    // Re-fetch per organization: environments are org-scoped, and the switcher
    // can change the org without remounting the pages that read this.
    if (!organizationId) {
      setEnvironments([]);
      return;
    }

    let cancelled = false;
    setIsLoading(true);
    setError(null);

    void (async () => {
      try {
        const list = await getEnvironments();
        if (cancelled) return;
        setEnvironments(
          list
            .filter((environment) => Boolean(environment.id))
            .map((environment) => ({
              id: environment.id,
              name: environment.displayName || environment.name,
            }))
            .sort((a, b) => a.name.localeCompare(b.name))
        );
      } catch (err) {
        if (cancelled) return;
        setError(
          err instanceof Error ? err : new Error('Failed to fetch environments')
        );
        setEnvironments([]);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [organizationId]);

  return {
    environments,
    isLoading,
    error,
  };
}
