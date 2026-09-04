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

import { describe, expect, it } from 'vitest';

import { describeStatus, isApplying } from './status';
import type { ConfigPhase } from '../types';

/** A fixed clock, so "5 minutes ago" is a fact and not the time of the run. */
const NOW = Date.parse('2026-09-04T10:00:00Z');
const FIVE_MINUTES_AGO = '2026-09-04T09:55:00Z';

describe('describeStatus', () => {
  it('spends the line on the timestamp when the gateway is healthy', () => {
    // The point of removing the chip: "Healthy" is the resting state of every
    // configured gateway and tells the reader nothing. When the configuration
    // last landed does.
    const display = describeStatus(
      { phase: 'healthy', lastTransitionTime: FIVE_MINUTES_AGO },
      NOW
    );
    expect(display?.text).toBe('Updated 5 minutes ago');
    expect(display?.tone).toBe('muted');
  });

  it('puts the exact moment behind the hover, for correlating with a deployment', () => {
    const display = describeStatus(
      { phase: 'healthy', lastTransitionTime: FIVE_MINUTES_AGO },
      NOW
    );
    // Locale- and zone-dependent, so assert only that it resolved to something
    // other than the relative form.
    expect(display?.detail).toBeTruthy();
    expect(display?.detail).not.toBe(display?.text);
  });

  it('shows nothing for a healthy gateway with no recorded transition', () => {
    // The platform records none until the resources report, and an empty line
    // beats "Updated —".
    expect(describeStatus({ phase: 'healthy' }, NOW)).toBeNull();
    expect(describeStatus({ phase: 'healthy', lastTransitionTime: '' }, NOW)).toBeNull();
    expect(
      describeStatus({ phase: 'healthy', lastTransitionTime: 'not a date' }, NOW)
    ).toBeNull();
  });

  it('says Applying while the change is still reaching the data plane', () => {
    const display = describeStatus({ phase: 'applying' }, NOW);
    expect(display?.text).toBe('Applying…');
    // Not an error: this is the expected state for minutes after ANY write.
    expect(display?.tone).toBe('muted');
  });

  it('ignores the transition time while applying', () => {
    // `applying` reads the same minutes after a write and stalled an hour
    // later; the phase is what the reader acts on, so the phase is the line.
    const display = describeStatus(
      { phase: 'applying', lastTransitionTime: FIVE_MINUTES_AGO },
      NOW
    );
    expect(display?.text).toBe('Applying…');
  });

  it('colours only a failure', () => {
    const display = describeStatus({ phase: 'failed', message: 'render failed' }, NOW);
    expect(display?.text).toBe('Failed');
    expect(display?.tone).toBe('error');
  });

  it('keeps the platform message out of the line and in the hover', () => {
    // Unbounded prose. In the line it pushed the form down the drawer.
    const message = 'Resource "apigateway" readyWhen returned false';
    expect(describeStatus({ phase: 'applying', message }, NOW)).toMatchObject({
      text: 'Applying…',
      detail: message,
    });
    expect(describeStatus({ phase: 'failed', message }, NOW)?.detail).toBe(message);
  });

  it('does not promise a settling for a phase the platform never reported', () => {
    // `unknown` will not resolve on its own, so it must not borrow
    // `applying`'s copy.
    const display = describeStatus({ phase: 'unknown' }, NOW);
    expect(display?.text).toBe('Status unavailable');
    expect(display?.text).not.toBe('Applying…');
  });

  it('says the word a newer platform sent rather than rounding it to healthy', () => {
    const display = describeStatus(
      { phase: 'superseding' as ConfigPhase, message: 'from a newer platform' },
      NOW
    );
    expect(display?.text).toBe('superseding');
    expect(display?.detail).toBe('from a newer platform');
  });
});

describe('isApplying', () => {
  it('gates the Save button on the phase, and only on applying', () => {
    expect(isApplying({ phase: 'applying' })).toBe(true);
    expect(isApplying({ phase: 'healthy' })).toBe(false);
    // A failure is a reason to write again, not a reason to be locked out; and
    // `unknown` never settles, so gating on it would lock the form forever.
    expect(isApplying({ phase: 'failed' })).toBe(false);
    expect(isApplying({ phase: 'unknown' })).toBe(false);
  });

  it('does not gate a form that has no configuration loaded yet', () => {
    expect(isApplying(undefined)).toBe(false);
  });
});
