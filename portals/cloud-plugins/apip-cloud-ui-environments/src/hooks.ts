/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useCallback, useEffect, useState } from 'react';

import { useEnvironmentFeature } from './portContext';
import type { CloudEnvironment, CreateEnvironmentInput } from './types';

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

/** List + create + delete environments via the feature's EnvironmentPort. */
export function useEnvironmentList() {
  const { port, host } = useEnvironmentFeature();

  const [environments, setEnvironments] = useState<CloudEnvironment[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const refetch = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      setEnvironments(await port.list());
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load environments'));
    } finally {
      setIsLoading(false);
    }
  }, [port]);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  const create = useCallback(
    async (input: CreateEnvironmentInput) => {
      try {
        const env = await port.create(input);
        host.notify(`Environment "${env.displayName}" created`, 'success');
        await refetch();
        return env;
      } catch (err) {
        host.notify(errorMessage(err, 'Failed to create environment'), 'error');
        throw err;
      }
    },
    [port, refetch, host]
  );

  const remove = useCallback(
    async (envId: string) => {
      try {
        await port.remove(envId);
        host.notify('Environment deleted', 'success');
        await refetch();
      } catch (err) {
        host.notify(errorMessage(err, 'Failed to delete environment'), 'error');
        throw err;
      }
    },
    [port, refetch, host]
  );

  return { environments, isLoading, error, refetch, create, remove };
}
