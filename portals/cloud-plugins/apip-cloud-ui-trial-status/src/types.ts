export type TrialDetails = {
  days_remaining: number;
  trial_end: string;
};

export type BillingOrganization = {
  subscription?: {
    status?: string;
    trial?: TrialDetails | null;
  } | null;
};
