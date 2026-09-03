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

import type { EditableField, GatewayConfiguration } from '../types';
import {
  fieldForServerMessage,
  validateFieldValue,
  validateForm,
} from './validate';

/*
 * Every expected message below is the sentence the PLATFORM returns for the
 * same value, taken from the rejection list that was run against a cluster on
 * 2026-08-30 (and the config_toml ones on 2026-08-31). That is the point of the
 * assertions: a user must read the same wording whichever side caught it, so a
 * drift here is a real defect even though the value is refused either way.
 */

const LOG_LEVEL: EditableField = {
  label: 'Gateway controller log level',
  path: 'gateway.config.controller.logging.level',
  type: 'enum',
  values: ['debug', 'info', 'warn', 'error'],
};

const ACCESS_LOGS: EditableField = {
  path: 'gateway.config.router.access_logs.enabled',
  type: 'boolean',
};

const ROUTE_TIMEOUT: EditableField = {
  max: '300000',
  min: '1000',
  path: 'gateway.config.router.upstream.timeouts.route_timeout_ms',
  type: 'integer',
};

const COST_SCALE: EditableField = {
  max: '1000000000000',
  min: '1',
  path: 'gateway.config.policy_configurations.llm_cost_ratelimit_v1.cost_scale_factor',
  type: 'integer',
};

const CLEANUP: EditableField = {
  max: '1h',
  min: '30s',
  path: 'gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval',
  type: 'duration',
};

const CPU_REQUEST: EditableField = {
  max: '4',
  min: '50m',
  path: 'gateway.controller.deployment.resources.requests.cpu',
  type: 'quantity',
};

const MEMORY_REQUEST: EditableField = {
  max: '8Gi',
  min: '128Mi',
  path: 'gateway.controller.deployment.resources.requests.memory',
  type: 'quantity',
};

const MEMORY_LIMIT: EditableField = {
  max: '8Gi',
  min: '128Mi',
  path: 'gateway.controller.deployment.resources.limits.memory',
  type: 'quantity',
};

const CONFIG_TOML: EditableField = {
  max: '8192',
  path: 'gateway.config_toml',
  type: 'string',
};

