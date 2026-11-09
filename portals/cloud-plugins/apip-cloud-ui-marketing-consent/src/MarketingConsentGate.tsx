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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Link,
  Typography,
} from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';

import { fetchConsent, recordConsent } from './consentApi';
import { markAsked, shouldPrompt, wasAsked } from './consentPrompt';
import type { MarketingConsentHostPort } from './hostPort';
import type { SettableConsentStatus } from './types';

const TITLE_ID = 'marketing-consent-title';
const PRIVACY_POLICY_URL = 'https://wso2.com/privacy-policy/';

// Fully rounded buttons, as in the other WSO2 Cloud consoles' welcome dialog.
const PILL = { borderRadius: 999, px: 3, textTransform: 'none' } as const;

/**
 * Asks a signed-in person once whether they want WSO2 marketing email.
 *
 * A headless-style `app.gate` entry: mounted on every in-shell route, it renders
 * nothing unless Pardot says this person has never answered. Consent is filed
 * against WSO2 Cloud as one platform, so an answer given in any console is not
 * asked for again here.
 *
 * Never blocks anything: a failed read shows nothing, closing it (Esc or a click
 * outside) postpones it for the session, and a failed save keeps the dialog open so the person can retry.
 */
export default function MarketingConsentGate({
  port,
  proxyBase,
}: {
  port: MarketingConsentHostPort;
  /** The host console's resolved proxy base, so consent calls agree with its own. */
  proxyBase?: string;
}) {
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState<SettableConsentStatus | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (wasAsked()) return;
    let cancelled = false;
    void fetchConsent(proxyBase).then((record) => {
      // Recheck the marker: a gate that remounted while a save was pending can see
      // this read return after the save marked the session, with a stale `No Opt-in`.
      if (!cancelled && !wasAsked() && shouldPrompt(record)) setOpen(true);
    });
    return () => {
      cancelled = true;
    };
  }, [proxyBase]);

  const answer = async (status: SettableConsentStatus) => {
    setSaving(status);
    setFailed(false);
    const accepted = await recordConsent(status, proxyBase);
    setSaving(null);
    if (!accepted) {
      setFailed(true);
      return;
    }
    markAsked();
    setOpen(false);
    port.notify?.(
      status === 'Confirmed'
        ? 'Thanks — you are subscribed to WSO2 updates.'
        : 'Got it — you will not receive marketing emails.',
      'success'
    );
  };

  const later = () => {
    markAsked();
    setOpen(false);
  };

  return (
    <Dialog
      open={open}
      onClose={saving ? undefined : later}
      maxWidth="sm"
      fullWidth
      aria-labelledby={TITLE_ID}
    >
      <DialogTitle id={TITLE_ID} sx={{ fontWeight: 700 }}>
        Welcome to WSO2 API Platform
      </DialogTitle>
      <DialogContent>
        <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
          Want to stay in the loop? Get product updates, resources, and event invites by email. You
          can unsubscribe at any time.
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Learn how we handle your data in our{' '}
          <Link href={PRIVACY_POLICY_URL} target="_blank" rel="noopener noreferrer">
            privacy policy
          </Link>
          .
        </Typography>
        {failed && (
          <Typography variant="body2" color="error" sx={{ mt: 1.5 }} role="alert">
            We couldn&apos;t save your choice. Please try again.
          </Typography>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2.5 }}>
        <Button
          onClick={() => void answer('Unsubscribed')}
          disabled={saving !== null}
          variant="outlined"
          sx={PILL}
        >
          No thanks
        </Button>
        <Button
          onClick={() => void answer('Confirmed')}
          disabled={saving !== null}
          variant="contained"
          sx={PILL}
        >
          Subscribe
        </Button>
      </DialogActions>
    </Dialog>
  );
}
