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

import { useState, useEffect } from 'react';
import { getEnvTemplates } from '../apis/environmentApis';
import { useAppShell } from '../contexts/AppShellContext';

export interface EnvironmentOption {
  id: string;
  name: string;
}

/**
 * Hook to fetch and manage organization environment templates
 */
export function useEnvironments() {
  const { currentOrganization } = useAppShell();
  // Use the integer id, not uuid
  const orgId = currentOrganization?.id;

  const [environments, setEnvironments] = useState<EnvironmentOption[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!orgId) {
      setEnvironments([]);
      return;
    }

    const fetchEnvironments = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const orgEnvironments = await getEnvTemplates(orgId);
        if (!orgEnvironments || orgEnvironments.length === 0) {
          setEnvironments([]);
        } else {
          const mappedEnvironments = orgEnvironments
            .filter((env) => env.id && env.env_name)
            .map((env) => ({ id: env.id, name: env.env_name }))
            .sort((a, b) => a.name.localeCompare(b.name));
          setEnvironments(mappedEnvironments);
        }
      } catch (err) {
        setError(
          err instanceof Error ? err : new Error('Failed to fetch environments')
        );
        setEnvironments([]);
      } finally {
        setIsLoading(false);
      }
    };

    fetchEnvironments();
  }, [orgId]);

  return {
    environments,
    isLoading,
    error,
  };
}
