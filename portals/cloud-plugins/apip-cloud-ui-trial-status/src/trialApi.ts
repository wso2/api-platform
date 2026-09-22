import type { BillingOrganization } from './types';

const TRIAL_URL = '/proxy/billing/organization?product=api-platform';

export async function getBillingOrganization(
  signal?: AbortSignal
): Promise<BillingOrganization> {
  const response = await fetch(TRIAL_URL, {
    headers: { Accept: 'application/json' },
    credentials: 'same-origin',
    signal,
  });

  if (!response.ok) {
    throw new Error(`Unable to load trial status (${response.status})`);
  }

  return response.json() as Promise<BillingOrganization>;
}