describe('validateFieldValue', () => {
  it('passes an untouched field', () => {
    expect(validateFieldValue(ROUTE_TIMEOUT, undefined)).toBeNull();
  });

  it('names the permitted values for an enum', () => {
    expect(validateFieldValue(LOG_LEVEL, 'debug')).toBeNull();
    expect(validateFieldValue(LOG_LEVEL, 'trace')).toBe(
      'gateway.config.controller.logging.level must be one of debug, info, warn, error'
    );
  });

  it('refuses a string for a boolean', () => {
    expect(validateFieldValue(ACCESS_LOGS, false)).toBeNull();
    // A literal "yes" is a 400 on the wire, so the widget must never send one.
    expect(validateFieldValue(ACCESS_LOGS, 'yes')).toBe(
      'gateway.config.router.access_logs.enabled must be true or false'
    );
  });

  it('bounds an integer, and reports the bound the platform declared', () => {
    expect(validateFieldValue(ROUTE_TIMEOUT, 60000)).toBeNull();
    expect(validateFieldValue(ROUTE_TIMEOUT, 999)).toBe(
      'gateway.config.router.upstream.timeouts.route_timeout_ms must be between 1000 and 300000'
    );
    expect(validateFieldValue(ROUTE_TIMEOUT, 300001)).not.toBeNull();
  });

  it('refuses a non-integer, an emptied input and a value past 2^53', () => {
    expect(validateFieldValue(ROUTE_TIMEOUT, 1500.5)).toBe(
      'gateway.config.router.upstream.timeouts.route_timeout_ms must be a whole number'
    );
    // An emptied number input arrives as '' rather than 0, so it is refused
    // instead of silently saving a number nobody typed.
    expect(validateFieldValue(ROUTE_TIMEOUT, '')).not.toBeNull();
    expect(validateFieldValue(COST_SCALE, 2 ** 53)).not.toBeNull();
  });

  it('accepts the whole integer range up to the 1e12 ceiling', () => {
    // A double holds 1e12 exactly; the guard is only against what is past 2^53.
    expect(validateFieldValue(COST_SCALE, 1_000_000_000_000)).toBeNull();
    expect(validateFieldValue(COST_SCALE, 1)).toBeNull();
    expect(validateFieldValue(COST_SCALE, 0)).not.toBeNull();
  });

  it('bounds a duration without normalising its spelling', () => {
    expect(validateFieldValue(CLEANUP, '5m')).toBeNull();
    expect(validateFieldValue(CLEANUP, '10s')).toBe(
      'gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval must be between 30s and 1h'
    );
    expect(validateFieldValue(CLEANUP, 'soon')).toBe(
      'gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval must be a duration such as "5m"'
    );
  });

  it('bounds a quantity and refuses one that is not positive', () => {
    expect(validateFieldValue(CPU_REQUEST, '500m')).toBeNull();
    expect(validateFieldValue(CPU_REQUEST, '10m')).toBe(
      'gateway.controller.deployment.resources.requests.cpu must be between 50m and 4'
    );
    expect(validateFieldValue(CPU_REQUEST, '8')).not.toBeNull();
    // Zero is legal to Kubernetes and meaningless for a gateway; a zero limit
    // would mean "no limit", which the ceiling assumes away.
    expect(validateFieldValue(CPU_REQUEST, '0')).toBe(
      'gateway.controller.deployment.resources.requests.cpu must be greater than zero'
    );
    expect(validateFieldValue(CPU_REQUEST, 'lots')).toBe(
      'gateway.controller.deployment.resources.requests.cpu must be a quantity such as "500m" or "512Mi"'
    );
  });

  it('counts the string field in code points, not bytes', () => {
    const ascii = 'a'.repeat(8192);
    expect(validateFieldValue(CONFIG_TOML, ascii)).toBeNull();
    expect(validateFieldValue(CONFIG_TOML, `${ascii}a`)).toBe(
      'gateway.config_toml must be between 0 and 8192 characters'
    );

    // 8192 astral characters are 16384 UTF-16 units and 32768 bytes, and are
    // 8192 characters to the person who typed them.
    const astral = '\u{1F600}'.repeat(8192);
    expect(astral.length).toBe(16384);
    expect(validateFieldValue(CONFIG_TOML, astral)).toBeNull();
  });

  it('clears the string field with an empty value', () => {
    expect(validateFieldValue(CONFIG_TOML, '')).toBeNull();
  });

  it('refuses the characters the values document cannot carry', () => {
    // Built from char codes rather than written literally: these are exactly
    // the characters that would be invisible in a source file.
    const nul = String.fromCharCode(0);
    const verticalTab = String.fromCharCode(11);
    const escape = String.fromCharCode(27);
    const del = String.fromCharCode(127);
    const nextLine = String.fromCharCode(0x85);
    const lineSeparator = String.fromCharCode(0x2028);
    const paragraphSeparator = String.fromCharCode(0x2029);
    const refused =
      'gateway.config_toml must not contain control characters or line separators';

    for (const character of [
      nul,
      verticalTab,
      escape,
      del,
      nextLine,
      lineSeparator,
      paragraphSeparator,
    ]) {
      expect(validateFieldValue(CONFIG_TOML, `[a]\nb = "${character}"`)).toBe(
        refused
      );
    }
  });

  it('allows tab, newline and carriage return, which a TOML block needs', () => {
    expect(validateFieldValue(CONFIG_TOML, '[a]\r\n\tb = 1\n')).toBeNull();
  });

  it('refuses a non-string for the string field', () => {
    expect(validateFieldValue(CONFIG_TOML, 42)).toBe(
      'gateway.config_toml must be a string'
    );
  });
});

