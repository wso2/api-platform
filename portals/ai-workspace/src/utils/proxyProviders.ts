/**
 * Helpers for reading a proxy's provider list.
 *
 * A proxy carries every provider it is attached to in one list, primary first.
 * These helpers are the only place that shape is interpreted, so the several
 * screens that display it cannot disagree about what a provider is called or
 * which one is primary.
 */

import type { TransformerResolution } from './transformerResolution';
import type {
  Proxy,
  ProxyProviderEntry,
  ProxyProviderTransformer,
} from './types';

/**
 * The policy version a proxy records for a translator.
 *
 * A gateway resolves a major to whichever build of the policy it carries, and
 * rejects anything more specific — so a catalogue version like `0.9` cannot be
 * stored as it arrives, and a translator recorded that way is accepted on
 * create and then fails at invocation.
 */
export const majorPolicyVersion = (version?: string): string => {
  const [major] = (version ?? '').trim().replace(/^v/i, '').split('.');
  return major ? `v${major}` : '';
};

/**
 * A catalogue policy as a proxy records it against one provider.
 *
 * The single place a translator reference is built, so a reference stored from
 * the picker and one stored from an automatic match cannot end up in different
 * shapes.
 */
export const transformerFromPolicy = (
  policy: { name: string; version: string },
  params?: Record<string, unknown>
): ProxyProviderTransformer => ({
  type: policy.name,
  version: majorPolicyVersion(policy.version),
  ...(params && Object.keys(params).length > 0 ? { params } : {}),
});

/**
 * The translator a provider is stored with.
 *
 * A match made from the two formats is as real as one chosen by hand — the
 * screen says it applies — so it has to be written to the proxy rather than
 * left to be worked out again later. A gateway attaches a translator only where
 * the proxy names one, so a match shown and not stored is a provider that
 * silently does not translate, and fails at invocation rather than at save.
 */
export const resolvedTransformerFor = (
  chosen: ProxyProviderTransformer | null | undefined,
  resolution: TransformerResolution
): ProxyProviderTransformer | undefined => {
  if (chosen?.type) {
    return chosen;
  }
  return resolution.status === 'auto' && resolution.policy
    ? transformerFromPolicy(resolution.policy)
    : undefined;
};

/**
 * The name a client uses to select this provider.
 *
 * An attachment is selected by its alias when it has one and by its id
 * otherwise, and this is the value that goes in a routing header — so it is what
 * every screen must display as the provider's request handle. Showing the id for
 * an aliased attachment would show something that does not route.
 *
 * Derived rather than stored, so it cannot drift from the alias it comes from.
 */
export const effectiveProviderName = (entry: ProxyProviderEntry): string =>
  entry.alias?.trim() || entry.id;

/**
 * Every provider attached to a proxy, in the order the proxy holds them.
 *
 * Display order, and deliberately not primary-first: a reader is looking at
 * this list while they change it, and re-sorting it underneath them moves the
 * row they just acted on somewhere else. Which provider is primary is said by
 * the row itself. The list is put in order on the way out instead, by
 * `withPrimaryFirst`, so what reaches the server is unaffected.
 *
 * Returns an empty list rather than throwing when a proxy carries none, because
 * callers are render paths: a screen with nothing to show should show nothing,
 * not fail.
 */
export const proxyProviderEntries = (
  proxy?: Proxy | null
): ProxyProviderEntry[] => proxy?.providers ?? [];

/**
 * The same attachments as a request carries them: primary first.
 *
 * The contract puts the primary at the head of the list, and a reader of the
 * stored proxy — including this application on its next load — takes the order
 * at face value. Applied where a request is built rather than where the list is
 * displayed, so the order settles on save instead of under the pointer.
 */
export const withPrimaryFirst = (
  entries: ProxyProviderEntry[]
): ProxyProviderEntry[] => {
  if (entries.length < 2) {
    return entries;
  }
  const primary = entries.filter((entry) => entry.isPrimary);
  const rest = entries.filter((entry) => !entry.isPrimary);
  return [...primary, ...rest];
};

/** The attachment supplying the proxy's identity and default upstream. */
export const primaryProviderEntry = (
  proxy?: Proxy | null
): ProxyProviderEntry | undefined =>
  proxy?.providers?.find((entry) => entry.isPrimary) ?? proxy?.providers?.[0];

/**
 * Whether removing a provider is allowed.
 *
 * A proxy always has at least one provider, so the last remaining row cannot be
 * removed — the action is withheld rather than offered and then refused.
 */
export const canRemoveProvider = (entries: ProxyProviderEntry[]): boolean =>
  entries.length > 1;

/**
 * Moves the primary marker to one entry, clearing it from the others.
 *
 * Exactly one entry is primary at any time, so this replaces the marker rather
 * than adding one — a list with two primaries is refused by the server.
 */
export const withPrimaryProvider = (
  entries: ProxyProviderEntry[],
  primaryId: string
): ProxyProviderEntry[] =>
  entries.map((entry) => ({ ...entry, isPrimary: entry.id === primaryId }));
