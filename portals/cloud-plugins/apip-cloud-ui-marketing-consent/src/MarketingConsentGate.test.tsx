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

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import MarketingConsentGate from './MarketingConsentGate';

type Held = { optoutOfEmails: boolean; consentStatus: string };

/** A fetch double: GET answers `held`; POST answers `postStatus`. */
function stubConsent(held: Held | number, postStatus = 202) {
  const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
    if (init?.method === 'POST') return Promise.resolve(new Response(null, { status: postStatus }));
    return Promise.resolve(
      typeof held === 'number'
        ? new Response(JSON.stringify({ error: 'x' }), { status: held })
        : new Response(JSON.stringify(held), { status: 200 })
    );
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

const posts = (fetchMock: ReturnType<typeof stubConsent>) =>
  fetchMock.mock.calls.filter(([, init]) => init?.method === 'POST');

const NEVER_ANSWERED: Held = { optoutOfEmails: false, consentStatus: 'No Opt-in' };

beforeEach(() => window.sessionStorage.clear());
afterEach(() => vi.unstubAllGlobals());

describe('MarketingConsentGate', () => {
  it.each(['No Opt-in', 'Unconfirmed'])('asks someone whose status is %s', async (consentStatus) => {
    stubConsent({ optoutOfEmails: false, consentStatus });
    render(<MarketingConsentGate port={{}} />);
    expect(await screen.findByRole('dialog', { name: 'Welcome to WSO2 API Platform' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'privacy policy' })).toHaveAttribute(
      'href',
      'https://wso2.com/privacy-policy/'
    );
  });

  it.each([
    ['already confirmed', { optoutOfEmails: false, consentStatus: 'Confirmed' }],
    ['unsubscribed', { optoutOfEmails: false, consentStatus: 'Unsubscribed' }],
    ['opted out of all email', { optoutOfEmails: true, consentStatus: 'No Opt-in' }],
  ])('stays quiet for someone %s', async (_label, held) => {
    const fetchMock = stubConsent(held);
    render(<MarketingConsentGate port={{}} />);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it.each([403, 404, 502])('stays quiet when the read fails with %i', async (status) => {
    const fetchMock = stubConsent(status);
    render(<MarketingConsentGate port={{}} />);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('does not even ask Pardot again once this session has dealt with it', () => {
    window.sessionStorage.setItem('apip.marketingConsent.asked', '1');
    const fetchMock = stubConsent(NEVER_ANSWERED);
    render(<MarketingConsentGate port={{}} />);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('Subscribe records Confirmed, closes, and confirms with a toast', async () => {
    const fetchMock = stubConsent(NEVER_ANSWERED);
    const notify = vi.fn();
    render(<MarketingConsentGate port={{ notify }} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Subscribe' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(JSON.parse(posts(fetchMock)[0][1]!.body as string)).toEqual({ consentstatus: 'Confirmed' });
    expect(notify).toHaveBeenCalledWith(expect.stringContaining('subscribed'), 'success');
  });

  it('No thanks records Unsubscribed', async () => {
    const fetchMock = stubConsent(NEVER_ANSWERED);
    const notify = vi.fn();
    render(<MarketingConsentGate port={{ notify }} />);
    fireEvent.click(await screen.findByRole('button', { name: 'No thanks' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(JSON.parse(posts(fetchMock)[0][1]!.body as string)).toEqual({ consentstatus: 'Unsubscribed' });
    expect(notify).toHaveBeenCalledWith(expect.stringContaining('will not receive'), 'success');
  });

  it('does not ask again on a remount after answering, even if Pardot still says No Opt-in', async () => {
    stubConsent(NEVER_ANSWERED);
    const first = render(<MarketingConsentGate port={{}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Subscribe' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    first.unmount();

    const fetchMock = stubConsent(NEVER_ANSWERED);
    render(<MarketingConsentGate port={{}} />);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('keeps the dialog open with an error when the save is refused, and lets the person retry', async () => {
    const fetchMock = stubConsent(NEVER_ANSWERED, 502);
    const notify = vi.fn();
    render(<MarketingConsentGate port={{ notify }} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Subscribe' }));
    expect(await screen.findByRole('alert')).toHaveTextContent("couldn't save");
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(notify).not.toHaveBeenCalled();
    expect(window.sessionStorage.getItem('apip.marketingConsent.asked')).toBeNull();

    // The retry goes through and clears the error.
    fetchMock.mockImplementation((_url, init) =>
      Promise.resolve(
        init?.method === 'POST'
          ? new Response(null, { status: 202 })
          : new Response(JSON.stringify(NEVER_ANSWERED), { status: 200 })
      )
    );
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('disables both buttons while a save is in flight', async () => {
    let finish!: (response: Response) => void;
    vi.stubGlobal(
      'fetch',
      vi.fn((_url: string, init?: RequestInit) =>
        init?.method === 'POST'
          ? new Promise<Response>((resolve) => {
              finish = resolve;
            })
          : Promise.resolve(new Response(JSON.stringify(NEVER_ANSWERED), { status: 200 }))
      )
    );
    render(<MarketingConsentGate port={{}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Subscribe' }));
    expect(screen.getByRole('button', { name: 'Subscribe' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'No thanks' })).toBeDisabled();
    finish(new Response(null, { status: 202 }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('Esc closes it for the session without recording anything with Pardot', async () => {
    const fetchMock = stubConsent(NEVER_ANSWERED);
    render(<MarketingConsentGate port={{}} />);
    fireEvent.keyDown(await screen.findByRole('dialog'), { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(posts(fetchMock)).toHaveLength(0);
    expect(window.sessionStorage.getItem('apip.marketingConsent.asked')).toBe('1');
  });

  it('works when the host supplies no notify', async () => {
    stubConsent(NEVER_ANSWERED);
    render(<MarketingConsentGate port={{}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'No thanks' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