const configuration = (
  values: Record<string, unknown>
): GatewayConfiguration => ({
  constraints: [
    {
      message:
        'the controller memory request must not exceed the controller memory limit',
      path: MEMORY_REQUEST.path,
      than: MEMORY_LIMIT.path,
      type: 'notGreaterThan',
    },
  ],
  editable: [
    LOG_LEVEL,
    ROUTE_TIMEOUT,
    CPU_REQUEST,
    MEMORY_REQUEST,
    MEMORY_LIMIT,
    CONFIG_TOML,
  ],
  id: 'wc-org-development-apip-default-gw',
  status: { phase: 'healthy' },
  values,
});

describe('validateForm', () => {
  const stored = {
    [MEMORY_LIMIT.path]: '512Mi',
    [MEMORY_REQUEST.path]: '256Mi',
    [ROUTE_TIMEOUT.path]: 60000,
  };

  it('reports nothing for an empty patch', () => {
    expect(validateForm(configuration(stored), {})).toEqual({});
  });

  it('refuses a path the response did not list', () => {
    const errors = validateForm(configuration(stored), { 'gateway.nope': 'x' });
    expect(errors['gateway.nope']).toBe(
      'gateway.nope is not an editable gateway setting'
    );
  });

  it('catches lowering only the limit, against the STORED request', () => {
    // 200Mi is inside the limit's own 128Mi–8Gi bounds, so per-field validation
    // accepts it. It is refused because the document the write would produce has
    // the stored 256Mi request above the new limit — which is what the server
    // checks, and what the kubelet would otherwise fail asynchronously long
    // after the 200.
    const errors = validateForm(configuration(stored), {
      [MEMORY_LIMIT.path]: '200Mi',
    });
    expect(errors[MEMORY_LIMIT.path]).toBe(
      'the controller memory request must not exceed the controller memory limit'
    );
    // On the edited field, not on the request the user never touched.
    expect(errors[MEMORY_REQUEST.path]).toBeUndefined();
  });

  it('accepts lowering both sides together', () => {
    expect(
      validateForm(configuration(stored), {
        [MEMORY_LIMIT.path]: '200Mi',
        [MEMORY_REQUEST.path]: '128Mi',
      })
    ).toEqual({});
  });

  it('compares canonicalized spellings rather than text', () => {
    // request 1024Mi === limit 1Gi, so equal is allowed — `notGreaterThan`.
    expect(
      validateForm(configuration(stored), {
        [MEMORY_LIMIT.path]: '1Gi',
        [MEMORY_REQUEST.path]: '1024Mi',
      })
    ).toEqual({});
  });

  it('skips a constraint whose other side the document does not carry', () => {
    // The missing side falls back to the chart's own default at render time, so
    // there is nothing to compare and nothing to refuse.
    const errors = validateForm(
      configuration({ [MEMORY_REQUEST.path]: '256Mi' }),
      { [MEMORY_REQUEST.path]: '8Gi' }
    );
    expect(errors).toEqual({});
  });

  it('does not run constraints while a field is individually malformed', () => {
    // Comparing against a value already known to be nonsense says nothing, so
    // the only message is the field's own.
    const errors = validateForm(configuration(stored), {
      [MEMORY_LIMIT.path]: 'not-a-quantity',
    });
    expect(Object.keys(errors)).toEqual([MEMORY_LIMIT.path]);
    expect(errors[MEMORY_LIMIT.path]).toContain('must be a quantity');
  });
});

describe('fieldForServerMessage', () => {
  const paths = [
    'gateway.config_toml',
    'gateway.config.router.upstream.timeouts.route_timeout_ms',
    'gateway.controller.deployment.replicaCount',
  ];

  it('attaches a field-level message to the field it names', () => {
    expect(
      fieldForServerMessage(
        'gateway.controller.deployment.replicaCount must be between 1 and 5',
        paths
      )
    ).toBe('gateway.controller.deployment.replicaCount');
  });

  it('prefers the longest matching path', () => {
    expect(
      fieldForServerMessage('gateway.config_toml.inner must be a string', [
        'gateway.config_toml',
        'gateway.config_toml.inner',
      ])
    ).toBe('gateway.config_toml.inner');
  });

  it('returns null for a message that names no field', () => {
    expect(
      fieldForServerMessage(
        'the controller CPU request must not exceed the controller CPU limit',
        paths
      )
    ).toBeNull();
  });
});
