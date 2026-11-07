import type { ServiceAccountCredentials } from '@/api/resources/serviceAccounts';

/**
 * Hands a freshly issued secret from the create page to the "created" page.
 *
 * It lives in module memory on purpose: router state is written to the
 * browser's history, and the secret must not outlive the tab's page view. A
 * reload therefore loses it, which is the same as navigating away. It is bound
 * to the organization that issued it, so history navigation into another
 * organization's completion page never shows it.
 */
let held: { credentials: ServiceAccountCredentials; org: string } | null = null;

export const holdCredentials = (credentials: ServiceAccountCredentials, org: string) => {
  held = { credentials, org };
};

/** The held secret, only for the organization that issued it; any other caller drops it. */
export const peekCredentials = (org: string): ServiceAccountCredentials | null => {
  if (held && held.org !== org) held = null;
  return held?.credentials ?? null;
};

export const clearCredentials = () => {
  held = null;
};

/** The held secret regardless of organization; for tests and cleanup checks. */
export const hasHeldCredentials = () => held !== null;
