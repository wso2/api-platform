/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useCallback, useEffect, useRef, useState } from 'react';

import { usePortalFeature } from './portContext';
import { isBeingChanged } from './utils/change';
import type {
  CreateManagedPortalInput,
  ManagedPortal,
  OrgEnvironment,
  PortalPort,
  UpdateManagedPortalInput,
} from './types';

type Notifier = { notify: (message: string, severity: 'success' | 'info' | 'error') => void };

/**
 * Portals whose background change was started from this console and whose
 * outcome is not reported yet, per port. Kept outside any component so the
 * outcome is still reported when the user saves on the edit page and the list
 * mounts after the change already settled.
 */
const awaitingOutcome = new WeakMap<PortalPort, Set<string>>();

/** The set of portals awaiting their change outcome for this port. */
function awaiting(port: PortalPort): Set<string> {
  let ids = awaitingOutcome.get(port);
  if (!ids) {
    ids = new Set();
    awaitingOutcome.set(port, ids);
  }
  return ids;
}

/**
 * A change that runs in the background answers with the portal pending: it has
 * started, not finished, so the user is told that much, and the list reports
 * how it ended (see useManagedPortalList). An update that leaves the portal
 * failed reports that failure, not success.
 */
function notifyUpdated(host: Notifier, port: PortalPort, portal: ManagedPortal) {
  if (portal.status === 'pending') {
    awaiting(port).add(portal.id);
    host.notify(`Updating portal "${portal.name}". This can take a minute.`, 'info');
    return;
  }
  if (portal.status === 'failed') {
    const reason = portal.lastChangeError?.message ?? 'the portal is in a failed state';
    host.notify(`Portal "${portal.name}" was saved, but ${lowerFirst(reason)}`, 'error');
    return;
  }
  host.notify(`Portal "${portal.name}" updated`, 'success');
}

/** `s` with its first character lowercased, to continue a sentence. */
function lowerFirst(s: string): string {
  return s.charAt(0).toLowerCase() + s.slice(1);
}

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
        notifyUpdated(host, port, updated);
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

/**
 * Poll interval for the pending-portal watch. Short enough that the "Visit"
 * button flips promptly after the backend poller marks a portal active,
 * long enough that a busy org does not hammer the BFF. Only fires when at
 * least one row in the current list has status=pending; steady state (every
 * row active) leaves polling off entirely.
 */
const PENDING_POLL_INTERVAL_MS = 3_000;

/** List + create + update + delete managed portals via the feature's PortalPort. */
export function useManagedPortalList() {
  const { port, host } = usePortalFeature();

  const [portals, setPortals] = useState<ManagedPortal[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  // Monotonic request token shared by refetch and silentRefetch so an older
  // in-flight port.list() cannot overwrite a newer one when responses arrive
  // out of order (e.g. background poll fires just as a Create triggers a
  // refetch, and the poll's response resolves first). Only the response whose
  // seq is still the latest gets to commit into state.
  const requestSeq = useRef(0);
  // Only refetch sets isLoading, so only the latest refetch clears it, even
  // when a poll that started meanwhile superseded its result.
  const loadingSeq = useRef(0);

  const refetch = useCallback(async () => {
    const seq = ++requestSeq.current;
    const loading = ++loadingSeq.current;
    setIsLoading(true);
    setError(null);
    try {
      const result = await port.list();
      if (seq !== requestSeq.current) return;
      setPortals(result);
    } catch (err) {
      if (seq !== requestSeq.current) return;
      setError(err instanceof Error ? err : new Error('Failed to load portals'));
    } finally {
      if (loading === loadingSeq.current) setIsLoading(false);
    }
  }, [port]);

  // Silent variant used by the pending-portal poll: refreshes state without
  // toggling isLoading (which would flicker skeletons every tick). Errors are
  // swallowed too - a transient BFF hiccup during background polling should
  // not tear down the whole list view; the next tick recovers.
  // A tick while the previous poll is still out is skipped: starting another
  // would supersede it, and a list slower than the interval would then never land.
  const pollInFlight = useRef(false);
  const silentRefetch = useCallback(async () => {
    if (pollInFlight.current) return;
    pollInFlight.current = true;
    const seq = ++requestSeq.current;
    try {
      const result = await port.list();
      if (seq !== requestSeq.current) return;
      setPortals(result);
    } catch {
      // ignore
    } finally {
      pollInFlight.current = false;
    }
  }, [port]);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  // Watch for any pending or deleting portal in the current list; re-poll at
  // PENDING_POLL_INTERVAL_MS until every row has settled (a deleting one by
  // leaving the list). The interval is torn down on the transition to steady
  // state and on unmount, so an org whose portals are all active does zero
  // background work.
  const hasPending = portals.some((p) => p.status === 'pending' || p.status === 'deleting');
  useEffect(() => {
    if (!hasPending) return undefined;
    const id = window.setInterval(() => {
      void silentRefetch();
    }, PENDING_POLL_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [hasPending, silentRefetch]);

  // Portals whose background change was running when last listed, plus those
  // whose change was started from this console (awaitingOutcome) and may have
  // settled before this list first loaded. When one settles, the portal is read
  // for how the change ended, since saving it only said it had started; a
  // failed change leaves its message there. A portal being deleted reports
  // nothing: its delete has its own notification.
  const changing = useRef(new Set<string>());
  useEffect(() => {
    if (isLoading) return;
    const started = awaiting(port);
    const settled: ManagedPortal[] = [];
    for (const p of portals) {
      if (isBeingChanged(p)) {
        changing.current.add(p.id);
        started.delete(p.id);
        continue;
      }
      if (p.status === 'pending') continue;
      const wasChanging = changing.current.delete(p.id);
      const wasStarted = started.delete(p.id);
      if ((wasChanging || wasStarted) && p.status !== 'deleting') settled.push(p);
    }
    for (const id of [...changing.current]) {
      if (!portals.some((p) => p.id === id)) changing.current.delete(id);
    }
    for (const id of [...started]) {
      if (!portals.some((p) => p.id === id)) started.delete(id);
    }
    for (const p of settled) {
      port
        .get(p.id)
        .then((full) => {
          if (full.status === 'deleting') return;
          if (full.lastChangeError) {
            host.notify(`Portal "${full.name}": ${full.lastChangeError.message}`, 'error');
          } else {
            host.notify(`Portal "${full.name}" updated`, 'success');
          }
        })
        .catch(() => {
          // The list still shows the portal's state; the next read reports it.
        });
    }
  }, [portals, isLoading, port, host]);

  const create = useCallback(
    async (input: CreateManagedPortalInput) => {
      try {
        const portal = await port.create(input);
        host.notify(`Portal "${portal.name}" created`, 'success');
        // Before the refetch, not after: neither call depends on the new list,
        // and awaiting a full list request first leaves the host's quota and
        // cached counts stale for its whole duration.
        host.resourceLimits?.refresh(); // a create consumes quota
        // The host caches portal counts and cannot see this write.
        host.resources?.invalidate('apiPortals');
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
        notifyUpdated(host, port, portal);
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
        const { pending } = await port.remove(id);
        host.notify(pending ? 'Portal is being deleted' : 'Portal deleted', pending ? 'info' : 'success');
        // Before the refetch, for the same reason as in `create`.
        host.resourceLimits?.refresh(); // a delete frees quota
        host.resources?.invalidate('apiPortals');
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
