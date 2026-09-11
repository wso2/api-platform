/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useCallback, useEffect, useState } from 'react';

import { usePortalFeature } from './portContext';
import type {
  CreateManagedPortalInput,
  ManagedPortal,
  OrgEnvironment,
  UpdateManagedPortalInput,
} from './types';

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

/** Loads one portal by id (GET returns metadata the list projection strips) and exposes update/delete. */
export function useManagedPortal(id: string) {
  const { port, host } = usePortalFeature();

  const [portal, setPortal] = useState<ManagedPortal | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const refetch = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      setPortal(await port.get(id));
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load portal'));
    } finally {
      setIsLoading(false);
    }
  }, [port, id]);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  const update = useCallback(
    async (input: UpdateManagedPortalInput) => {
      try {
        const updated = await port.update(id, input);
        host.notify(`Portal "${updated.name}" updated`, 'success');
        setPortal(updated);
        return updated;
      } catch (err) {
        host.notify(errorMessage(err, 'Failed to update portal'), 'error');
        throw err;
      }
    },
    [port, id, host]
  );

  const remove = useCallback(async () => {
    try {
      await port.remove(id);
      host.notify('Portal deleted', 'success');
    } catch (err) {
      host.notify(errorMessage(err, 'Failed to delete portal'), 'error');
      throw err;
    }
  }, [port, id, host]);

  return { portal, isLoading, error, refetch, update, remove };
}

/** List + create + update + delete managed portals via the feature's PortalPort. */
export function useManagedPortalList() {
  const { port, host } = usePortalFeature();

  const [portals, setPortals] = useState<ManagedPortal[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const refetch = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      setPortals(await port.list());
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load portals'));
    } finally {
      setIsLoading(false);
    }
  }, [port]);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  const create = useCallback(
    async (input: CreateManagedPortalInput) => {
      try {
        const portal = await port.create(input);
        host.notify(`Portal "${portal.name}" created`, 'success');
        await refetch();
        return portal;
      } catch (err) {
        host.notify(errorMessage(err, 'Failed to create portal'), 'error');
        throw err;
      }
    },
    [port, refetch, host]
  );

  const update = useCallback(
    async (id: string, input: UpdateManagedPortalInput) => {
      try {
        const portal = await port.update(id, input);
        host.notify(`Portal "${portal.name}" updated`, 'success');
        await refetch();
        return portal;
      } catch (err) {
        host.notify(errorMessage(err, 'Failed to update portal'), 'error');
        throw err;
      }
    },
    [port, refetch, host]
  );

  const remove = useCallback(
    async (id: string) => {
      try {
        await port.remove(id);
        host.notify('Portal deleted', 'success');
        await refetch();
      } catch (err) {
        host.notify(errorMessage(err, 'Failed to delete portal'), 'error');
        throw err;
      }
    },
    [port, refetch, host]
  );

  return { portals, isLoading, error, refetch, create, update, remove };
}

/** Loads the org's data-plane environments once on mount; used to populate env selectors. */
export function useOrgEnvironments() {
  const { port } = usePortalFeature();

  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    port
      .listEnvironments()
      .then((envs) => {
        if (!cancelled) setEnvironments(envs);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err : new Error('Failed to load environments'));
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [port]);

  return { environments, isLoading, error };
}
